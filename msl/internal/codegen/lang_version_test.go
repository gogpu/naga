// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// langVersionCase is a shader that uses a feature of a newer MSL than the
// version below its requirement: Compile refuses it there with a
// *backend.VersionError naming the feature and both versions, and writes it
// at the required version, with that version in the header. The backend
// never raises Options.LangVersion.
type langVersionCase struct {
	name     string
	src      string // WGSL; module builds the IR when it is nil
	module   func() *ir.Module
	feature  string
	required Version
	below    Version
	// atRequired is the error Compile returns at the required version for a
	// feature this backend does not write at all (mesh shaders); empty when
	// the shader compiles there.
	atRequired string
}

// Each case cites the Rust naga check it mirrors (back/msl at the pinned
// commit).
var langVersionCases = []langVersionCase{
	// mod.rs:477 UnsupportedAttribute("invariant").
	{name: "invariant", feature: "@invariant", required: Version2_1, below: Version2_0, src: `
@vertex fn main() -> @builtin(position) @invariant vec4<f32> { return vec4<f32>(0.0); }`},
	// mod.rs:490 UnsupportedAttribute("instance_id").
	{name: "instance_index", feature: "@builtin(instance_index)", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
@vertex fn main(@builtin(instance_index) i: u32) -> @builtin(position) vec4<f32> { return vec4<f32>(f32(i)); }`},
	{name: "instance_index member", feature: "@builtin(instance_index)", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
struct In { @builtin(instance_index) i: u32, @location(0) p: vec4<f32> }
@vertex fn main(in: In) -> @builtin(position) vec4<f32> { return in.p * f32(in.i); }`},
	// mod.rs:495 UnsupportedAttribute("primitive_id").
	{name: "primitive_index", feature: "@builtin(primitive_index)", required: Version2_3, below: Version{Major: 2, Minor: 2}, src: `
enable primitive_index;
@fragment fn main(@builtin(primitive_index) p: u32) -> @location(0) vec4<f32> { return vec4<f32>(f32(p)); }`},
	// mod.rs:501 UnsupportedAttribute("amplification_id"): 2.2, not 2.3.
	{name: "view_index", feature: "@builtin(view_index)", required: Version{Major: 2, Minor: 2}, below: Version2_1, src: `
@fragment fn main(@builtin(view_index) v: u32) -> @location(0) vec4<f32> { return vec4<f32>(f32(v)); }`},
	// mod.rs:506 UnsupportedAttribute("barycentric_coord").
	{name: "barycentric", feature: "@builtin(barycentric)", required: Version2_3, below: Version{Major: 2, Minor: 2}, src: `
@fragment fn main(@builtin(barycentric) b: vec3<f32>) -> @location(0) vec4<f32> { return vec4<f32>(b, 1.0); }`},
	{name: "barycentric_no_perspective", feature: "@builtin(barycentric_no_perspective)", required: Version2_3, below: Version{Major: 2, Minor: 2}, src: `
@fragment fn main(@builtin(barycentric_no_perspective) b: vec3<f32>) -> @location(0) vec4<f32> { return vec4<f32>(b, 1.0); }`},
	// mod.rs:523 UnsupportedAttribute("blend_src").
	{name: "blend_src", feature: "@blend_src", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
enable dual_source_blending;
struct Out { @location(0) @blend_src(0) a: vec4<f32>, @location(0) @blend_src(1) b: vec4<f32> }
@fragment fn main() -> Out { return Out(vec4<f32>(0.4), vec4<f32>(0.9)); }`},
	// writer.rs:2743 UnsupportedFunction.
	{name: "reverseBits", feature: "reverseBits", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
@group(0) @binding(0) var<storage, read_write> b: u32;
@compute @workgroup_size(1) fn main() { b = reverseBits(b); }`},
	{name: "extractBits", feature: "extractBits", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
@group(0) @binding(0) var<storage, read_write> b: u32;
@compute @workgroup_size(1) fn main() { b = extractBits(b, 1u, 2u); }`},
	{name: "insertBits in a function", feature: "insertBits", required: Version1_2, below: Version{Major: 1, Minor: 1}, src: `
@group(0) @binding(0) var<storage, read_write> b: u32;
fn f(x: u32) -> u32 { return insertBits(x, 3u, 1u, 2u); }
@compute @workgroup_size(1) fn main() { b = f(b); }`},
	// writer.rs:4600 (module), :3046 (getIntersection), ray.rs:205
	// (statements) UnsupportedRayTracing.
	{name: "ray query", feature: "a ray query", required: Version2_4, below: Version2_3, src: `
enable wgpu_ray_query;
@group(0) @binding(0) var acc: acceleration_structure;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1) fn main() {
	var rq: ray_query;
	rayQueryInitialize(&rq, acc, RayDesc(RAY_FLAG_TERMINATE_ON_FIRST_HIT, 0xFFu, 0.1, 100.0, vec3<f32>(0.0), vec3<f32>(0.0, 1.0, 0.0)));
	rayQueryProceed(&rq);
	out = rayQueryGetCommittedIntersection(&rq).kind;
}`},
	// writer.rs:7381 PerVertexNotSupported.
	{name: "per_vertex", feature: "@interpolate(per_vertex)", required: Version{Major: 4}, below: Version{Major: 3, Minor: 2}, src: `
enable wgpu_per_vertex;
@fragment fn main(@location(0) @interpolate(per_vertex) v: array<f32, 3>) -> @location(0) vec4<f32> { return vec4<f32>(v[0], v[1], v[2], 1.0); }`},
	// writer.rs:7636: below 1.2, UnsupportedWritableStorageBuffer,
	// UnsupportedWritableStorageTexture and UnsupportedRWStorageTexture.
	{name: "writable storage buffer in a fragment shader", feature: "a writable storage buffer in a fragment shader",
		required: Version1_2, below: Version1_0, src: `
@group(0) @binding(0) var<storage, read_write> b: u32;
@fragment fn main() -> @location(0) vec4<f32> { b = 1u; return vec4<f32>(0.0); }`},
	{name: "writable storage texture in a vertex shader", feature: "a writable storage texture in a vertex or fragment shader",
		required: Version1_2, below: Version1_0, src: `
@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;
@vertex fn main() -> @builtin(position) vec4<f32> { textureStore(t, vec2<i32>(0), vec4<f32>(0.0)); return vec4<f32>(0.0); }`},
	{name: "read-write storage texture", feature: "a read-write storage texture",
		required: Version1_2, below: Version1_0, src: `
@group(0) @binding(0) var t: texture_storage_2d<r32float, read_write>;
@compute @workgroup_size(1) fn main() { textureStore(t, vec2<i32>(0), textureLoad(t, vec2<i32>(1))); }`},
	// writer.rs:7690-7740 UnsupportedArrayOf.
	{name: "binding array of samplers", feature: "a binding array of samplers", required: Version2_0, below: Version1_2, src: `
enable wgpu_binding_array;
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: binding_array<sampler, 2>;
@fragment fn main() -> @location(0) vec4<f32> { return textureSample(t, s[1], vec2<f32>(0.0)); }`},
	{name: "binding array of textures", feature: "a binding array of textures", required: Version2_0, below: Version1_2, src: `
enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_2d<f32>, 2>;
@fragment fn main() -> @location(0) vec4<f32> { return textureLoad(t[1], vec2<i32>(0), 0); }`},
	{name: "binding array of read-only storage textures", feature: "a binding array of textures", required: Version2_0, below: Version1_2, src: `
enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_storage_2d<r32float, read>, 2>;
@group(0) @binding(1) var<storage, read_write> out: f32;
@compute @workgroup_size(1) fn main() { out = textureLoad(t[1], vec2<i32>(0)).x; }`},
	{name: "binding array of write-only storage textures", feature: "a binding array of write-only storage textures",
		required: Version2_0, below: Version1_2, src: `
enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_storage_2d<r32float, write>, 2>;
@compute @workgroup_size(1) fn main() { textureStore(t[1], vec2<i32>(0), vec4<f32>(0.0)); }`},
	{name: "binding array of read-write storage textures", feature: "a binding array of read-write storage textures",
		required: Version3_0, below: Version2_4, src: `
enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_storage_2d<r32float, read_write>, 2>;
@compute @workgroup_size(1) fn main() { textureStore(t[1], vec2<i32>(0), textureLoad(t[0], vec2<i32>(0))); }`},
	// Not a Rust check: Rust writes metal::texture2d<ulong> at any version,
	// but 64-bit texture atomics are an MSL 3.1 feature.
	{name: "64-bit storage texture", feature: "a 64-bit storage texture (r64uint, r64sint)", required: Version3_1, below: Version3_0, src: `
@group(0) @binding(0) var image: texture_storage_2d<r64uint, atomic>;
@compute @workgroup_size(1) fn main() { textureAtomicMax(image, vec2<i32>(0, 0), 1lu); }`},
	// writer.rs:4589 UnsupportedMeshShader; at 3.0 the stage is still
	// refused, since this backend writes no mesh shaders.
	{name: "mesh shader", feature: "a mesh or task shader", required: Version3_0, below: Version2_4,
		atRequired: "unsupported shader stage", module: func() *ir.Module {
			return &ir.Module{EntryPoints: []ir.EntryPoint{{
				Name: "mesh_main", Stage: ir.StageMesh, Workgroup: [3]uint32{1, 1, 1},
			}}}
		}},
}

func (c *langVersionCase) lower(t *testing.T) *ir.Module {
	t.Helper()
	if c.module != nil {
		return c.module()
	}
	return lowerWGSLForTest(t, c.src)
}

func TestLangVersionRefused(t *testing.T) {
	for i := range langVersionCases {
		c := &langVersionCases[i]
		t.Run(c.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.LangVersion = c.below
			_, _, err := Compile(c.lower(t), opts)
			var vErr *backend.VersionError
			if !errors.As(err, &vErr) {
				t.Fatalf("MSL %v: err = %v, want a *backend.VersionError", c.below, err)
			}
			want := backend.VersionError{
				Space:     backend.VersionMSL,
				Feature:   c.feature,
				Required:  backend.Version{Major: c.required.Major, Minor: c.required.Minor},
				Requested: backend.Version{Major: c.below.Major, Minor: c.below.Minor},
			}
			if *vErr != want {
				t.Errorf("MSL %v: VersionError = %+v, want %+v", c.below, *vErr, want)
			}
			if !errors.Is(err, backend.ErrUnsupported) {
				t.Errorf("MSL %v: %v does not match backend.ErrUnsupported", c.below, err)
			}
			if !strings.HasPrefix(err.Error(), "msl: ") {
				t.Errorf("MSL %v: %q lacks the msl: prefix", c.below, err)
			}
		})
	}
}

func TestLangVersionAtRequired(t *testing.T) {
	for i := range langVersionCases {
		c := &langVersionCases[i]
		t.Run(c.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.LangVersion = c.required
			code, _, err := Compile(c.lower(t), opts)
			if c.atRequired != "" {
				var vErr *backend.VersionError
				if err == nil || errors.As(err, &vErr) || !strings.Contains(err.Error(), c.atRequired) {
					t.Fatalf("MSL %v: err = %v, want %q", c.required, err, c.atRequired)
				}
				return
			}
			if err != nil {
				t.Fatalf("MSL %v: %v", c.required, err)
			}
			header := "// language: metal" + c.required.String() + "\n"
			if !strings.HasPrefix(code, header) {
				t.Errorf("MSL %v: header is not %q:\n%s", c.required, header, firstLine(code))
			}
		})
	}
}

// TestLangVersionNotRaised: the header is Options.LangVersion whatever the
// shader uses, and the zero version is 2.1.
func TestLangVersionNotRaised(t *testing.T) {
	m := lowerWGSLForTest(t, `
@vertex fn main(@builtin(instance_index) i: u32) -> @builtin(position) @invariant vec4<f32> { return vec4<f32>(f32(i)); }`)
	for _, tc := range []struct {
		v    Version
		want string
	}{
		{Version{}, "// language: metal2.1\n"},
		{Version2_1, "// language: metal2.1\n"},
		{Version3_1, "// language: metal3.1\n"},
	} {
		opts := DefaultOptions()
		opts.LangVersion = tc.v
		code, _, err := Compile(m, opts)
		if err != nil {
			t.Fatalf("MSL %v: %v", tc.v, err)
		}
		if !strings.HasPrefix(code, tc.want) {
			t.Errorf("MSL %v: header %q, want %q", tc.v, firstLine(code), tc.want)
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
