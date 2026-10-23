package codegen

import "testing"

// A location argument of a vertex entry point is unpacked into a local with
// the argument's own name, which the body reads, as Rust naga does. Every
// argument was named after argument 0 and given a fresh name besides (pos_1,
// pos_2), so the body read names nothing declared and the MSL did not
// compile.
func TestVertexPullingDirectArguments(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec4<f32>,
}
@vertex fn vs_main(@location(0) pos: vec3<f32>, @builtin(vertex_index) vi: u32, @location(1) color: vec4<f32>, @location(2) offset: vec2<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(pos.xy + offset, pos.z, 1.0 + f32(vi) * 0.0);
    out.color = color;
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{ID: 30, Stride: 20, StepMode: VertexStepModeByVertex, Attributes: []AttributeMapping{
			{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat32x3},
			{ShaderLocation: 1, Offset: 12, Format: VertexFormatUnorm8x4},
		}},
		{ID: 29, Stride: 8, StepMode: VertexStepModeByInstance, Attributes: []AttributeMapping{
			{ShaderLocation: 2, Offset: 2, Format: VertexFormatUnorm16x2},
		}},
	}
	code := compileWGSLWithOpts(t, src, opts)
	for _, want := range []string{
		"metal::float3 pos = {};",
		"metal::float4 color = {};",
		"metal::float2 offset = {};",
		"if (vi < (_buffer_sizes.buffer_size30 / 20)) {",
		"pos = unpackFloat32x3_(vb_30_elem.data[0], ",
		"color = unpackUnorm8x4_(vb_30_elem.data[12], vb_30_elem.data[13], vb_30_elem.data[14], vb_30_elem.data[15]);",
		"if (i_id < (_buffer_sizes.buffer_size29 / 8)) {",
		"offset = unpackUnorm16x2_(vb_29_elem.data[2], vb_29_elem.data[3], vb_29_elem.data[4], vb_29_elem.data[5]);",
		"out.pos = metal::float4(pos.xy + offset, pos.z, ",
		"out.color = color;",
	} {
		mustContainMSL(t, code, want)
	}
	mustNotContainMSL(t, code, "pos_1")
}
