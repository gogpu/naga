// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/hlsl/internal/codegen"
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// --- Configuration types ---

// ShaderModel represents a DirectX Shader Model version.
// Shader Models define the feature set available for shader compilation.
type ShaderModel uint8

// Supported Shader Model versions.
const (
	// ShaderModel5_0 is the base SM5 version (DirectX 11).
	ShaderModel5_0 ShaderModel = iota

	// ShaderModel5_1 provides improved resource binding (default).
	ShaderModel5_1

	// ShaderModel6_0 introduces wave intrinsics and DXIL.
	ShaderModel6_0

	// ShaderModel6_1 adds SV_ViewID and barycentrics.
	ShaderModel6_1

	// ShaderModel6_2 adds float16 and denorm control.
	ShaderModel6_2

	// ShaderModel6_3 adds DirectX Raytracing (DXR).
	ShaderModel6_3

	// ShaderModel6_4 adds variable rate shading and library subobjects.
	ShaderModel6_4

	// ShaderModel6_5 adds mesh shaders and sampler feedback.
	ShaderModel6_5

	// ShaderModel6_6 adds 64-bit atomics and dynamic resources.
	ShaderModel6_6

	// ShaderModel6_7 adds advanced mesh shaders and work graphs.
	ShaderModel6_7
)

// String returns a human-readable representation of the shader model.
func (sm ShaderModel) String() string {
	major, minor := sm.version()
	return fmt.Sprintf("SM %d.%d", major, minor)
}

// ProfileSuffix returns the shader profile suffix for this model.
// Example: "5_1", "6_0". Used to construct profiles like "vs_5_1".
func (sm ShaderModel) ProfileSuffix() string {
	major, minor := sm.version()
	return fmt.Sprintf("%d_%d", major, minor)
}

// Major returns the major version number.
func (sm ShaderModel) Major() uint8 {
	major, _ := sm.version()
	return major
}

// Minor returns the minor version number.
func (sm ShaderModel) Minor() uint8 {
	_, minor := sm.version()
	return minor
}

// SupportsDXIL returns true if this shader model uses DXIL output (SM 6.0+).
func (sm ShaderModel) SupportsDXIL() bool {
	return sm >= ShaderModel6_0
}

// SupportsWaveOps returns true if this shader model supports wave intrinsics (SM 6.0+).
func (sm ShaderModel) SupportsWaveOps() bool {
	return sm >= ShaderModel6_0
}

// SupportsMeshShaders returns true if this shader model supports mesh shaders (SM 6.5+).
func (sm ShaderModel) SupportsMeshShaders() bool {
	return sm >= ShaderModel6_5
}

// SupportsRayTracing returns true if this shader model supports ray tracing (SM 6.3+).
func (sm ShaderModel) SupportsRayTracing() bool {
	return sm >= ShaderModel6_3
}

// Supports64BitAtomics returns true if this shader model supports 64-bit atomics (SM 6.6+).
func (sm ShaderModel) Supports64BitAtomics() bool {
	return sm >= ShaderModel6_6
}

// SupportsFloat16 returns true if this shader model supports native float16 (SM 6.2+).
func (sm ShaderModel) SupportsFloat16() bool {
	return sm >= ShaderModel6_2
}

// SupportsVariableRateShading returns true if this shader model supports VRS (SM 6.4+).
func (sm ShaderModel) SupportsVariableRateShading() bool {
	return sm >= ShaderModel6_4
}

// version returns the major and minor version numbers.
func (sm ShaderModel) version() (major, minor uint8) {
	switch sm {
	case ShaderModel5_0:
		return 5, 0
	case ShaderModel5_1:
		return 5, 1
	case ShaderModel6_0:
		return 6, 0
	case ShaderModel6_1:
		return 6, 1
	case ShaderModel6_2:
		return 6, 2
	case ShaderModel6_3:
		return 6, 3
	case ShaderModel6_4:
		return 6, 4
	case ShaderModel6_5:
		return 6, 5
	case ShaderModel6_6:
		return 6, 6
	case ShaderModel6_7:
		return 6, 7
	default:
		return 5, 1
	}
}

// --- Binding types ---

// BindTarget specifies the HLSL register binding for a resource.
type BindTarget struct {
	// Space is the register space (0-based).
	Space uint8

	// Register is the register index within the space.
	Register uint32

	// BindingArraySize is the array size for binding arrays.
	// If nil, the resource is not an array.
	BindingArraySize *uint32

	// DynamicStorageBufferOffsetsIndex is the index into the dynamic buffer offsets
	// constant buffer for this binding.
	DynamicStorageBufferOffsetsIndex *uint32

	// RestrictIndexing indicates that this binding should have bounds checking.
	RestrictIndexing bool
}

// WithSpace returns a copy of the BindTarget with the specified space.
func (bt BindTarget) WithSpace(space uint8) BindTarget {
	bt.Space = space
	return bt
}

// WithRegister returns a copy of the BindTarget with the specified register.
func (bt BindTarget) WithRegister(register uint32) BindTarget {
	bt.Register = register
	return bt
}

// WithArraySize returns a copy of the BindTarget with the specified array size.
func (bt BindTarget) WithArraySize(size uint32) BindTarget {
	bt.BindingArraySize = &size
	return bt
}

// OffsetsBindTarget specifies the HLSL register binding for a dynamic buffer
// offsets constant buffer.
type OffsetsBindTarget struct {
	Space    uint8
	Register uint32
	Size     uint32
}

// SamplerHeapBindTargets specifies bind targets for sampler heaps.
type SamplerHeapBindTargets struct {
	// StandardSamplers is the binding for non-comparison samplers.
	StandardSamplers BindTarget

	// ComparisonSamplers is the binding for comparison samplers.
	ComparisonSamplers BindTarget
}

// ResourceBinding identifies a resource in the source shader.
type ResourceBinding struct {
	// Group corresponds to WGSL @group or SPIR-V DescriptorSet.
	Group uint32

	// Binding corresponds to WGSL @binding or SPIR-V Binding.
	Binding uint32
}

// RegisterType represents the HLSL register type.
type RegisterType uint8

const (
	// RegisterTypeB is for constant buffers (cbuffer).
	RegisterTypeB RegisterType = iota

	// RegisterTypeT is for textures and shader resource views.
	RegisterTypeT

	// RegisterTypeS is for samplers.
	RegisterTypeS

	// RegisterTypeU is for unordered access views (UAV).
	RegisterTypeU
)

// String returns the single-character register prefix.
func (rt RegisterType) String() string {
	switch rt {
	case RegisterTypeB:
		return "b"
	case RegisterTypeT:
		return "t"
	case RegisterTypeS:
		return "s"
	case RegisterTypeU:
		return "u"
	default:
		return "b"
	}
}

// ExternalTextureBindTarget specifies HLSL binding information for an external
// texture global variable.
type ExternalTextureBindTarget struct {
	// Planes contains the bind targets for the 3 plane textures.
	Planes [3]BindTarget
	// Params is the bind target for the parameters cbuffer.
	Params BindTarget
}

// ExternalTextureBindingMap maps resource bindings to external texture bind targets.
type ExternalTextureBindingMap map[ResourceBinding]ExternalTextureBindTarget

// --- Options ---

// Options configures HLSL code generation.
type Options struct {
	// ShaderModel specifies the target shader model.
	ShaderModel ShaderModel

	// BindingMap maps source resource bindings to HLSL register targets.
	BindingMap map[ResourceBinding]BindTarget

	// SamplerHeapTargets specifies binding targets for sampler heaps.
	SamplerHeapTargets SamplerHeapBindTargets

	// SamplerBufferBindingMap maps group numbers to bind targets for
	// sampler index buffers.
	SamplerBufferBindingMap map[uint32]BindTarget

	// ExternalTextureBindingMap maps resource bindings to external texture bind targets.
	ExternalTextureBindingMap ExternalTextureBindingMap

	// FakeMissingBindings generates automatic bindings for resources
	// not found in BindingMap.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory emits code to zero-initialize
	// groupshared variables at the start of compute shaders.
	ZeroInitializeWorkgroupMemory bool

	// RestrictIndexing adds bounds checks to array/buffer accesses.
	RestrictIndexing bool

	// ForceLoopBounding adds maximum iteration limits to loops.
	ForceLoopBounding bool

	// DynamicStorageBufferOffsetsTargets maps group indices to their bind targets
	// for dynamic storage buffer offset constant buffers.
	DynamicStorageBufferOffsetsTargets map[uint32]OffsetsBindTarget

	// SpecialConstantsBinding specifies the binding for the NagaConstants
	// constant buffer.
	SpecialConstantsBinding *BindTarget

	// EntryPoint specifies which entry point to compile.
	EntryPoint string

	// FragmentEntryPoint specifies a fragment entry point to consider when
	// generating the output interface of vertex entry points.
	FragmentEntryPoint *FragmentEntryPoint
}

// FragmentEntryPoint describes a fragment entry point used to filter
// vertex shader outputs.
type FragmentEntryPoint struct {
	// Module is the IR module containing the fragment entry point.
	Module *ir.Module
	// Function is the fragment entry point function.
	Function *ir.Function
}

// --- Feature flags ---

// FeatureFlags indicates which HLSL features are used by the generated code.
type FeatureFlags uint32

const (
	// FeatureNone indicates no special features are used.
	FeatureNone FeatureFlags = 0

	// FeatureWaveOps indicates wave intrinsics are used (SM 6.0+).
	FeatureWaveOps FeatureFlags = 1 << iota

	// FeatureRayTracing indicates DXR features are used (SM 6.3+).
	FeatureRayTracing

	// FeatureMeshShaders indicates mesh shader features are used (SM 6.5+).
	FeatureMeshShaders

	// Feature64BitIntegers indicates 64-bit integer types are used.
	Feature64BitIntegers

	// Feature64BitAtomics indicates 64-bit atomic operations are used (SM 6.6+).
	Feature64BitAtomics

	// FeatureFloat16 indicates native float16 types are used (SM 6.2+).
	FeatureFloat16

	// FeatureSubgroupOps indicates subgroup operations are used.
	FeatureSubgroupOps
)

// Has returns true if the flags contain the specified feature.
func (f FeatureFlags) Has(feature FeatureFlags) bool {
	return f&feature != 0
}

// String returns a human-readable list of enabled features.
func (f FeatureFlags) String() string {
	if f == FeatureNone {
		return "none"
	}

	var features []string
	if f.Has(FeatureWaveOps) {
		features = append(features, "WaveOps")
	}
	if f.Has(FeatureRayTracing) {
		features = append(features, "RayTracing")
	}
	if f.Has(FeatureMeshShaders) {
		features = append(features, "MeshShaders")
	}
	if f.Has(Feature64BitIntegers) {
		features = append(features, "64BitIntegers")
	}
	if f.Has(Feature64BitAtomics) {
		features = append(features, "64BitAtomics")
	}
	if f.Has(FeatureFloat16) {
		features = append(features, "Float16")
	}
	if f.Has(FeatureSubgroupOps) {
		features = append(features, "SubgroupOps")
	}

	if len(features) == 0 {
		return "none"
	}

	result := features[0]
	for i := 1; i < len(features); i++ {
		result += ", " + features[i]
	}
	return result
}

// --- Error types ---

// ErrorKind categorizes HLSL compilation errors.
type ErrorKind uint8

const (
	// ErrUnsupportedFeature indicates a shader feature not supported by the target.
	ErrUnsupportedFeature ErrorKind = iota

	// ErrMissingBinding indicates a resource binding was not found in BindingMap.
	ErrMissingBinding

	// ErrInvalidShaderModel indicates an invalid or unsupported shader model.
	ErrInvalidShaderModel

	// ErrInternalError indicates an internal compiler error.
	ErrInternalError

	// ErrInvalidModule indicates the IR module is malformed.
	ErrInvalidModule

	// ErrUnsupportedType indicates a type that cannot be represented in HLSL.
	ErrUnsupportedType

	// ErrEntryPointNotFound indicates the specified entry point doesn't exist.
	ErrEntryPointNotFound
)

// String returns a human-readable error kind name.
func (k ErrorKind) String() string {
	switch k {
	case ErrUnsupportedFeature:
		return "UnsupportedFeature"
	case ErrMissingBinding:
		return "MissingBinding"
	case ErrInvalidShaderModel:
		return "InvalidShaderModel"
	case ErrInternalError:
		return "InternalError"
	case ErrInvalidModule:
		return "InvalidModule"
	case ErrUnsupportedType:
		return "UnsupportedType"
	case ErrEntryPointNotFound:
		return "EntryPointNotFound"
	default:
		return "Unknown"
	}
}

// Span represents a source location for error reporting.
type Span struct {
	// Start is the byte offset of the span start.
	Start uint32

	// End is the byte offset of the span end.
	End uint32
}

// Error represents an HLSL compilation error.
type Error struct {
	// Kind categorizes the error.
	Kind ErrorKind

	// Message provides details about the error.
	Message string

	// Span optionally identifies the source location.
	Span *Span
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Span != nil {
		return fmt.Sprintf("hlsl %s at [%d:%d]: %s", e.Kind, e.Span.Start, e.Span.End, e.Message)
	}
	return fmt.Sprintf("hlsl %s: %s", e.Kind, e.Message)
}

// IsUnsupportedFeature returns true if the error is ErrUnsupportedFeature.
func (e *Error) IsUnsupportedFeature() bool {
	return e.Kind == ErrUnsupportedFeature
}

// IsMissingBinding returns true if the error is ErrMissingBinding.
func (e *Error) IsMissingBinding() bool {
	return e.Kind == ErrMissingBinding
}

// IsInternalError returns true if the error is ErrInternalError.
func (e *Error) IsInternalError() bool {
	return e.Kind == ErrInternalError
}

// --- I/O types ---

// Io distinguishes input from output in entry point interfaces.
type Io int

const (
	// IoInput marks entry point inputs.
	IoInput Io = iota
	// IoOutput marks entry point outputs.
	IoOutput
)

// --- Writer type ---

// Writer generates HLSL source code from IR.
// Use Compile for the standard compilation workflow.
type Writer = codegen.Writer

// --- Translation info ---

// TranslationInfo contains metadata about the HLSL translation.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated HLSL names.
	EntryPointNames map[string]string

	// UsedFeatures indicates which shader features are used.
	UsedFeatures FeatureFlags

	// RequiredShaderModel is the minimum shader model needed for this shader.
	RequiredShaderModel ShaderModel

	// RegisterBindings maps resource names to their HLSL register bindings.
	RegisterBindings map[string]string

	// HelperFunctions lists any helper functions that were generated.
	HelperFunctions []string
}

// --- Keyword constants ---

// UnnamedIdentifier is the default name for empty identifiers.
const UnnamedIdentifier = "_unnamed"

// Naga helper function names.
const (
	NagaModfFunction               = "naga_modf"
	NagaFrexpFunction              = "naga_frexp"
	NagaExtractBitsFunction        = "naga_extractBits"
	NagaInsertBitsFunction         = "naga_insertBits"
	SamplerHeapVar                 = "_naga_sampler_heap"
	ComparisonSamplerHeapVar       = "_naga_comparison_sampler_heap"
	SampleExternalTextureFunction  = "_naga_sample_external_texture"
	NagaAbsFunction                = "_naga_abs"
	NagaDivFunction                = "naga_div"
	NagaModFunction                = "naga_mod"
	NagaNegFunction                = "_naga_neg"
	NagaF2I32Function              = "_naga_f2i32"
	NagaF2U32Function              = "_naga_f2u32"
	NagaF2I64Function              = "_naga_f2i64"
	NagaF2U64Function              = "_naga_f2u64"
	ImageLoadExternalFunction      = "_naga_image_load_external"
	ImageSampleBaseClampToEdgeFunc = "_naga_image_sample_base_clamp_to_edge"
	DynamicBufferOffsetsPrefix     = "__dynamic_buffer_offsets"
	ImageStorageLoadScalarWrapper  = "_naga_image_storage_load_scalar"
)

// --- Functions ---

// Compile generates HLSL source code from an IR module.
// Returns the HLSL source, translation info, or an error.
func Compile(module *ir.Module, options *Options) (string, *TranslationInfo, error) {
	copts := toCodegenOptions(options)
	src, cinfo, err := codegen.Compile(module, copts)
	if err != nil {
		return "", nil, err
	}
	info := fromCodegenTranslationInfo(cinfo)
	return src, &info, nil
}

// DefaultOptions returns sensible default options for HLSL generation.
func DefaultOptions() *Options {
	return &Options{
		ShaderModel: ShaderModel5_1,
		BindingMap:  make(map[ResourceBinding]BindTarget),
		SamplerHeapTargets: SamplerHeapBindTargets{
			StandardSamplers:   BindTarget{Space: 0, Register: 0},
			ComparisonSamplers: BindTarget{Space: 1, Register: 0},
		},
		FakeMissingBindings:           true,
		ZeroInitializeWorkgroupMemory: true,
		RestrictIndexing:              true,
		ForceLoopBounding:             true,
	}
}

// DefaultBindTarget returns a BindTarget with default values.
func DefaultBindTarget() BindTarget {
	return BindTarget{
		Space:            0,
		Register:         0,
		BindingArraySize: nil,
	}
}

// NewError creates a new HLSL error without span information.
func NewError(kind ErrorKind, message string) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
		Span:    nil,
	}
}

// NewErrorWithSpan creates a new HLSL error with span information.
func NewErrorWithSpan(kind ErrorKind, message string, start, end uint32) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
		Span:    &Span{Start: start, End: end},
	}
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

// --- Internal conversion ---

// toCodegenOptions converts public Options to internal codegen Options.
func toCodegenOptions(o *Options) *codegen.Options {
	if o == nil {
		return nil
	}

	var bindingMap map[codegen.ResourceBinding]codegen.BindTarget
	if o.BindingMap != nil {
		bindingMap = make(map[codegen.ResourceBinding]codegen.BindTarget, len(o.BindingMap))
		for k, v := range o.BindingMap {
			bindingMap[codegen.ResourceBinding{Group: k.Group, Binding: k.Binding}] = toCodegenBindTarget(v)
		}
	}

	var samplerBufferBindingMap map[uint32]codegen.BindTarget
	if o.SamplerBufferBindingMap != nil {
		samplerBufferBindingMap = make(map[uint32]codegen.BindTarget, len(o.SamplerBufferBindingMap))
		for k, v := range o.SamplerBufferBindingMap {
			samplerBufferBindingMap[k] = toCodegenBindTarget(v)
		}
	}

	var extTexMap codegen.ExternalTextureBindingMap
	if o.ExternalTextureBindingMap != nil {
		extTexMap = make(codegen.ExternalTextureBindingMap, len(o.ExternalTextureBindingMap))
		for k, v := range o.ExternalTextureBindingMap {
			extTexMap[codegen.ResourceBinding{Group: k.Group, Binding: k.Binding}] = codegen.ExternalTextureBindTarget{
				Planes: [3]codegen.BindTarget{toCodegenBindTarget(v.Planes[0]), toCodegenBindTarget(v.Planes[1]), toCodegenBindTarget(v.Planes[2])},
				Params: toCodegenBindTarget(v.Params),
			}
		}
	}

	var dynamicOffsets map[uint32]codegen.OffsetsBindTarget
	if o.DynamicStorageBufferOffsetsTargets != nil {
		dynamicOffsets = make(map[uint32]codegen.OffsetsBindTarget, len(o.DynamicStorageBufferOffsetsTargets))
		for k, v := range o.DynamicStorageBufferOffsetsTargets {
			dynamicOffsets[k] = codegen.OffsetsBindTarget{
				Space:    v.Space,
				Register: v.Register,
				Size:     v.Size,
			}
		}
	}

	var specialBinding *codegen.BindTarget
	if o.SpecialConstantsBinding != nil {
		bt := toCodegenBindTarget(*o.SpecialConstantsBinding)
		specialBinding = &bt
	}

	var fragEP *codegen.FragmentEntryPoint
	if o.FragmentEntryPoint != nil {
		fragEP = &codegen.FragmentEntryPoint{
			Module:   o.FragmentEntryPoint.Module,
			Function: o.FragmentEntryPoint.Function,
		}
	}

	return &codegen.Options{
		ShaderModel:                        codegen.ShaderModel(o.ShaderModel),
		BindingMap:                         bindingMap,
		SamplerHeapTargets:                 toCodegenSamplerHeapTargets(o.SamplerHeapTargets),
		SamplerBufferBindingMap:            samplerBufferBindingMap,
		ExternalTextureBindingMap:          extTexMap,
		FakeMissingBindings:                o.FakeMissingBindings,
		ZeroInitializeWorkgroupMemory:      o.ZeroInitializeWorkgroupMemory,
		RestrictIndexing:                   o.RestrictIndexing,
		ForceLoopBounding:                  o.ForceLoopBounding,
		DynamicStorageBufferOffsetsTargets: dynamicOffsets,
		SpecialConstantsBinding:            specialBinding,
		EntryPoint:                         o.EntryPoint,
		FragmentEntryPoint:                 fragEP,
	}
}

// toCodegenBindTarget converts public BindTarget to codegen BindTarget.
func toCodegenBindTarget(bt BindTarget) codegen.BindTarget {
	return codegen.BindTarget{
		Space:                            bt.Space,
		Register:                         bt.Register,
		BindingArraySize:                 bt.BindingArraySize,
		DynamicStorageBufferOffsetsIndex: bt.DynamicStorageBufferOffsetsIndex,
		RestrictIndexing:                 bt.RestrictIndexing,
	}
}

// toCodegenSamplerHeapTargets converts public SamplerHeapBindTargets to codegen type.
func toCodegenSamplerHeapTargets(t SamplerHeapBindTargets) codegen.SamplerHeapBindTargets {
	return codegen.SamplerHeapBindTargets{
		StandardSamplers:   toCodegenBindTarget(t.StandardSamplers),
		ComparisonSamplers: toCodegenBindTarget(t.ComparisonSamplers),
	}
}

// fromCodegenTranslationInfo converts internal codegen TranslationInfo to public type.
func fromCodegenTranslationInfo(ci *codegen.TranslationInfo) TranslationInfo {
	if ci == nil {
		return TranslationInfo{}
	}
	return TranslationInfo{
		EntryPointNames:     ci.EntryPointNames,
		UsedFeatures:        FeatureFlags(ci.UsedFeatures),
		RequiredShaderModel: ShaderModel(ci.RequiredShaderModel),
		RegisterBindings:    ci.RegisterBindings,
		HelperFunctions:     ci.HelperFunctions,
	}
}
