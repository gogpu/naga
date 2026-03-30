package msl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Version represents an MSL language version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common MSL versions.
var (
	Version1_0 = Version{Major: 1, Minor: 0}
	Version1_2 = Version{Major: 1, Minor: 2}
	Version2_0 = Version{Major: 2, Minor: 0}
	Version2_1 = Version{Major: 2, Minor: 1}
	Version2_3 = Version{Major: 2, Minor: 3}
	Version2_4 = Version{Major: 2, Minor: 4}
	Version3_0 = Version{Major: 3, Minor: 0}
	Version3_1 = Version{Major: 3, Minor: 1}
)

// String returns the version as "major.minor".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// Less returns true if v is strictly less than other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	return v.Minor < other.Minor
}

// BoundsCheckPolicy controls how out-of-bounds accesses are handled.
type BoundsCheckPolicy uint8

const (
	// BoundsCheckUnchecked performs no bounds checking.
	// Out-of-bounds accesses have undefined behavior.
	BoundsCheckUnchecked BoundsCheckPolicy = iota

	// BoundsCheckReadZeroSkipWrite returns zero for out-of-bounds reads
	// and skips out-of-bounds writes.
	BoundsCheckReadZeroSkipWrite

	// BoundsCheckRestrict clamps indices to valid range.
	BoundsCheckRestrict
)

// BoundsCheckPolicies configures bounds checking for different access types.
type BoundsCheckPolicies struct {
	// Index applies to array, vector, and matrix indexing.
	Index BoundsCheckPolicy

	// Buffer applies to buffer (storage/uniform) accesses.
	Buffer BoundsCheckPolicy

	// Image applies to texture read/write operations.
	Image BoundsCheckPolicy

	// BindingArray applies to binding array (texture array) indexing.
	BindingArray BoundsCheckPolicy
}

// Contains returns true if any of the policy fields equals the given policy.
// This matches Rust naga's BoundsCheckPolicies::contains method,
// used to determine if helper structs like DefaultConstructible are needed.
// Note: BindingArray is intentionally excluded, matching the Rust implementation.
func (p BoundsCheckPolicies) Contains(policy BoundsCheckPolicy) bool {
	return p.Index == policy || p.Buffer == policy || p.Image == policy
}

// DefaultBoundsCheckPolicies returns conservative bounds check policies.
func DefaultBoundsCheckPolicies() BoundsCheckPolicies {
	return BoundsCheckPolicies{
		Index:        BoundsCheckReadZeroSkipWrite,
		Buffer:       BoundsCheckReadZeroSkipWrite,
		Image:        BoundsCheckReadZeroSkipWrite,
		BindingArray: BoundsCheckReadZeroSkipWrite,
	}
}

// SamplerCoord specifies the coordinate system for an inline sampler.
type SamplerCoord int

const (
	SamplerCoordNormalized SamplerCoord = iota
	SamplerCoordPixel
)

// SamplerAddress specifies the addressing mode for an inline sampler.
type SamplerAddress int

const (
	SamplerAddressRepeat SamplerAddress = iota
	SamplerAddressMirroredRepeat
	SamplerAddressClampToEdge
	SamplerAddressClampToZero
	SamplerAddressClampToBorder
)

// SamplerBorderColor specifies the border color for an inline sampler.
type SamplerBorderColor int

const (
	SamplerBorderColorTransparentBlack SamplerBorderColor = iota
	SamplerBorderColorOpaqueBlack
	SamplerBorderColorOpaqueWhite
)

// SamplerFilter specifies the filtering mode for an inline sampler.
type SamplerFilter int

const (
	SamplerFilterNearest SamplerFilter = iota
	SamplerFilterLinear
)

// SamplerCompareFunc specifies the comparison function for an inline sampler.
type SamplerCompareFunc int

const (
	SamplerCompareFuncNever SamplerCompareFunc = iota
	SamplerCompareFuncLess
	SamplerCompareFuncLessEqual
	SamplerCompareFuncGreater
	SamplerCompareFuncGreaterEqual
	SamplerCompareFuncEqual
	SamplerCompareFuncNotEqual
	SamplerCompareFuncAlways
)

// InlineSampler defines an inline (constexpr) sampler in Metal.
type InlineSampler struct {
	Coord       SamplerCoord
	Address     [3]SamplerAddress
	BorderColor SamplerBorderColor
	MagFilter   SamplerFilter
	MinFilter   SamplerFilter
	MipFilter   *SamplerFilter // nil if not set
	CompareFunc SamplerCompareFunc
}

// BindSamplerTarget specifies how a sampler is bound.
type BindSamplerTarget struct {
	// IsInline indicates the sampler is an inline (constexpr) sampler.
	IsInline bool
	// Slot is the binding slot (for resource samplers) or index into
	// Options.InlineSamplers (for inline samplers).
	Slot uint8
}

// BindExternalTextureTarget specifies the Metal binding slots for an external
// texture global variable. External textures are lowered to 3 texture planes
// and a constant buffer of NagaExternalTextureParams.
type BindExternalTextureTarget struct {
	Planes [3]uint8
	Params uint8
}

// BindTarget specifies the Metal binding slots for a resource.
type BindTarget struct {
	// Buffer is the buffer binding slot. Nil if not bound as buffer.
	Buffer *uint8

	// Texture is the texture binding slot. Nil if not bound as texture.
	Texture *uint8

	// Sampler is the sampler binding slot. Nil if not bound as sampler.
	Sampler *BindSamplerTarget

	// ExternalTexture is the binding for external texture planes + params.
	ExternalTexture *BindExternalTextureTarget

	// Mutable indicates if this is a read-write resource.
	Mutable bool
}

// EntryPointResources maps WGSL resource bindings to Metal binding slots.
type EntryPointResources struct {
	// Resources maps (group, binding) pairs to Metal bind targets.
	Resources map[ir.ResourceBinding]BindTarget

	// PushConstantBuffer is the buffer slot for push constants.
	// Nil if push constants are not used.
	PushConstantBuffer *uint8

	// SizesBuffer is the buffer slot for runtime array sizes.
	// Required when using runtime-sized arrays.
	SizesBuffer *uint8

	// ImmediatesBuffer is the buffer slot for immediate data.
	// Nil if immediate data is not used.
	ImmediatesBuffer *uint8
}

// Options configures MSL code generation.
type Options struct {
	// LangVersion is the target MSL version.
	// Defaults to Version2_1 if zero.
	LangVersion Version

	// PerEntryPointMap maps entry point names to their resource bindings.
	// If nil, bindings are auto-generated.
	PerEntryPointMap map[string]EntryPointResources

	// InlineSamplers defines constexpr samplers to be inlined into the code.
	// Referenced by BindSamplerTarget.Slot when IsInline is true.
	InlineSamplers []InlineSampler

	// BoundsCheckPolicies controls bounds checking behavior.
	BoundsCheckPolicies BoundsCheckPolicies

	// ZeroInitializeWorkgroupMemory enables zero-initialization of
	// workgroup (threadgroup) memory at the start of compute shaders.
	// This adds overhead but ensures defined behavior.
	ZeroInitializeWorkgroupMemory bool

	// ForceLoopBounding adds loop iteration limits to prevent infinite loops.
	// Recommended for untrusted shaders.
	ForceLoopBounding bool

	// FakeMissingBindings generates placeholder bindings for resources
	// that are referenced but not in the PerEntryPointMap.
	FakeMissingBindings bool

	// PipelineConstants specifies values for pipeline-overridable constants.
	// Keys are either the numeric @id as a decimal string (e.g., "0", "1300")
	// or the override's identifier name (e.g., "depth", "width").
	// Values are f64 representations that get converted to the override's type.
	// Matches Rust naga's PipelineConstants type.
	PipelineConstants map[string]float64

	// AllowAndForcePointSize forces point size output for vertex shaders.
	// When true, vertex output structs get an extra _point_size [[point_size]]
	// member, and the entry point return initializer includes 1.0.
	// Required for point primitive topologies.
	AllowAndForcePointSize bool

	// VertexPullingTransform enables vertex pulling transformation.
	// When true, vertex shaders receive raw buffer data instead of assembled
	// vertex attributes. The shader reads bytes from vertex buffers and
	// unpacks attribute data manually with bounds checking.
	VertexPullingTransform bool

	// VertexBufferMappings describes the vertex buffer layout for vertex pulling.
	// Each entry describes one vertex buffer with its stride, step mode, and attributes.
	VertexBufferMappings []VertexBufferMapping
}

// VertexFormat describes the format of a vertex attribute.
// Matches Rust naga's back::msl::VertexFormat enum.
type VertexFormat int

const (
	VertexFormatUint8           VertexFormat = 0
	VertexFormatUint8x2         VertexFormat = 1
	VertexFormatUint8x4         VertexFormat = 2
	VertexFormatSint8           VertexFormat = 3
	VertexFormatSint8x2         VertexFormat = 4
	VertexFormatSint8x4         VertexFormat = 5
	VertexFormatUnorm8          VertexFormat = 6
	VertexFormatUnorm8x2        VertexFormat = 7
	VertexFormatUnorm8x4        VertexFormat = 8
	VertexFormatSnorm8          VertexFormat = 9
	VertexFormatSnorm8x2        VertexFormat = 10
	VertexFormatSnorm8x4        VertexFormat = 11
	VertexFormatUint16          VertexFormat = 12
	VertexFormatUint16x2        VertexFormat = 13
	VertexFormatUint16x4        VertexFormat = 14
	VertexFormatSint16          VertexFormat = 15
	VertexFormatSint16x2        VertexFormat = 16
	VertexFormatSint16x4        VertexFormat = 17
	VertexFormatUnorm16         VertexFormat = 18
	VertexFormatUnorm16x2       VertexFormat = 19
	VertexFormatUnorm16x4       VertexFormat = 20
	VertexFormatSnorm16         VertexFormat = 21
	VertexFormatSnorm16x2       VertexFormat = 22
	VertexFormatSnorm16x4       VertexFormat = 23
	VertexFormatFloat16         VertexFormat = 24
	VertexFormatFloat16x2       VertexFormat = 25
	VertexFormatFloat16x4       VertexFormat = 26
	VertexFormatFloat32         VertexFormat = 27
	VertexFormatFloat32x2       VertexFormat = 28
	VertexFormatFloat32x3       VertexFormat = 29
	VertexFormatFloat32x4       VertexFormat = 30
	VertexFormatUint32          VertexFormat = 31
	VertexFormatUint32x2        VertexFormat = 32
	VertexFormatUint32x3        VertexFormat = 33
	VertexFormatUint32x4        VertexFormat = 34
	VertexFormatSint32          VertexFormat = 35
	VertexFormatSint32x2        VertexFormat = 36
	VertexFormatSint32x3        VertexFormat = 37
	VertexFormatSint32x4        VertexFormat = 38
	VertexFormatUnorm10_10_10_2 VertexFormat = 43
	VertexFormatUnorm8x4Bgra    VertexFormat = 44
)

// VertexBufferStepMode defines how to advance the data in vertex buffers.
type VertexBufferStepMode int

const (
	VertexStepModeConstant   VertexBufferStepMode = 0
	VertexStepModeByVertex   VertexBufferStepMode = 1
	VertexStepModeByInstance VertexBufferStepMode = 2
)

// AttributeMapping maps a vertex attribute to a shader location.
type AttributeMapping struct {
	ShaderLocation uint32
	Offset         uint32
	Format         VertexFormat
}

// VertexBufferMapping describes a vertex buffer and its attributes.
type VertexBufferMapping struct {
	ID         uint32
	Stride     uint32
	StepMode   VertexBufferStepMode
	Attributes []AttributeMapping
}

// DefaultOptions returns sensible default options for MSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:                   Version2_1,
		BoundsCheckPolicies:           DefaultBoundsCheckPolicies(),
		ZeroInitializeWorkgroupMemory: true,
		ForceLoopBounding:             true,
	}
}

// PipelineOptions configures options specific to a single pipeline/entry point.
type PipelineOptions struct {
	// EntryPoint specifies which entry point to compile.
	// If nil, all entry points are compiled.
	EntryPoint *EntryPointSelector

	// AllowAndForcePointSize forces point size output for vertex shaders.
	// Required for point primitive topology.
	AllowAndForcePointSize bool
}

// EntryPointSelector identifies a specific entry point.
type EntryPointSelector struct {
	Stage ir.ShaderStage
	Name  string
}

// TranslationInfo contains information about the compiled MSL output.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated MSL names.
	EntryPointNames map[string]string

	// RequiresSizesBuffer indicates if a sizes buffer is needed for
	// runtime-sized arrays.
	RequiresSizesBuffer bool
}

// Compile generates MSL source code from an IR module.
// Returns the MSL source as a string and translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	return CompileWithPipeline(module, options, PipelineOptions{})
}

// CompileWithPipeline generates MSL source code with pipeline-specific options.
func CompileWithPipeline(module *ir.Module, options Options, pipeline PipelineOptions) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version2_1
	}

	// Apply pipeline constants to override values if any are specified.
	if len(options.PipelineConstants) > 0 && len(module.Overrides) > 0 {
		module = applyPipelineConstants(module, options.PipelineConstants)
	}

	// Create writer
	w := newWriter(module, &options, &pipeline)

	// Generate MSL code
	if err := w.writeModule(); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}

	info := TranslationInfo{
		EntryPointNames:     w.entryPointNames,
		RequiresSizesBuffer: w.needsSizesBuffer,
	}

	return w.String(), info, nil
}
