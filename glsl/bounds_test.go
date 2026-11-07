// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

// TestRestrictBoundsChecks compares the GLSL of shaders that index arrays,
// vectors and matrices in every address space, compiled with the Restrict
// index and buffer policies, with testdata/bounds: each index that may be
// out of bounds is clamped to the last element, by length() for a
// runtime-sized array. bounds-check-restrict.wgsl is Rust naga's test of
// the policy; its MSL clamps the same indices. Regenerate with
// UPDATE_GOLDEN=1 and review the diff.
func TestRestrictBoundsChecks(t *testing.T) {
	for _, tc := range []struct{ source, golden string }{
		{"../snapshot/testdata/rust/in/wgsl/bounds-check-restrict.wgsl", "testdata/bounds/bounds-check-restrict.comp"},
		{"testdata/bounds/spaces.wgsl", "testdata/bounds/spaces.comp"},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			m := lowerWGSL(t, mustReadFile(tc.source))
			got, _, err := glsl.Compile(m, glsl.Options{
				LangVersion: glsl.Version430,
				EntryPoint:  entry(t, m, "main"),
				BoundsCheckPolicies: backend.BoundsCheckPolicies{
					Index:        backend.BoundsCheckRestrict,
					Buffer:       backend.BoundsCheckRestrict,
					ImageLoad:    backend.BoundsCheckUnchecked,
					BindingArray: backend.BoundsCheckUnchecked,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(tc.golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want := mustReadFile(tc.golden); got != want {
				t.Errorf("output differs from %s:\n%s", tc.golden, got)
			}
		})
	}
}

// TestImageLoadUnsignedLevelAndSample: GLSL's texelFetch, textureSize, clamp
// and the comparisons in the bounds checks take an int level or sample, so a
// u32 one is cast to int under every ImageLoad policy (P1-GLSL-013). Without
// the cast glslang and the GL drivers reject the shader ("cannot convert from
// uint to int", "textureSize: no matching overloaded function"); wgpu uses
// ReadZeroSkipWrite on GL 4.30+. Rust naga 14328f4 writes the u32 level as is
// under Restrict and in the ReadZeroSkipWrite condition and textureSize (the
// same bug); it casts only the texelFetch level of ReadZeroSkipWrite.
func TestImageLoadUnsignedLevelAndSample(t *testing.T) {
	module := lowerWGSL(t, `
@group(0) @binding(0) var tex: texture_2d<u32>;
@group(0) @binding(1) var ms: texture_multisampled_2d<f32>;
@group(0) @binding(2) var<uniform> lvl: u32;
@fragment fn main() -> @location(0) vec4<f32> {
    let a = textureLoad(tex, vec2<i32>(0, 0), lvl);
    let b = textureLoad(ms, vec2<i32>(0, 0), lvl);
    return vec4<f32>(a) + b;
}`)
	for _, tc := range []struct {
		policy backend.BoundsCheckPolicy
		want   []string
	}{
		{backend.BoundsCheckRestrict, []string{
			"int _e6_clamped_lod = clamp(int(_e5), 0, textureQueryLevels(_group_0_binding_0_fs) - 1);",
			"clamp(int(_e12), 0, textureSamples(_group_0_binding_1_fs) - 1)",
		}},
		{backend.BoundsCheckReadZeroSkipWrite, []string{
			"(int(_e5) < textureQueryLevels(_group_0_binding_0_fs) && all(lessThan(ivec2(0, 0), " +
				"textureSize(_group_0_binding_0_fs, int(_e5)))) ? texelFetch(_group_0_binding_0_fs, ivec2(0, 0), int(_e5))",
			"(int(_e12) < textureSamples(_group_0_binding_1_fs) && all(lessThan(ivec2(0, 0), " +
				"textureSize(_group_0_binding_1_fs))) ? texelFetch(_group_0_binding_1_fs, ivec2(0, 0), int(_e12))",
		}},
		{backend.BoundsCheckUnchecked, []string{
			"texelFetch(_group_0_binding_0_fs, ivec2(0, 0), int(_e5))",
			"texelFetch(_group_0_binding_1_fs, ivec2(0, 0), int(_e12))",
		}},
	} {
		opts := glsl.Options{LangVersion: glsl.Version430, BoundsCheckPolicies: uncheckedPolicies}
		opts.BoundsCheckPolicies.ImageLoad = tc.policy
		code, _, err := glsl.Compile(module, opts)
		if err != nil {
			t.Fatalf("ImageLoad %v: %v", tc.policy, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(code, w) {
				t.Errorf("ImageLoad %v: missing %q in:\n%s", tc.policy, w, code)
			}
		}
		// Negative: the u32 uniform loads (_e5, _e12) never reach an int operand uncast.
		for _, bad := range []string{"(_e5,", "(_e5 <", ", _e5)", "(_e12,", "(_e12 <", ", _e12)"} {
			if strings.Contains(code, bad) {
				t.Errorf("ImageLoad %v: uncast unsigned operand %q in:\n%s", tc.policy, bad, code)
			}
		}
	}
}

func TestReadZeroSkipWriteIndicesAreRefused(t *testing.T) {
	module := lowerWGSL(t, `@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`)
	for _, p := range []backend.BoundsCheckPolicies{
		{Index: backend.BoundsCheckReadZeroSkipWrite},
		{Buffer: backend.BoundsCheckReadZeroSkipWrite},
	} {
		if _, _, err := glsl.Compile(module, glsl.Options{BoundsCheckPolicies: p}); err == nil {
			t.Errorf("%+v compiled", p)
		}
	}
}

// uncheckedPolicies adds no bounds checks, Rust naga's default; the zero value
// of backend.BoundsCheckPolicies is Restrict. Tests of other features use it
// so their output holds only what they test.
var uncheckedPolicies = backend.BoundsCheckPolicies{
	Index:        backend.BoundsCheckUnchecked,
	Buffer:       backend.BoundsCheckUnchecked,
	ImageLoad:    backend.BoundsCheckUnchecked,
	BindingArray: backend.BoundsCheckUnchecked,
}

// TestFeatureAvailability: a feature the target version lacks is an error, as
// Rust naga's check_availability makes it, instead of GLSL the driver
// rejects. GLSL ES has no textureQueryLevels or textureSamples, so it cannot
// bounds-check the level or sample of a texelFetch: the error names the
// ImageLoad policy, and Unchecked compiles. A global only another entry point
// uses asks for nothing (Rust skips globals the entry point does not use).
func TestFeatureAvailability(t *testing.T) {
	const load = `
@group(0) @binding(0) var tex: texture_2d<f32>;
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(p.xy), 0);
}`
	module := lowerWGSL(t, load)
	for _, policy := range []backend.BoundsCheckPolicy{backend.BoundsCheckRestrict, backend.BoundsCheckReadZeroSkipWrite} {
		opts := glsl.Options{LangVersion: glsl.VersionES300, BoundsCheckPolicies: uncheckedPolicies}
		opts.BoundsCheckPolicies.ImageLoad = policy
		_, _, err := glsl.Compile(module, opts)
		if err == nil || !strings.Contains(err.Error(), "GLSL 300 es doesn't support the required feature(s): TEXTURE_LEVELS") ||
			!strings.Contains(err.Error(), "BoundsCheckPolicies.ImageLoad to Unchecked") {
			t.Errorf("ES 3.00, ImageLoad %v: err = %v, want TEXTURE_LEVELS missing and the policy named", policy, err)
		}
		var missing *glsl.MissingFeaturesError
		if !errors.As(err, &missing) || missing.Version != glsl.VersionES300 || !missing.ForImageLoadChecks ||
			len(missing.Features) != 1 || missing.Features[0] != "TEXTURE_LEVELS" {
			t.Errorf("ES 3.00, ImageLoad %v: errors.As = %+v, want TEXTURE_LEVELS for ImageLoad on 300 es", policy, missing)
		}
	}
	// A load without a level or sample (a storage texture) needs neither
	// query, so ES checks it, as Rust does.
	storage := lowerWGSL(t, `
@group(0) @binding(0) var st: texture_storage_2d<r32float, read>;
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(st, vec2<i32>(p.xy));
}`)
	for _, policy := range []backend.BoundsCheckPolicy{backend.BoundsCheckRestrict, backend.BoundsCheckReadZeroSkipWrite} {
		opts := glsl.Options{LangVersion: glsl.VersionES310, BoundsCheckPolicies: uncheckedPolicies}
		opts.BoundsCheckPolicies.ImageLoad = policy
		if _, _, err := glsl.Compile(storage, opts); err != nil {
			t.Errorf("ES 3.10 storage texture load, ImageLoad %v: %v", policy, err)
		}
	}
	if _, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES300, BoundsCheckPolicies: uncheckedPolicies}); err != nil {
		t.Errorf("ES 3.00, ImageLoad Unchecked: %v", err)
	}
	code, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version330})
	if err != nil || !strings.Contains(code, "#extension GL_ARB_texture_query_levels : require") {
		t.Errorf("GLSL 3.30, ImageLoad Restrict: err = %v, want the extension:\n%s", err, code)
	}

	compute := lowerWGSL(t, `@compute @workgroup_size(1) fn main() {}`)
	if _, _, err := glsl.Compile(compute, glsl.Options{LangVersion: glsl.Version330}); err == nil ||
		!strings.Contains(err.Error(), "COMPUTE_SHADER") {
		t.Errorf("compute on GLSL 3.30: err = %v, want COMPUTE_SHADER missing", err)
	}

	two := lowerWGSL(t, `
@group(0) @binding(0) var<storage, read_write> data: array<u32>;
@compute @workgroup_size(1) fn cs() { data[0] = 1u; }
@fragment fn fs() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`)
	code, _, err = glsl.Compile(two, glsl.Options{LangVersion: glsl.Version330, EntryPoint: entry(t, two, "fs")})
	if err != nil || strings.Contains(code, "GL_ARB_shader_storage_buffer_object") {
		t.Errorf("fragment entry point beside a storage buffer it does not use: err = %v\n%s", err, code)
	}
}

// entry selects m's entry point name; the tests name entry points only.
func entry(t *testing.T, m *ir.Module, name string) *backend.EntryPointSelector {
	t.Helper()
	for _, ep := range m.EntryPoints {
		if ep.Name == name {
			return &backend.EntryPointSelector{Stage: ep.Stage, Name: name}
		}
	}
	t.Fatalf("no entry point %q", name)
	return nil
}
