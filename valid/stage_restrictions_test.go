package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// stageRestrictionCases are operations Rust naga's validator allows only in
// some stages: each case's shader uses one in an entry point of a stage that
// may not, directly or through a helper, and is rejected with Rust's
// EntryPointError::ForbiddenStageOperations; ok uses it where it may.
// Rust records the stages per function (FunctionInfo::available_stages,
// narrowed in valid/function.rs and valid/expression.rs at the pinned
// commit) and checks each entry point's stage against them
// (valid/interface.rs, ForbiddenStageOperations).
var stageRestrictionCases = []struct {
	name   string
	shader string
	want   string
	ok     string
}{
	{
		// function.rs, Statement::Kill → FRAGMENT.
		name: "discard in vertex helper",
		shader: `fn f() { discard; }
@vertex fn main() -> @builtin(position) vec4f { f(); return vec4f(); }`,
		want: "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
		ok: `fn f() { discard; }
@fragment fn main() -> @location(0) vec4f { f(); return vec4f(); }`,
	},
	{
		name:   "discard in compute",
		shader: `@compute @workgroup_size(1) fn main() { discard; }`,
		want:   "Entry point main at Compute is invalid: Uses operations forbidden at this stage",
	},
	{
		// function.rs, Statement::ControlBarrier → COMPUTE_LIKE.
		name:   "workgroupBarrier in fragment",
		shader: `@fragment fn main() -> @location(0) vec4f { workgroupBarrier(); return vec4f(); }`,
		want:   "Entry point main at Fragment is invalid: Uses operations forbidden at this stage",
		ok:     `@compute @workgroup_size(1) fn main() { workgroupBarrier(); }`,
	},
	{
		name: "storageBarrier in vertex helper",
		shader: `fn f() { storageBarrier(); }
@vertex fn main() -> @builtin(position) vec4f { f(); return vec4f(); }`,
		want: "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
		ok: `fn f() { storageBarrier(); }
@compute @workgroup_size(1) fn main() { f(); }`,
	},
	{
		name:   "textureBarrier in fragment",
		shader: `@fragment fn main() -> @location(0) vec4f { textureBarrier(); return vec4f(); }`,
		want:   "Entry point main at Fragment is invalid: Uses operations forbidden at this stage",
	},
	{
		// function.rs, Statement::WorkGroupUniformLoad and expression.rs,
		// WorkGroupUniformLoadResult → COMPUTE_LIKE.
		name: "workgroupUniformLoad in vertex helper",
		shader: `var<workgroup> w: u32;
fn f() -> u32 { return workgroupUniformLoad(&w); }
@vertex fn main() -> @builtin(position) vec4f { return vec4f(f32(f())); }`,
		want: "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
		ok: `var<workgroup> w: u32;
fn f() -> u32 { return workgroupUniformLoad(&w); }
@compute @workgroup_size(1) fn main() { _ = f(); }`,
	},
	{
		// expression.rs, ImageSample with SampleLevel::Auto → FRAGMENT.
		name: "textureSample in vertex",
		shader: `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@vertex fn main() -> @builtin(position) vec4f { return textureSample(t, s, vec2f()); }`,
		want: "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
		ok: `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment fn main() -> @location(0) vec4f { return textureSample(t, s, vec2f()); }`,
	},
	{
		name: "textureSampleCompare in compute helper",
		shader: `@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
fn f() -> f32 { return textureSampleCompare(t, s, vec2f(), 0.5); }
@compute @workgroup_size(1) fn main() { _ = f(); }`,
		want: "Entry point main at Compute is invalid: Uses operations forbidden at this stage",
	},
	{
		// expression.rs, ImageSample with SampleLevel::Bias → FRAGMENT.
		name: "textureSampleBias in vertex",
		shader: `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@vertex fn main() -> @builtin(position) vec4f { return textureSampleBias(t, s, vec2f(), 1.0); }`,
		want: "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
	},
	{
		// expression.rs, Derivative → FRAGMENT.
		name: "dpdx in compute helper",
		shader: `fn f(x: f32) -> f32 { return dpdx(x); }
@compute @workgroup_size(1) fn main() { _ = f(1.0); }`,
		want: "Entry point main at Compute is invalid: Uses operations forbidden at this stage",
		ok: `fn f(x: f32) -> f32 { return dpdx(x); }
@fragment fn main() -> @location(0) vec4f { return vec4f(f(1.0)); }`,
	},
	{
		name:   "fwidth in vertex",
		shader: `@vertex fn main(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f { return vec4f(fwidth(f32(i))); }`,
		want:   "Entry point main at Vertex is invalid: Uses operations forbidden at this stage",
	},
}

func TestStageRestrictions(t *testing.T) {
	for _, tt := range stageRestrictionCases {
		t.Run(tt.name, func(t *testing.T) {
			expectRejected(t, parse(t, tt.shader), valid.Options{}, tt.want)
			if tt.ok != "" {
				expectValid(t, parse(t, tt.ok), valid.Options{})
			}
		})
	}
}

// TestStageRestrictionsAllowedEverywhere: sampling at an explicit level, at
// level zero or with gradients, and a helper with a stage-restricted
// operation that no entry point of another stage calls, validate in every
// stage (Rust: ShaderStages::all() for SampleLevel::Zero, Exact and
// Gradient; an uncalled function is checked by no entry point).
func TestStageRestrictionsAllowedEverywhere(t *testing.T) {
	expectValid(t, parse(t, `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@group(0) @binding(2) var d: texture_depth_2d;
@group(0) @binding(3) var c: sampler_comparison;
@vertex fn main() -> @builtin(position) vec4f {
    let a = textureSampleLevel(t, s, vec2f(), 2.0);
    let b = textureSampleGrad(t, s, vec2f(), vec2f(), vec2f());
    let z = textureSampleCompareLevel(d, c, vec2f(), 0.5);
    return a + b + vec4f(z);
}`), valid.Options{})
	expectValid(t, parse(t, `fn unused() { discard; }
@compute @workgroup_size(1) fn main() { workgroupBarrier(); }`), valid.Options{})
}
