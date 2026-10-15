package codegen

import "testing"

// A select with a vector-bool condition must be written as
// mix(reject, accept, condition): a GLSL ternary requires a scalar bool
// condition, so `(bvec2(...) ? a : b)` is rejected by glslangValidator
// ("boolean expression expected"). Expected lines are Rust naga's output
// (naga-cli --profile es310, reference wgpu 18da915b; back/glsl/writer.rs
// Expression::Select), which glslangValidator accepts.
func TestSelectVectorConditionUsesMix(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> buf: array<u32>;
@group(0) @binding(1) var<storage, read_write> fbuf: array<f32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let c = vec2<bool>(i > 1u, i < 3u);
    let vu = select(vec2<u32>(1u), vec2<u32>(2u), c);
    let vf = select(vec2<f32>(1.0), vec2<f32>(2.0), c);
    let s = select(5u, 6u, i == 0u);
    buf[0] = vu.x + s;
    fbuf[0] = vf.y;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version{Major: 3, Minor: 10, ES: true}})

	// Vector condition: mix(reject, accept, condition), any element type.
	glslMustContain(t, output, "uvec2 vu = mix(uvec2(1u), uvec2(2u), c);")
	glslMustContain(t, output, "vec2 vf = mix(vec2(1.0), vec2(2.0), c);")
	// Scalar condition stays a parenthesized ternary.
	glslMustContain(t, output, "uint s = ((i == 0u) ? 6u : 5u);")
}
