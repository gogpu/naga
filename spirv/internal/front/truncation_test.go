package front

import (
	"fmt"
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
		{"OpTypeVoid", instWords(spvenums.OpTypeVoid, 1, 2), "invalid operand count 3 for TypeVoid"},
		{"OpTypeBool", instWords(spvenums.OpTypeBool, 1, 2), "invalid operand count 3 for TypeBool"},
		{"OpTypeInt", instWords(spvenums.OpTypeInt, 1, 32), "invalid operand count 3 for TypeInt"},
		{"OpTypeFloat", instWords(spvenums.OpTypeFloat, 1, 32, 0), "invalid operand count 4 for TypeFloat"},
		{"OpTypeVector", instWords(spvenums.OpTypeVector, 1, 2), "invalid operand count 3 for TypeVector"},
		{"OpTypeMatrix", instWords(spvenums.OpTypeMatrix, 1, 2), "invalid operand count 3 for TypeMatrix"},
		{"OpTypePointer", instWords(spvenums.OpTypePointer, 1, 2), "invalid operand count 3 for TypePointer"},
		{"OpTypeArray", instWords(spvenums.OpTypeArray, 1, 2), "invalid operand count 3 for TypeArray"},
		{"OpTypeRuntimeArray", instWords(spvenums.OpTypeRuntimeArray, 1, 2, 3), "invalid operand count 4 for TypeRuntimeArray"},
		{"OpTypeImage", instWords(spvenums.OpTypeImage, 1, 2, 3), "invalid operand count 4 for TypeImage"},
		{"OpTypeSampledImage", instWords(spvenums.OpTypeSampledImage, 1, 2, 3), "invalid operand count 4 for TypeSampledImage"},
		{"OpTypeSampler", instWords(spvenums.OpTypeSampler, 1, 2), "invalid operand count 3 for TypeSampler"},
		{"OpConstantNull", instWords(spvenums.OpConstantNull, 1, 2, 3), "invalid operand count 4 for ConstantNull"},
		{"OpConstantTrue", instWords(spvenums.OpConstantTrue, 1, 2, 3), "invalid operand count 4 for ConstantTrue"},
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

// TestImageAtomicSubgroupTruncation: every operand read of the image, atomic
// and subgroup instructions fails cleanly when the input ends inside the
// instruction. Each instruction is truncated at each of its operand positions
// in turn, so the error arm behind every read is exercised: untrusted input
// may end anywhere. Operands are constants of the shared body module (id 20;
// 31 is the subgroup scope), so a handler that resolves an id between two
// reads gets that far and the next read is the one that fails.
func TestImageAtomicSubgroupTruncation(t *testing.T) {
	const (
		expr = 20 // the i32 constant 5: a valid expression id
		sub  = 31 // the u32 constant 3: ScopeSubgroup
	)
	cases := []struct {
		op       spvenums.Op
		operands []uint32 // the full operand list, truncated in turn
	}{
		{spvenums.OpSampledImage, []uint32{expr, expr, expr, expr}},
		{spvenums.OpImage, []uint32{expr, expr, expr}},
		{spvenums.OpImageWrite, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageFetch, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageRead, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleImplicitLod, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleExplicitLod, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleProjImplicitLod, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleDrefImplicitLod, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleDrefExplicitLod, []uint32{expr, expr, expr, expr, expr, expr, expr}},
		{spvenums.OpImageSampleProjDrefImplicitLod, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpImageGather, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpImageDrefGather, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpImageQuerySize, []uint32{expr, expr, expr}},
		{spvenums.OpImageQuerySizeLod, []uint32{expr, expr, expr, expr}},
		{spvenums.OpImageQueryLevels, []uint32{expr, expr, expr}},
		{spvenums.OpImageQuerySamples, []uint32{expr, expr, expr}},
		{spvenums.OpAtomicLoad, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicStore, []uint32{expr, expr, expr, expr}},
		{spvenums.OpAtomicIIncrement, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicIDecrement, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicExchange, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicIAdd, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicISub, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicSMin, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicUMax, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicAnd, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicOr, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicXor, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicFAddEXT, []uint32{expr, expr, expr, expr, expr, expr}},
		{spvenums.OpAtomicCompareExchange, []uint32{expr, expr, expr, expr, expr, expr, expr, expr}},
		{spvenums.OpArrayLength, []uint32{expr, expr, expr, expr}},
		{spvenums.OpCompositeInsert, []uint32{expr, expr, expr, expr, expr}},
		{spvenums.OpGroupNonUniformBallot, []uint32{expr, expr, sub, expr}},
		{spvenums.OpGroupNonUniformAll, []uint32{expr, expr, sub, expr}},
		{spvenums.OpGroupNonUniformAny, []uint32{expr, expr, sub, expr}},
		{spvenums.OpGroupNonUniformIAdd, []uint32{expr, expr, sub, 0, expr}},
		{spvenums.OpGroupNonUniformFAdd, []uint32{expr, expr, sub, 0, expr}},
		{spvenums.OpGroupNonUniformBitwiseAnd, []uint32{expr, expr, sub, 0, expr}},
		{spvenums.OpGroupNonUniformLogicalOr, []uint32{expr, expr, sub, 0, expr}},
		{spvenums.OpGroupNonUniformBroadcastFirst, []uint32{expr, expr, sub, expr}},
		{spvenums.OpGroupNonUniformBroadcast, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformShuffle, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformShuffleDown, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformShuffleUp, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformShuffleXor, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformQuadBroadcast, []uint32{expr, expr, sub, expr, expr}},
		{spvenums.OpGroupNonUniformQuadSwap, []uint32{expr, expr, sub, expr, expr}},
	}
	for _, tc := range cases {
		for words := 0; words < len(tc.operands); words++ {
			t.Run(fmt.Sprintf("%s/%d words", tc.op, words), func(t *testing.T) {
				inst := make([]uint32, 0, words+1)
				inst = append(inst, uint32(len(tc.operands)+1)<<16|uint32(tc.op))
				inst = append(inst, tc.operands[:words]...)
				m := newBodyModule()
				_, _, err := Parse(m.buildRaw(inst), Options{})
				if got := requireError(t, err).Message; got != "incomplete data" {
					t.Errorf("message = %q, want incomplete data", got)
				}
			})
		}
	}
}

// TestImageOperandMaskTruncation: an image-operands mask that names an operand
// the input does not provide fails at that operand's read, for every operand
// the IR reads.
func TestImageOperandMaskTruncation(t *testing.T) {
	const expr = 20 // the i32 constant 5: a valid expression id
	cases := []struct {
		name string
		op   spvenums.Op
		mask spvenums.ImageOperands
	}{
		{"bias", spvenums.OpImageSampleImplicitLod, spvenums.ImageOperandsBias},
		{"level", spvenums.OpImageSampleExplicitLod, spvenums.ImageOperandsLod},
		{"gradient", spvenums.OpImageSampleExplicitLod, spvenums.ImageOperandsGrad},
		{"const offset", spvenums.OpImageSampleImplicitLod, spvenums.ImageOperandsConstOffset},
		{"fetch sample", spvenums.OpImageFetch, spvenums.ImageOperandsSample},
		{"read level", spvenums.OpImageRead, spvenums.ImageOperandsLod},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The instruction claims room for the mask and its operand; the
			// module ends right after the mask word.
			inst := []uint32{
				7<<16 | uint32(tc.op),
				expr, expr, expr, expr, uint32(tc.mask),
			}
			m := newBodyModule()
			_, _, err := Parse(m.buildRaw(inst), Options{})
			if got := requireError(t, err).Message; got != "incomplete data" {
				t.Errorf("message = %q, want incomplete data", got)
			}
		})
	}
}

// TestImageAtomicSubgroupOperandCounts: the image, atomic and subgroup
// instructions refuse a word count they do not handle before reading any
// operand, as Rust naga's Instruction::expect does.
func TestImageAtomicSubgroupOperandCounts(t *testing.T) {
	cases := []struct {
		inst []uint32
		want string
	}{
		// The instruction's own word count is consistent with the words
		// given; it is the count the handler does not accept.
		{instWords(spvenums.OpSampledImage, 1, 2, 3), "invalid operand count 4 for SampledImage"},
		{instWords(spvenums.OpImage, 1, 2), "invalid operand count 3 for Image"},
		{instWords(spvenums.OpImageWrite, 1, 2), "invalid operand count 3 for ImageWrite"},
		{instWords(spvenums.OpImageFetch, 1, 2, 3), "invalid operand count 4 for ImageFetch"},
		{instWords(spvenums.OpImageRead, 1, 2, 3), "invalid operand count 4 for ImageRead"},
		{instWords(spvenums.OpImageSampleImplicitLod, 1, 2, 3), "invalid operand count 4 for ImageSampleImplicitLod"},
		{instWords(spvenums.OpImageSampleDrefImplicitLod, 1, 2, 3, 4), "invalid operand count 5 for ImageSampleDrefImplicitLod"},
		{instWords(spvenums.OpImageSampleProjDrefExplicitLod, 1, 2, 3, 4), "invalid operand count 5 for ImageSampleProjDrefExplicitLod"},
		{instWords(spvenums.OpImageGather, 1, 2, 3, 4), "invalid operand count 5 for ImageGather"},
		{instWords(spvenums.OpImageDrefGather, 1, 2, 3, 4), "invalid operand count 5 for ImageDrefGather"},
		{instWords(spvenums.OpImageQuerySize, 1, 2, 3, 4), "invalid operand count 5 for ImageQuerySize"},
		{instWords(spvenums.OpImageQuerySizeLod, 1, 2, 3), "invalid operand count 4 for ImageQuerySizeLod"},
		{instWords(spvenums.OpImageQueryLevels, 1, 2, 3, 4), "invalid operand count 5 for ImageQueryLevels"},
		{instWords(spvenums.OpAtomicLoad, 1, 2, 3, 4), "invalid operand count 5 for AtomicLoad"},
		{instWords(spvenums.OpAtomicStore, 1, 2, 3, 4, 5), "invalid operand count 6 for AtomicStore"},
		{instWords(spvenums.OpAtomicIIncrement, 1, 2, 3, 4), "invalid operand count 5 for AtomicIIncrement"},
		{instWords(spvenums.OpAtomicIAdd, 1, 2, 3, 4, 5), "invalid operand count 6 for AtomicIAdd"},
		{instWords(spvenums.OpAtomicCompareExchange, 1, 2, 3, 4, 5, 6, 7), "invalid operand count 8 for AtomicCompareExchange"},
		{instWords(spvenums.OpGroupNonUniformBallot, 1, 2, 3), "invalid operand count 4 for GroupNonUniformBallot"},
		{instWords(spvenums.OpGroupNonUniformAll, 1, 2, 3), "invalid operand count 4 for GroupNonUniformAll"},
		{instWords(spvenums.OpGroupNonUniformIAdd, 1, 2, 3, 4), "invalid operand count 5 for GroupNonUniformIAdd"},
		{instWords(spvenums.OpGroupNonUniformBroadcastFirst, 1, 2, 3), "invalid operand count 4 for GroupNonUniformBroadcastFirst"},
		{instWords(spvenums.OpGroupNonUniformShuffle, 1, 2, 3, 4), "invalid operand count 5 for GroupNonUniformShuffle"},
		{instWords(spvenums.OpGroupNonUniformQuadSwap, 1, 2, 3, 4), "invalid operand count 5 for GroupNonUniformQuadSwap"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			m := newBodyModule()
			_, _, err := Parse(m.build(tc.inst), Options{})
			if got := requireError(t, err).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}
