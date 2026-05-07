// Package spirv provides SPIR-V code generation from naga IR.
//
// This package exposes the public API for SPIR-V compilation.
// All implementation details live in spirv/internal/codegen.
package spirv

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/codegen"
)

// --- Configuration types ---

// Version represents a SPIR-V version.
type Version = codegen.Version

// Common SPIR-V versions.
var (
	Version1_0 = codegen.Version1_0
	Version1_1 = codegen.Version1_1
	Version1_2 = codegen.Version1_2
	Version1_3 = codegen.Version1_3
	Version1_4 = codegen.Version1_4
	Version1_5 = codegen.Version1_5
	Version1_6 = codegen.Version1_6
)

// Options configures SPIR-V generation.
type Options = codegen.Options

// BoundsCheckPolicy controls how out-of-bounds resource accesses are handled.
type BoundsCheckPolicy = codegen.BoundsCheckPolicy

// BoundsCheckPolicy values.
const (
	BoundsCheckUnchecked         = codegen.BoundsCheckUnchecked
	BoundsCheckRestrict          = codegen.BoundsCheckRestrict
	BoundsCheckReadZeroSkipWrite = codegen.BoundsCheckReadZeroSkipWrite
)

// BoundsCheckPolicies holds per-resource-type bounds check policies.
type BoundsCheckPolicies = codegen.BoundsCheckPolicies

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return codegen.DefaultOptions()
}

// --- Capability ---

// Capability represents a SPIR-V capability.
type Capability = codegen.Capability

// Common capabilities.
const (
	CapabilityMatrix                             = codegen.CapabilityMatrix
	CapabilityShader                             = codegen.CapabilityShader
	CapabilityFloat16                            = codegen.CapabilityFloat16
	CapabilityFloat64                            = codegen.CapabilityFloat64
	CapabilityInt64                              = codegen.CapabilityInt64
	CapabilityInt16                              = codegen.CapabilityInt16
	CapabilityImageGatherExtended                = codegen.CapabilityImageGatherExtended
	CapabilityInt8                               = codegen.CapabilityInt8
	CapabilityLinkage                            = codegen.CapabilityLinkage
	CapabilityInt64Atomics                       = codegen.CapabilityInt64Atomics
	CapabilityClipDistance                       = codegen.CapabilityClipDistance
	CapabilityImageCubeArray                     = codegen.CapabilityImageCubeArray
	CapabilitySampleRateShading                  = codegen.CapabilitySampleRateShading
	CapabilitySampled1D                          = codegen.CapabilitySampled1D
	CapabilityImage1D                            = codegen.CapabilityImage1D
	CapabilitySampledCubeArray                   = codegen.CapabilitySampledCubeArray
	CapabilityStorageImageExtendedFormats        = codegen.CapabilityStorageImageExtendedFormats
	CapabilityImageQuery                         = codegen.CapabilityImageQuery
	CapabilityDerivativeControl                  = codegen.CapabilityDerivativeControl
	CapabilityStorageBuffer16BitAccess           = codegen.CapabilityStorageBuffer16BitAccess
	CapabilityUniformAndStorageBuffer16BitAccess = codegen.CapabilityUniformAndStorageBuffer16BitAccess
	CapabilityStorageInputOutput16               = codegen.CapabilityStorageInputOutput16
	CapabilityMultiView                          = codegen.CapabilityMultiView
	CapabilityFragmentBarycentricKHR             = codegen.CapabilityFragmentBarycentricKHR
	CapabilityShaderNonUniform                   = codegen.CapabilityShaderNonUniform
	CapabilityAtomicFloat32AddEXT                = codegen.CapabilityAtomicFloat32AddEXT
	CapabilityDotProductInput4x8BitPacked        = codegen.CapabilityDotProductInput4x8BitPacked
	CapabilityDotProduct                         = codegen.CapabilityDotProduct
	CapabilityGroupNonUniform                    = codegen.CapabilityGroupNonUniform
	CapabilityGroupNonUniformVote                = codegen.CapabilityGroupNonUniformVote
	CapabilityGroupNonUniformArithmetic          = codegen.CapabilityGroupNonUniformArithmetic
	CapabilityGroupNonUniformBallot              = codegen.CapabilityGroupNonUniformBallot
	CapabilityGroupNonUniformShuffle             = codegen.CapabilityGroupNonUniformShuffle
	CapabilityGroupNonUniformShuffleRel          = codegen.CapabilityGroupNonUniformShuffleRel
	CapabilityGroupNonUniformQuad                = codegen.CapabilityGroupNonUniformQuad
	CapabilityGeometry                           = codegen.CapabilityGeometry
	CapabilitySubgroupBallotKHR                  = codegen.CapabilitySubgroupBallotKHR
	CapabilityInt64ImageEXT                      = codegen.CapabilityInt64ImageEXT
)

// --- Backend (high-level IR → SPIR-V) ---

// Backend translates IR to SPIR-V.
type Backend = codegen.Backend

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return codegen.NewBackend(options)
}

// --- Low-level builder types (used by examples and advanced consumers) ---

// ModuleBuilder builds complete SPIR-V modules.
type ModuleBuilder = codegen.ModuleBuilder

// NewModuleBuilder creates a new SPIR-V module builder.
func NewModuleBuilder(version Version) *ModuleBuilder {
	return codegen.NewModuleBuilder(version)
}

// Instruction represents a SPIR-V instruction.
type Instruction = codegen.Instruction

// InstructionBuilder builds SPIR-V instructions.
type InstructionBuilder = codegen.InstructionBuilder

// NewInstructionBuilder creates a new instruction builder.
func NewInstructionBuilder() *InstructionBuilder {
	return codegen.NewInstructionBuilder()
}

// Writer generates SPIR-V from IR.
type Writer = codegen.Writer

// NewWriter creates a new SPIR-V writer.
func NewWriter(options Options) *Writer {
	return codegen.NewWriter(options)
}

// --- Block model types ---

// Block represents a SPIR-V basic block under construction.
type Block = codegen.Block

// NewBlock creates a new block with the given label ID and an empty body.
func NewBlock(labelID uint32) Block {
	return codegen.NewBlock(labelID)
}

// TerminatedBlock is a finalized basic block.
type TerminatedBlock = codegen.TerminatedBlock

// FunctionBuilder collects terminated blocks for a single SPIR-V function.
type FunctionBuilder = codegen.FunctionBuilder

// ExpressionEmitter handles expression and statement emission.
type ExpressionEmitter = codegen.ExpressionEmitter

// BlockExitKind specifies how a block should be terminated.
type BlockExitKind = codegen.BlockExitKind

// BlockExitKind values.
const (
	BlockExitReturn  = codegen.BlockExitReturn
	BlockExitBranch  = codegen.BlockExitBranch
	BlockExitBreakIf = codegen.BlockExitBreakIf
)

// BlockExit specifies how a block should end.
type BlockExit = codegen.BlockExit

// BlockExitDisposition indicates whether writeBlock consumed the provided exit.
type BlockExitDisposition = codegen.BlockExitDisposition

// BlockExitDisposition values.
const (
	ExitUsed      = codegen.ExitUsed
	ExitDiscarded = codegen.ExitDiscarded
)

// LoopContext provides break/continue targets for loop bodies.
type LoopContext = codegen.LoopContext

// --- SPIR-V spec constants and types ---

// OpCode represents a SPIR-V opcode.
type OpCode = codegen.OpCode

// Decoration represents a SPIR-V decoration.
type Decoration = codegen.Decoration

// BuiltIn represents a SPIR-V built-in decoration value.
type BuiltIn = codegen.BuiltIn

// ExecutionModel represents a SPIR-V execution model.
type ExecutionModel = codegen.ExecutionModel

// ExecutionMode represents a SPIR-V execution mode.
type ExecutionMode = codegen.ExecutionMode

// StorageClass represents a SPIR-V storage class.
type StorageClass = codegen.StorageClass

// AddressingModel represents a SPIR-V addressing model.
type AddressingModel = codegen.AddressingModel

// MemoryModel represents a SPIR-V memory model.
type MemoryModel = codegen.MemoryModel

// FunctionControl represents a SPIR-V function control.
type FunctionControl = codegen.FunctionControl

// SelectionControl flags for OpSelectionMerge.
type SelectionControl = codegen.SelectionControl

// LoopControl flags for OpLoopMerge.
type LoopControl = codegen.LoopControl

// ImageFormat represents a SPIR-V image format.
type ImageFormat = codegen.ImageFormat

// SPIR-V magic number and constants.
const (
	MagicNumber = codegen.MagicNumber
	GeneratorID = codegen.GeneratorID
)

// --- Opcodes ---

// Common opcodes.
const (
	OpNop               = codegen.OpNop
	OpSource            = codegen.OpSource
	OpString            = codegen.OpString
	OpName              = codegen.OpName
	OpMemberName        = codegen.OpMemberName
	OpExtInstImport     = codegen.OpExtInstImport
	OpMemoryModel       = codegen.OpMemoryModel
	OpEntryPoint        = codegen.OpEntryPoint
	OpExecutionMode     = codegen.OpExecutionMode
	OpCapability        = codegen.OpCapability
	OpTypeVoid          = codegen.OpTypeVoid
	OpTypeBool          = codegen.OpTypeBool
	OpTypeInt           = codegen.OpTypeInt
	OpTypeFloat         = codegen.OpTypeFloat
	OpTypeVector        = codegen.OpTypeVector
	OpTypeMatrix        = codegen.OpTypeMatrix
	OpTypeArray         = codegen.OpTypeArray
	OpTypeRuntimeArray  = codegen.OpTypeRuntimeArray
	OpTypeStruct        = codegen.OpTypeStruct
	OpTypePointer       = codegen.OpTypePointer
	OpTypeFunction      = codegen.OpTypeFunction
	OpConstant          = codegen.OpConstant
	OpConstantComposite = codegen.OpConstantComposite
	OpConstantNull      = codegen.OpConstantNull
	OpFunction          = codegen.OpFunction
	OpFunctionParameter = codegen.OpFunctionParameter
	OpFunctionEnd       = codegen.OpFunctionEnd
	OpFunctionCall      = codegen.OpFunctionCall
	OpVariable          = codegen.OpVariable
	OpLoad              = codegen.OpLoad
	OpStore             = codegen.OpStore
	OpAccessChain       = codegen.OpAccessChain
	OpDecorate          = codegen.OpDecorate
	OpMemberDecorate    = codegen.OpMemberDecorate
	OpLabel             = codegen.OpLabel
	OpBranch            = codegen.OpBranch
	OpPhi               = codegen.OpPhi
	OpReturn            = codegen.OpReturn
	OpReturnValue       = codegen.OpReturnValue
	OpUnreachable       = codegen.OpUnreachable
	OpExtension         = codegen.OpExtension
)

// Arithmetic opcodes.
const (
	OpFNegate           = codegen.OpFNegate
	OpSNegate           = codegen.OpSNegate
	OpFAdd              = codegen.OpFAdd
	OpFSub              = codegen.OpFSub
	OpFMul              = codegen.OpFMul
	OpUDiv              = codegen.OpUDiv
	OpSDiv              = codegen.OpSDiv
	OpFDiv              = codegen.OpFDiv
	OpUMod              = codegen.OpUMod
	OpSRem              = codegen.OpSRem
	OpSMod              = codegen.OpSMod
	OpFRem              = codegen.OpFRem
	OpFMod              = codegen.OpFMod
	OpVectorTimesScalar = codegen.OpVectorTimesScalar
	OpMatrixTimesScalar = codegen.OpMatrixTimesScalar
	OpVectorTimesMatrix = codegen.OpVectorTimesMatrix
	OpMatrixTimesVector = codegen.OpMatrixTimesVector
	OpMatrixTimesMatrix = codegen.OpMatrixTimesMatrix
	OpIAdd              = codegen.OpIAdd
	OpISub              = codegen.OpISub
	OpIMul              = codegen.OpIMul
)

// Comparison opcodes.
const (
	OpFOrdEqual            = codegen.OpFOrdEqual
	OpFOrdNotEqual         = codegen.OpFOrdNotEqual
	OpFOrdLessThan         = codegen.OpFOrdLessThan
	OpFOrdGreaterThan      = codegen.OpFOrdGreaterThan
	OpFOrdLessThanEqual    = codegen.OpFOrdLessThanEqual
	OpFOrdGreaterThanEqual = codegen.OpFOrdGreaterThanEqual
	OpIEqual               = codegen.OpIEqual
	OpINotEqual            = codegen.OpINotEqual
	OpSLessThan            = codegen.OpSLessThan
	OpSLessThanEqual       = codegen.OpSLessThanEqual
	OpSGreaterThan         = codegen.OpSGreaterThan
	OpSGreaterThanEqual    = codegen.OpSGreaterThanEqual
	OpULessThan            = codegen.OpULessThan
	OpULessThanEqual       = codegen.OpULessThanEqual
	OpUGreaterThan         = codegen.OpUGreaterThan
	OpUGreaterThanEqual    = codegen.OpUGreaterThanEqual
)

// Logical opcodes.
const (
	OpLogicalEqual    = codegen.OpLogicalEqual
	OpLogicalNotEqual = codegen.OpLogicalNotEqual
	OpLogicalOr       = codegen.OpLogicalOr
	OpLogicalAnd      = codegen.OpLogicalAnd
	OpLogicalNot      = codegen.OpLogicalNot
	OpSelect          = codegen.OpSelect
	OpNot             = codegen.OpNot
	OpAny             = codegen.OpAny
	OpAll             = codegen.OpAll
	OpIsNan           = codegen.OpIsNan
	OpIsInf           = codegen.OpIsInf
)

// Composite opcodes.
const (
	OpVectorExtractDynamic = codegen.OpVectorExtractDynamic
	OpVectorShuffle        = codegen.OpVectorShuffle
	OpCompositeConstruct   = codegen.OpCompositeConstruct
	OpCompositeExtract     = codegen.OpCompositeExtract
)

// Bitwise opcodes.
const (
	OpShiftRightLogical    = codegen.OpShiftRightLogical
	OpShiftRightArithmetic = codegen.OpShiftRightArithmetic
	OpShiftLeftLogical     = codegen.OpShiftLeftLogical
	OpBitwiseOr            = codegen.OpBitwiseOr
	OpBitwiseXor           = codegen.OpBitwiseXor
	OpBitwiseAnd           = codegen.OpBitwiseAnd
	OpBitFieldInsert       = codegen.OpBitFieldInsert
	OpBitFieldSExtract     = codegen.OpBitFieldSExtract
	OpBitFieldUExtract     = codegen.OpBitFieldUExtract
	OpBitReverse           = codegen.OpBitReverse
	OpBitCount             = codegen.OpBitCount
)

// Control flow opcodes.
const (
	OpSelectionMerge    = codegen.OpSelectionMerge
	OpLoopMerge         = codegen.OpLoopMerge
	OpBranchConditional = codegen.OpBranchConditional
	OpSwitch            = codegen.OpSwitch
	OpKill              = codegen.OpKill
)

// Derivative opcodes.
const (
	OpDPdx         = codegen.OpDPdx
	OpDPdy         = codegen.OpDPdy
	OpFwidth       = codegen.OpFwidth
	OpDPdxFine     = codegen.OpDPdxFine
	OpDPdyFine     = codegen.OpDPdyFine
	OpFwidthFine   = codegen.OpFwidthFine
	OpDPdxCoarse   = codegen.OpDPdxCoarse
	OpDPdyCoarse   = codegen.OpDPdyCoarse
	OpFwidthCoarse = codegen.OpFwidthCoarse
)

// Conversion opcodes.
const (
	OpConvertFToU   = codegen.OpConvertFToU
	OpConvertFToS   = codegen.OpConvertFToS
	OpConvertSToF   = codegen.OpConvertSToF
	OpConvertUToF   = codegen.OpConvertUToF
	OpUConvert      = codegen.OpUConvert
	OpSConvert      = codegen.OpSConvert
	OpFConvert      = codegen.OpFConvert
	OpQuantizeToF16 = codegen.OpQuantizeToF16
	OpBitcast       = codegen.OpBitcast
)

// Extended instruction set opcodes.
const (
	OpExtInst = codegen.OpExtInst
)

// Atomic operation opcodes.
const (
	OpAtomicLoad        = codegen.OpAtomicLoad
	OpAtomicStore       = codegen.OpAtomicStore
	OpAtomicExchange    = codegen.OpAtomicExchange
	OpAtomicCompareExch = codegen.OpAtomicCompareExch
	OpAtomicIIncrement  = codegen.OpAtomicIIncrement
	OpAtomicIDecrement  = codegen.OpAtomicIDecrement
	OpAtomicIAdd        = codegen.OpAtomicIAdd
	OpAtomicFAddEXT     = codegen.OpAtomicFAddEXT
	OpAtomicISub        = codegen.OpAtomicISub
	OpAtomicSMin        = codegen.OpAtomicSMin
	OpAtomicUMin        = codegen.OpAtomicUMin
	OpAtomicSMax        = codegen.OpAtomicSMax
	OpAtomicUMax        = codegen.OpAtomicUMax
	OpAtomicAnd         = codegen.OpAtomicAnd
	OpAtomicOr          = codegen.OpAtomicOr
	OpAtomicXor         = codegen.OpAtomicXor
)

// Integer dot product opcodes.
const (
	OpSDotKHR = codegen.OpSDotKHR
	OpUDotKHR = codegen.OpUDotKHR

	PackedVectorFormat4x8Bit = codegen.PackedVectorFormat4x8Bit
)

// Memory scope for atomic operations.
const (
	ScopeDevice    = codegen.ScopeDevice
	ScopeWorkgroup = codegen.ScopeWorkgroup
	ScopeSubgroup  = codegen.ScopeSubgroup
)

// Memory semantics for atomic operations.
const (
	MemorySemanticsNone                = codegen.MemorySemanticsNone
	MemorySemanticsAcquire             = codegen.MemorySemanticsAcquire
	MemorySemanticsRelease             = codegen.MemorySemanticsRelease
	MemorySemanticsAcquireRelease      = codegen.MemorySemanticsAcquireRelease
	MemorySemanticsUniformMemory       = codegen.MemorySemanticsUniformMemory
	MemorySemanticsSubgroupMemory      = codegen.MemorySemanticsSubgroupMemory
	MemorySemanticsWorkgroupMemory     = codegen.MemorySemanticsWorkgroupMemory
	MemorySemanticsAtomicCounterMemory = codegen.MemorySemanticsAtomicCounterMemory
	MemorySemanticsImageMemory         = codegen.MemorySemanticsImageMemory
)

// Barrier opcodes.
const (
	OpControlBarrier = codegen.OpControlBarrier
	OpMemoryBarrier  = codegen.OpMemoryBarrier
)

// Subgroup opcodes.
const (
	OpGroupNonUniformElect            = codegen.OpGroupNonUniformElect
	OpGroupNonUniformAll              = codegen.OpGroupNonUniformAll
	OpGroupNonUniformAny              = codegen.OpGroupNonUniformAny
	OpGroupNonUniformAllEqual         = codegen.OpGroupNonUniformAllEqual
	OpGroupNonUniformBroadcast        = codegen.OpGroupNonUniformBroadcast
	OpGroupNonUniformBroadcastFirst   = codegen.OpGroupNonUniformBroadcastFirst
	OpGroupNonUniformBallot           = codegen.OpGroupNonUniformBallot
	OpGroupNonUniformInverseBallot    = codegen.OpGroupNonUniformInverseBallot
	OpGroupNonUniformBallotBitExtract = codegen.OpGroupNonUniformBallotBitExtract
	OpGroupNonUniformBallotBitCount   = codegen.OpGroupNonUniformBallotBitCount
	OpGroupNonUniformBallotFindLSB    = codegen.OpGroupNonUniformBallotFindLSB
	OpGroupNonUniformBallotFindMSB    = codegen.OpGroupNonUniformBallotFindMSB
	OpGroupNonUniformShuffle          = codegen.OpGroupNonUniformShuffle
	OpGroupNonUniformShuffleXor       = codegen.OpGroupNonUniformShuffleXor
	OpGroupNonUniformShuffleUp        = codegen.OpGroupNonUniformShuffleUp
	OpGroupNonUniformShuffleDown      = codegen.OpGroupNonUniformShuffleDown
	OpGroupNonUniformIAdd             = codegen.OpGroupNonUniformIAdd
	OpGroupNonUniformFAdd             = codegen.OpGroupNonUniformFAdd
	OpGroupNonUniformIMul             = codegen.OpGroupNonUniformIMul
	OpGroupNonUniformFMul             = codegen.OpGroupNonUniformFMul
	OpGroupNonUniformSMin             = codegen.OpGroupNonUniformSMin
	OpGroupNonUniformUMin             = codegen.OpGroupNonUniformUMin
	OpGroupNonUniformFMin             = codegen.OpGroupNonUniformFMin
	OpGroupNonUniformSMax             = codegen.OpGroupNonUniformSMax
	OpGroupNonUniformUMax             = codegen.OpGroupNonUniformUMax
	OpGroupNonUniformFMax             = codegen.OpGroupNonUniformFMax
	OpGroupNonUniformBitwiseAnd       = codegen.OpGroupNonUniformBitwiseAnd
	OpGroupNonUniformBitwiseOr        = codegen.OpGroupNonUniformBitwiseOr
	OpGroupNonUniformBitwiseXor       = codegen.OpGroupNonUniformBitwiseXor
	OpGroupNonUniformLogicalAnd       = codegen.OpGroupNonUniformLogicalAnd
	OpGroupNonUniformLogicalOr        = codegen.OpGroupNonUniformLogicalOr
	OpGroupNonUniformLogicalXor       = codegen.OpGroupNonUniformLogicalXor
	OpGroupNonUniformQuadBroadcast    = codegen.OpGroupNonUniformQuadBroadcast
	OpGroupNonUniformQuadSwap         = codegen.OpGroupNonUniformQuadSwap
)

// GroupOperation for subgroup collective operations.
const (
	GroupOperationReduce        = codegen.GroupOperationReduce
	GroupOperationInclusiveScan = codegen.GroupOperationInclusiveScan
	GroupOperationExclusiveScan = codegen.GroupOperationExclusiveScan
)

// --- Decorations ---

const (
	DecorationBlock         = codegen.DecorationBlock
	DecorationColMajor      = codegen.DecorationColMajor
	DecorationRowMajor      = codegen.DecorationRowMajor
	DecorationArrayStride   = codegen.DecorationArrayStride
	DecorationMatrixStride  = codegen.DecorationMatrixStride
	DecorationBuiltIn       = codegen.DecorationBuiltIn
	DecorationFlat          = codegen.DecorationFlat
	DecorationNoPerspective = codegen.DecorationNoPerspective
	DecorationCentroid      = codegen.DecorationCentroid
	DecorationSample        = codegen.DecorationSample
	DecorationNonWritable   = codegen.DecorationNonWritable
	DecorationNonReadable   = codegen.DecorationNonReadable
	DecorationLocation      = codegen.DecorationLocation
	DecorationIndex         = codegen.DecorationIndex
	DecorationBinding       = codegen.DecorationBinding
	DecorationDescriptorSet = codegen.DecorationDescriptorSet
	DecorationOffset        = codegen.DecorationOffset
	DecorationNonUniform    = codegen.DecorationNonUniform
)

// --- BuiltIn values ---

const (
	BuiltInPosition             = codegen.BuiltInPosition
	BuiltInPointSize            = codegen.BuiltInPointSize
	BuiltInClipDistance         = codegen.BuiltInClipDistance
	BuiltInCullDistance         = codegen.BuiltInCullDistance
	BuiltInVertexID             = codegen.BuiltInVertexID
	BuiltInInstanceID           = codegen.BuiltInInstanceID
	BuiltInPrimitiveID          = codegen.BuiltInPrimitiveID
	BuiltInInvocationID         = codegen.BuiltInInvocationID
	BuiltInLayer                = codegen.BuiltInLayer
	BuiltInViewportIndex        = codegen.BuiltInViewportIndex
	BuiltInTessLevelOuter       = codegen.BuiltInTessLevelOuter
	BuiltInTessLevelInner       = codegen.BuiltInTessLevelInner
	BuiltInTessCoord            = codegen.BuiltInTessCoord
	BuiltInPatchVertices        = codegen.BuiltInPatchVertices
	BuiltInFragCoord            = codegen.BuiltInFragCoord
	BuiltInPointCoord           = codegen.BuiltInPointCoord
	BuiltInFrontFacing          = codegen.BuiltInFrontFacing
	BuiltInSampleID             = codegen.BuiltInSampleID
	BuiltInSamplePosition       = codegen.BuiltInSamplePosition
	BuiltInSampleMask           = codegen.BuiltInSampleMask
	BuiltInFragDepth            = codegen.BuiltInFragDepth
	BuiltInHelperInvocation     = codegen.BuiltInHelperInvocation
	BuiltInNumWorkgroups        = codegen.BuiltInNumWorkgroups
	BuiltInWorkgroupSize        = codegen.BuiltInWorkgroupSize
	BuiltInWorkgroupID          = codegen.BuiltInWorkgroupID
	BuiltInLocalInvocationID    = codegen.BuiltInLocalInvocationID
	BuiltInGlobalInvocationID   = codegen.BuiltInGlobalInvocationID
	BuiltInLocalInvocationIndex = codegen.BuiltInLocalInvocationIndex
	BuiltInVertexIndex          = codegen.BuiltInVertexIndex
	BuiltInInstanceIndex        = codegen.BuiltInInstanceIndex
	BuiltInSubgroupSize         = codegen.BuiltInSubgroupSize
	BuiltInSubgroupLocalInvID   = codegen.BuiltInSubgroupLocalInvID
	BuiltInNumSubgroups         = codegen.BuiltInNumSubgroups
	BuiltInSubgroupID           = codegen.BuiltInSubgroupID
	BuiltInViewIndex            = codegen.BuiltInViewIndex
	BuiltInBaryCoordKHR         = codegen.BuiltInBaryCoordKHR
)

// --- Execution models ---

const (
	ExecutionModelVertex                 = codegen.ExecutionModelVertex
	ExecutionModelTessellationControl    = codegen.ExecutionModelTessellationControl
	ExecutionModelTessellationEvaluation = codegen.ExecutionModelTessellationEvaluation
	ExecutionModelGeometry               = codegen.ExecutionModelGeometry
	ExecutionModelFragment               = codegen.ExecutionModelFragment
	ExecutionModelGLCompute              = codegen.ExecutionModelGLCompute
	ExecutionModelKernel                 = codegen.ExecutionModelKernel
)

// --- Execution modes ---

const (
	ExecutionModeInvocations              = codegen.ExecutionModeInvocations
	ExecutionModeSpacingEqual             = codegen.ExecutionModeSpacingEqual
	ExecutionModeSpacingFractionalEven    = codegen.ExecutionModeSpacingFractionalEven
	ExecutionModeSpacingFractionalOdd     = codegen.ExecutionModeSpacingFractionalOdd
	ExecutionModeVertexOrderCw            = codegen.ExecutionModeVertexOrderCw
	ExecutionModeVertexOrderCcw           = codegen.ExecutionModeVertexOrderCcw
	ExecutionModePixelCenterInteger       = codegen.ExecutionModePixelCenterInteger
	ExecutionModeOriginUpperLeft          = codegen.ExecutionModeOriginUpperLeft
	ExecutionModeOriginLowerLeft          = codegen.ExecutionModeOriginLowerLeft
	ExecutionModeEarlyFragmentTests       = codegen.ExecutionModeEarlyFragmentTests
	ExecutionModePointMode                = codegen.ExecutionModePointMode
	ExecutionModeXfb                      = codegen.ExecutionModeXfb
	ExecutionModeDepthReplacing           = codegen.ExecutionModeDepthReplacing
	ExecutionModeDepthGreater             = codegen.ExecutionModeDepthGreater
	ExecutionModeDepthLess                = codegen.ExecutionModeDepthLess
	ExecutionModeDepthUnchanged           = codegen.ExecutionModeDepthUnchanged
	ExecutionModeLocalSize                = codegen.ExecutionModeLocalSize
	ExecutionModeLocalSizeHint            = codegen.ExecutionModeLocalSizeHint
	ExecutionModeInputPoints              = codegen.ExecutionModeInputPoints
	ExecutionModeInputLines               = codegen.ExecutionModeInputLines
	ExecutionModeInputLinesAdjacency      = codegen.ExecutionModeInputLinesAdjacency
	ExecutionModeTriangles                = codegen.ExecutionModeTriangles
	ExecutionModeInputTrianglesAdjacency  = codegen.ExecutionModeInputTrianglesAdjacency
	ExecutionModeQuads                    = codegen.ExecutionModeQuads
	ExecutionModeIsolines                 = codegen.ExecutionModeIsolines
	ExecutionModeOutputVertices           = codegen.ExecutionModeOutputVertices
	ExecutionModeOutputPoints             = codegen.ExecutionModeOutputPoints
	ExecutionModeOutputLineStrip          = codegen.ExecutionModeOutputLineStrip
	ExecutionModeOutputTriangleStrip      = codegen.ExecutionModeOutputTriangleStrip
	ExecutionModeVecTypeHint              = codegen.ExecutionModeVecTypeHint
	ExecutionModeContractionOff           = codegen.ExecutionModeContractionOff
	ExecutionModeInitializer              = codegen.ExecutionModeInitializer
	ExecutionModeFinalizer                = codegen.ExecutionModeFinalizer
	ExecutionModeSubgroupSize             = codegen.ExecutionModeSubgroupSize
	ExecutionModeSubgroupsPerWorkgroup    = codegen.ExecutionModeSubgroupsPerWorkgroup
	ExecutionModeSubgroupsPerWorkgroupID  = codegen.ExecutionModeSubgroupsPerWorkgroupID
	ExecutionModeLocalSizeID              = codegen.ExecutionModeLocalSizeID
	ExecutionModeLocalSizeHintID          = codegen.ExecutionModeLocalSizeHintID
	ExecutionModePostDepthCoverage        = codegen.ExecutionModePostDepthCoverage
	ExecutionModeDenormPreserve           = codegen.ExecutionModeDenormPreserve
	ExecutionModeDenormFlushToZero        = codegen.ExecutionModeDenormFlushToZero
	ExecutionModeSignedZeroInfNanPreserve = codegen.ExecutionModeSignedZeroInfNanPreserve
	ExecutionModeRoundingModeRTE          = codegen.ExecutionModeRoundingModeRTE
	ExecutionModeRoundingModeRTZ          = codegen.ExecutionModeRoundingModeRTZ
)

// --- Storage classes ---

const (
	StorageClassUniformConstant         = codegen.StorageClassUniformConstant
	StorageClassInput                   = codegen.StorageClassInput
	StorageClassUniform                 = codegen.StorageClassUniform
	StorageClassOutput                  = codegen.StorageClassOutput
	StorageClassWorkgroup               = codegen.StorageClassWorkgroup
	StorageClassCrossWorkgroup          = codegen.StorageClassCrossWorkgroup
	StorageClassPrivate                 = codegen.StorageClassPrivate
	StorageClassFunction                = codegen.StorageClassFunction
	StorageClassGeneric                 = codegen.StorageClassGeneric
	StorageClassPushConstant            = codegen.StorageClassPushConstant
	StorageClassAtomicCounter           = codegen.StorageClassAtomicCounter
	StorageClassImage                   = codegen.StorageClassImage
	StorageClassStorageBuffer           = codegen.StorageClassStorageBuffer
	StorageClassTaskPayloadWorkgroupEXT = codegen.StorageClassTaskPayloadWorkgroupEXT
)

// --- Addressing models ---

const (
	AddressingModelLogical    = codegen.AddressingModelLogical
	AddressingModelPhysical32 = codegen.AddressingModelPhysical32
	AddressingModelPhysical64 = codegen.AddressingModelPhysical64
)

// --- Memory models ---

const (
	MemoryModelSimple  = codegen.MemoryModelSimple
	MemoryModelGLSL450 = codegen.MemoryModelGLSL450
	MemoryModelOpenCL  = codegen.MemoryModelOpenCL
	MemoryModelVulkan  = codegen.MemoryModelVulkan
)

// --- Function control ---

const (
	FunctionControlNone       = codegen.FunctionControlNone
	FunctionControlInline     = codegen.FunctionControlInline
	FunctionControlDontInline = codegen.FunctionControlDontInline
	FunctionControlPure       = codegen.FunctionControlPure
	FunctionControlConst      = codegen.FunctionControlConst
)

// --- Selection control ---

const (
	SelectionControlNone        = codegen.SelectionControlNone
	SelectionControlFlatten     = codegen.SelectionControlFlatten
	SelectionControlDontFlatten = codegen.SelectionControlDontFlatten
)

// --- Loop control ---

const (
	LoopControlNone               = codegen.LoopControlNone
	LoopControlUnroll             = codegen.LoopControlUnroll
	LoopControlDontUnroll         = codegen.LoopControlDontUnroll
	LoopControlDependencyInfinite = codegen.LoopControlDependencyInfinite
	LoopControlDependencyLength   = codegen.LoopControlDependencyLength
	LoopControlMinIterations      = codegen.LoopControlMinIterations
	LoopControlMaxIterations      = codegen.LoopControlMaxIterations
	LoopControlIterationMultiple  = codegen.LoopControlIterationMultiple
	LoopControlPeelCount          = codegen.LoopControlPeelCount
	LoopControlPartialCount       = codegen.LoopControlPartialCount
)

// --- Image formats ---

const (
	ImageFormatUnknown      = codegen.ImageFormatUnknown
	ImageFormatRgba32f      = codegen.ImageFormatRgba32f
	ImageFormatRgba16f      = codegen.ImageFormatRgba16f
	ImageFormatR32f         = codegen.ImageFormatR32f
	ImageFormatRgba8        = codegen.ImageFormatRgba8
	ImageFormatRgba8Snorm   = codegen.ImageFormatRgba8Snorm
	ImageFormatRg32f        = codegen.ImageFormatRg32f
	ImageFormatRg16f        = codegen.ImageFormatRg16f
	ImageFormatR11fG11fB10f = codegen.ImageFormatR11fG11fB10f
	ImageFormatR16f         = codegen.ImageFormatR16f
	ImageFormatRgba16       = codegen.ImageFormatRgba16
	ImageFormatRgb10A2      = codegen.ImageFormatRgb10A2
	ImageFormatRg16         = codegen.ImageFormatRg16
	ImageFormatRg8          = codegen.ImageFormatRg8
	ImageFormatR16          = codegen.ImageFormatR16
	ImageFormatR8           = codegen.ImageFormatR8
	ImageFormatRgba16Snorm  = codegen.ImageFormatRgba16Snorm
	ImageFormatRg16Snorm    = codegen.ImageFormatRg16Snorm
	ImageFormatRg8Snorm     = codegen.ImageFormatRg8Snorm
	ImageFormatR16Snorm     = codegen.ImageFormatR16Snorm
	ImageFormatR8Snorm      = codegen.ImageFormatR8Snorm
	ImageFormatRgba32i      = codegen.ImageFormatRgba32i
	ImageFormatRgba16i      = codegen.ImageFormatRgba16i
	ImageFormatRgba8i       = codegen.ImageFormatRgba8i
	ImageFormatR32i         = codegen.ImageFormatR32i
	ImageFormatRg32i        = codegen.ImageFormatRg32i
	ImageFormatRg16i        = codegen.ImageFormatRg16i
	ImageFormatRg8i         = codegen.ImageFormatRg8i
	ImageFormatR16i         = codegen.ImageFormatR16i
	ImageFormatR8i          = codegen.ImageFormatR8i
	ImageFormatRgba32ui     = codegen.ImageFormatRgba32ui
	ImageFormatRgba16ui     = codegen.ImageFormatRgba16ui
	ImageFormatRgba8ui      = codegen.ImageFormatRgba8ui
	ImageFormatR32ui        = codegen.ImageFormatR32ui
	ImageFormatRgb10a2ui    = codegen.ImageFormatRgb10a2ui
	ImageFormatRg32ui       = codegen.ImageFormatRg32ui
	ImageFormatRg16ui       = codegen.ImageFormatRg16ui
	ImageFormatRg8ui        = codegen.ImageFormatRg8ui
	ImageFormatR16ui        = codegen.ImageFormatR16ui
	ImageFormatR8ui         = codegen.ImageFormatR8ui
	ImageFormatR64ui        = codegen.ImageFormatR64ui
	ImageFormatR64i         = codegen.ImageFormatR64i
)

// --- GLSL.std.450 extended instruction set ---

const (
	GLSLstd450Round                 = codegen.GLSLstd450Round
	GLSLstd450RoundEven             = codegen.GLSLstd450RoundEven
	GLSLstd450Trunc                 = codegen.GLSLstd450Trunc
	GLSLstd450FAbs                  = codegen.GLSLstd450FAbs
	GLSLstd450SAbs                  = codegen.GLSLstd450SAbs
	GLSLstd450FSign                 = codegen.GLSLstd450FSign
	GLSLstd450SSign                 = codegen.GLSLstd450SSign
	GLSLstd450Floor                 = codegen.GLSLstd450Floor
	GLSLstd450Ceil                  = codegen.GLSLstd450Ceil
	GLSLstd450Fract                 = codegen.GLSLstd450Fract
	GLSLstd450Radians               = codegen.GLSLstd450Radians
	GLSLstd450Degrees               = codegen.GLSLstd450Degrees
	GLSLstd450Sin                   = codegen.GLSLstd450Sin
	GLSLstd450Cos                   = codegen.GLSLstd450Cos
	GLSLstd450Tan                   = codegen.GLSLstd450Tan
	GLSLstd450Asin                  = codegen.GLSLstd450Asin
	GLSLstd450Acos                  = codegen.GLSLstd450Acos
	GLSLstd450Atan                  = codegen.GLSLstd450Atan
	GLSLstd450Sinh                  = codegen.GLSLstd450Sinh
	GLSLstd450Cosh                  = codegen.GLSLstd450Cosh
	GLSLstd450Tanh                  = codegen.GLSLstd450Tanh
	GLSLstd450Asinh                 = codegen.GLSLstd450Asinh
	GLSLstd450Acosh                 = codegen.GLSLstd450Acosh
	GLSLstd450Atanh                 = codegen.GLSLstd450Atanh
	GLSLstd450Atan2                 = codegen.GLSLstd450Atan2
	GLSLstd450Pow                   = codegen.GLSLstd450Pow
	GLSLstd450Exp                   = codegen.GLSLstd450Exp
	GLSLstd450Log                   = codegen.GLSLstd450Log
	GLSLstd450Exp2                  = codegen.GLSLstd450Exp2
	GLSLstd450Log2                  = codegen.GLSLstd450Log2
	GLSLstd450Sqrt                  = codegen.GLSLstd450Sqrt
	GLSLstd450InverseSqrt           = codegen.GLSLstd450InverseSqrt
	GLSLstd450Determinant           = codegen.GLSLstd450Determinant
	GLSLstd450MatrixInverse         = codegen.GLSLstd450MatrixInverse
	GLSLstd450Modf                  = codegen.GLSLstd450Modf
	GLSLstd450ModfStruct            = codegen.GLSLstd450ModfStruct
	GLSLstd450FMin                  = codegen.GLSLstd450FMin
	GLSLstd450UMin                  = codegen.GLSLstd450UMin
	GLSLstd450SMin                  = codegen.GLSLstd450SMin
	GLSLstd450FMax                  = codegen.GLSLstd450FMax
	GLSLstd450UMax                  = codegen.GLSLstd450UMax
	GLSLstd450SMax                  = codegen.GLSLstd450SMax
	GLSLstd450FClamp                = codegen.GLSLstd450FClamp
	GLSLstd450UClamp                = codegen.GLSLstd450UClamp
	GLSLstd450SClamp                = codegen.GLSLstd450SClamp
	GLSLstd450FMix                  = codegen.GLSLstd450FMix
	GLSLstd450IMix                  = codegen.GLSLstd450IMix
	GLSLstd450Step                  = codegen.GLSLstd450Step
	GLSLstd450SmoothStep            = codegen.GLSLstd450SmoothStep
	GLSLstd450Fma                   = codegen.GLSLstd450Fma
	GLSLstd450Frexp                 = codegen.GLSLstd450Frexp
	GLSLstd450FrexpStruct           = codegen.GLSLstd450FrexpStruct
	GLSLstd450Ldexp                 = codegen.GLSLstd450Ldexp
	GLSLstd450PackSnorm4x8          = codegen.GLSLstd450PackSnorm4x8
	GLSLstd450PackUnorm4x8          = codegen.GLSLstd450PackUnorm4x8
	GLSLstd450PackSnorm2x16         = codegen.GLSLstd450PackSnorm2x16
	GLSLstd450PackUnorm2x16         = codegen.GLSLstd450PackUnorm2x16
	GLSLstd450PackHalf2x16          = codegen.GLSLstd450PackHalf2x16
	GLSLstd450PackDouble2x32        = codegen.GLSLstd450PackDouble2x32
	GLSLstd450UnpackSnorm2x16       = codegen.GLSLstd450UnpackSnorm2x16
	GLSLstd450UnpackUnorm2x16       = codegen.GLSLstd450UnpackUnorm2x16
	GLSLstd450UnpackHalf2x16        = codegen.GLSLstd450UnpackHalf2x16
	GLSLstd450UnpackSnorm4x8        = codegen.GLSLstd450UnpackSnorm4x8
	GLSLstd450UnpackUnorm4x8        = codegen.GLSLstd450UnpackUnorm4x8
	GLSLstd450UnpackDouble2x32      = codegen.GLSLstd450UnpackDouble2x32
	GLSLstd450Length                = codegen.GLSLstd450Length
	GLSLstd450Distance              = codegen.GLSLstd450Distance
	GLSLstd450Cross                 = codegen.GLSLstd450Cross
	GLSLstd450Normalize             = codegen.GLSLstd450Normalize
	GLSLstd450FaceForward           = codegen.GLSLstd450FaceForward
	GLSLstd450Reflect               = codegen.GLSLstd450Reflect
	GLSLstd450Refract               = codegen.GLSLstd450Refract
	GLSLstd450FindILsb              = codegen.GLSLstd450FindILsb
	GLSLstd450FindSMsb              = codegen.GLSLstd450FindSMsb
	GLSLstd450FindUMsb              = codegen.GLSLstd450FindUMsb
	GLSLstd450InterpolateAtCentroid = codegen.GLSLstd450InterpolateAtCentroid
	GLSLstd450InterpolateAtSample   = codegen.GLSLstd450InterpolateAtSample
	GLSLstd450InterpolateAtOffset   = codegen.GLSLstd450InterpolateAtOffset
	GLSLstd450NMin                  = codegen.GLSLstd450NMin
	GLSLstd450NMax                  = codegen.GLSLstd450NMax
	GLSLstd450NClamp                = codegen.GLSLstd450NClamp
)

// StorageFormatToImageFormat converts an IR storage format to a SPIR-V image format.
func StorageFormatToImageFormat(format ir.StorageFormat) ImageFormat {
	return codegen.StorageFormatToImageFormat(format)
}
