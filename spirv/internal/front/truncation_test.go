package front

import (
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestModuleLevelTruncation: an instruction whose word count claims more
// words than the module holds is caught by the handler that reads it, never
// read past the end of the input.
func TestModuleLevelTruncation(t *testing.T) {
	base := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
	}
	// The integer type is there for the constant handlers, which resolve
	// their result type before reading the value; it also moves the section
	// to Type, so the annotation and entry point cases go without it.
	withType := append(append([][]uint32{}, base...), instWords(spvenums.OpTypeInt, 1, 32, 0))
	cases := []struct {
		name  string
		inst  []uint32
		early bool
	}{
		{"OpTypeVoid", []uint32{2<<16 | uint32(spvenums.OpTypeVoid)}, false},
		{"OpTypeBool", []uint32{2<<16 | uint32(spvenums.OpTypeBool)}, false},
		{"OpTypeInt width", []uint32{4<<16 | uint32(spvenums.OpTypeInt), 1}, false},
		{"OpTypeInt sign", []uint32{4<<16 | uint32(spvenums.OpTypeInt), 1, 32}, false},
		{"OpTypeFloat", []uint32{3<<16 | uint32(spvenums.OpTypeFloat), 1}, false},
		{"OpTypeVector", []uint32{4<<16 | uint32(spvenums.OpTypeVector), 1}, false},
		{"OpTypeMatrix", []uint32{4<<16 | uint32(spvenums.OpTypeMatrix), 1, 2}, false},
		{"OpTypeFunction", []uint32{4<<16 | uint32(spvenums.OpTypeFunction), 1, 2}, false},
		{"OpTypePointer", []uint32{4<<16 | uint32(spvenums.OpTypePointer), 1, 2}, false},
		{"OpTypeArray", []uint32{4<<16 | uint32(spvenums.OpTypeArray), 1, 2}, false},
		{"OpTypeRuntimeArray", []uint32{3<<16 | uint32(spvenums.OpTypeRuntimeArray), 1}, false},
		{"OpTypeStruct", []uint32{3<<16 | uint32(spvenums.OpTypeStruct), 1}, false},
		{"OpTypeImage", []uint32{9<<16 | uint32(spvenums.OpTypeImage), 1, 2}, false},
		{"OpTypeSampledImage", []uint32{3<<16 | uint32(spvenums.OpTypeSampledImage), 1}, false},
		{"OpTypeSampler", []uint32{2<<16 | uint32(spvenums.OpTypeSampler)}, false},
		{"OpConstant", []uint32{4<<16 | uint32(spvenums.OpConstant), 1, 2}, false},
		{"OpConstantComposite", []uint32{4<<16 | uint32(spvenums.OpConstantComposite), 1, 2}, false},
		{"OpConstantNull", []uint32{3<<16 | uint32(spvenums.OpConstantNull), 1}, false},
		{"OpConstantTrue", []uint32{3<<16 | uint32(spvenums.OpConstantTrue), 1}, false},
		{"OpVariable", []uint32{4<<16 | uint32(spvenums.OpVariable), 1, 2}, false},
		{"OpDecorate", []uint32{3<<16 | uint32(spvenums.OpDecorate), 1}, true},
		{"OpMemberDecorate", []uint32{4<<16 | uint32(spvenums.OpMemberDecorate), 1, 0}, true},
		{"OpEntryPoint", []uint32{4<<16 | uint32(spvenums.OpEntryPoint), 0, 1}, true},
		{"OpExecutionMode", []uint32{3<<16 | uint32(spvenums.OpExecutionMode), 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prelude := withType
			if tc.early {
				prelude = base
			}
			groups := append(append([][]uint32{}, prelude...), tc.inst)
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != "incomplete data" {
				t.Errorf("message = %q, want incomplete data", got)
			}
		})
	}
}

// TestFunctionBodyTruncation: the same for the instructions inside a body.
func TestFunctionBodyTruncation(t *testing.T) {
	cases := []struct {
		name string
		inst []uint32
	}{
		{"OpVariable", []uint32{5<<16 | uint32(spvenums.OpVariable), 11, 200}},
		{"OpAccessChain", []uint32{4<<16 | uint32(spvenums.OpAccessChain), 11, 201}},
		{"OpCompositeExtract", []uint32{4<<16 | uint32(spvenums.OpCompositeExtract), 5, 201}},
		{"OpCompositeConstruct", []uint32{3<<16 | uint32(spvenums.OpCompositeConstruct), 9}},
		{"OpLoad", []uint32{4<<16 | uint32(spvenums.OpLoad), 3, 200}},
		{"OpStore", []uint32{3<<16 | uint32(spvenums.OpStore), 200}},
		{"OpIAdd", []uint32{5<<16 | uint32(spvenums.OpIAdd), 3, 200}},
		{"OpExtInst", []uint32{7<<16 | uint32(spvenums.OpExtInst), 5, 200, 40, uint32(spvenums.GlslStd450Pow)}},
		{"OpControlBarrier", []uint32{4<<16 | uint32(spvenums.OpControlBarrier), 29}},
		{"OpMemoryBarrier", []uint32{3<<16 | uint32(spvenums.OpMemoryBarrier), 30}},
		{"OpSelect", []uint32{6<<16 | uint32(spvenums.OpSelect), 3, 200, 24}},
		{"OpVectorShuffle", []uint32{5<<16 | uint32(spvenums.OpVectorShuffle), 8, 201, 200}},
		{"OpReturnValue", []uint32{2<<16 | uint32(spvenums.OpReturnValue)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newBodyModule().withExtInst()
			_, _, err := Parse(m.buildRaw(tc.inst), Options{})
			if got := requireError(t, err).Message; got != "incomplete data" {
				t.Errorf("message = %q, want incomplete data", got)
			}
		})
	}
}

// TestOperandCountChecks: an instruction with the wrong number of operands is
// refused before its operands are read.
func TestOperandCountChecks(t *testing.T) {
	prelude := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
	}
	cases := []struct {
		name string
		inst []uint32
		want string
	}{
		{"OpTypeVoid", instWords(spvenums.OpTypeVoid, 1, 2), "invalid operand count 3 for OpTypeVoid"},
		{"OpTypeBool", instWords(spvenums.OpTypeBool, 1, 2), "invalid operand count 3 for OpTypeBool"},
		{"OpTypeInt", instWords(spvenums.OpTypeInt, 1, 32), "invalid operand count 3 for OpTypeInt"},
		{"OpTypeFloat", instWords(spvenums.OpTypeFloat, 1, 32, 0), "invalid operand count 4 for OpTypeFloat"},
		{"OpTypeVector", instWords(spvenums.OpTypeVector, 1, 2), "invalid operand count 3 for OpTypeVector"},
		{"OpTypeMatrix", instWords(spvenums.OpTypeMatrix, 1, 2), "invalid operand count 3 for OpTypeMatrix"},
		{"OpTypePointer", instWords(spvenums.OpTypePointer, 1, 2), "invalid operand count 3 for OpTypePointer"},
		{"OpTypeArray", instWords(spvenums.OpTypeArray, 1, 2), "invalid operand count 3 for OpTypeArray"},
		{"OpTypeRuntimeArray", instWords(spvenums.OpTypeRuntimeArray, 1, 2, 3), "invalid operand count 4 for OpTypeRuntimeArray"},
		{"OpTypeImage", instWords(spvenums.OpTypeImage, 1, 2, 3), "invalid operand count 4 for OpTypeImage"},
		{"OpTypeSampledImage", instWords(spvenums.OpTypeSampledImage, 1, 2, 3), "invalid operand count 4 for OpTypeSampledImage"},
		{"OpTypeSampler", instWords(spvenums.OpTypeSampler, 1, 2), "invalid operand count 3 for OpTypeSampler"},
		{"OpConstantNull", instWords(spvenums.OpConstantNull, 1, 2, 3), "invalid operand count 4 for OpConstantNull"},
		{"OpConstantTrue", instWords(spvenums.OpConstantTrue, 1, 2, 3), "invalid operand count 4 for OpConstantTrue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := append(append([][]uint32{}, prelude...), tc.inst)
			_, _, err := parseWords(t, Options{}, groups...)
			if got := requireError(t, err).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}
