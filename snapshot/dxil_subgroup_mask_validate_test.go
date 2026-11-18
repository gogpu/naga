package snapshot_test

// P2-IR-042: the five ballot-mask built-ins the DXIL emitter synthesizes (the
// mask spans every invocation of the wave, so it is built from the lane index
// and the wave size per 32-bit word — never from dx.op.ballot(true), which
// holds only the active lanes) produce a blob IDxcValidator accepts. The
// synthesis runs through the real exposure route: GLSL source -> glsl.Parse ->
// dxil.Compile -> IDxcValidator.

import (
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

func TestDxilSubgroupMaskValidate(t *testing.T) {
	requireIDxcValidator(t)

	const ext = "#version 460 core\n#extension GL_KHR_shader_subgroup_ballot : require\nlayout(local_size_x = 64) in;\n"
	const buf = "layout(std430, binding = 0) buffer Out { uvec4 data[]; };\n"
	cases := []struct {
		name string
		body string
	}{
		{"eq-mask", buf + "void main() { data[0] = gl_SubgroupEqMask; }"},
		{"ge-mask", buf + "void main() { data[0] = gl_SubgroupGeMask; }"},
		{"gt-mask", buf + "void main() { data[0] = gl_SubgroupGtMask; }"},
		{"le-mask", buf + "void main() { data[0] = gl_SubgroupLeMask; }"},
		{"lt-mask", buf + "void main() { data[0] = gl_SubgroupLtMask; }"},
		{"all-five", buf + "void main() { data[0] = gl_SubgroupEqMask | gl_SubgroupGeMask | gl_SubgroupGtMask | gl_SubgroupLeMask | gl_SubgroupLtMask; }"},
		{"with-subgroup-builtins", buf + "void main() { data[0] = gl_SubgroupEqMask + uvec4(gl_SubgroupSize, gl_SubgroupInvocationID, gl_SubgroupID, gl_NumSubgroups); }"},
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
