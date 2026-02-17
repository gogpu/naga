// spvdis - SPIR-V disassembler
// Generates valid .spvasm text format
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

var opcodeNames = map[uint16]string{
	0: "OpNop", 1: "OpUndef", 2: "OpSourceContinued", 3: "OpSource",
	4: "OpSourceExtension", 5: "OpName", 6: "OpMemberName", 7: "OpString",
	10: "OpExtension", 11: "OpExtInstImport", 12: "OpExtInst",
	14: "OpMemoryModel", 15: "OpEntryPoint", 16: "OpExecutionMode",
	17: "OpCapability", 19: "OpTypeVoid", 20: "OpTypeBool",
	21: "OpTypeInt", 22: "OpTypeFloat", 23: "OpTypeVector",
	24: "OpTypeMatrix", 25: "OpTypeImage", 26: "OpTypeSampler",
	27: "OpTypeSampledImage", 28: "OpTypeArray", 29: "OpTypeRuntimeArray",
	30: "OpTypeStruct", 31: "OpTypeOpaque", 32: "OpTypePointer",
	33: "OpTypeFunction", 41: "OpConstantTrue", 42: "OpConstantFalse",
	43: "OpConstant", 44: "OpConstantComposite", 45: "OpConstantSampler",
	46: "OpConstantNull", 48: "OpSpecConstantTrue", 49: "OpSpecConstantFalse",
	50: "OpSpecConstant", 51: "OpSpecConstantComposite", 52: "OpSpecConstantOp",
	54: "OpFunction", 55: "OpFunctionParameter", 56: "OpFunctionEnd",
	57: "OpFunctionCall", 59: "OpVariable", 60: "OpImageTexelPointer",
	61: "OpLoad", 62: "OpStore", 63: "OpCopyMemory", 64: "OpCopyMemorySized",
	65: "OpAccessChain", 66: "OpInBoundsAccessChain", 67: "OpPtrAccessChain",
	68: "OpArrayLength", 69: "OpGenericPtrMemSemantics",
	70: "OpInBoundsPtrAccessChain", 71: "OpDecorate", 72: "OpMemberDecorate",
	73: "OpDecorationGroup", 74: "OpGroupDecorate", 75: "OpGroupMemberDecorate",
	77: "OpVectorExtractDynamic", 78: "OpVectorInsertDynamic",
	79: "OpVectorShuffle", 80: "OpCompositeConstruct", 81: "OpCompositeExtract",
	82: "OpCompositeInsert", 83: "OpCopyObject", 84: "OpTranspose",
	86: "OpSampledImage", 87: "OpImageSampleImplicitLod",
	88: "OpImageSampleExplicitLod", 89: "OpImageSampleDrefImplicitLod",
	90: "OpImageSampleDrefExplicitLod", 91: "OpImageSampleProjImplicitLod",
	92: "OpImageSampleProjExplicitLod", 93: "OpImageSampleProjDrefImplicitLod",
	94: "OpImageSampleProjDrefExplicitLod", 95: "OpImageFetch",
	96: "OpImageGather", 97: "OpImageDrefGather", 98: "OpImageRead",
	99: "OpImageWrite", 100: "OpImage", 101: "OpImageQueryFormat",
	102: "OpImageQueryOrder", 103: "OpImageQuerySizeLod", 104: "OpImageQuerySize",
	105: "OpImageQueryLod", 106: "OpImageQueryLevels", 107: "OpImageQuerySamples",
	109: "OpConvertFToU", 110: "OpConvertFToS", 111: "OpConvertSToF",
	112: "OpConvertUToF", 113: "OpUConvert", 114: "OpSConvert",
	115: "OpFConvert", 116: "OpQuantizeToF16", 117: "OpConvertPtrToU",
	118: "OpSatConvertSToU", 119: "OpSatConvertUToS", 120: "OpConvertUToPtr",
	121: "OpPtrCastToGeneric", 122: "OpGenericCastToPtr",
	123: "OpGenericCastToPtrExplicit", 124: "OpBitcast",
	126: "OpSNegate", 127: "OpFNegate", 128: "OpIAdd", 129: "OpFAdd",
	130: "OpISub", 131: "OpFSub", 132: "OpIMul", 133: "OpFMul",
	134: "OpUDiv", 135: "OpSDiv", 136: "OpFDiv", 137: "OpUMod",
	138: "OpSRem", 139: "OpSMod", 140: "OpFRem", 141: "OpFMod",
	142: "OpVectorTimesScalar", 143: "OpMatrixTimesScalar",
	144: "OpVectorTimesMatrix", 145: "OpMatrixTimesVector",
	146: "OpMatrixTimesMatrix", 147: "OpOuterProduct", 148: "OpDot",
	149: "OpIAddCarry", 150: "OpISubBorrow", 151: "OpUMulExtended",
	152: "OpSMulExtended", 164: "OpAny", 165: "OpAll",
	166: "OpIsNan", 167: "OpIsInf", 168: "OpIsFinite", 169: "OpIsNormal",
	170: "OpSignBitSet", 171: "OpLessOrGreater", 172: "OpOrdered",
	173: "OpUnordered", 174: "OpLogicalEqual", 175: "OpLogicalNotEqual",
	176: "OpLogicalOr", 177: "OpLogicalAnd", 178: "OpLogicalNot",
	179: "OpSelect", 180: "OpIEqual", 181: "OpINotEqual",
	182: "OpUGreaterThan", 183: "OpSGreaterThan", 184: "OpUGreaterThanEqual",
	185: "OpSGreaterThanEqual", 186: "OpULessThan", 187: "OpSLessThan",
	188: "OpULessThanEqual", 189: "OpSLessThanEqual",
	190: "OpFOrdEqual", 191: "OpFUnordEqual", 192: "OpFOrdNotEqual",
	193: "OpFUnordNotEqual", 194: "OpShiftRightLogical", 195: "OpShiftRightArithmetic",
	196: "OpShiftLeftLogical", 197: "OpBitwiseOr", 198: "OpBitwiseXor",
	199: "OpBitwiseAnd", 200: "OpNot", 201: "OpBitFieldInsert",
	202: "OpBitFieldSExtract", 203: "OpBitFieldUExtract",
	204: "OpBitReverse", 205: "OpBitCount",
	245: "OpPhi", 246: "OpLoopMerge", 247: "OpSelectionMerge",
	248: "OpLabel", 249: "OpBranch", 250: "OpBranchConditional",
	251: "OpSwitch", 252: "OpKill", 253: "OpReturn", 254: "OpReturnValue",
	255: "OpUnreachable", 256: "OpLifetimeStart", 257: "OpLifetimeStop",
}

var capabilities = map[uint32]string{
	0: "Matrix", 1: "Shader", 2: "Geometry", 3: "Tessellation",
	4: "Addresses", 5: "Linkage", 6: "Kernel", 7: "Vector16",
	8: "Float16Buffer", 9: "Float16", 10: "Float64", 11: "Int64",
	12: "Int64Atomics", 13: "ImageBasic", 14: "ImageReadWrite", 15: "ImageMipmap",
	17: "Pipes", 18: "Groups", 19: "DeviceEnqueue", 20: "LiteralSampler",
	21: "AtomicStorage", 22: "Int16", 23: "TessellationPointSize",
	24: "GeometryPointSize", 25: "ImageGatherExtended", 26: "StorageImageMultisample",
	27: "UniformBufferArrayDynamicIndexing", 28: "SampledImageArrayDynamicIndexing",
	29: "StorageBufferArrayDynamicIndexing", 30: "StorageImageArrayDynamicIndexing",
	31: "ClipDistance", 32: "CullDistance", 33: "ImageCubeArray",
	34: "SampleRateShading", 35: "ImageRect", 36: "SampledRect",
	37: "GenericPointer", 38: "Int8", 39: "InputAttachment",
	40: "SparseResidency", 41: "MinLod", 42: "Sampled1D", 43: "Image1D",
	44: "SampledCubeArray", 45: "SampledBuffer", 46: "ImageBuffer",
	47: "ImageMSArray", 48: "StorageImageExtendedFormats",
	49: "ImageQuery", 50: "DerivativeControl", 51: "InterpolationFunction",
	52: "TransformFeedback", 53: "GeometryStreams", 54: "StorageImageReadWithoutFormat",
	55: "StorageImageWriteWithoutFormat", 56: "MultiViewport",
	57: "SubgroupDispatch", 58: "NamedBarrier", 59: "PipeStorage",
	60: "GroupNonUniform", 61: "GroupNonUniformVote", 62: "GroupNonUniformArithmetic",
	63: "GroupNonUniformBallot", 64: "GroupNonUniformShuffle",
	65: "GroupNonUniformShuffleRelative", 66: "GroupNonUniformClustered",
	67: "GroupNonUniformQuad", 4423: "SubgroupBallotKHR", 4427: "DrawParameters",
	4437: "StorageBuffer16BitAccess", 4438: "UniformAndStorageBuffer16BitAccess",
	4439: "StoragePushConstant16", 4440: "StorageInputOutput16",
	4441: "DeviceGroup", 4442: "MultiView", 4445: "VariablePointersStorageBuffer",
	4446: "VariablePointers", 5009: "StencilExportEXT", 5010: "SampleMaskPostDepthCoverage",
	5013: "ShaderNonUniform", 5015: "RuntimeDescriptorArray",
	5016: "InputAttachmentArrayDynamicIndexing", 5017: "UniformTexelBufferArrayDynamicIndexing",
	5018: "StorageTexelBufferArrayDynamicIndexing", 5019: "UniformBufferArrayNonUniformIndexing",
}

var storageClasses = map[uint32]string{
	0: "UniformConstant", 1: "Input", 2: "Uniform", 3: "Output",
	4: "Workgroup", 5: "CrossWorkgroup", 6: "Private", 7: "Function",
	8: "Generic", 9: "PushConstant", 10: "AtomicCounter", 11: "Image",
	12: "StorageBuffer",
}

var decorations = map[uint32]string{
	0: "RelaxedPrecision", 1: "SpecId", 2: "Block", 3: "BufferBlock",
	4: "RowMajor", 5: "ColMajor", 6: "ArrayStride", 7: "MatrixStride",
	8: "GLSLShared", 9: "GLSLPacked", 10: "CPacked", 11: "BuiltIn",
	13: "NoPerspective", 14: "Flat", 15: "Patch", 16: "Centroid",
	17: "Sample", 18: "Invariant", 19: "Restrict", 20: "Aliased",
	21: "Volatile", 22: "Constant", 23: "Coherent", 24: "NonWritable",
	25: "NonReadable", 26: "Uniform", 28: "SaturatedConversion",
	29: "Stream", 30: "Location", 31: "Component", 32: "Index",
	33: "Binding", 34: "DescriptorSet", 35: "Offset", 36: "XfbBuffer",
	37: "XfbStride", 38: "FuncParamAttr", 39: "FPRoundingMode",
	40: "FPFastMathMode", 41: "LinkageAttributes", 42: "NoContraction",
	43: "InputAttachmentIndex", 44: "Alignment",
}

var builtins = map[uint32]string{
	0: "Position", 1: "PointSize", 2: "ClipDistance", 3: "CullDistance",
	4: "VertexId", 5: "InstanceId", 6: "PrimitiveId", 7: "InvocationId",
	8: "Layer", 9: "ViewportIndex", 10: "TessLevelOuter", 11: "TessLevelInner",
	12: "TessCoord", 13: "PatchVertices", 14: "FragCoord", 15: "PointCoord",
	16: "FrontFacing", 17: "SampleId", 18: "SamplePosition", 19: "SampleMask",
	22: "FragDepth", 23: "HelperInvocation", 24: "NumWorkgroups",
	25: "WorkgroupSize", 26: "WorkgroupId", 27: "LocalInvocationId",
	28: "GlobalInvocationId", 29: "LocalInvocationIndex",
	30: "WorkDim", 31: "GlobalSize", 32: "EnqueuedWorkgroupSize",
	33: "GlobalOffset", 34: "GlobalLinearId", 36: "SubgroupSize",
	37: "SubgroupMaxSize", 38: "NumSubgroups", 39: "NumEnqueuedSubgroups",
	40: "SubgroupId", 41: "SubgroupLocalInvocationId",
	42: "VertexIndex", 43: "InstanceIndex",
}

var executionModes = map[uint32]string{
	0: "Invocations", 1: "SpacingEqual", 2: "SpacingFractionalEven",
	3: "SpacingFractionalOdd", 4: "VertexOrderCw", 5: "VertexOrderCcw",
	6: "PixelCenterInteger", 7: "OriginUpperLeft", 8: "OriginLowerLeft",
	9: "EarlyFragmentTests", 10: "PointMode", 11: "Xfb", 12: "DepthReplacing",
	14: "DepthGreater", 15: "DepthLess", 16: "DepthUnchanged",
	17: "LocalSize", 18: "LocalSizeHint", 19: "InputPoints", 20: "InputLines",
	21: "InputLinesAdjacency", 22: "Triangles", 23: "InputTrianglesAdjacency",
	24: "Quads", 25: "Isolines", 26: "OutputVertices", 27: "OutputPoints",
	28: "OutputLineStrip", 29: "OutputTriangleStrip", 30: "VecTypeHint",
	31: "ContractionOff", 33: "Initializer", 34: "Finalizer",
	35: "SubgroupSize", 36: "SubgroupsPerWorkgroup",
}

var executionModels = map[uint32]string{
	0: "Vertex", 1: "TessellationControl", 2: "TessellationEvaluation",
	3: "Geometry", 4: "Fragment", 5: "GLCompute", 6: "Kernel",
}

var dims = map[uint32]string{
	0: "1D", 1: "2D", 2: "3D", 3: "Cube", 4: "Rect", 5: "Buffer", 6: "SubpassData",
}

func readString(data []byte, offset int, maxWords int) (string, int) {
	var sb strings.Builder
	words := 0
	for i := 0; i < maxWords*4; i++ {
		if offset+i >= len(data) {
			break
		}
		b := data[offset+i]
		if b == 0 {
			words = (i / 4) + 1
			break
		}
		sb.WriteByte(b)
	}
	return sb.String(), words
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: spvdis <file.spv>")
		return
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(data) < 20 {
		fmt.Fprintln(os.Stderr, "Error: file too small")
		os.Exit(1)
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x07230203 {
		fmt.Fprintf(os.Stderr, "Error: invalid SPIR-V magic: 0x%08X\n", magic)
		os.Exit(1)
	}

	version := binary.LittleEndian.Uint32(data[4:8])
	bound := binary.LittleEndian.Uint32(data[12:16])

	fmt.Printf("; SPIR-V\n")
	fmt.Printf("; Version: %d.%d\n", (version>>16)&0xFF, (version>>8)&0xFF)
	fmt.Printf("; Generator: 0x%08X\n", binary.LittleEndian.Uint32(data[8:12]))
	fmt.Printf("; Bound: %d\n", bound)
	fmt.Printf("; Schema: %d\n", binary.LittleEndian.Uint32(data[16:20]))
	fmt.Println()

	offset := 20
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		word := binary.LittleEndian.Uint32(data[offset:])
		opcode := uint16(word & 0xFFFF)
		wordCount := int(word >> 16)

		if wordCount == 0 || offset+wordCount*4 > len(data) {
			fmt.Fprintf(os.Stderr, "; ERROR: invalid word count %d at offset 0x%X\n", wordCount, offset)
			break
		}

		ops := make([]uint32, wordCount-1)
		for i := range ops {
			ops[i] = binary.LittleEndian.Uint32(data[offset+4+i*4:])
		}

		name := opcodeNames[opcode]
		if name == "" {
			name = fmt.Sprintf("Op%d", opcode)
		}

		printInstruction(name, opcode, ops, data, offset)
		offset += wordCount * 4
	}
}

func id(n uint32) string {
	return fmt.Sprintf("%%_%d", n)
}

func lookup(m map[uint32]string, v uint32) string {
	if s, ok := m[v]; ok {
		return s
	}
	return fmt.Sprintf("%d", v)
}

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // dev tool: switch cases for SPIR-V opcodes
func printInstruction(name string, opcode uint16, ops []uint32, data []byte, offset int) {
	switch opcode {
	case 17: // OpCapability
		fmt.Printf("               %s %s\n", name, lookup(capabilities, ops[0]))

	case 11: // OpExtInstImport
		str, _ := readString(data, offset+8, len(ops)-1)
		fmt.Printf("         %s = %s \"%s\"\n", id(ops[0]), name, str)

	case 14: // OpMemoryModel
		addrModels := map[uint32]string{0: "Logical", 1: "Physical32", 2: "Physical64", 5348: "PhysicalStorageBuffer64"}
		memModels := map[uint32]string{0: "Simple", 1: "GLSL450", 2: "OpenCL", 3: "Vulkan"}
		a, m := lookup(addrModels, ops[0]), lookup(memModels, ops[1])
		fmt.Printf("               %s %s %s\n", name, a, m)

	case 15: // OpEntryPoint
		model := lookup(executionModels, ops[0])
		str, strWords := readString(data, offset+12, len(ops)-2)
		fmt.Printf("               %s %s %s \"%s\"", name, model, id(ops[1]), str)
		for i := 2 + strWords; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 16: // OpExecutionMode
		mode := lookup(executionModes, ops[1])
		fmt.Printf("               %s %s %s", name, id(ops[0]), mode)
		for i := 2; i < len(ops); i++ {
			fmt.Printf(" %d", ops[i])
		}
		fmt.Println()

	case 5: // OpName
		str, _ := readString(data, offset+8, len(ops)-1)
		fmt.Printf("               %s %s \"%s\"\n", name, id(ops[0]), str)

	case 6: // OpMemberName
		str, _ := readString(data, offset+12, len(ops)-2)
		fmt.Printf("               %s %s %d \"%s\"\n", name, id(ops[0]), ops[1], str)

	case 71: // OpDecorate
		dec := lookup(decorations, ops[1])
		fmt.Printf("               %s %s %s", name, id(ops[0]), dec)
		if ops[1] == 11 && len(ops) > 2 { // BuiltIn
			fmt.Printf(" %s", lookup(builtins, ops[2]))
		} else {
			for i := 2; i < len(ops); i++ {
				fmt.Printf(" %d", ops[i])
			}
		}
		fmt.Println()

	case 72: // OpMemberDecorate
		dec := lookup(decorations, ops[2])
		fmt.Printf("               %s %s %d %s", name, id(ops[0]), ops[1], dec)
		for i := 3; i < len(ops); i++ {
			fmt.Printf(" %d", ops[i])
		}
		fmt.Println()

	case 19: // OpTypeVoid
		fmt.Printf("         %s = %s\n", id(ops[0]), name)

	case 20: // OpTypeBool
		fmt.Printf("         %s = %s\n", id(ops[0]), name)

	case 21: // OpTypeInt
		sign := "0"
		if ops[2] == 1 {
			sign = "1"
		}
		fmt.Printf("         %s = %s %d %s\n", id(ops[0]), name, ops[1], sign)

	case 22: // OpTypeFloat
		fmt.Printf("         %s = %s %d\n", id(ops[0]), name, ops[1])

	case 23: // OpTypeVector
		fmt.Printf("         %s = %s %s %d\n", id(ops[0]), name, id(ops[1]), ops[2])

	case 24: // OpTypeMatrix
		fmt.Printf("         %s = %s %s %d\n", id(ops[0]), name, id(ops[1]), ops[2])

	case 25: // OpTypeImage
		dim := lookup(dims, ops[2])
		// Format: OpTypeImage Result Sampled-Type Dim Depth Arrayed MS Sampled Image-Format [Access-Qualifier]
		// Access Qualifier is only present when Sampled=0 or Sampled=2
		fmt.Printf("         %s = %s %s %s %d %d %d %d Unknown", id(ops[0]), name, id(ops[1]), dim, ops[3], ops[4], ops[5], ops[6])
		// Only output Access Qualifier if Sampled != 1 and we have the extra operand
		if ops[6] != 1 && len(ops) > 7 {
			fmt.Printf(" %d", ops[7])
		}
		fmt.Println()

	case 26: // OpTypeSampler
		fmt.Printf("         %s = %s\n", id(ops[0]), name)

	case 27: // OpTypeSampledImage
		fmt.Printf("         %s = %s %s\n", id(ops[0]), name, id(ops[1]))

	case 28: // OpTypeArray
		fmt.Printf("         %s = %s %s %s\n", id(ops[0]), name, id(ops[1]), id(ops[2]))

	case 30: // OpTypeStruct
		fmt.Printf("         %s = %s", id(ops[0]), name)
		for i := 1; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 32: // OpTypePointer
		sc := lookup(storageClasses, ops[1])
		fmt.Printf("         %s = %s %s %s\n", id(ops[0]), name, sc, id(ops[2]))

	case 33: // OpTypeFunction
		fmt.Printf("         %s = %s %s", id(ops[0]), name, id(ops[1]))
		for i := 2; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 43: // OpConstant
		fmt.Printf("         %s = %s %s %d\n", id(ops[1]), name, id(ops[0]), ops[2])

	case 44: // OpConstantComposite
		fmt.Printf("         %s = %s %s", id(ops[1]), name, id(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 54: // OpFunction
		fmt.Printf("         %s = %s %s None %s\n", id(ops[1]), name, id(ops[0]), id(ops[3]))

	case 55: // OpFunctionParameter
		fmt.Printf("         %s = %s %s\n", id(ops[1]), name, id(ops[0]))

	case 56: // OpFunctionEnd
		fmt.Printf("               %s\n", name)

	case 59: // OpVariable
		sc := lookup(storageClasses, ops[2])
		fmt.Printf("         %s = %s %s %s\n", id(ops[1]), name, id(ops[0]), sc)

	case 61: // OpLoad
		fmt.Printf("         %s = %s %s %s\n", id(ops[1]), name, id(ops[0]), id(ops[2]))

	case 62: // OpStore
		fmt.Printf("               %s %s %s\n", name, id(ops[0]), id(ops[1]))

	case 65: // OpAccessChain
		fmt.Printf("         %s = %s %s %s", id(ops[1]), name, id(ops[0]), id(ops[2]))
		for i := 3; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 80: // OpCompositeConstruct
		fmt.Printf("         %s = %s %s", id(ops[1]), name, id(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
		fmt.Println()

	case 81: // OpCompositeExtract
		fmt.Printf("         %s = %s %s %s", id(ops[1]), name, id(ops[0]), id(ops[2]))
		for i := 3; i < len(ops); i++ {
			fmt.Printf(" %d", ops[i])
		}
		fmt.Println()

	case 79: // OpVectorShuffle
		fmt.Printf("         %s = %s %s %s %s", id(ops[1]), name, id(ops[0]), id(ops[2]), id(ops[3]))
		for i := 4; i < len(ops); i++ {
			fmt.Printf(" %d", ops[i])
		}
		fmt.Println()

	case 86: // OpSampledImage
		fmt.Printf("         %s = %s %s %s %s\n", id(ops[1]), name, id(ops[0]), id(ops[2]), id(ops[3]))

	case 87: // OpImageSampleImplicitLod
		fmt.Printf("         %s = %s %s %s %s\n", id(ops[1]), name, id(ops[0]), id(ops[2]), id(ops[3]))

	case 248: // OpLabel
		fmt.Printf("         %s = %s\n", id(ops[0]), name)

	case 249: // OpBranch
		fmt.Printf("               %s %s\n", name, id(ops[0]))

	case 253: // OpReturn
		fmt.Printf("               %s\n", name)

	case 254: // OpReturnValue
		fmt.Printf("               %s %s\n", name, id(ops[0]))

	default:
		// Generic fallback
		printGenericInstruction(name, opcode, ops)
	}
}

func printGenericInstruction(name string, opcode uint16, ops []uint32) {
	fmt.Printf("         ")
	switch {
	case len(ops) >= 2 && opcode >= 126 && opcode <= 200:
		// Arithmetic/logic ops: type result operands...
		fmt.Printf("%s = %s %s", id(ops[1]), name, id(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Printf(" %s", id(ops[i]))
		}
	case len(ops) >= 1:
		fmt.Printf("%s", name)
		for _, op := range ops {
			fmt.Printf(" %s", id(op))
		}
	default:
		fmt.Printf("%s", name)
	}
	fmt.Println()
}
