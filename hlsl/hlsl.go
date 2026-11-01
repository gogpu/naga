// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl/internal/codegen"
	"github.com/gogpu/naga/internal/d3d"
	"github.com/gogpu/naga/ir"
)

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

	// DynamicStorageBufferSizeIndex is the index into the same constant
	// buffer of the binding's size in bytes, which arrayLength() counts the
	// elements of. A binding with a dynamic offset is viewed to the end of
	// its buffer, which arrayLength() would count otherwise, past the
	// binding and without the offset. Rust naga has no such field.
	DynamicStorageBufferSizeIndex *uint32

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
	// ShaderModel is the shader model the HLSL targets: 5.0, 5.1 or 6.0 to
	// 6.7. The zero value is 5.1, as Rust naga's default; another
	// version is an error.
	ShaderModel backend.ShaderModel

	// BindingMap maps source resource bindings to HLSL register targets.
	// A nil map puts every resource at register N, spaceG for its WGSL
	// @group(G) @binding(N). A non-nil map, empty too, must list every
	// resource the module writes: one without an entry is an error wrapping
	// backend.ErrMissingBinding, unless FakeMissingBindings is set.
	BindingMap map[ResourceBinding]BindTarget

	// SamplerHeapTargets places the sampler heaps samplers are read from (as
	// Rust naga and wgpu-hal's DX12 backend bind samplers). Nil places the
	// standard samplers at register(s0, space0) and the comparison samplers
	// at register(s0, space1), as Rust naga's default; the zero value of the
	// struct would put both heaps in one register.
	SamplerHeapTargets *SamplerHeapBindTargets

	// SamplerBufferBindingMap maps group numbers to bind targets for
	// sampler index buffers. A nil map puts the buffer of group G at
	// register(tG, space255), Rust naga's fallback; a non-nil map without
	// the group is an error wrapping backend.ErrMissingBinding, unless
	// FakeMissingBindings is set.
	SamplerBufferBindingMap map[uint32]BindTarget

	// ExternalTextureBindingMap maps resource bindings to external texture
	// bind targets: three planes and a parameters buffer each. Every
	// texture_external needs an entry, a nil map too: there is no default
	// for four registers. One without an entry is an error wrapping
	// backend.ErrMissingBinding, unless FakeMissingBindings places it at
	// t0-t2 and b3 as Rust naga does.
	ExternalTextureBindingMap ExternalTextureBindingMap

	// FakeMissingBindings places a resource that a non-nil BindingMap,
	// SamplerBufferBindingMap or ExternalTextureBindingMap leaves out where a
	// nil map would, instead of failing. Rust naga's fake_missing_bindings
	// defaults to true; wgpu-hal passes false, the zero value here.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// groupshared variables they use, as WebGPU requires. The zero value,
	// backend.ZeroInitWorkgroupPolyfill, stores zero to them from the first
	// invocation and adds a group barrier, as Rust naga's default
	// zero_initialize_workgroup_memory: true does. ZeroInitWorkgroupNone
	// leaves them as they are; ZeroInitWorkgroupNative is an error, Direct3D
	// has no way to ask for it.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// BoundsCheckPolicies selects how out-of-bounds indices and image loads
	// are handled. The zero value is backend.BoundsCheckRestrict everywhere,
	// which WebGPU requires; Rust naga's HLSL writer has one switch,
	// restrict_indexing (default true, what wgpu-hal passes), that Index and
	// Buffer here mirror. Per field:
	//   - Index (function, private, workgroup memory and values): Restrict
	//     clamps the index with min(); Unchecked adds nothing.
	//   - Buffer: Restrict clamps indices into a uniform buffer whose
	//     BindTarget.RestrictIndexing is set (Rust: for dynamic buffers). A
	//     storage buffer is a ByteAddressBuffer, which Direct3D bounds-checks
	//     when it is bound through a descriptor table (out-of-bounds reads
	//     return zero, writes are dropped), so it needs no code under any
	//     policy. A root descriptor is not bounds-checked: a caller that
	//     binds storage that way gets no check.
	//   - ImageLoad: Direct3D returns zero for an out-of-bounds Load, so every
	//     policy is provided by the platform and no code is written.
	//   - BindingArray: Restrict clamps the index to the array's size, or to
	//     BindTarget.BindingArraySize for an unbounded one (an error without
	//     it); Rust's HLSL writer never checks binding arrays.
	// ReadZeroSkipWrite on Index, Buffer or BindingArray is not supported and
	// makes Compile fail.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// SkipLoopBounding leaves loops as they are. By default every loop gets
	// a decrementing 64-bit counter that breaks out of it, so an infinite
	// loop, undefined behavior that can hang the device, ends. Rust naga's
	// force_loop_bounding defaults to true, and wgpu-hal passes true.
	SkipLoopBounding bool

	// DynamicStorageBufferOffsetsTargets maps group indices to their bind targets
	// for dynamic storage buffer offset constant buffers.
	DynamicStorageBufferOffsetsTargets map[uint32]OffsetsBindTarget

	// SpecialConstantsBinding specifies the binding for the NagaConstants
	// constant buffer.
	SpecialConstantsBinding *BindTarget

	// ImmediatesTarget is the constant buffer register of the immediate data
	// (var<immediate>), the root constants D3D12 binds in the root signature
	// (Rust naga's immediates_target). The data has no WGSL binding, so a
	// module with immediate data and no target is an error wrapping
	// backend.ErrMissingBinding, FakeMissingBindings or not.
	ImmediatesTarget *BindTarget

	// EntryPoint selects the one entry point to write; nil writes every entry
	// point. One the module does not have is an error wrapping
	// backend.ErrEntryPointNotFound.
	EntryPoint *backend.EntryPointSelector

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
	RequiredShaderModel backend.ShaderModel

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
	if err := ir.CheckWorkgroupSizesResolved(module); err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
	}
	module, err := ir.ResolvePendingArraySizes(module)
	if err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
	}
	copts := toCodegenOptions(options)
	if copts != nil {
		if copts.ShaderModel, err = toCodegenShaderModel(options.ShaderModel); err != nil {
			return "", nil, err
		}
	}
	src, cinfo, err := codegen.Compile(module, copts)
	if err != nil {
		return "", nil, err
	}
	info := fromCodegenTranslationInfo(cinfo)
	return src, &info, nil
}

// toCodegenShaderModel maps sm onto the HLSL writer's shader models; the
// zero value is 5.1, as Rust naga's default.
func toCodegenShaderModel(sm backend.ShaderModel) (codegen.ShaderModel, error) {
	if sm == (backend.ShaderModel{}) {
		return codegen.ShaderModel5_1, nil
	}
	for m := codegen.ShaderModel5_0; m <= codegen.ShaderModel6_7; m++ {
		if m.Major() == sm.Major && m.Minor() == sm.Minor {
			return m, nil
		}
	}
	return 0, fmt.Errorf("hlsl: shader model %v is not supported (5.0, 5.1 and 6.0 to 6.7 are)", sm)
}

// DefaultOptions returns sensible default options for HLSL generation.
func DefaultOptions() *Options {
	return &Options{}
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
	return d3d.NeedsTrailingUnderscore(name)
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

	var specialBinding, immediatesTarget *codegen.BindTarget
	if o.SpecialConstantsBinding != nil {
		bt := toCodegenBindTarget(*o.SpecialConstantsBinding)
		specialBinding = &bt
	}
	if o.ImmediatesTarget != nil {
		bt := toCodegenBindTarget(*o.ImmediatesTarget)
		immediatesTarget = &bt
	}

	var fragEP *codegen.FragmentEntryPoint
	if o.FragmentEntryPoint != nil {
		fragEP = &codegen.FragmentEntryPoint{
			Module:   o.FragmentEntryPoint.Module,
			Function: o.FragmentEntryPoint.Function,
		}
	}

	return &codegen.Options{
		BindingMap:                         bindingMap,
		SamplerHeapTargets:                 toCodegenSamplerHeapTargets(o.SamplerHeapTargets),
		SamplerBufferBindingMap:            samplerBufferBindingMap,
		ExternalTextureBindingMap:          extTexMap,
		FakeMissingBindings:                o.FakeMissingBindings,
		ZeroInitializeWorkgroupMemory:      o.ZeroInitializeWorkgroupMemory,
		BoundsCheckPolicies:                o.BoundsCheckPolicies,
		ForceLoopBounding:                  !o.SkipLoopBounding,
		DynamicStorageBufferOffsetsTargets: dynamicOffsets,
		SpecialConstantsBinding:            specialBinding,
		ImmediatesTarget:                   immediatesTarget,
		FragmentEntryPoint:                 fragEP,
		EntryPoint:                         o.EntryPoint,
	}
}

// toCodegenBindTarget converts public BindTarget to codegen BindTarget.
func toCodegenBindTarget(bt BindTarget) codegen.BindTarget {
	return codegen.BindTarget{
		Space:                            bt.Space,
		Register:                         bt.Register,
		BindingArraySize:                 bt.BindingArraySize,
		DynamicStorageBufferOffsetsIndex: bt.DynamicStorageBufferOffsetsIndex,
		DynamicStorageBufferSizeIndex:    bt.DynamicStorageBufferSizeIndex,
		RestrictIndexing:                 bt.RestrictIndexing,
	}
}

// toCodegenSamplerHeapTargets converts public SamplerHeapBindTargets to codegen type;
// nil is the default std (s0, space0) and comparison (s0, space1).
func toCodegenSamplerHeapTargets(t *SamplerHeapBindTargets) codegen.SamplerHeapBindTargets {
	if t == nil {
		return codegen.SamplerHeapBindTargets{
			StandardSamplers:   codegen.BindTarget{Space: 0, Register: 0},
			ComparisonSamplers: codegen.BindTarget{Space: 1, Register: 0},
		}
	}
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
		RequiredShaderModel: backend.ShaderModel{Major: ci.RequiredShaderModel.Major(), Minor: ci.RequiredShaderModel.Minor()},
		RegisterBindings:    ci.RegisterBindings,
		HelperFunctions:     ci.HelperFunctions,
	}
}
