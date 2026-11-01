// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package msl

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl/internal/codegen"
)

// Version represents an MSL language version.
type Version struct {
	Major uint8
	Minor uint8
}

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
	// If nil, bindings are auto-generated: the module's resource globals,
	// sorted by (group, binding), get consecutive buffer, texture and sampler
	// slots. These slots depend on which globals the module has, so
	// naga.ProcessOverrides with an EntryPoint, which removes unused globals,
	// can move them. Pass a map for stable slots (Rust naga requires one).
	// A non-nil map, empty too, must give every resource an entry point uses
	// a slot of its kind in that entry point's entry, and the sizes and
	// immediates buffers it needs; an entry point without an entry has none.
	// Otherwise Compile fails with an error wrapping
	// backend.ErrMissingBinding, unless FakeMissingBindings is set.
	PerEntryPointMap map[string]EntryPointResources

	// InlineSamplers defines constexpr samplers to be inlined into the code.
	// Referenced by BindSamplerTarget.Slot when IsInline is true.
	InlineSamplers []InlineSampler

	// BoundsCheckPolicies selects how out-of-bounds indices and image loads
	// are handled. The zero value is backend.BoundsCheckRestrict everywhere,
	// which WebGPU requires; Rust naga's default is Unchecked, and wgpu-hal's
	// Metal backend passes ReadZeroSkipWrite (DefaultBoundsCheckPolicies).
	// Every field honors every policy, as Rust naga's MSL writer does:
	// Restrict with metal::min on the index (a runtime-sized array takes its
	// length from the sizes buffer) and on image coordinates,
	// ReadZeroSkipWrite with a ternary or an if around the access,
	// Unchecked with no code.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// threadgroup variables they use, as WebGPU requires. The zero value,
	// backend.ZeroInitWorkgroupPolyfill, stores zero to them from the first
	// invocation and adds a threadgroup barrier, as Rust naga's default
	// zero_initialize_workgroup_memory: true does. ZeroInitWorkgroupNone
	// leaves them as they are; ZeroInitWorkgroupNative is an error, Metal
	// has no way to ask for it.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// SkipLoopBounding leaves loops as they are. By default every loop gets
	// a decrementing 64-bit counter that breaks out of it, so an infinite
	// loop, undefined behavior that can hang the device, ends. Rust naga's
	// force_loop_bounding defaults to true, and wgpu-hal passes true.
	SkipLoopBounding bool

	// FakeMissingBindings writes a placeholder attribute, [[user(fake0)]],
	// for a resource a non-nil PerEntryPointMap gives no slot, instead of
	// failing. Rust naga's fake_missing_bindings defaults to true; wgpu-hal
	// passes false, the zero value here.
	FakeMissingBindings bool

	// EntryPoint selects the one entry point to write, as Rust naga's
	// msl::PipelineOptions::entry_point; nil writes every entry point. One
	// the module does not have is an error wrapping
	// backend.ErrEntryPointNotFound.
	EntryPoint *backend.EntryPointSelector

	// AllowAndForcePointSize forces point size output for vertex shaders.
	AllowAndForcePointSize bool

	// VertexPulling, when set, makes vertex entry points read their inputs
	// from vertex buffers instead of stage_in (Rust naga's
	// vertex_pulling_transform with its vertex_buffer_mappings).
	VertexPulling *VertexPullingOptions

	// ClampFragDepth, when set, clamps the depth a fragment entry point
	// writes to the range in two f32, min then max, at byte offset
	// ClampFragDepth.Offset of the entry point's immediates buffer
	// (EntryPointResources.ImmediatesBuffer), which it then needs. WebGPU
	// clamps frag_depth to the viewport's depth range; the caller puts the
	// viewport's minDepth and maxDepth there, where Dawn's ClampFragDepth
	// transform reads them on Metal, and as the spirv option of the same
	// name reads them from push constants. An entry point with immediate
	// data of its own finds the range in the same buffer, so Offset has to
	// be past that data. Rust naga has no such option.
	ClampFragDepth *backend.ClampFragDepth

	// FixedSampleMask, when set, is the render pipeline's sample mask, which
	// a fragment entry point applies itself: it ANDs it into the sample_mask
	// it returns, or returns it as a [[sample_mask]] output when it returns
	// none. Metal has no pipeline sample mask; Dawn folds it into the shader
	// the same way. Leave it nil for a mask of all ones. Rust naga has no
	// such option.
	FixedSampleMask *uint32
}

// VertexFormat describes the format of a vertex attribute.
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

// VertexPullingOptions configures vertex pulling: the vertex buffers vertex
// entry points read their inputs from.
type VertexPullingOptions struct {
	// VertexBuffers describes the vertex buffers and their attributes.
	VertexBuffers []VertexBufferMapping
}

// TranslationInfo contains information about the compiled MSL output.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated MSL names.
	EntryPointNames map[string]string

	// RequiresSizesBuffer indicates if a sizes buffer is needed for
	// runtime-sized arrays.
	RequiresSizesBuffer bool
}

// DefaultBoundsCheckPolicies returns ReadZeroSkipWrite on every field, the
// policies of DefaultOptions and of wgpu's native default.
func DefaultBoundsCheckPolicies() backend.BoundsCheckPolicies {
	return backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckReadZeroSkipWrite,
		Buffer:       backend.BoundsCheckReadZeroSkipWrite,
		ImageLoad:    backend.BoundsCheckReadZeroSkipWrite,
		BindingArray: backend.BoundsCheckReadZeroSkipWrite,
	}
}

// DefaultOptions returns sensible default options for MSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:         Version2_1,
		BoundsCheckPolicies: DefaultBoundsCheckPolicies(),
	}
}

// Compile generates MSL source code from an IR module.
// Returns the MSL source as a string and translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	// The writer names identifiers over the whole module and writes the
	// selected entry point only (Rust naga's PipelineOptions.entry_point).
	if _, err := entrypoint.Index(module, options.EntryPoint, false); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}
	copts := toCodegenOptions(options)
	cpipe := codegen.PipelineOptions{EntryPoint: options.EntryPoint, AllowAndForcePointSize: options.AllowAndForcePointSize}
	src, cinfo, err := codegen.CompileWithPipeline(module, copts, cpipe)
	if err != nil {
		return "", TranslationInfo{}, err
	}
	return src, fromCodegenTranslationInfo(cinfo), nil
}

// toCodegenOptions converts public Options to internal codegen Options.
func toCodegenOptions(o Options) codegen.Options {
	var perEntryPointMap map[string]codegen.EntryPointResources
	if o.PerEntryPointMap != nil {
		perEntryPointMap = make(map[string]codegen.EntryPointResources, len(o.PerEntryPointMap))
		for name, epr := range o.PerEntryPointMap {
			perEntryPointMap[name] = toCodegenEntryPointResources(epr)
		}
	}

	var inlineSamplers []codegen.InlineSampler
	if o.InlineSamplers != nil {
		inlineSamplers = make([]codegen.InlineSampler, len(o.InlineSamplers))
		for i, s := range o.InlineSamplers {
			inlineSamplers[i] = toCodegenInlineSampler(s)
		}
	}

	var vbMappings []codegen.VertexBufferMapping
	vertexPulling := o.VertexPulling != nil
	if vertexPulling && o.VertexPulling.VertexBuffers != nil {
		vbMappings = make([]codegen.VertexBufferMapping, len(o.VertexPulling.VertexBuffers))
		for i, m := range o.VertexPulling.VertexBuffers {
			attrs := make([]codegen.AttributeMapping, len(m.Attributes))
			for j, a := range m.Attributes {
				attrs[j] = codegen.AttributeMapping{
					ShaderLocation: a.ShaderLocation,
					Offset:         a.Offset,
					Format:         codegen.VertexFormat(a.Format),
				}
			}
			vbMappings[i] = codegen.VertexBufferMapping{
				ID:         m.ID,
				Stride:     m.Stride,
				StepMode:   codegen.VertexBufferStepMode(m.StepMode),
				Attributes: attrs,
			}
		}
	}

	return codegen.Options{
		LangVersion: codegen.Version{
			Major: o.LangVersion.Major,
			Minor: o.LangVersion.Minor,
		},
		PerEntryPointMap:              perEntryPointMap,
		InlineSamplers:                inlineSamplers,
		BoundsCheckPolicies:           o.BoundsCheckPolicies,
		ZeroInitializeWorkgroupMemory: o.ZeroInitializeWorkgroupMemory,
		ForceLoopBounding:             !o.SkipLoopBounding,
		FakeMissingBindings:           o.FakeMissingBindings,
		AllowAndForcePointSize:        o.AllowAndForcePointSize,
		VertexPullingTransform:        vertexPulling,
		VertexBufferMappings:          vbMappings,
		ClampFragDepth:                o.ClampFragDepth,
		FixedSampleMask:               o.FixedSampleMask,
	}
}

// toCodegenEntryPointResources converts public EntryPointResources to codegen type.
func toCodegenEntryPointResources(epr EntryPointResources) codegen.EntryPointResources {
	var resources map[ir.ResourceBinding]codegen.BindTarget
	if epr.Resources != nil {
		resources = make(map[ir.ResourceBinding]codegen.BindTarget, len(epr.Resources))
		for k, v := range epr.Resources {
			resources[k] = toCodegenBindTarget(v)
		}
	}
	return codegen.EntryPointResources{
		Resources:          resources,
		PushConstantBuffer: epr.PushConstantBuffer,
		SizesBuffer:        epr.SizesBuffer,
		ImmediatesBuffer:   epr.ImmediatesBuffer,
	}
}

// toCodegenBindTarget converts public BindTarget to codegen type.
func toCodegenBindTarget(bt BindTarget) codegen.BindTarget {
	var sampler *codegen.BindSamplerTarget
	if bt.Sampler != nil {
		sampler = &codegen.BindSamplerTarget{
			IsInline: bt.Sampler.IsInline,
			Slot:     bt.Sampler.Slot,
		}
	}
	var extTex *codegen.BindExternalTextureTarget
	if bt.ExternalTexture != nil {
		extTex = &codegen.BindExternalTextureTarget{
			Planes: bt.ExternalTexture.Planes,
			Params: bt.ExternalTexture.Params,
		}
	}
	return codegen.BindTarget{
		Buffer:          bt.Buffer,
		Texture:         bt.Texture,
		Sampler:         sampler,
		ExternalTexture: extTex,
		Mutable:         bt.Mutable,
	}
}

// toCodegenInlineSampler converts public InlineSampler to codegen type.
func toCodegenInlineSampler(s InlineSampler) codegen.InlineSampler {
	var mipFilter *codegen.SamplerFilter
	if s.MipFilter != nil {
		f := codegen.SamplerFilter(*s.MipFilter)
		mipFilter = &f
	}
	return codegen.InlineSampler{
		Coord:       codegen.SamplerCoord(s.Coord),
		Address:     [3]codegen.SamplerAddress{codegen.SamplerAddress(s.Address[0]), codegen.SamplerAddress(s.Address[1]), codegen.SamplerAddress(s.Address[2])},
		BorderColor: codegen.SamplerBorderColor(s.BorderColor),
		MagFilter:   codegen.SamplerFilter(s.MagFilter),
		MinFilter:   codegen.SamplerFilter(s.MinFilter),
		MipFilter:   mipFilter,
		CompareFunc: codegen.SamplerCompareFunc(s.CompareFunc),
	}
}

// fromCodegenTranslationInfo converts internal codegen TranslationInfo to public type.
func fromCodegenTranslationInfo(ci codegen.TranslationInfo) TranslationInfo {
	return TranslationInfo{
		EntryPointNames:     ci.EntryPointNames,
		RequiresSizesBuffer: ci.RequiresSizesBuffer,
	}
}
