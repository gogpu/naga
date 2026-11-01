// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/ir"
)

// Options configures HLSL code generation.
type Options struct {
	// ShaderModel specifies the target shader model.
	// Defaults to ShaderModel5_1 for maximum compatibility.
	ShaderModel ShaderModel

	// BindingMap maps source resource bindings to HLSL register targets.
	// If a binding is not found in the map and FakeMissingBindings is false,
	// compilation will fail with ErrMissingBinding.
	BindingMap map[ResourceBinding]BindTarget

	// SamplerHeapTargets specifies binding targets for sampler heaps.
	// Used with SM 6.6+ bindless resources.
	SamplerHeapTargets SamplerHeapBindTargets

	// SamplerBufferBindingMap maps group numbers to bind targets for
	// sampler index buffers (StructuredBuffer<uint>). When a sampler is
	// encountered, the sampler heap arrays and corresponding index buffer
	// are written. Matches Rust naga's sampler_buffer_binding_map.
	SamplerBufferBindingMap map[uint32]BindTarget

	// ExternalTextureBindingMap maps resource bindings to external texture
	// bind targets. External textures are decomposed into 3 plane textures
	// and a parameters cbuffer.
	// Matches Rust naga's Options::external_texture_binding_map.
	ExternalTextureBindingMap ExternalTextureBindingMap

	// FakeMissingBindings generates automatic bindings for resources
	// not found in BindingMap. Useful for testing or simple shaders.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// groupshared variables they use: Polyfill (zero) or None; Native is an
	// error.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// BoundsCheckPolicies selects the bounds checks; see
	// hlsl.Options.BoundsCheckPolicies.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// ForceLoopBounding adds maximum iteration limits to loops.
	// Prevents infinite loops that could hang the GPU.
	ForceLoopBounding bool

	// DynamicStorageBufferOffsetsTargets maps group indices to their bind targets
	// for dynamic storage buffer offset constant buffers. When a storage buffer
	// binding has DynamicStorageBufferOffsetsIndex set, the generated HLSL adds
	// the dynamic offset from the corresponding constant buffer to Load/Store addresses.
	// Matches Rust naga's Options::dynamic_storage_buffer_offsets_targets.
	DynamicStorageBufferOffsetsTargets map[uint32]OffsetsBindTarget

	// SpecialConstantsBinding specifies the binding for the NagaConstants
	// constant buffer. When set, the HLSL output includes a NagaConstants
	// struct with first_vertex, first_instance, and other fields. Vertex
	// and instance indices are offset by these values, and NumWorkGroups
	// is replaced with a uint3 from these values.
	// Matches Rust naga's special_constants_binding option.
	SpecialConstantsBinding *BindTarget

	// ImmediatesTarget is the constant buffer register of the immediate
	// data (Rust naga's immediates_target): the root constants D3D12 binds
	// directly in the root signature. A module with immediate data needs it.
	ImmediatesTarget *BindTarget

	// EntryPoint selects the one entry point to write; nil writes every
	// entry point.
	EntryPoint *backend.EntryPointSelector

	// FragmentEntryPoint specifies a fragment entry point to consider when
	// generating the output interface of vertex entry points.
	// If provided, vertex outputs not consumed by this fragment shader's
	// inputs will be stripped from the vertex output struct.
	// Matches Rust naga's FragmentEntryPoint.
	FragmentEntryPoint *FragmentEntryPoint
}

// FragmentEntryPoint describes a fragment entry point used to filter
// vertex shader outputs. Only vertex outputs that match fragment inputs
// (by location) are kept in the vertex output struct.
type FragmentEntryPoint struct {
	// Module is the IR module containing the fragment entry point.
	Module *ir.Module
	// Function is the fragment entry point function.
	Function *ir.Function
}

// DefaultOptions returns sensible default options for HLSL generation.
// Uses Shader Model 5.1 with safe defaults enabled.
// Matches Rust naga's Default for Options.
func DefaultOptions() *Options {
	return &Options{
		ShaderModel: ShaderModel5_1,
		BindingMap:  make(map[ResourceBinding]BindTarget),
		SamplerHeapTargets: SamplerHeapBindTargets{
			StandardSamplers:   BindTarget{Space: 0, Register: 0},
			ComparisonSamplers: BindTarget{Space: 1, Register: 0},
		},
		FakeMissingBindings: true,
		ForceLoopBounding:   true,
	}
}

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

// TranslationInfo contains metadata about the HLSL translation.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated HLSL names.
	// HLSL requires "main" for the entry point in single-shader compilation.
	EntryPointNames map[string]string

	// UsedFeatures indicates which shader features are used.
	UsedFeatures FeatureFlags

	// RequiredShaderModel is the minimum shader model needed for this shader.
	// May be higher than the requested model if features require it.
	RequiredShaderModel ShaderModel

	// RegisterBindings maps resource names to their HLSL register bindings.
	// Format: "resourceName" -> "register(t0, space0)"
	RegisterBindings map[string]string

	// HelperFunctions lists any helper functions that were generated.
	HelperFunctions []string
}

// checkBoundsCheckPolicies refuses the policies the HLSL writer cannot honor:
// a value outside the enum, and ReadZeroSkipWrite on Index, Buffer and
// BindingArray, which it does not implement (Rust naga's HLSL writer has only
// restrict_indexing). ImageLoad takes any value: Direct3D returns zero for an
// out-of-bounds Load.
func checkBoundsCheckPolicies(p backend.BoundsCheckPolicies) error {
	for _, policy := range []backend.BoundsCheckPolicy{p.Index, p.Buffer, p.ImageLoad, p.BindingArray} {
		if policy > backend.BoundsCheckUnchecked {
			return fmt.Errorf("BoundsCheckPolicies: %v is not supported", policy)
		}
	}
	for _, f := range []struct {
		name   string
		policy backend.BoundsCheckPolicy
	}{{"Index", p.Index}, {"Buffer", p.Buffer}, {"BindingArray", p.BindingArray}} {
		if f.policy == backend.BoundsCheckReadZeroSkipWrite {
			return fmt.Errorf("BoundsCheckPolicies.%s: ReadZeroSkipWrite is not supported; use Restrict or Unchecked", f.name)
		}
	}
	return nil
}

// checkShaderModel refuses what Options.ShaderModel cannot express, as Rust
// naga's ShaderModelTooLow (back/hlsl/writer.rs): a mesh or task entry point
// below 6.5, and the view index (SV_ViewID), barycentrics (SV_Barycentrics)
// or a per-vertex input (GetAttributeAtVertex) below 6.1, with a
// *backend.ShaderModelError.
func checkShaderModel(module *ir.Module, options *Options) error {
	tooLow := func(feature string, required ShaderModel) error {
		return fmt.Errorf("hlsl: %w", &backend.ShaderModelError{
			Feature:   feature,
			Required:  backend.ShaderModel{Major: required.Major(), Minor: required.Minor()},
			Requested: backend.ShaderModel{Major: options.ShaderModel.Major(), Minor: options.ShaderModel.Minor()},
		})
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if !selects(options.EntryPoint, ep) {
			continue
		}
		if (ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask) && options.ShaderModel < ShaderModel6_5 {
			return tooLow("a mesh or task shader", ShaderModel6_5)
		}
		// SV_ViewID and SV_Barycentrics are shader model 6.1 semantics; Rust
		// checks only the view index and writes SV_Barycentrics at any model.
		feature := func(b *ir.Binding) string {
			if b == nil {
				return ""
			}
			if isPerVertexBinding(b) {
				return "per_vertex fragment inputs" // GetAttributeAtVertex
			}
			bi, ok := (*b).(ir.BuiltinBinding)
			switch {
			case !ok:
				return ""
			case bi.Builtin == ir.BuiltinViewIndex:
				return "the view index"
			case bi.Builtin == ir.BuiltinBarycentric, bi.Builtin == ir.BuiltinBarycentricNoPerspective:
				return "barycentrics"
			}
			return ""
		}
		for _, arg := range ep.Function.Arguments {
			bindings := []*ir.Binding{arg.Binding}
			if st, ok := module.Types[arg.Type].Inner.(ir.StructType); ok {
				for j := range st.Members {
					bindings = append(bindings, st.Members[j].Binding)
				}
			}
			for _, b := range bindings {
				if f := feature(b); f != "" && options.ShaderModel < ShaderModel6_1 {
					return tooLow(f, ShaderModel6_1)
				}
			}
		}
	}
	return nil
}

// checkBuiltins refuses an entry point whose interface uses a built-in HLSL
// has no semantic for (draw_index, point_size: Rust naga's "Unsupported
// builtin"); the old fallback wrote SV_Position for them.
func checkBuiltins(module *ir.Module, options *Options) error {
	check := func(ep *ir.EntryPoint, b *ir.Binding) error {
		// Subgroup built-ins map to wave intrinsics, never to a semantic.
		if b == nil || isSubgroupBuiltinBinding(b) {
			return nil
		}
		if bi, ok := (*b).(ir.BuiltinBinding); ok && BuiltInToSemantic(bi.Builtin) == "" {
			return NewError(ErrUnsupportedFeature, fmt.Sprintf("entry point %q: built-in %d has no HLSL semantic", ep.Name, bi.Builtin))
		}
		return nil
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if !selects(options.EntryPoint, ep) {
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
		if res := ep.Function.Result; res != nil {
			bindings = append(bindings, res.Binding)
			if st, ok := module.Types[res.Type].Inner.(ir.StructType); ok {
				for j := range st.Members {
					bindings = append(bindings, st.Members[j].Binding)
				}
			}
		}
		for _, b := range bindings {
			if err := check(ep, b); err != nil {
				return err
			}
		}
	}
	return nil
}

// selects reports whether sel selects ep; a nil sel selects every entry point.
func selects(sel *backend.EntryPointSelector, ep *ir.EntryPoint) bool {
	return sel == nil || (ep.Name == sel.Name && ep.Stage == sel.Stage)
}

// checkNumWorkgroups refuses an entry point that reads num_workgroups
// without SpecialConstantsBinding: HLSL has no system value for it, and the
// value comes from the special constants buffer. Rust naga writes it as
// SV_GroupID then, the workgroup's ID instead of the count; that is never
// right, so FakeMissingBindings does not allow it either.
func checkNumWorkgroups(module *ir.Module, options *Options) error {
	if options.SpecialConstantsBinding != nil {
		return nil
	}
	isNumWorkgroups := func(b *ir.Binding) bool {
		if b == nil {
			return false
		}
		bi, ok := (*b).(ir.BuiltinBinding)
		return ok && bi.Builtin == ir.BuiltinNumWorkGroups
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if !selects(options.EntryPoint, ep) {
			continue
		}
		for _, arg := range ep.Function.Arguments {
			found := isNumWorkgroups(arg.Binding)
			if st, ok := module.Types[arg.Type].Inner.(ir.StructType); ok {
				for _, m := range st.Members {
					found = found || isNumWorkgroups(m.Binding)
				}
			}
			if found {
				return NewError(ErrMissingBinding, fmt.Sprintf(
					"entry point %q reads num_workgroups, which needs SpecialConstantsBinding", ep.Name))
			}
		}
	}
	return nil
}

// Compile generates HLSL source code from an IR module.
// Returns the HLSL source, translation info, or an error.
func Compile(module *ir.Module, options *Options) (string, *TranslationInfo, error) {
	if module == nil {
		return "", nil, &Error{
			Kind:    ErrInternalError,
			Message: "module is nil",
		}
	}

	// Apply defaults for nil options
	if options == nil {
		options = DefaultOptions()
	}
	if m := options.ZeroInitializeWorkgroupMemory; m != backend.ZeroInitWorkgroupPolyfill && m != backend.ZeroInitWorkgroupNone {
		return "", nil, fmt.Errorf("hlsl: ZeroInitializeWorkgroupMemory %v is not supported", m)
	}
	if err := checkBoundsCheckPolicies(options.BoundsCheckPolicies); err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
	}
	if _, err := entrypoint.Index(module, options.EntryPoint, false); err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
	}
	if err := checkShaderModel(module, options); err != nil {
		return "", nil, err
	}
	if err := checkBuiltins(module, options); err != nil {
		return "", nil, err
	}
	if err := checkNumWorkgroups(module, options); err != nil {
		return "", nil, err
	}

	// Create writer
	w := newWriter(module, options)

	// Generate HLSL code
	if err := w.writeModule(); err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
	}
	if w.bindingErr != nil {
		return "", nil, w.bindingErr
	}

	info := &TranslationInfo{
		EntryPointNames:     w.entryPointNames,
		UsedFeatures:        w.usedFeatures,
		RequiredShaderModel: w.requiredShaderModel,
		RegisterBindings:    w.registerBindings,
		HelperFunctions:     w.helperFunctions,
	}

	return w.String(), info, nil
}
