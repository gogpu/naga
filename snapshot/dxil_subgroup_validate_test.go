package snapshot_test

// P1-DXIL-115 / P1-DXIL-116: the subgroup forms the DXIL emitter writes produce
// a blob IDxcValidator accepts. A vector operand is expanded per component (the
// form DXC itself emits for a vector wave intrinsic), and the Add/Mul scans use
// dx.op.wavePrefixOp. The min/max/and/or/xor scans are refused at the emit level
// (dxil/internal/emit/subgroup_scan_vector_test.go); the GLSL frontend refuses
// them too, so they cannot reach this path. This test runs the real exposure
// route: GLSL source -> glsl.Parse -> dxil.Compile -> IDxcValidator.

import (
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

func TestDxilSubgroupFormsValidate(t *testing.T) {
	requireIDxcValidator(t)

	const ext = "#version 460 core\n#extension GL_KHR_shader_subgroup_arithmetic : require\nlayout(local_size_x = 64) in;\n"
	cases := []struct {
		name string
		body string
	}{
		{"vec4-add-reduce", "layout(std430, binding = 0) buffer Out { vec4 data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupAdd(vec4(float(i), float(i) + 1.0, float(i) + 2.0, float(i) + 3.0)); }"},
		{"vec4-min-reduce", "layout(std430, binding = 0) buffer Out { vec4 data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupMin(vec4(float(i), float(i) + 1.0, float(i) + 2.0, float(i) + 3.0)); }"},
		{"vec4-and-reduce", "layout(std430, binding = 0) buffer Out { uvec4 data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupAnd(uvec4(i, i + 1u, i + 2u, i + 3u)); }"},
		{"vec4-inclusive-add-scan", "layout(std430, binding = 0) buffer Out { vec4 data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupInclusiveAdd(vec4(float(i), float(i) + 1.0, float(i) + 2.0, float(i) + 3.0)); }"},
		{"vec4-exclusive-add-scan", "layout(std430, binding = 0) buffer Out { vec4 data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupExclusiveAdd(vec4(float(i), float(i) + 1.0, float(i) + 2.0, float(i) + 3.0)); }"},
		{"scalar-min-reduce", "layout(std430, binding = 0) buffer Out { float data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupMin(float(i)); }"},
		{"scalar-and-reduce", "layout(std430, binding = 0) buffer Out { uint data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupAnd(i); }"},
		{"scalar-inclusive-add-scan", "layout(std430, binding = 0) buffer Out { float data[]; };\n" +
			"void main() { uint i = gl_LocalInvocationIndex; data[i] = subgroupInclusiveAdd(float(i)); }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod, _, err := glsl.Parse(ext+tc.body, glsl.ParseOptions{Stage: ir.StageCompute})
			if err != nil {
				t.Fatalf("glsl.Parse: %v", err)
			}
			blob, _, err := dxil.Compile(mod, dxil.DefaultOptions())
			if err != nil {
				t.Fatalf("dxil.Compile: %v", err)
			}
			if err := dxil.Validate(blob, dxil.ValidateFull); err != nil {
				t.Errorf("IDxcValidator rejected the %s blob: %v", tc.name, err)
			}
		})
	}
}
