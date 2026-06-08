// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"

	"github.com/gogpu/naga/glsl/internal/codegen"
	"github.com/gogpu/naga/ir"
)

// Version represents a GLSL version.
type Version struct {
	Major uint8
	Minor uint8
	ES    bool // true for GLSL ES (OpenGL ES / WebGL)
}

// String returns the version as a GLSL version directive value.
func (v Version) String() string {
	if v.ES {
		return fmt.Sprintf("%d%02d es", v.Major, v.Minor)
	}
	return fmt.Sprintf("%d%02d core", v.Major, v.Minor)
}

// VersionNumber returns just the numeric version (e.g., "330", "300").
func (v Version) VersionNumber() string {
	return fmt.Sprintf("%d%02d", v.Major, v.Minor)
}

// SupportsCompute returns true if this version supports compute shaders.
func (v Version) SupportsCompute() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// SupportsExplicitLocations returns true if explicit layout locations for bindings
// are supported (layout(binding=N) qualifiers).
// Desktop GLSL 420+, ES 310+.
// Matches Rust naga Version::supports_explicit_locations.
func (v Version) SupportsExplicitLocations() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 20)
}

// SupportsStorageBuffers returns true if this version supports storage buffers.
func (v Version) SupportsStorageBuffers() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// Common GLSL versions.
var (
	// Desktop OpenGL versions
	Version330 = Version{Major: 3, Minor: 30, ES: false} // OpenGL 3.3 Core
	Version400 = Version{Major: 4, Minor: 0, ES: false}  // OpenGL 4.0
	Version410 = Version{Major: 4, Minor: 10, ES: false} // OpenGL 4.1
	Version420 = Version{Major: 4, Minor: 20, ES: false} // OpenGL 4.2
	Version430 = Version{Major: 4, Minor: 30, ES: false} // OpenGL 4.3 (compute shaders)
	Version450 = Version{Major: 4, Minor: 50, ES: false} // OpenGL 4.5
	Version460 = Version{Major: 4, Minor: 60, ES: false} // OpenGL 4.6

	// OpenGL ES / WebGL versions
	VersionES300 = Version{Major: 3, Minor: 0, ES: true}  // ES 3.0 / WebGL 2.0
	VersionES310 = Version{Major: 3, Minor: 10, ES: true} // ES 3.1 (compute shaders)
	VersionES320 = Version{Major: 3, Minor: 20, ES: true} // ES 3.2
)

// WriterFlags control output formatting.
type WriterFlags uint32

// Writer flag constants.
const (
	// WriterFlagNone uses default settings.
	WriterFlagNone WriterFlags = 0

	// WriterFlagExplicitTypes forces explicit type annotations.
	WriterFlagExplicitTypes WriterFlags = 1 << iota

	// WriterFlagDebugInfo adds source comments for debugging.
	WriterFlagDebugInfo

	// WriterFlagMinify removes unnecessary whitespace.
	WriterFlagMinify

	// WriterFlagAdjustCoordinateSpace adds gl_Position coordinate adjustment
	// at the end of vertex shaders to convert from Vulkan/WebGPU conventions
	// (Y-down, Z in [0,1]) to OpenGL conventions (Y-up, Z in [-1,1]).
	WriterFlagAdjustCoordinateSpace

	// WriterFlagForcePointSize emits gl_PointSize = 1.0 in vertex shaders.
	// Required for WebGL compatibility.
	WriterFlagForcePointSize

	// WriterFlagTextureShadowLod enables GL_EXT_texture_shadow_lod extension
	// for sampling cube/array shadow textures with explicit LOD.
	WriterFlagTextureShadowLod
)

// BoundsCheckPolicy controls how out-of-bounds resource accesses are handled.
type BoundsCheckPolicy uint8

// Bounds check policy constants.
const (
	// BoundsCheckUnchecked performs no bounds checking.
	BoundsCheckUnchecked BoundsCheckPolicy = iota
	// BoundsCheckRestrict clamps indices to valid range.
	BoundsCheckRestrict
	// BoundsCheckReadZeroSkipWrite returns zero for reads, skips writes.
	BoundsCheckReadZeroSkipWrite
)

// BoundsCheckPolicies holds per-resource-type bounds check policies.
type BoundsCheckPolicies struct {
	// ImageLoad controls bounds checking for image load operations.
	ImageLoad BoundsCheckPolicy
	// ImageStore controls bounds checking for image store operations.
	ImageStore BoundsCheckPolicy
}

// BindingMapKey identifies a resource binding for the BindingMap.
type BindingMapKey struct {
	Group   uint32
	Binding uint32
}

// Options configures GLSL code generation.
type Options struct {
	// LangVersion is the target GLSL version.
	// Defaults to Version330 if zero.
	LangVersion Version

	// EntryPoint specifies which entry point to compile.
	// If empty, the first entry point is compiled.
	EntryPoint string

	// SamplerBindingBase adds offset to sampler binding indices.
	SamplerBindingBase uint32

	// TextureBindingBase adds offset to texture binding indices.
	TextureBindingBase uint32

	// UniformBindingBase adds offset to uniform buffer binding indices.
	UniformBindingBase uint32

	// StorageBindingBase adds offset to storage buffer binding indices.
	StorageBindingBase uint32

	// WriterFlags control output formatting.
	WriterFlags WriterFlags

	// ForceHighPrecision forces highp precision for all float types (ES only).
	// If false, uses default precision qualifiers.
	ForceHighPrecision bool

	// BoundsCheckPolicies controls bounds checking for resource accesses.
	BoundsCheckPolicies BoundsCheckPolicies

	// BindingMap maps resource bindings to flat GL binding indices.
	// When set, layout(binding = N) qualifiers are emitted.
	BindingMap map[BindingMapKey]uint8

	// PipelineConstants provides values for pipeline-overridable constants.
	// Keys are either "@id(N)" numeric IDs as strings or override names.
	// Values are float64 (NaN means "not set, use default").
	// If provided, overrides are resolved before compilation.
	PipelineConstants ir.PipelineConstants
}

// TextureMapping describes a combined texture-sampler pair generated by the
// GLSL backend.
//
// In GLSL, separate texture2D + sampler become a single sampler2D uniform.
// The combined uniform uses the texture's binding. The GLES HAL needs to know
// which sampler is associated with which texture to bind the GL sampler object
// to the correct texture unit (not the sampler's own WGSL binding).
type TextureMapping struct {
	// TextureBinding is the (group, binding) of the texture global variable.
	TextureBinding ir.ResourceBinding

	// SamplerBinding is the (group, binding) of the associated sampler, if any.
	// Nil for storage images which have no sampler.
	SamplerBinding *ir.ResourceBinding
}

// UniformInfo describes a GLSL uniform or storage buffer block for reflection.
// Used by the HAL runtime binding fallback on GL < 4.2 where layout(binding=N)
// is unavailable and bindings must be assigned after linking via GL calls.
// Matches Rust naga ReflectionInfo.uniforms.
type UniformInfo struct {
	// BlockName is the GLSL block name (e.g., "Uniforms_block_0Vertex").
	// Used with glGetUniformBlockIndex (uniform buffers) or
	// glGetShaderStorageBlockIndex (storage buffers).
	BlockName string

	// Binding is the source (group, binding) from the IR.
	Binding ir.ResourceBinding

	// IsStorage is true for storage buffers (SSBO), false for uniform buffers (UBO).
	IsStorage bool
}

// TranslationInfo contains metadata about the translation.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated GLSL names.
	EntryPointNames map[string]string

	// UsedExtensions lists GLSL extensions required by the shader.
	UsedExtensions []string

	// RequiredVersion is the minimum GLSL version needed for this shader.
	// May be higher than the requested version if features require it.
	RequiredVersion Version

	// TextureSamplerPairs lists the combined texture-sampler pairs generated.
	// Each entry is "textureName_samplerName".
	TextureSamplerPairs []string

	// TextureMappings maps combined sampler2D uniform names to their texture
	// and sampler source bindings. Used by GLES HAL to build SamplerBindMap
	// (bind GL sampler to texture's unit, not sampler's own binding).
	TextureMappings map[string]TextureMapping

	// Uniforms lists uniform/storage buffer blocks with their GLSL block
	// names and source bindings. Used by GLES HAL for runtime binding
	// fallback on GL < 4.2. Matches Rust naga ReflectionInfo.uniforms.
	Uniforms []UniformInfo
}

// DefaultOptions returns sensible default options for GLSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:        Version330,
		ForceHighPrecision: true,
	}
}

// Compile generates GLSL source code from an IR module.
// Returns the GLSL source as a string, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	copts := toCodegenOptions(options)
	src, cinfo, err := codegen.Compile(module, copts)
	if err != nil {
		return "", TranslationInfo{}, err
	}
	return src, fromCodegenTranslationInfo(cinfo), nil
}

// toCodegenOptions converts public Options to internal codegen Options.
func toCodegenOptions(o Options) codegen.Options {
	var bindingMap map[codegen.BindingMapKey]uint8
	if o.BindingMap != nil {
		bindingMap = make(map[codegen.BindingMapKey]uint8, len(o.BindingMap))
		for k, v := range o.BindingMap {
			bindingMap[codegen.BindingMapKey{Group: k.Group, Binding: k.Binding}] = v
		}
	}
	return codegen.Options{
		LangVersion: codegen.Version{
			Major: o.LangVersion.Major,
			Minor: o.LangVersion.Minor,
			ES:    o.LangVersion.ES,
		},
		EntryPoint:         o.EntryPoint,
		SamplerBindingBase: o.SamplerBindingBase,
		TextureBindingBase: o.TextureBindingBase,
		UniformBindingBase: o.UniformBindingBase,
		StorageBindingBase: o.StorageBindingBase,
		WriterFlags:        codegen.WriterFlags(o.WriterFlags),
		ForceHighPrecision: o.ForceHighPrecision,
		BoundsCheckPolicies: codegen.BoundsCheckPolicies{
			ImageLoad:  codegen.BoundsCheckPolicy(o.BoundsCheckPolicies.ImageLoad),
			ImageStore: codegen.BoundsCheckPolicy(o.BoundsCheckPolicies.ImageStore),
		},
		BindingMap:        bindingMap,
		PipelineConstants: o.PipelineConstants,
	}
}

// fromCodegenTranslationInfo converts internal codegen TranslationInfo to public type.
func fromCodegenTranslationInfo(ci codegen.TranslationInfo) TranslationInfo {
	var texMappings map[string]TextureMapping
	if ci.TextureMappings != nil {
		texMappings = make(map[string]TextureMapping, len(ci.TextureMappings))
		for k, v := range ci.TextureMappings {
			texMappings[k] = TextureMapping{
				TextureBinding: v.TextureBinding,
				SamplerBinding: v.SamplerBinding,
			}
		}
	}
	var uniforms []UniformInfo
	if len(ci.Uniforms) > 0 {
		uniforms = make([]UniformInfo, len(ci.Uniforms))
		for i, u := range ci.Uniforms {
			uniforms[i] = UniformInfo{
				BlockName: u.BlockName,
				Binding:   u.Binding,
				IsStorage: u.IsStorage,
			}
		}
	}
	return TranslationInfo{
		EntryPointNames: ci.EntryPointNames,
		UsedExtensions:  ci.UsedExtensions,
		RequiredVersion: Version{
			Major: ci.RequiredVersion.Major,
			Minor: ci.RequiredVersion.Minor,
			ES:    ci.RequiredVersion.ES,
		},
		TextureSamplerPairs: ci.TextureSamplerPairs,
		TextureMappings:     texMappings,
		Uniforms:            uniforms,
	}
}
