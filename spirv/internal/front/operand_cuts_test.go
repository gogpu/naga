package front

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestEveryOperandReadIsGuarded cuts each instruction of the decoder after
// every one of its operands: the handler must stop with a *Error instead of
// reading past the input. This is the untrusted-input guarantee of design §16
// applied to every read the module-level and body handlers perform.
func TestEveryOperandReadIsGuarded(t *testing.T) {
	prelude := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
	}
	cases := []struct {
		name     string
		operands []uint32
		op       spvenums.Op
		body     bool
	}{
		// Module level: types and constants, with a valid type id 1 in the
		// prelude where the handler resolves one before reading further.
		{"OpTypeBool", []uint32{2}, spvenums.OpTypeBool, false},
		{"OpTypeInt", []uint32{2, 32, 1}, spvenums.OpTypeInt, false},
		{"OpTypeFloat", []uint32{2, 32}, spvenums.OpTypeFloat, false},
		{"OpTypeVector", []uint32{2, 1, 3}, spvenums.OpTypeVector, false},
		{"OpTypeMatrix", []uint32{2, 1, 2}, spvenums.OpTypeMatrix, false},
		{"OpTypeFunction", []uint32{2, 1, 1, 1}, spvenums.OpTypeFunction, false},
		{"OpTypePointer", []uint32{2, uint32(spvenums.StorageClassFunction), 1}, spvenums.OpTypePointer, false},
		{"OpTypeArray", []uint32{2, 1, 3}, spvenums.OpTypeArray, false},
		{"OpTypeRuntimeArray", []uint32{2, 1}, spvenums.OpTypeRuntimeArray, false},
		{"OpTypeStruct", []uint32{2, 1, 1}, spvenums.OpTypeStruct, false},
		{"OpTypeImage", []uint32{2, 1, uint32(spvenums.Dim2D), 0, 0, 0, 1, 0}, spvenums.OpTypeImage, false},
		{"OpTypeSampledImage", []uint32{2, 1}, spvenums.OpTypeSampledImage, false},
		{"OpTypeSampler", []uint32{2}, spvenums.OpTypeSampler, false},
		{"OpConstant", []uint32{1, 2, 0}, spvenums.OpConstant, false},
		{"OpConstantComposite", []uint32{1, 2, 3}, spvenums.OpConstantComposite, false},
		{"OpConstantNull", []uint32{1, 2}, spvenums.OpConstantNull, false},
		{"OpConstantTrue", []uint32{1, 2}, spvenums.OpConstantTrue, false},
		{"OpVariable", []uint32{1, 2, uint32(spvenums.StorageClassPrivate)}, spvenums.OpVariable, false},

		// Body instructions.
		{"OpVariable body", []uint32{11, 200, uint32(spvenums.StorageClassFunction)}, spvenums.OpVariable, true},
		{"OpAccessChain", []uint32{11, 201, 200, 20}, spvenums.OpAccessChain, true},
		{"OpCompositeExtract", []uint32{5, 201, 200, 2}, spvenums.OpCompositeExtract, true},
		{"OpCompositeConstruct", []uint32{9, 201, 22, 22}, spvenums.OpCompositeConstruct, true},
		{"OpLoad", []uint32{3, 201, 200}, spvenums.OpLoad, true},
		{"OpStore", []uint32{200, 20}, spvenums.OpStore, true},
		{"OpIAdd", []uint32{3, 201, 20, 20}, spvenums.OpIAdd, true},
		{"OpSelect", []uint32{3, 201, 24, 20, 20}, spvenums.OpSelect, true},
		{"OpVectorShuffle", []uint32{9, 202, 200, 200, 0, 1, 2, 3}, spvenums.OpVectorShuffle, true},
		{"OpBitcast", []uint32{3, 201, 22}, spvenums.OpBitcast, true},
		{"OpTranspose", []uint32{10, 201, 200}, spvenums.OpTranspose, true},
		{"OpDot", []uint32{5, 201, 200, 200}, spvenums.OpDot, true},
		{"OpControlBarrier", []uint32{29, 29, 32}, spvenums.OpControlBarrier, true},
		{"OpMemoryBarrier", []uint32{30, 32}, spvenums.OpMemoryBarrier, true},
		{"OpReturnValue", []uint32{20}, spvenums.OpReturnValue, true},
	}
	for _, tc := range cases {
		for cut := 0; cut < len(tc.operands); cut++ {
			words := []uint32{uint32(len(tc.operands)+1)<<16 | uint32(tc.op)}
			words = append(words, tc.operands[:cut]...)
			t.Run(tc.name+"/cut"+itoaTest(uint32(cut)), func(t *testing.T) {
				var err error
				if tc.body {
					_, _, err = Parse(newBodyModule().buildRaw(words), Options{})
				} else {
					groups := append([][]uint32{}, prelude...)
					groups = append(groups, instWords(spvenums.OpTypeInt, 1, 32, 0), words)
					_, _, err = parseWords(t, Options{}, groups...)
				}
				var parseErr *Error
				if !errors.As(err, &parseErr) {
					t.Fatalf("error %v (%T), want a *Error", err, err)
				}
			})
		}
	}
}
