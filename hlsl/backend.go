// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"

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

	// FakeMissingBindings generates automatic bindings for resources
	// not found in BindingMap. Useful for testing or simple shaders.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory emits code to zero-initialize
	// groupshared variables at the start of compute shaders.
	// Required for portability as HLSL doesn't guarantee zero initialization.
	ZeroInitializeWorkgroupMemory bool

	// RestrictIndexing adds bounds checks to array/buffer accesses.
	// Prevents undefined behavior from out-of-bounds reads/writes.
	RestrictIndexing bool

	// ForceLoopBounding adds maximum iteration limits to loops.
	// Prevents infinite loops that could hang the GPU.
	ForceLoopBounding bool

	// EntryPoint specifies which entry point to compile.
	// If empty, the first entry point is used.
	EntryPoint string
}

// DefaultOptions returns sensible default options for HLSL generation.
// Uses Shader Model 5.1 with safe defaults enabled.
func DefaultOptions() *Options {
	return &Options{
		ShaderModel:                   ShaderModel5_1,
		BindingMap:                    make(map[ResourceBinding]BindTarget),
		FakeMissingBindings:           true,
		ZeroInitializeWorkgroupMemory: true,
		RestrictIndexing:              false,
		ForceLoopBounding:             false,
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

	// Create writer
	w := newWriter(module, options)

	// Generate HLSL code
	if err := w.writeModule(); err != nil {
		return "", nil, fmt.Errorf("hlsl: %w", err)
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
