// Package spirv provides SPIR-V code generation from naga IR.
//
// SPIR-V is the standard intermediate language for GPU shaders,
// used by Vulkan, OpenCL, and other APIs.
package spirv

import "github.com/gogpu/naga/ir"

// Version represents a SPIR-V version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common SPIR-V versions
var (
	Version1_0 = Version{1, 0}
	Version1_3 = Version{1, 3}
	Version1_4 = Version{1, 4}
	Version1_5 = Version{1, 5}
	Version1_6 = Version{1, 6}
)

// Options configures SPIR-V generation.
type Options struct {
	// Version is the SPIR-V version to target
	Version Version

	// Capabilities are additional capabilities to declare
	Capabilities []Capability

	// Debug includes debug information
	Debug bool

	// Validation enables output validation
	Validation bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Version:    Version1_3,
		Debug:      false,
		Validation: true,
	}
}

// Capability represents a SPIR-V capability.
type Capability uint32

// Common capabilities
const (
	CapabilityMatrix  Capability = 0 // Implied by Shader
	CapabilityShader  Capability = 1
	CapabilityFloat16 Capability = 9  // Required for OpTypeFloat 16
	CapabilityFloat64 Capability = 10 // Required for OpTypeFloat 64
	CapabilityInt64   Capability = 11 // Required for OpTypeInt 64
	CapabilityInt16   Capability = 22 // Required for OpTypeInt 16
	CapabilityInt8    Capability = 39 // Required for OpTypeInt 8
)

// Writer generates SPIR-V from IR.
type Writer struct {
	options Options

	// Internal state
	nextID      uint32
	typeIDs     map[uint32]uint32
	constantIDs map[uint32]uint32
}

// NewWriter creates a new SPIR-V writer.
func NewWriter(options Options) *Writer {
	return &Writer{
		options:     options,
		nextID:      1,
		typeIDs:     make(map[uint32]uint32),
		constantIDs: make(map[uint32]uint32),
	}
}

// SPIR-V magic number and constants
const (
	MagicNumber = 0x07230203
	GeneratorID = 0x00000000 // Unregistered generator
)

// OpCode represents a SPIR-V opcode.
type OpCode uint16

// Common opcodes
const (
	OpNop               OpCode = 0
	OpSource            OpCode = 3
	OpString            OpCode = 7
	OpName              OpCode = 5
	OpMemberName        OpCode = 6
	OpExtInstImport     OpCode = 11
	OpMemoryModel       OpCode = 14
	OpEntryPoint        OpCode = 15
	OpExecutionMode     OpCode = 16
	OpCapability        OpCode = 17
	OpTypeVoid          OpCode = 19
	OpTypeBool          OpCode = 20
	OpTypeInt           OpCode = 21
	OpTypeFloat         OpCode = 22
	OpTypeVector        OpCode = 23
	OpTypeMatrix        OpCode = 24
	OpTypeArray         OpCode = 28
	OpTypeRuntimeArray  OpCode = 29
	OpTypeStruct        OpCode = 30
	OpTypePointer       OpCode = 32
	OpTypeFunction      OpCode = 33
	OpConstant          OpCode = 43
	OpConstantComposite OpCode = 44
	OpConstantNull      OpCode = 46
	OpFunction          OpCode = 54
	OpFunctionParameter OpCode = 55
	OpFunctionEnd       OpCode = 56
	OpVariable          OpCode = 59
	OpLoad              OpCode = 61
	OpStore             OpCode = 62
	OpAccessChain       OpCode = 65
	OpDecorate          OpCode = 71
	OpMemberDecorate    OpCode = 72
	OpLabel             OpCode = 248
	OpBranch            OpCode = 249
	OpReturn            OpCode = 253
	OpReturnValue       OpCode = 254
	OpUnreachable       OpCode = 255
)

// Decoration represents a SPIR-V decoration.
type Decoration uint32

// Common decorations
const (
	DecorationBlock         Decoration = 2
	DecorationColMajor      Decoration = 5
	DecorationRowMajor      Decoration = 4
	DecorationArrayStride   Decoration = 6
	DecorationMatrixStride  Decoration = 7
	DecorationBuiltIn       Decoration = 11
	DecorationLocation      Decoration = 30
	DecorationBinding       Decoration = 33
	DecorationDescriptorSet Decoration = 34
	DecorationOffset        Decoration = 35
)

// BuiltIn represents a SPIR-V built-in decoration value.
type BuiltIn uint32

// SPIR-V built-in values (used with DecorationBuiltIn).
const (
	BuiltInPosition             BuiltIn = 0
	BuiltInPointSize            BuiltIn = 1
	BuiltInClipDistance         BuiltIn = 3
	BuiltInCullDistance         BuiltIn = 4
	BuiltInVertexID             BuiltIn = 5
	BuiltInInstanceID           BuiltIn = 6
	BuiltInPrimitiveID          BuiltIn = 7
	BuiltInInvocationID         BuiltIn = 8
	BuiltInLayer                BuiltIn = 9
	BuiltInViewportIndex        BuiltIn = 10
	BuiltInTessLevelOuter       BuiltIn = 11
	BuiltInTessLevelInner       BuiltIn = 12
	BuiltInTessCoord            BuiltIn = 13
	BuiltInPatchVertices        BuiltIn = 14
	BuiltInFragCoord            BuiltIn = 15
	BuiltInPointCoord           BuiltIn = 16
	BuiltInFrontFacing          BuiltIn = 17
	BuiltInSampleID             BuiltIn = 18
	BuiltInSamplePosition       BuiltIn = 19
	BuiltInSampleMask           BuiltIn = 20
	BuiltInFragDepth            BuiltIn = 22
	BuiltInHelperInvocation     BuiltIn = 23
	BuiltInNumWorkgroups        BuiltIn = 24
	BuiltInWorkgroupSize        BuiltIn = 25
	BuiltInWorkgroupID          BuiltIn = 26
	BuiltInLocalInvocationID    BuiltIn = 27
	BuiltInGlobalInvocationID   BuiltIn = 28
	BuiltInLocalInvocationIndex BuiltIn = 29
	BuiltInVertexIndex          BuiltIn = 42
	BuiltInInstanceIndex        BuiltIn = 43
)

// ExecutionModel represents a SPIR-V execution model.
type ExecutionModel uint32

// Common execution models
const (
	ExecutionModelVertex                 ExecutionModel = 0
	ExecutionModelTessellationControl    ExecutionModel = 1
	ExecutionModelTessellationEvaluation ExecutionModel = 2
	ExecutionModelGeometry               ExecutionModel = 3
	ExecutionModelFragment               ExecutionModel = 4
	ExecutionModelGLCompute              ExecutionModel = 5
	ExecutionModelKernel                 ExecutionModel = 6
)

// ExecutionMode represents a SPIR-V execution mode.
type ExecutionMode uint32

// Common execution modes
const (
	ExecutionModeInvocations              ExecutionMode = 0
	ExecutionModeSpacingEqual             ExecutionMode = 1
	ExecutionModeSpacingFractionalEven    ExecutionMode = 2
	ExecutionModeSpacingFractionalOdd     ExecutionMode = 3
	ExecutionModeVertexOrderCw            ExecutionMode = 4
	ExecutionModeVertexOrderCcw           ExecutionMode = 5
	ExecutionModePixelCenterInteger       ExecutionMode = 6
	ExecutionModeOriginUpperLeft          ExecutionMode = 7
	ExecutionModeOriginLowerLeft          ExecutionMode = 8
	ExecutionModeEarlyFragmentTests       ExecutionMode = 9
	ExecutionModePointMode                ExecutionMode = 10
	ExecutionModeXfb                      ExecutionMode = 11
	ExecutionModeDepthReplacing           ExecutionMode = 12
	ExecutionModeDepthGreater             ExecutionMode = 14
	ExecutionModeDepthLess                ExecutionMode = 15
	ExecutionModeDepthUnchanged           ExecutionMode = 16
	ExecutionModeLocalSize                ExecutionMode = 17
	ExecutionModeLocalSizeHint            ExecutionMode = 18
	ExecutionModeInputPoints              ExecutionMode = 19
	ExecutionModeInputLines               ExecutionMode = 20
	ExecutionModeInputLinesAdjacency      ExecutionMode = 21
	ExecutionModeTriangles                ExecutionMode = 22
	ExecutionModeInputTrianglesAdjacency  ExecutionMode = 23
	ExecutionModeQuads                    ExecutionMode = 24
	ExecutionModeIsolines                 ExecutionMode = 25
	ExecutionModeOutputVertices           ExecutionMode = 26
	ExecutionModeOutputPoints             ExecutionMode = 27
	ExecutionModeOutputLineStrip          ExecutionMode = 28
	ExecutionModeOutputTriangleStrip      ExecutionMode = 29
	ExecutionModeVecTypeHint              ExecutionMode = 30
	ExecutionModeContractionOff           ExecutionMode = 31
	ExecutionModeInitializer              ExecutionMode = 33
	ExecutionModeFinalizer                ExecutionMode = 34
	ExecutionModeSubgroupSize             ExecutionMode = 35
	ExecutionModeSubgroupsPerWorkgroup    ExecutionMode = 36
	ExecutionModeSubgroupsPerWorkgroupID  ExecutionMode = 37
	ExecutionModeLocalSizeID              ExecutionMode = 38
	ExecutionModeLocalSizeHintID          ExecutionMode = 39
	ExecutionModePostDepthCoverage        ExecutionMode = 4446
	ExecutionModeDenormPreserve           ExecutionMode = 4459
	ExecutionModeDenormFlushToZero        ExecutionMode = 4460
	ExecutionModeSignedZeroInfNanPreserve ExecutionMode = 4461
	ExecutionModeRoundingModeRTE          ExecutionMode = 4462
	ExecutionModeRoundingModeRTZ          ExecutionMode = 4463
)

// StorageClass represents a SPIR-V storage class.
type StorageClass uint32

// Common storage classes
const (
	StorageClassUniformConstant StorageClass = 0
	StorageClassInput           StorageClass = 1
	StorageClassUniform         StorageClass = 2
	StorageClassOutput          StorageClass = 3
	StorageClassWorkgroup       StorageClass = 4
	StorageClassCrossWorkgroup  StorageClass = 5
	StorageClassPrivate         StorageClass = 6
	StorageClassFunction        StorageClass = 7
	StorageClassGeneric         StorageClass = 8
	StorageClassPushConstant    StorageClass = 9
	StorageClassAtomicCounter   StorageClass = 10
	StorageClassImage           StorageClass = 11
	StorageClassStorageBuffer   StorageClass = 12
)

// AddressingModel represents a SPIR-V addressing model.
type AddressingModel uint32

// Common addressing models
const (
	AddressingModelLogical    AddressingModel = 0
	AddressingModelPhysical32 AddressingModel = 1
	AddressingModelPhysical64 AddressingModel = 2
)

// MemoryModel represents a SPIR-V memory model.
type MemoryModel uint32

// Common memory models
const (
	MemoryModelSimple  MemoryModel = 0
	MemoryModelGLSL450 MemoryModel = 1
	MemoryModelOpenCL  MemoryModel = 2
	MemoryModelVulkan  MemoryModel = 3
)

// FunctionControl represents a SPIR-V function control.
type FunctionControl uint32

// Common function control flags
const (
	FunctionControlNone       FunctionControl = 0x0
	FunctionControlInline     FunctionControl = 0x1
	FunctionControlDontInline FunctionControl = 0x2
	FunctionControlPure       FunctionControl = 0x4
	FunctionControlConst      FunctionControl = 0x8
)

// OpExtension represents OpExtension opcode.
const OpExtension OpCode = 10

// Arithmetic opcodes
const (
	OpFNegate OpCode = 127 // Float negation
	OpSNegate OpCode = 126 // Signed integer negation
	OpFAdd    OpCode = 129 // Float addition
	OpFSub    OpCode = 131 // Float subtraction
	OpFMul    OpCode = 133 // Float multiplication
	OpFDiv    OpCode = 136 // Float division
	OpFMod    OpCode = 141 // Float modulo
	OpIAdd    OpCode = 128 // Integer addition
	OpISub    OpCode = 130 // Integer subtraction
	OpIMul    OpCode = 132 // Integer multiplication
	OpSDiv    OpCode = 135 // Signed integer division
	OpUDiv    OpCode = 137 // Unsigned integer division
	OpSMod    OpCode = 139 // Signed integer modulo
	OpUMod    OpCode = 142 // Unsigned integer modulo
)

// Comparison opcodes
const (
	OpFOrdEqual            OpCode = 180 // Float ordered equal
	OpFOrdNotEqual         OpCode = 182 // Float ordered not equal
	OpFOrdLessThan         OpCode = 184 // Float ordered less than
	OpFOrdGreaterThan      OpCode = 186 // Float ordered greater than
	OpFOrdLessThanEqual    OpCode = 188 // Float ordered less than or equal
	OpFOrdGreaterThanEqual OpCode = 190 // Float ordered greater than or equal
	OpIEqual               OpCode = 170 // Integer equal
	OpINotEqual            OpCode = 171 // Integer not equal
	OpSLessThan            OpCode = 177 // Signed integer less than
	OpSLessThanEqual       OpCode = 179 // Signed integer less than or equal
	OpSGreaterThan         OpCode = 173 // Signed integer greater than
	OpSGreaterThanEqual    OpCode = 175 // Signed integer greater than or equal
	OpULessThan            OpCode = 176 // Unsigned integer less than
	OpULessThanEqual       OpCode = 178 // Unsigned integer less than or equal
	OpUGreaterThan         OpCode = 172 // Unsigned integer greater than
	OpUGreaterThanEqual    OpCode = 174 // Unsigned integer greater than or equal
)

// Logical opcodes
const (
	OpLogicalEqual    OpCode = 164 // Logical equal
	OpLogicalNotEqual OpCode = 165 // Logical not equal
	OpLogicalOr       OpCode = 166 // Logical or
	OpLogicalAnd      OpCode = 167 // Logical and
	OpLogicalNot      OpCode = 168 // Logical not
	OpSelect          OpCode = 169 // Select between values
	OpNot             OpCode = 200 // Bitwise not
)

// Composite opcodes
const (
	OpVectorExtractDynamic OpCode = 77 // Extract from vector with dynamic index
	OpVectorShuffle        OpCode = 79 // Shuffle vector components
	OpCompositeConstruct   OpCode = 80 // Construct composite
	OpCompositeExtract     OpCode = 81 // Extract from composite
)

// Bitwise opcodes
const (
	OpShiftRightLogical    OpCode = 194 // Shift right logical
	OpShiftRightArithmetic OpCode = 195 // Shift right arithmetic
	OpShiftLeftLogical     OpCode = 196 // Shift left logical
	OpBitwiseOr            OpCode = 197 // Bitwise OR
	OpBitwiseXor           OpCode = 198 // Bitwise XOR
	OpBitwiseAnd           OpCode = 199 // Bitwise AND
)

// Control flow opcodes
const (
	OpSelectionMerge    OpCode = 247 // Selection merge point
	OpLoopMerge         OpCode = 246 // Loop merge point
	OpBranchConditional OpCode = 250 // Conditional branch
	OpSwitch            OpCode = 251 // Switch statement
	OpKill              OpCode = 252 // Fragment discard
)

// Derivative opcodes
const (
	OpDPdx         OpCode = 207 // Derivative in X
	OpDPdy         OpCode = 208 // Derivative in Y
	OpFwidth       OpCode = 209 // Sum of absolute derivatives
	OpDPdxFine     OpCode = 210 // Fine derivative in X
	OpDPdyFine     OpCode = 211 // Fine derivative in Y
	OpFwidthFine   OpCode = 212 // Fine fwidth
	OpDPdxCoarse   OpCode = 213 // Coarse derivative in X
	OpDPdyCoarse   OpCode = 214 // Coarse derivative in Y
	OpFwidthCoarse OpCode = 215 // Coarse fwidth
)

// Conversion opcodes
const (
	OpConvertFToU OpCode = 109 // Float to unsigned int
	OpConvertFToS OpCode = 110 // Float to signed int
	OpConvertSToF OpCode = 111 // Signed int to float
	OpConvertUToF OpCode = 112 // Unsigned int to float
	OpBitcast     OpCode = 124 // Bitcast between types of same width
)

// Extended instruction set opcodes
const (
	OpExtInst OpCode = 12 // Extended instruction
)

// Atomic operation opcodes
const (
	OpAtomicLoad        OpCode = 227 // Atomic load
	OpAtomicStore       OpCode = 228 // Atomic store
	OpAtomicExchange    OpCode = 229 // Atomic exchange
	OpAtomicCompareExch OpCode = 230 // Atomic compare-exchange
	OpAtomicIIncrement  OpCode = 232 // Atomic integer increment
	OpAtomicIDecrement  OpCode = 233 // Atomic integer decrement
	OpAtomicIAdd        OpCode = 234 // Atomic integer add
	OpAtomicISub        OpCode = 235 // Atomic integer subtract
	OpAtomicSMin        OpCode = 236 // Atomic signed min
	OpAtomicUMin        OpCode = 237 // Atomic unsigned min
	OpAtomicSMax        OpCode = 238 // Atomic signed max
	OpAtomicUMax        OpCode = 239 // Atomic unsigned max
	OpAtomicAnd         OpCode = 240 // Atomic bitwise and
	OpAtomicOr          OpCode = 241 // Atomic bitwise or
	OpAtomicXor         OpCode = 242 // Atomic bitwise xor
)

// Memory scope for atomic operations
const (
	ScopeDevice    uint32 = 1 // Visible to all invocations in the device
	ScopeWorkgroup uint32 = 2 // Visible to all invocations in the workgroup
)

// Memory semantics for atomic operations
const (
	MemorySemanticsNone                uint32 = 0x0
	MemorySemanticsAcquire             uint32 = 0x2
	MemorySemanticsRelease             uint32 = 0x4
	MemorySemanticsAcquireRelease      uint32 = 0x8
	MemorySemanticsUniformMemory       uint32 = 0x40
	MemorySemanticsWorkgroupMemory     uint32 = 0x100
	MemorySemanticsImageMemory         uint32 = 0x800
	MemorySemanticsAtomicCounterMemory uint32 = 0x400
)

// Barrier opcodes
const (
	OpControlBarrier OpCode = 224 // Control barrier (execution + memory)
	OpMemoryBarrier  OpCode = 225 // Memory barrier only
)

// SelectionControl flags for OpSelectionMerge
type SelectionControl uint32

const (
	SelectionControlNone        SelectionControl = 0x0
	SelectionControlFlatten     SelectionControl = 0x1
	SelectionControlDontFlatten SelectionControl = 0x2
)

// LoopControl flags for OpLoopMerge
type LoopControl uint32

const (
	LoopControlNone               LoopControl = 0x0
	LoopControlUnroll             LoopControl = 0x1
	LoopControlDontUnroll         LoopControl = 0x2
	LoopControlDependencyInfinite LoopControl = 0x4
	LoopControlDependencyLength   LoopControl = 0x8
	LoopControlMinIterations      LoopControl = 0x10
	LoopControlMaxIterations      LoopControl = 0x20
	LoopControlIterationMultiple  LoopControl = 0x40
	LoopControlPeelCount          LoopControl = 0x80
	LoopControlPartialCount       LoopControl = 0x100
)

// ImageFormat represents a SPIR-V image format.
type ImageFormat uint32

// SPIR-V image format values (for OpTypeImage)
const (
	ImageFormatUnknown      ImageFormat = 0
	ImageFormatRgba32f      ImageFormat = 1
	ImageFormatRgba16f      ImageFormat = 2
	ImageFormatR32f         ImageFormat = 3
	ImageFormatRgba8        ImageFormat = 4
	ImageFormatRgba8Snorm   ImageFormat = 5
	ImageFormatRg32f        ImageFormat = 6
	ImageFormatRg16f        ImageFormat = 7
	ImageFormatR11fG11fB10f ImageFormat = 8
	ImageFormatR16f         ImageFormat = 9
	ImageFormatRgba16       ImageFormat = 10
	ImageFormatRgb10A2      ImageFormat = 11
	ImageFormatRg16         ImageFormat = 12
	ImageFormatRg8          ImageFormat = 13
	ImageFormatR16          ImageFormat = 14
	ImageFormatR8           ImageFormat = 15
	ImageFormatRgba16Snorm  ImageFormat = 16
	ImageFormatRg16Snorm    ImageFormat = 17
	ImageFormatRg8Snorm     ImageFormat = 18
	ImageFormatR16Snorm     ImageFormat = 19
	ImageFormatR8Snorm      ImageFormat = 20
	ImageFormatRgba32i      ImageFormat = 21
	ImageFormatRgba16i      ImageFormat = 22
	ImageFormatRgba8i       ImageFormat = 23
	ImageFormatR32i         ImageFormat = 24
	ImageFormatRg32i        ImageFormat = 25
	ImageFormatRg16i        ImageFormat = 26
	ImageFormatRg8i         ImageFormat = 27
	ImageFormatR16i         ImageFormat = 28
	ImageFormatR8i          ImageFormat = 29
	ImageFormatRgba32ui     ImageFormat = 30
	ImageFormatRgba16ui     ImageFormat = 31
	ImageFormatRgba8ui      ImageFormat = 32
	ImageFormatR32ui        ImageFormat = 33
	ImageFormatRgb10a2ui    ImageFormat = 34
	ImageFormatRg32ui       ImageFormat = 35
	ImageFormatRg16ui       ImageFormat = 36
	ImageFormatRg8ui        ImageFormat = 37
	ImageFormatR16ui        ImageFormat = 38
	ImageFormatR8ui         ImageFormat = 39
)

// GLSL.std.450 extended instruction set constants
const (
	GLSLstd450Round                 uint32 = 1
	GLSLstd450RoundEven             uint32 = 2
	GLSLstd450Trunc                 uint32 = 3
	GLSLstd450FAbs                  uint32 = 4
	GLSLstd450SAbs                  uint32 = 5
	GLSLstd450FSign                 uint32 = 6
	GLSLstd450SSign                 uint32 = 7
	GLSLstd450Floor                 uint32 = 8
	GLSLstd450Ceil                  uint32 = 9
	GLSLstd450Fract                 uint32 = 10
	GLSLstd450Radians               uint32 = 11
	GLSLstd450Degrees               uint32 = 12
	GLSLstd450Sin                   uint32 = 13
	GLSLstd450Cos                   uint32 = 14
	GLSLstd450Tan                   uint32 = 15
	GLSLstd450Asin                  uint32 = 16
	GLSLstd450Acos                  uint32 = 17
	GLSLstd450Atan                  uint32 = 18
	GLSLstd450Sinh                  uint32 = 19
	GLSLstd450Cosh                  uint32 = 20
	GLSLstd450Tanh                  uint32 = 21
	GLSLstd450Asinh                 uint32 = 22
	GLSLstd450Acosh                 uint32 = 23
	GLSLstd450Atanh                 uint32 = 24
	GLSLstd450Atan2                 uint32 = 25
	GLSLstd450Pow                   uint32 = 26
	GLSLstd450Exp                   uint32 = 27
	GLSLstd450Log                   uint32 = 28
	GLSLstd450Exp2                  uint32 = 29
	GLSLstd450Log2                  uint32 = 30
	GLSLstd450Sqrt                  uint32 = 31
	GLSLstd450InverseSqrt           uint32 = 32
	GLSLstd450Determinant           uint32 = 33
	GLSLstd450MatrixInverse         uint32 = 34
	GLSLstd450Modf                  uint32 = 35
	GLSLstd450ModfStruct            uint32 = 36
	GLSLstd450FMin                  uint32 = 37
	GLSLstd450UMin                  uint32 = 38
	GLSLstd450SMin                  uint32 = 39
	GLSLstd450FMax                  uint32 = 40
	GLSLstd450UMax                  uint32 = 41
	GLSLstd450SMax                  uint32 = 42
	GLSLstd450FClamp                uint32 = 43
	GLSLstd450UClamp                uint32 = 44
	GLSLstd450SClamp                uint32 = 45
	GLSLstd450FMix                  uint32 = 46
	GLSLstd450IMix                  uint32 = 47
	GLSLstd450Step                  uint32 = 48
	GLSLstd450SmoothStep            uint32 = 49
	GLSLstd450Fma                   uint32 = 50
	GLSLstd450Frexp                 uint32 = 51
	GLSLstd450FrexpStruct           uint32 = 52
	GLSLstd450Ldexp                 uint32 = 53
	GLSLstd450PackSnorm4x8          uint32 = 54
	GLSLstd450PackUnorm4x8          uint32 = 55
	GLSLstd450PackSnorm2x16         uint32 = 56
	GLSLstd450PackUnorm2x16         uint32 = 57
	GLSLstd450PackHalf2x16          uint32 = 58
	GLSLstd450PackDouble2x32        uint32 = 59
	GLSLstd450UnpackSnorm2x16       uint32 = 60
	GLSLstd450UnpackUnorm2x16       uint32 = 61
	GLSLstd450UnpackHalf2x16        uint32 = 62
	GLSLstd450UnpackSnorm4x8        uint32 = 63
	GLSLstd450UnpackUnorm4x8        uint32 = 64
	GLSLstd450UnpackDouble2x32      uint32 = 65
	GLSLstd450Length                uint32 = 66
	GLSLstd450Distance              uint32 = 67
	GLSLstd450Cross                 uint32 = 68
	GLSLstd450Normalize             uint32 = 69
	GLSLstd450FaceForward           uint32 = 70
	GLSLstd450Reflect               uint32 = 71
	GLSLstd450Refract               uint32 = 72
	GLSLstd450FindILsb              uint32 = 73
	GLSLstd450FindSMsb              uint32 = 74
	GLSLstd450FindUMsb              uint32 = 75
	GLSLstd450InterpolateAtCentroid uint32 = 76
	GLSLstd450InterpolateAtSample   uint32 = 77
	GLSLstd450InterpolateAtOffset   uint32 = 78
	GLSLstd450NMin                  uint32 = 79
	GLSLstd450NMax                  uint32 = 80
	GLSLstd450NClamp                uint32 = 81
)

// StorageFormatToImageFormat converts an IR storage format to a SPIR-V image format.
//
//nolint:gocyclo,cyclop,funlen // Large switch for exhaustive format mapping is inherently complex
func StorageFormatToImageFormat(format ir.StorageFormat) ImageFormat {
	switch format {
	// 8-bit formats
	case ir.StorageFormatR8Unorm:
		return ImageFormatR8
	case ir.StorageFormatR8Snorm:
		return ImageFormatR8Snorm
	case ir.StorageFormatR8Uint:
		return ImageFormatR8ui
	case ir.StorageFormatR8Sint:
		return ImageFormatR8i

	// 16-bit formats
	case ir.StorageFormatR16Uint:
		return ImageFormatR16ui
	case ir.StorageFormatR16Sint:
		return ImageFormatR16i
	case ir.StorageFormatR16Float:
		return ImageFormatR16f
	case ir.StorageFormatRg8Unorm:
		return ImageFormatRg8
	case ir.StorageFormatRg8Snorm:
		return ImageFormatRg8Snorm
	case ir.StorageFormatRg8Uint:
		return ImageFormatRg8ui
	case ir.StorageFormatRg8Sint:
		return ImageFormatRg8i

	// 32-bit formats
	case ir.StorageFormatR32Uint:
		return ImageFormatR32ui
	case ir.StorageFormatR32Sint:
		return ImageFormatR32i
	case ir.StorageFormatR32Float:
		return ImageFormatR32f
	case ir.StorageFormatRg16Uint:
		return ImageFormatRg16ui
	case ir.StorageFormatRg16Sint:
		return ImageFormatRg16i
	case ir.StorageFormatRg16Float:
		return ImageFormatRg16f
	case ir.StorageFormatRgba8Unorm:
		return ImageFormatRgba8
	case ir.StorageFormatRgba8Snorm:
		return ImageFormatRgba8Snorm
	case ir.StorageFormatRgba8Uint:
		return ImageFormatRgba8ui
	case ir.StorageFormatRgba8Sint:
		return ImageFormatRgba8i
	case ir.StorageFormatBgra8Unorm:
		return ImageFormatRgba8 // BGRA not directly supported, use RGBA

	// Packed 32-bit formats
	case ir.StorageFormatRgb10a2Uint:
		return ImageFormatRgb10a2ui
	case ir.StorageFormatRgb10a2Unorm:
		return ImageFormatRgb10A2
	case ir.StorageFormatRg11b10Ufloat:
		return ImageFormatR11fG11fB10f

	// 64-bit formats
	case ir.StorageFormatRg32Uint:
		return ImageFormatRg32ui
	case ir.StorageFormatRg32Sint:
		return ImageFormatRg32i
	case ir.StorageFormatRg32Float:
		return ImageFormatRg32f
	case ir.StorageFormatRgba16Uint:
		return ImageFormatRgba16ui
	case ir.StorageFormatRgba16Sint:
		return ImageFormatRgba16i
	case ir.StorageFormatRgba16Float:
		return ImageFormatRgba16f

	// 128-bit formats
	case ir.StorageFormatRgba32Uint:
		return ImageFormatRgba32ui
	case ir.StorageFormatRgba32Sint:
		return ImageFormatRgba32i
	case ir.StorageFormatRgba32Float:
		return ImageFormatRgba32f

	// Normalized 16-bit per channel formats
	case ir.StorageFormatR16Unorm:
		return ImageFormatR16
	case ir.StorageFormatR16Snorm:
		return ImageFormatR16Snorm
	case ir.StorageFormatRg16Unorm:
		return ImageFormatRg16
	case ir.StorageFormatRg16Snorm:
		return ImageFormatRg16Snorm
	case ir.StorageFormatRgba16Unorm:
		return ImageFormatRgba16
	case ir.StorageFormatRgba16Snorm:
		return ImageFormatRgba16Snorm

	default:
		return ImageFormatUnknown
	}
}
