package valid_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// parse lowers WGSL for a validation test.
func parse(t *testing.T, src string) *ir.Module {
	t.Helper()
	m, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

// errorTexts are the messages of a Validate error, one per problem.
func errorTexts(err error) []string {
	var errs valid.Errors
	if !errors.As(err, &errs) {
		if err == nil {
			return nil
		}
		return []string{err.Error()}
	}
	texts := make([]string, len(errs))
	for i := range errs {
		texts[i] = errs[i].Error()
	}
	return texts
}

// expectRejected validates m with opts and wants an error containing want.
func expectRejected(t *testing.T, m *ir.Module, opts valid.Options, want string) {
	t.Helper()
	_, err := valid.Validate(m, opts)
	if err == nil {
		t.Fatalf("validated; want an error containing %q", want)
	}
	for _, text := range errorTexts(err) {
		if strings.Contains(text, want) {
			return
		}
	}
	t.Fatalf("errors %q; want one containing %q", errorTexts(err), want)
}

// expectValid validates m with opts and wants no error.
func expectValid(t *testing.T, m *ir.Module, opts valid.Options) {
	t.Helper()
	if _, err := valid.Validate(m, opts); err != nil {
		t.Fatalf("errors %q; want none", errorTexts(err))
	}
}

// capabilityGateCases are WGSL shaders that each use one feature behind a
// capability: rejected with the options of without (the zero Options when
// nil), with Rust's error text, and valid with the options of with. Each
// names the Rust check it ports (naga/src/valid at the pinned commit).
var capabilityGateCases = []struct {
	name    string
	rust    string // the Rust check site
	src     string
	without *valid.Options
	with    valid.Options
	want    string
}{
	{
		name: "subgroup ballot", rust: "function.rs:1596 Statement::SubgroupBallot",
		src:  `@compute @workgroup_size(64) fn main() { _ = subgroupBallot(true); }`,
		with: valid.Options{Capabilities: valid.CapSubgroup},
		want: "in function main, statement 0: Shader requires capability Capabilities(SUBGROUP)",
	},
	{
		name: "subgroup collective operation", rust: "function.rs:1632 Statement::SubgroupCollectiveOperation",
		src:  `@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) i: u32) { _ = subgroupAdd(i); }`,
		with: valid.Options{Capabilities: valid.CapSubgroup},
		want: "Shader requires capability Capabilities(SUBGROUP)",
	},
	{
		name: "subgroup gather", rust: "function.rs:1654 Statement::SubgroupGather",
		src:  `@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) i: u32) { _ = subgroupShuffle(i, 1u); }`,
		with: valid.Options{Capabilities: valid.CapSubgroup},
		want: "Shader requires capability Capabilities(SUBGROUP)",
	},
	{
		name: "subgroup built-in", rust: "interface.rs:262 required, BuiltIn::SubgroupSize",
		src:  `@compute @workgroup_size(64) fn main(@builtin(subgroup_size) s: u32) {}`,
		with: valid.Options{Capabilities: valid.CapSubgroup},
		want: `entry point "main": argument 0: Capability Capabilities(SUBGROUP) is not supported`,
	},
	{
		name: "subgroup barrier", rust: "function.rs:1038 Barrier::SUB_GROUP",
		src:     `@compute @workgroup_size(64) fn main() { subgroupBarrier(); }`,
		without: &valid.Options{Capabilities: valid.CapSubgroup},
		with:    valid.Options{Capabilities: valid.CapSubgroup | valid.CapSubgroupBarrier},
		want:    "Shader requires capability Capabilities(SUBGROUP | SUBGROUP_BARRIER)",
	},
	{
		name: "subgroup operation in a vertex shader", rust: "mod.rs:617 subgroup_stages, interface.rs:1379 ForbiddenStageOperations",
		src: `@vertex fn main(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f {
    return vec4f(f32(subgroupAdd(i)));
}`,
		without: &valid.Options{Capabilities: valid.CapSubgroup},
		with:    valid.Options{Capabilities: valid.CapSubgroup | valid.CapSubgroupVertexStage},
		want:    "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
	},
	{
		name: "64-bit atomic min in storage", rust: "function.rs:476 Scalar::I64 | U64, SHADER_INT64_ATOMIC_MIN_MAX",
		src: `@group(0) @binding(0) var<storage, read_write> a: atomic<u64>;
@compute @workgroup_size(1) fn main() { atomicMax(&a, 1lu); }`,
		without: &valid.Options{Capabilities: valid.CapShaderInt64},
		with:    valid.Options{Capabilities: valid.CapShaderInt64 | valid.CapShaderInt64AtomicMinMax},
		want:    "Atomic operation is invalid: Capability Capabilities(SHADER_INT64_ATOMIC_MIN_MAX) is required",
	},
	{
		name: "64-bit atomic add", rust: "function.rs:476 Scalar::I64 | U64, SHADER_INT64_ATOMIC_ALL_OPS",
		src: `@group(0) @binding(0) var<storage, read_write> a: atomic<u64>;
@compute @workgroup_size(1) fn main() { atomicAdd(&a, 1lu); }`,
		without: &valid.Options{Capabilities: valid.CapShaderInt64 | valid.CapShaderInt64AtomicMinMax},
		with:    valid.Options{Capabilities: valid.CapShaderInt64 | valid.CapShaderInt64AtomicAllOps},
		want:    "Atomic operation is invalid: Capability Capabilities(SHADER_INT64_ATOMIC_ALL_OPS) is required",
	},
	{
		name: "64-bit atomic type", rust: "type.rs:476 TypeInner::Atomic",
		src: `@group(0) @binding(0) var<storage, read_write> a: atomic<u64>;
@compute @workgroup_size(1) fn main() { _ = atomicLoad(&a); }`,
		without: &valid.Options{Capabilities: valid.CapShaderInt64},
		with:    valid.Options{Capabilities: valid.CapShaderInt64 | valid.CapShaderInt64AtomicMinMax},
		want:    "is invalid: Capability Capabilities(SHADER_INT64_ATOMIC_ALL_OPS) is required",
	},
	{
		name: "f32 atomic", rust: "function.rs:516 and type.rs:495 SHADER_FLOAT32_ATOMIC",
		src: `@group(0) @binding(0) var<storage, read_write> a: atomic<f32>;
@compute @workgroup_size(1) fn main() { atomicAdd(&a, 1.0); }`,
		with: valid.Options{Capabilities: valid.CapShaderFloat32Atomic},
		want: "Atomic operation is invalid: Capability Capabilities(SHADER_FLOAT32_ATOMIC) is required",
	},
	{
		name: "texture atomic", rust: "function.rs:1408 TEXTURE_ATOMIC",
		src: `@group(0) @binding(0) var t: texture_storage_2d<r32uint, atomic>;
@compute @workgroup_size(1) fn main() { textureAtomicAdd(t, vec2i(0), 1u); }`,
		with: valid.Options{Capabilities: valid.CapTextureAtomic},
		want: "Shader requires capability Capabilities(TEXTURE_ATOMIC)",
	},
	{
		name: "64-bit texture atomic", rust: "function.rs:1380 TEXTURE_INT64_ATOMIC",
		src: `@group(0) @binding(0) var t: texture_storage_2d<r64uint, atomic>;
@compute @workgroup_size(1) fn main() { textureAtomicMax(t, vec2i(0), 1lu); }`,
		without: &valid.Options{Capabilities: valid.CapShaderInt64},
		with:    valid.Options{Capabilities: valid.CapShaderInt64 | valid.CapTextureInt64Atomic},
		want:    "Shader requires capability Capabilities(TEXTURE_INT64_ATOMIC)",
	},
	{
		name: "ray query", rust: "type.rs:939 TypeInner::RayQuery",
		src: `enable wgpu_ray_query;
@compute @workgroup_size(1) fn main() { var rq: ray_query; }`,
		with: valid.Options{Capabilities: valid.CapRayQuery},
		want: "is invalid: Capability Capabilities(RAY_QUERY) is required",
	},
	{
		name: "acceleration structure", rust: "type.rs:928 TypeInner::AccelerationStructure",
		src: `enable wgpu_ray_query;
@group(0) @binding(0) var acc: acceleration_structure;
@compute @workgroup_size(1) fn main() { _ = acc; }`,
		with: valid.Options{Capabilities: valid.CapRayQuery},
		want: "is invalid: Capability Capabilities(RAY_QUERY) is required",
	},
	{
		name: "view index", rust: "interface.rs:270 BuiltIn::ViewIndex",
		src:  `@fragment fn main(@builtin(view_index) v: u32) {}`,
		with: valid.Options{Capabilities: valid.CapMultiview},
		want: `entry point "main": argument 0: Capability Capabilities(MULTIVIEW) is not supported`,
	},
	{
		name: "primitive index", rust: "interface.rs:266 BuiltIn::PrimitiveIndex",
		src: `enable primitive_index;
@fragment fn main(@builtin(primitive_index) p: u32) {}`,
		with: valid.Options{Capabilities: valid.CapPrimitiveIndex},
		want: "Capability Capabilities(PRIMITIVE_INDEX) is not supported",
	},
	{
		name: "clip distances", rust: "interface.rs:263 BuiltIn::ClipDistances",
		src: `enable clip_distances;
struct V { @builtin(position) p: vec4f, @builtin(clip_distances) c: array<f32, 1> }
@vertex fn main() -> V { return V(vec4f(0.0), array<f32, 1>(0.5)); }`,
		with: valid.Options{Capabilities: valid.CapClipDistances},
		want: `entry point "main": result: Capability Capabilities(CLIP_DISTANCES) is not supported`,
	},
	{
		name: "barycentric", rust: "interface.rs:269 BuiltIn::Barycentric",
		src:  `@fragment fn main(@builtin(barycentric) b: vec3f) -> @location(0) vec4f { return vec4f(b, 1.0); }`,
		with: valid.Options{Capabilities: valid.CapShaderBarycentrics},
		want: "Capability Capabilities(SHADER_BARYCENTRICS) is not supported",
	},
	{
		name: "draw index", rust: "interface.rs:276 BuiltIn::DrawIndex",
		src: `enable draw_index;
@vertex fn main(@builtin(draw_index) d: u32) -> @builtin(position) vec4f { return vec4f(f32(d)); }`,
		with: valid.Options{Capabilities: valid.CapDrawIndex},
		want: "Capability Capabilities(DRAW_INDEX) is not supported",
	},
	{
		name: "dual source blending", rust: "type.rs:713 Binding::Location { blend_src }",
		src: `enable dual_source_blending;
struct O { @location(0) @blend_src(0) a: vec4f, @location(0) @blend_src(1) b: vec4f }
@fragment fn main() -> O { return O(vec4f(0.0), vec4f(1.0)); }`,
		with: valid.Options{Capabilities: valid.CapDualSourceBlending},
		want: "is invalid: Capability Capabilities(DUAL_SOURCE_BLENDING) is required",
	},
	{
		name: "early depth test", rust: "interface.rs:1286 early_depth_test",
		src:  `@fragment @early_depth_test(force) fn main() -> @location(0) vec4f { return vec4f(0.0); }`,
		with: valid.Options{Capabilities: valid.CapEarlyDepthTest},
		want: "Entry point main at Fragment is invalid: Capability Capabilities(EARLY_DEPTH_TEST) is not supported",
	},
	{
		name: "per-vertex input", rust: "interface.rs:710 Interpolation::PerVertex",
		src: `enable wgpu_per_vertex;
@fragment fn main(@location(0) @interpolate(per_vertex) v: array<f32, 3>) -> @location(0) vec4f { return vec4f(v[0]); }`,
		with: valid.Options{Capabilities: valid.CapPerVertex},
		want: `entry point "main": argument 0: Capability Capabilities(PER_VERTEX) is not supported`,
	},
	{
		name: "16-bit norm storage texture", rust: "interface.rs:1066 STORAGE_TEXTURE_16BIT_NORM_FORMATS",
		src: `@group(0) @binding(0) var t: texture_storage_2d<r16unorm, write>;
@compute @workgroup_size(1) fn main() { textureStore(t, vec2i(0), vec4f(0.0)); }`,
		with: valid.Options{Capabilities: valid.CapStorageTexture16BitNormFormats},
		want: "Global variable [0] 't' is invalid: Capability Capabilities(STORAGE_TEXTURE_16BIT_NORM_FORMATS) is not supported",
	},
	{
		name: "binding array of textures", rust: "interface.rs:973 TEXTURE_AND_SAMPLER_BINDING_ARRAY",
		src: `enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_2d<f32>, 2>;
@compute @workgroup_size(1) fn main() { _ = textureDimensions(t[0]); }`,
		with: valid.Options{Capabilities: valid.CapTextureAndSamplerBindingArray},
		want: "Global variable [0] 't' is invalid: Capability Capabilities(TEXTURE_AND_SAMPLER_BINDING_ARRAY) is not supported",
	},
	{
		name: "binding array of samplers", rust: "interface.rs:988 Sampler in a binding array",
		src: `enable wgpu_binding_array;
@group(0) @binding(0) var s: binding_array<sampler, 2>;
@group(0) @binding(1) var t: texture_2d<f32>;
@fragment fn main() -> @location(0) vec4f { return textureSample(t, s[1], vec2f(0.0)); }`,
		with: valid.Options{Capabilities: valid.CapTextureAndSamplerBindingArray},
		want: "Global variable [0] 's' is invalid: Capability Capabilities(TEXTURE_AND_SAMPLER_BINDING_ARRAY) is not supported",
	},
	{
		name: "binding array of storage textures", rust: "interface.rs:963 STORAGE_TEXTURE_BINDING_ARRAY",
		src: `enable wgpu_binding_array;
@group(0) @binding(0) var t: binding_array<texture_storage_2d<rgba8unorm, write>, 2>;
@compute @workgroup_size(1) fn main() { textureStore(t[0], vec2i(0), vec4f(0.0)); }`,
		with: valid.Options{Capabilities: valid.CapStorageTextureBindingArray},
		want: "Capability Capabilities(STORAGE_TEXTURE_BINDING_ARRAY) is not supported",
	},
	{
		name: "binding array of storage buffers", rust: "interface.rs:938 STORAGE_BUFFER_BINDING_ARRAY",
		src: `enable wgpu_binding_array;
struct S { x: u32 }
@group(0) @binding(0) var<storage, read_write> b: binding_array<S, 2>;
@compute @workgroup_size(1) fn main() { b[0].x = 1u; }`,
		with: valid.Options{Capabilities: valid.CapStorageBufferBindingArray},
		want: "Capability Capabilities(STORAGE_BUFFER_BINDING_ARRAY) is not supported",
	},
	{
		name: "binding array of uniform buffers", rust: "interface.rs:949 BUFFER_BINDING_ARRAY",
		src: `enable wgpu_binding_array;
struct U { x: vec4f }
@group(0) @binding(0) var<uniform> b: binding_array<U, 2>;
@fragment fn main() -> @location(0) vec4f { return b[1].x; }`,
		with: valid.Options{Capabilities: valid.CapBufferBindingArray},
		want: "Capability Capabilities(BUFFER_BINDING_ARRAY) is not supported",
	},
}

// TestCapabilityGates: each feature is rejected without its capability,
// with Rust's error text, and validates with it.
func TestCapabilityGates(t *testing.T) {
	for _, tt := range capabilityGateCases {
		t.Run(tt.name, func(t *testing.T) {
			m := parse(t, tt.src)
			without := valid.Options{}
			if tt.without != nil {
				without = *tt.without
			}
			expectRejected(t, m, without, tt.want)
			expectValid(t, m, tt.with)
		})
	}
}
