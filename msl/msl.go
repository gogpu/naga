package msl

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl/internal/codegen"
)

// --- Type aliases (zero overhead, methods inherited) ---

// Version represents an MSL language version.
type Version = codegen.Version

// BoundsCheckPolicy controls how out-of-bounds accesses are handled.
type BoundsCheckPolicy = codegen.BoundsCheckPolicy

// BoundsCheckPolicies configures bounds checking for different access types.
type BoundsCheckPolicies = codegen.BoundsCheckPolicies

// SamplerCoord specifies the coordinate system for an inline sampler.
type SamplerCoord = codegen.SamplerCoord

// SamplerAddress specifies the addressing mode for an inline sampler.
type SamplerAddress = codegen.SamplerAddress

// SamplerBorderColor specifies the border color for an inline sampler.
type SamplerBorderColor = codegen.SamplerBorderColor

// SamplerFilter specifies the filtering mode for an inline sampler.
type SamplerFilter = codegen.SamplerFilter

// SamplerCompareFunc specifies the comparison function for an inline sampler.
type SamplerCompareFunc = codegen.SamplerCompareFunc

// InlineSampler defines an inline (constexpr) sampler in Metal.
type InlineSampler = codegen.InlineSampler

// BindSamplerTarget specifies how a sampler is bound.
type BindSamplerTarget = codegen.BindSamplerTarget

// BindExternalTextureTarget specifies the Metal binding slots for an external
// texture global variable.
type BindExternalTextureTarget = codegen.BindExternalTextureTarget

// BindTarget specifies the Metal binding slots for a resource.
type BindTarget = codegen.BindTarget

// EntryPointResources maps WGSL resource bindings to Metal binding slots.
type EntryPointResources = codegen.EntryPointResources

// Options configures MSL code generation.
type Options = codegen.Options

// VertexFormat describes the format of a vertex attribute.
type VertexFormat = codegen.VertexFormat

// VertexBufferStepMode defines how to advance the data in vertex buffers.
type VertexBufferStepMode = codegen.VertexBufferStepMode

// AttributeMapping maps a vertex attribute to a shader location.
type AttributeMapping = codegen.AttributeMapping

// VertexBufferMapping describes a vertex buffer and its attributes.
type VertexBufferMapping = codegen.VertexBufferMapping

// PipelineOptions configures options specific to a single pipeline/entry point.
type PipelineOptions = codegen.PipelineOptions

// EntryPointSelector identifies a specific entry point.
type EntryPointSelector = codegen.EntryPointSelector

// TranslationInfo contains information about the compiled MSL output.
type TranslationInfo = codegen.TranslationInfo

// --- Version variables ---

// Common MSL versions.
var (
	Version1_0 = codegen.Version1_0
	Version1_2 = codegen.Version1_2
	Version2_0 = codegen.Version2_0
	Version2_1 = codegen.Version2_1
	Version2_3 = codegen.Version2_3
	Version2_4 = codegen.Version2_4
	Version3_0 = codegen.Version3_0
	Version3_1 = codegen.Version3_1
)

// --- BoundsCheckPolicy constants ---

const (
	// BoundsCheckUnchecked performs no bounds checking.
	BoundsCheckUnchecked = codegen.BoundsCheckUnchecked

	// BoundsCheckReadZeroSkipWrite returns zero for out-of-bounds reads
	// and skips out-of-bounds writes.
	BoundsCheckReadZeroSkipWrite = codegen.BoundsCheckReadZeroSkipWrite

	// BoundsCheckRestrict clamps indices to valid range.
	BoundsCheckRestrict = codegen.BoundsCheckRestrict
)

// --- Sampler constants ---

const (
	SamplerCoordNormalized = codegen.SamplerCoordNormalized
	SamplerCoordPixel      = codegen.SamplerCoordPixel
)

const (
	SamplerAddressRepeat         = codegen.SamplerAddressRepeat
	SamplerAddressMirroredRepeat = codegen.SamplerAddressMirroredRepeat
	SamplerAddressClampToEdge    = codegen.SamplerAddressClampToEdge
	SamplerAddressClampToZero    = codegen.SamplerAddressClampToZero
	SamplerAddressClampToBorder  = codegen.SamplerAddressClampToBorder
)

const (
	SamplerBorderColorTransparentBlack = codegen.SamplerBorderColorTransparentBlack
	SamplerBorderColorOpaqueBlack      = codegen.SamplerBorderColorOpaqueBlack
	SamplerBorderColorOpaqueWhite      = codegen.SamplerBorderColorOpaqueWhite
)

const (
	SamplerFilterNearest = codegen.SamplerFilterNearest
	SamplerFilterLinear  = codegen.SamplerFilterLinear
)

const (
	SamplerCompareFuncNever        = codegen.SamplerCompareFuncNever
	SamplerCompareFuncLess         = codegen.SamplerCompareFuncLess
	SamplerCompareFuncLessEqual    = codegen.SamplerCompareFuncLessEqual
	SamplerCompareFuncGreater      = codegen.SamplerCompareFuncGreater
	SamplerCompareFuncGreaterEqual = codegen.SamplerCompareFuncGreaterEqual
	SamplerCompareFuncEqual        = codegen.SamplerCompareFuncEqual
	SamplerCompareFuncNotEqual     = codegen.SamplerCompareFuncNotEqual
	SamplerCompareFuncAlways       = codegen.SamplerCompareFuncAlways
)

// --- VertexFormat constants ---

const (
	VertexFormatUint8           = codegen.VertexFormatUint8
	VertexFormatUint8x2         = codegen.VertexFormatUint8x2
	VertexFormatUint8x4         = codegen.VertexFormatUint8x4
	VertexFormatSint8           = codegen.VertexFormatSint8
	VertexFormatSint8x2         = codegen.VertexFormatSint8x2
	VertexFormatSint8x4         = codegen.VertexFormatSint8x4
	VertexFormatUnorm8          = codegen.VertexFormatUnorm8
	VertexFormatUnorm8x2        = codegen.VertexFormatUnorm8x2
	VertexFormatUnorm8x4        = codegen.VertexFormatUnorm8x4
	VertexFormatSnorm8          = codegen.VertexFormatSnorm8
	VertexFormatSnorm8x2        = codegen.VertexFormatSnorm8x2
	VertexFormatSnorm8x4        = codegen.VertexFormatSnorm8x4
	VertexFormatUint16          = codegen.VertexFormatUint16
	VertexFormatUint16x2        = codegen.VertexFormatUint16x2
	VertexFormatUint16x4        = codegen.VertexFormatUint16x4
	VertexFormatSint16          = codegen.VertexFormatSint16
	VertexFormatSint16x2        = codegen.VertexFormatSint16x2
	VertexFormatSint16x4        = codegen.VertexFormatSint16x4
	VertexFormatUnorm16         = codegen.VertexFormatUnorm16
	VertexFormatUnorm16x2       = codegen.VertexFormatUnorm16x2
	VertexFormatUnorm16x4       = codegen.VertexFormatUnorm16x4
	VertexFormatSnorm16         = codegen.VertexFormatSnorm16
	VertexFormatSnorm16x2       = codegen.VertexFormatSnorm16x2
	VertexFormatSnorm16x4       = codegen.VertexFormatSnorm16x4
	VertexFormatFloat16         = codegen.VertexFormatFloat16
	VertexFormatFloat16x2       = codegen.VertexFormatFloat16x2
	VertexFormatFloat16x4       = codegen.VertexFormatFloat16x4
	VertexFormatFloat32         = codegen.VertexFormatFloat32
	VertexFormatFloat32x2       = codegen.VertexFormatFloat32x2
	VertexFormatFloat32x3       = codegen.VertexFormatFloat32x3
	VertexFormatFloat32x4       = codegen.VertexFormatFloat32x4
	VertexFormatUint32          = codegen.VertexFormatUint32
	VertexFormatUint32x2        = codegen.VertexFormatUint32x2
	VertexFormatUint32x3        = codegen.VertexFormatUint32x3
	VertexFormatUint32x4        = codegen.VertexFormatUint32x4
	VertexFormatSint32          = codegen.VertexFormatSint32
	VertexFormatSint32x2        = codegen.VertexFormatSint32x2
	VertexFormatSint32x3        = codegen.VertexFormatSint32x3
	VertexFormatSint32x4        = codegen.VertexFormatSint32x4
	VertexFormatUnorm10_10_10_2 = codegen.VertexFormatUnorm10_10_10_2
	VertexFormatUnorm8x4Bgra    = codegen.VertexFormatUnorm8x4Bgra
)

// --- VertexBufferStepMode constants ---

const (
	VertexStepModeConstant   = codegen.VertexStepModeConstant
	VertexStepModeByVertex   = codegen.VertexStepModeByVertex
	VertexStepModeByInstance = codegen.VertexStepModeByInstance
)

// --- Functions ---

// DefaultBoundsCheckPolicies returns conservative bounds check policies.
func DefaultBoundsCheckPolicies() BoundsCheckPolicies {
	return codegen.DefaultBoundsCheckPolicies()
}

// DefaultOptions returns sensible default options for MSL generation.
func DefaultOptions() Options {
	return codegen.DefaultOptions()
}

// Compile generates MSL source code from an IR module.
// Returns the MSL source as a string and translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	return codegen.Compile(module, options)
}

// CompileWithPipeline generates MSL source code with pipeline-specific options.
func CompileWithPipeline(module *ir.Module, options Options, pipeline PipelineOptions) (string, TranslationInfo, error) {
	return codegen.CompileWithPipeline(module, options, pipeline)
}
