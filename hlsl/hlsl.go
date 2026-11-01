// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl/internal/codegen"
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

// ExternalTextureBindTarget specifies HLSL binding information for an external
// texture global variable.
type ExternalTextureBindTarget struct {
	// Planes contains the bind targets for the 3 plane textures.
	Planes [3]BindTarget
	// Params is the bind target for the parameters cbuffer.
	Params BindTarget
}

// ExternalTextureBindingMap maps resource bindings to external texture bind targets.
type ExternalTextureBindingMap map[ir.ResourceBinding]ExternalTextureBindTarget

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
	BindingMap map[ir.ResourceBinding]BindTarget

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

// TranslationInfo describes what Compile wrote (Rust naga's hlsl::ReflectionInfo).
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated HLSL names.
	EntryPointNames map[string]string
}

// --- Functions ---

// Compile generates HLSL source code from an IR module.
// Returns the HLSL source, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	if err := ir.CheckWorkgroupSizesResolved(module); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("hlsl: %w", err)
	}
	module, err := ir.ResolvePendingArraySizes(module)
	if err != nil {
		return "", TranslationInfo{}, fmt.Errorf("hlsl: %w", err)
	}
	copts := toCodegenOptions(&options)
	if copts.ShaderModel, err = toCodegenShaderModel(options.ShaderModel); err != nil {
		return "", TranslationInfo{}, err
	}
	src, cinfo, err := codegen.Compile(module, copts)
	if err != nil {
		return "", TranslationInfo{}, err
	}
	return src, fromCodegenTranslationInfo(cinfo), nil
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

// DefaultOptions returns the zero Options, the safe defaults.
func DefaultOptions() Options {
	return Options{}
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
	return TranslationInfo{EntryPointNames: ci.EntryPointNames}
}
