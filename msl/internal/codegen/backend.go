package codegen

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/internal/unsupported"
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

// policiesContain reports whether policy is one of p's Index, Buffer and
// ImageLoad policies (Rust naga's BoundsCheckPolicies::contains, which also
// leaves out BindingArray); it decides whether helpers such as
// DefaultConstructible are needed.
func policiesContain(p backend.BoundsCheckPolicies, policy backend.BoundsCheckPolicy) bool {
	return p.Index == policy || p.Buffer == policy || p.ImageLoad == policy
}

// DefaultBoundsCheckPolicies returns ReadZeroSkipWrite on every field, the
// policies of DefaultOptions.
func DefaultBoundsCheckPolicies() backend.BoundsCheckPolicies {
	return backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckReadZeroSkipWrite,
		Buffer:       backend.BoundsCheckReadZeroSkipWrite,
		ImageLoad:    backend.BoundsCheckReadZeroSkipWrite,
		BindingArray: backend.BoundsCheckReadZeroSkipWrite,
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
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// threadgroup variables they use: Polyfill (zero) or None; Native is an
	// error.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// ForceLoopBounding adds loop iteration limits to prevent infinite loops.
	// Recommended for untrusted shaders.
	ForceLoopBounding bool

	// FakeMissingBindings generates placeholder bindings for resources
	// that are referenced but not in the PerEntryPointMap.
	FakeMissingBindings bool

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

	// ClampFragDepth clamps the depth a fragment entry point writes to the
	// range at byte ClampFragDepth.Offset of its immediates buffer (see the
	// msl package).
	ClampFragDepth *backend.ClampFragDepth

	// FixedSampleMask is the render pipeline's sample mask, which fragment
	// entry points apply themselves (see the msl package).
	FixedSampleMask *uint32
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
	VertexFormatSnorm10_10_10_2 VertexFormat = 45
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
		LangVersion:         Version2_1,
		BoundsCheckPolicies: DefaultBoundsCheckPolicies(),
		ForceLoopBounding:   true,
	}
}

// PipelineOptions configures options specific to a single pipeline/entry point.
type PipelineOptions struct {
	// EntryPoint specifies which entry point to compile.
	// If nil, all entry points are compiled.
	EntryPoint *backend.EntryPointSelector

	// AllowAndForcePointSize forces point size output for vertex shaders.
	// Required for point primitive topology.
	AllowAndForcePointSize bool
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

// checkBuiltins refuses a written entry point whose interface uses a built-in
// Metal has no attribute for (draw_index: Rust naga's UnsupportedBuiltIn).
func checkBuiltins(module *ir.Module, sel *backend.EntryPointSelector) error {
	refused := func(b *ir.Binding) bool {
		if b == nil {
			return false
		}
		bi, ok := (*b).(ir.BuiltinBinding)
		return ok && bi.Builtin == ir.BuiltinDrawIndex
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if sel != nil && (sel.Name != ep.Name || sel.Stage != ep.Stage) {
			continue
		}
		var bindings []*ir.Binding
		for _, arg := range ep.Function.Arguments {
			bindings = append(bindings, arg.Binding)
			if st, ok := module.Types[arg.Type].Inner.(ir.StructType); ok {
				for j := range st.Members {
					bindings = append(bindings, st.Members[j].Binding)
				}
			}
		}
		for _, b := range bindings {
			if refused(b) {
				return unsupported.Errorf("msl: entry point %q: built-in draw_index is not supported", ep.Name)
			}
		}
	}
	return nil
}

// CompileWithPipeline generates MSL source code with pipeline-specific options.
func CompileWithPipeline(module *ir.Module, options Options, pipeline PipelineOptions) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version2_1
	}
	options.AllowAndForcePointSize = options.AllowAndForcePointSize || pipeline.AllowAndForcePointSize
	if m := options.ZeroInitializeWorkgroupMemory; m != backend.ZeroInitWorkgroupPolyfill && m != backend.ZeroInitWorkgroupNone {
		return "", TranslationInfo{}, unsupported.Errorf("msl: ZeroInitializeWorkgroupMemory %v is not supported", m)
	}
	p := options.BoundsCheckPolicies
	for _, policy := range []backend.BoundsCheckPolicy{p.Index, p.Buffer, p.ImageLoad, p.BindingArray} {
		if policy > backend.BoundsCheckUnchecked {
			return "", TranslationInfo{}, unsupported.Errorf("msl: BoundsCheckPolicies: %v is not supported", policy)
		}
	}

	// Overrides must have been replaced by naga.ProcessOverrides.
	if err := irutil.CheckWorkgroupSizesResolved(module); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}
	module, err := irutil.ResolvePendingArraySizes(module)
	if err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}
	if err := checkBuiltins(module, pipeline.EntryPoint); err != nil {
		return "", TranslationInfo{}, err
	}

	// Rust's writer reads expression types from the validator's ModuleInfo;
	// Compile takes no such info, so it types the module it writes.
	types, err := typifier.ForModule(module)
	if err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}

	// Create writer
	w := newWriter(module, &options, &pipeline)
	w.typifiers = types

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
