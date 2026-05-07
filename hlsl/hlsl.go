// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"github.com/gogpu/naga/hlsl/internal/codegen"
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// --- Types re-exported from codegen ---

// Options configures HLSL code generation.
type Options = codegen.Options

// FragmentEntryPoint describes a fragment entry point used to filter
// vertex shader outputs.
type FragmentEntryPoint = codegen.FragmentEntryPoint

// TranslationInfo contains metadata about the HLSL translation.
type TranslationInfo = codegen.TranslationInfo

// FeatureFlags indicates which HLSL features are used by the generated code.
type FeatureFlags = codegen.FeatureFlags

// BindTarget specifies the HLSL register binding for a resource.
type BindTarget = codegen.BindTarget

// OffsetsBindTarget specifies the HLSL register binding for a dynamic buffer
// offsets constant buffer.
type OffsetsBindTarget = codegen.OffsetsBindTarget

// SamplerHeapBindTargets specifies bind targets for sampler heaps.
type SamplerHeapBindTargets = codegen.SamplerHeapBindTargets

// ResourceBinding identifies a resource in the source shader.
type ResourceBinding = codegen.ResourceBinding

// RegisterType represents the HLSL register type.
type RegisterType = codegen.RegisterType

// ExternalTextureBindTarget specifies HLSL binding information for an external
// texture global variable.
type ExternalTextureBindTarget = codegen.ExternalTextureBindTarget

// ExternalTextureBindingMap maps resource bindings to external texture bind targets.
type ExternalTextureBindingMap = codegen.ExternalTextureBindingMap

// ShaderModel represents a DirectX Shader Model version.
type ShaderModel = codegen.ShaderModel

// ErrorKind categorizes HLSL compilation errors.
type ErrorKind = codegen.ErrorKind

// Span represents a source location for error reporting.
type Span = codegen.Span

// Error represents an HLSL compilation error.
type Error = codegen.Error

// Writer generates HLSL source code from IR.
type Writer = codegen.Writer

// Io distinguishes input from output in entry point interfaces.
type Io = codegen.Io

// --- ShaderModel constants ---

const (
	ShaderModel5_0 = codegen.ShaderModel5_0
	ShaderModel5_1 = codegen.ShaderModel5_1
	ShaderModel6_0 = codegen.ShaderModel6_0
	ShaderModel6_1 = codegen.ShaderModel6_1
	ShaderModel6_2 = codegen.ShaderModel6_2
	ShaderModel6_3 = codegen.ShaderModel6_3
	ShaderModel6_4 = codegen.ShaderModel6_4
	ShaderModel6_5 = codegen.ShaderModel6_5
	ShaderModel6_6 = codegen.ShaderModel6_6
	ShaderModel6_7 = codegen.ShaderModel6_7
)

// --- FeatureFlags constants ---

const (
	FeatureNone          = codegen.FeatureNone
	FeatureWaveOps       = codegen.FeatureWaveOps
	FeatureRayTracing    = codegen.FeatureRayTracing
	FeatureMeshShaders   = codegen.FeatureMeshShaders
	Feature64BitIntegers = codegen.Feature64BitIntegers
	Feature64BitAtomics  = codegen.Feature64BitAtomics
	FeatureFloat16       = codegen.FeatureFloat16
	FeatureSubgroupOps   = codegen.FeatureSubgroupOps
)

// --- RegisterType constants ---

const (
	RegisterTypeB = codegen.RegisterTypeB
	RegisterTypeT = codegen.RegisterTypeT
	RegisterTypeS = codegen.RegisterTypeS
	RegisterTypeU = codegen.RegisterTypeU
)

// --- ErrorKind constants ---

const (
	ErrUnsupportedFeature = codegen.ErrUnsupportedFeature
	ErrMissingBinding     = codegen.ErrMissingBinding
	ErrInvalidShaderModel = codegen.ErrInvalidShaderModel
	ErrInternalError      = codegen.ErrInternalError
	ErrInvalidModule      = codegen.ErrInvalidModule
	ErrUnsupportedType    = codegen.ErrUnsupportedType
	ErrEntryPointNotFound = codegen.ErrEntryPointNotFound
)

// --- Io constants ---

const (
	IoInput  = codegen.IoInput
	IoOutput = codegen.IoOutput
)

// --- Keyword constants ---

const UnnamedIdentifier = codegen.UnnamedIdentifier

// Naga helper function names.
const (
	NagaModfFunction               = codegen.NagaModfFunction
	NagaFrexpFunction              = codegen.NagaFrexpFunction
	NagaExtractBitsFunction        = codegen.NagaExtractBitsFunction
	NagaInsertBitsFunction         = codegen.NagaInsertBitsFunction
	SamplerHeapVar                 = codegen.SamplerHeapVar
	ComparisonSamplerHeapVar       = codegen.ComparisonSamplerHeapVar
	SampleExternalTextureFunction  = codegen.SampleExternalTextureFunction
	NagaAbsFunction                = codegen.NagaAbsFunction
	NagaDivFunction                = codegen.NagaDivFunction
	NagaModFunction                = codegen.NagaModFunction
	NagaNegFunction                = codegen.NagaNegFunction
	NagaF2I32Function              = codegen.NagaF2I32Function
	NagaF2U32Function              = codegen.NagaF2U32Function
	NagaF2I64Function              = codegen.NagaF2I64Function
	NagaF2U64Function              = codegen.NagaF2U64Function
	ImageLoadExternalFunction      = codegen.ImageLoadExternalFunction
	ImageSampleBaseClampToEdgeFunc = codegen.ImageSampleBaseClampToEdgeFunc
	DynamicBufferOffsetsPrefix     = codegen.DynamicBufferOffsetsPrefix
	ImageStorageLoadScalarWrapper  = codegen.ImageStorageLoadScalarWrapper
)

// --- Functions ---

// Compile generates HLSL source code from an IR module.
// Returns the HLSL source, translation info, or an error.
func Compile(module *ir.Module, options *Options) (string, *TranslationInfo, error) {
	return codegen.Compile(module, options)
}

// DefaultOptions returns sensible default options for HLSL generation.
func DefaultOptions() *Options {
	return codegen.DefaultOptions()
}

// DefaultBindTarget returns a BindTarget with default values.
func DefaultBindTarget() BindTarget {
	return codegen.DefaultBindTarget()
}

// NewError creates a new HLSL error without span information.
func NewError(kind ErrorKind, message string) *Error {
	return codegen.NewError(kind, message)
}

// NewErrorWithSpan creates a new HLSL error with span information.
func NewErrorWithSpan(kind ErrorKind, message string, start, end uint32) *Error {
	return codegen.NewErrorWithSpan(kind, message, start, end)
}

// NeedsTrailingUnderscore reports whether a variable name requires a
// trailing underscore suffix in HLSL output.
func NeedsTrailingUnderscore(name string) bool {
	return backend.NeedsTrailingUnderscore(name)
}

// --- Conversion functions ---

// ScalarToHLSL converts a scalar type to its HLSL string representation.
func ScalarToHLSL(s ir.ScalarType) string {
	return codegen.ScalarToHLSL(s)
}

// VectorToHLSL converts a vector type to its HLSL string representation.
func VectorToHLSL(v ir.VectorType) string {
	return codegen.VectorToHLSL(v)
}

// MatrixToHLSL converts a matrix type to its HLSL string representation.
func MatrixToHLSL(m ir.MatrixType) string {
	return codegen.MatrixToHLSL(m)
}

// ScalarCast returns the HLSL cast function for a scalar kind.
func ScalarCast(k ir.ScalarKind) string {
	return codegen.ScalarCast(k)
}

// BuiltInToSemantic converts a built-in value to its HLSL semantic string.
func BuiltInToSemantic(b ir.BuiltinValue) string {
	return codegen.BuiltInToSemantic(b)
}

// InterpolationToHLSL converts an interpolation kind to HLSL modifier.
func InterpolationToHLSL(k ir.InterpolationKind) string {
	return codegen.InterpolationToHLSL(k)
}

// SamplingToHLSL converts an interpolation sampling to HLSL modifier.
func SamplingToHLSL(s ir.InterpolationSampling) string {
	return codegen.SamplingToHLSL(s)
}

// AtomicOpToHLSL converts an atomic operation to its HLSL function name.
func AtomicOpToHLSL(op string) string {
	return codegen.AtomicOpToHLSL(op)
}

// AddressSpaceToHLSL converts an address space to HLSL representation.
func AddressSpaceToHLSL(space ir.AddressSpace) string {
	return codegen.AddressSpaceToHLSL(space)
}

// ImageDimToHLSL converts an image dimension to HLSL suffix.
func ImageDimToHLSL(dim ir.ImageDimension, arrayed bool) string {
	return codegen.ImageDimToHLSL(dim, arrayed)
}

// ImageClassToHLSL converts an image class to HLSL prefix.
func ImageClassToHLSL(class ir.ImageClass, readWrite bool) string {
	return codegen.ImageClassToHLSL(class, readWrite)
}

// ImageToHLSL converts an image type to full HLSL type string.
func ImageToHLSL(img ir.ImageType, readWrite bool) string {
	return codegen.ImageToHLSL(img, readWrite)
}

// SamplerToHLSL returns the HLSL sampler type.
func SamplerToHLSL(comparison bool) string {
	return codegen.SamplerToHLSL(comparison)
}

// ShaderStageToHLSL converts a shader stage to HLSL attribute name.
func ShaderStageToHLSL(stage ir.ShaderStage) string {
	return codegen.ShaderStageToHLSL(stage)
}

// ShaderProfile returns the full HLSL shader profile string.
func ShaderProfile(stage ir.ShaderStage, major, minor uint8) string {
	return codegen.ShaderProfile(stage, major, minor)
}

// --- Keyword functions ---

// IsReserved checks if a name is an HLSL reserved keyword.
func IsReserved(name string) bool {
	return codegen.IsReserved(name)
}

// IsCaseInsensitiveReserved checks if a name conflicts with case-insensitive keywords.
func IsCaseInsensitiveReserved(name string) bool {
	return codegen.IsCaseInsensitiveReserved(name)
}

// Escape returns a safe identifier name.
func Escape(name string) string {
	return codegen.Escape(name)
}
