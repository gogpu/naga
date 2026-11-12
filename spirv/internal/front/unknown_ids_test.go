package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestBodyArmsRefuseUnknownIDs gives each body instruction an operand that
// names no definition, in every operand position the arm resolves. Each must
// be refused with invalid id, never read as some other value; the ids a SPIR-V
// module names are untrusted (design §16).
//
// The fixture is bodyModule's: 3 i32, 5 f32, 9 vec4<f32>, 11 *i32, 20 an i32
// constant, 22 an f32 constant, 24 true, 40 GLSL.std.450. Id 999 is undefined.
func TestBodyArmsRefuseUnknownIDs(t *testing.T) {
	const unknown = 999
	cases := []struct {
		name string
		inst []uint32
	}{
		{"OpUndef type", instWords(spvenums.OpUndef, unknown, 200)},
		{"OpVariable type", instWords(spvenums.OpVariable, unknown, 200, uint32(spvenums.StorageClassFunction))},
		{"OpVariable initializer", instWords(spvenums.OpVariable, 11, 200, uint32(spvenums.StorageClassFunction), unknown)},
		{"OpAccessChain base", instWords(spvenums.OpAccessChain, 11, 200, unknown, 20)},
		{"OpCompositeExtract base", instWords(spvenums.OpCompositeExtract, 5, 200, unknown, 0)},
		{"OpCompositeConstruct component", instWords(spvenums.OpCompositeConstruct, 9, 200, 22, unknown, 22, 22)},
		{"OpCompositeConstruct type", instWords(spvenums.OpCompositeConstruct, unknown, 200, 22)},
		{"OpLoad pointer", instWords(spvenums.OpLoad, 3, 200, unknown)},
		{"OpStore pointer", instWords(spvenums.OpStore, unknown, 20)},
		{"OpStore value", instWords(spvenums.OpStore, 20, unknown)},
		{"OpCopyObject operand", instWords(spvenums.OpCopyObject, 3, 200, unknown)},
		{"OpSNegate operand", instWords(spvenums.OpSNegate, 3, 200, unknown)},
		{"OpSNegate result type", instWords(spvenums.OpSNegate, unknown, 200, 20)},
		{"OpIAdd left", instWords(spvenums.OpIAdd, 3, 200, unknown, 20)},
		{"OpIAdd right", instWords(spvenums.OpIAdd, 3, 200, 20, unknown)},
		{"OpIAdd result type", instWords(spvenums.OpIAdd, unknown, 200, 20, 20)},
		{"OpFAdd left", instWords(spvenums.OpFAdd, 5, 200, unknown, 22)},
		{"OpFAdd right", instWords(spvenums.OpFAdd, 5, 200, 22, unknown)},
		{"OpSMod left", instWords(spvenums.OpSMod, 3, 200, unknown, 20)},
		{"OpSMod right", instWords(spvenums.OpSMod, 3, 200, 20, unknown)},
		{"OpSMod result type", instWords(spvenums.OpSMod, unknown, 200, 20, 20)},
		{"OpFMod left", instWords(spvenums.OpFMod, 5, 200, unknown, 22)},
		{"OpFMod right", instWords(spvenums.OpFMod, 5, 200, 22, unknown)},
		{"OpShiftLeftLogical left", instWords(spvenums.OpShiftLeftLogical, 3, 200, unknown, 20)},
		{"OpShiftLeftLogical right", instWords(spvenums.OpShiftLeftLogical, 3, 200, 20, unknown)},
		{"OpSLessThan left", instWords(spvenums.OpSLessThan, 6, 200, unknown, 20)},
		{"OpSLessThan right", instWords(spvenums.OpSLessThan, 6, 200, 20, unknown)},
		{"OpAny argument", instWords(spvenums.OpAny, 6, 200, unknown)},
		{"OpSelect condition", instWords(spvenums.OpSelect, 3, 200, unknown, 20, 20)},
		{"OpSelect accept", instWords(spvenums.OpSelect, 3, 200, 24, unknown, 20)},
		{"OpSelect reject", instWords(spvenums.OpSelect, 3, 200, 24, 20, unknown)},
		{"OpVectorShuffle first", instWords(spvenums.OpVectorShuffle, 9, 200, unknown, 20, 0)},
		{"OpBitcast value", instWords(spvenums.OpBitcast, 3, 200, unknown)},
		{"OpBitcast result type", instWords(spvenums.OpBitcast, unknown, 200, 22)},
		{"OpTranspose matrix", instWords(spvenums.OpTranspose, 10, 200, unknown)},
		{"OpDot left", instWords(spvenums.OpDot, 5, 200, unknown, 22)},
		{"OpDot right", instWords(spvenums.OpDot, 5, 200, 22, unknown)},
		{"OpExtInst argument", instWords(spvenums.OpExtInst, 5, 200, 40, uint32(spvenums.GlslStd450Sin), unknown)},
		{"OpDPdx argument", instWords(spvenums.OpDPdx, 5, 200, unknown)},
		{"OpControlBarrier scope", instWords(spvenums.OpControlBarrier, unknown, 29, 32)},
		{"OpControlBarrier semantics", instWords(spvenums.OpControlBarrier, 29, 29, unknown)},
		{"OpMemoryBarrier scope", instWords(spvenums.OpMemoryBarrier, unknown, 32)},
		{"OpMemoryBarrier semantics", instWords(spvenums.OpMemoryBarrier, 30, unknown)},
		{"OpReturnValue value", instWords(spvenums.OpReturnValue, unknown)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newBodyModule().withExtInst()
			got := m.parseError(t, tc.inst).Message
			if !strings.HasPrefix(got, "invalid id %") {
				t.Errorf("message = %q, want invalid id", got)
			}
		})
	}
}

// TestAccessChainIndexUnknown: an access chain resolves its index after its
// base, so a valid pointer base (a local array, type 18) and an unknown index
// is its own path.
func TestAccessChainIndexUnknown(t *testing.T) {
	m := newBodyModule()
	got := m.parseError(t,
		instWords(spvenums.OpVariable, 18, 200, uint32(spvenums.StorageClassFunction)),
		instWords(spvenums.OpAccessChain, 11, 201, 200, 999),
	).Message
	if got != "invalid id %999" {
		t.Errorf("message = %q, want invalid id %%999", got)
	}
}

// TestShuffleSecondOperandUnknown: VectorShuffle resolves its second vector
// after the first, so a valid first and an unknown second is its own path.
func TestShuffleSecondOperandUnknown(t *testing.T) {
	m := newBodyModule()
	got := m.parseError(t,
		instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
		instWords(spvenums.OpVectorShuffle, 9, 201, 200, 999, 0, 1, 2, 3),
	).Message
	if got != "invalid id %999" {
		t.Errorf("message = %q, want invalid id %%999", got)
	}
}
