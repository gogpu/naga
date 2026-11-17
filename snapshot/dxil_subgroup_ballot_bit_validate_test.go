package snapshot_test

// P2-IR-039: the ballot bit operations the DXIL emitter writes produce a blob
// IDxcValidator accepts. The forms are arithmetic over the four mask words
// (dx.op.unaryBits Countbits / FirstbitLo / FirstbitHi), masked to the wave
// size; the wave count ops are not used, as they count the live predicate, not
// a mask value. This test runs the real exposure route: GLSL source ->
// glsl.Parse -> dxil.Compile -> IDxcValidator. The with-subgroup-builtins case
// also pins that a ballot bit count and a built-in subgroup-size read share
// one dx.op.waveGetLaneCount declaration (AddFunction does not deduplicate).

import (
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

func TestDxilSubgroupBallotBitValidate(t *testing.T) {
	requireIDxcValidator(t)

	const ext = "#version 460 core\n#extension GL_KHR_shader_subgroup_ballot : require\nlayout(local_size_x = 64) in;\n"
	const buf = "layout(std430, binding = 0) buffer Out { uint data[]; };\n"
	const mask = "uvec4 b = subgroupBallot(gl_LocalInvocationIndex < 32u); "
	cases := []struct {
		name string
		body string
	}{
		{"count", buf + "void main() { " + mask + "data[0] = subgroupBallotBitCount(b); }"},
		{"inclusive-count", buf + "void main() { " + mask + "data[0] = subgroupBallotInclusiveBitCount(b); }"},
		{"exclusive-count", buf + "void main() { " + mask + "data[0] = subgroupBallotExclusiveBitCount(b); }"},
		{"find-lsb", buf + "void main() { " + mask + "data[0] = subgroupBallotFindLSB(b); }"},
		{"find-msb", buf + "void main() { " + mask + "data[0] = subgroupBallotFindMSB(b); }"},
		{"extract", buf + "void main() { " + mask + "data[0] = subgroupBallotBitExtract(b, 3u) ? 1u : 0u; }"},
		{"inverse-ballot", buf + "void main() { " + mask + "data[0] = subgroupInverseBallot(b) ? 1u : 0u; }"},
		{"extract-int-index", buf + "void main() { " + mask + "data[0] = subgroupBallotBitExtract(b, 3) ? 1u : 0u; }"},
		{"count-ivec4-mask", buf + "void main() { ivec4 b = ivec4(subgroupBallot(true)); data[0] = subgroupBallotBitCount(b); }"},
		{"with-subgroup-builtins", buf + "void main() { " + mask + "data[0] = subgroupBallotBitCount(b) + gl_SubgroupSize + gl_SubgroupID; }"},
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
