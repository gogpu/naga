// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Version represents a GLSL version.
type Version struct {
	Major uint8
	Minor uint8
	ES    bool // true for GLSL ES (OpenGL ES / WebGL)
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

// versionLessThan returns true if the numeric version (Major*100+Minor) is
// less than the given number. For example, versionLessThan(410) returns true
// for GLSL 330 (3*100+30=330 < 410) and false for GLSL 410 (4*100+10=410).
func (v Version) versionLessThan(number int) bool {
	return int(v.Major)*100+int(v.Minor) < number
}

// SupportsCompute returns true if this version supports compute shaders.
func (v Version) SupportsCompute() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// supportsDerivativeControl returns true if Coarse/Fine derivative control is available.
// GLSL 450+ desktop, ES 320+.
func (v Version) supportsDerivativeControl() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 20)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 50)
}

// supportsIOLocations returns true if layout(location=N) is supported for IO.
// Desktop GLSL 330+, ES 300+.
func (v Version) supportsIOLocations() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 0)
	}
	return v.Major > 3 || (v.Major == 3 && v.Minor >= 30)
}

// supportsExplicitLocations returns true if explicit layout locations for bindings are supported.
// Desktop GLSL 420+, ES 310+.
func (v Version) supportsExplicitLocations() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 20)
}

// supportsPack2x16snorm returns true if packSnorm2x16/unpackSnorm2x16 are available.
// Desktop GLSL 420+, ES 300+. (Rust: Version::Desktop(420) || Version::new_gles(300))
func (v Version) supportsPack2x16snorm() bool {
	if v.ES {
		return v.Major >= 3
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 20)
}

// supportsPack2x16unorm returns true if packUnorm2x16/unpackUnorm2x16 are available.
// Desktop GLSL 400+, ES 300+. (Rust: Version::Desktop(400) || Version::new_gles(300))
func (v Version) supportsPack2x16unorm() bool {
	if v.ES {
		return v.Major >= 3
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 0)
}

// supportsPack4x8 returns true if packSnorm4x8/unpackSnorm4x8 etc are available.
// Desktop GLSL 400+, ES 310+.
func (v Version) supportsPack4x8() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 0)
}

// supportsFma returns true if fma() is available.
// Desktop GLSL 400+, ES 320+.
func (v Version) supportsFma() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 20)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 0)
}

// SupportsStorageBuffers returns true if this version supports storage buffers.
func (v Version) SupportsStorageBuffers() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major > 4 || (v.Major == 4 && v.Minor >= 30)
}

// WriterFlags control output formatting.
type WriterFlags uint32

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
	// Matches Rust naga's WriterFlags::ADJUST_COORDINATE_SPACE.
	// Emits: gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
	WriterFlagAdjustCoordinateSpace

	// WriterFlagForcePointSize emits gl_PointSize = 1.0 in vertex shaders.
	// Required for WebGL compatibility.
	WriterFlagForcePointSize

	// WriterFlagTextureShadowLod enables GL_EXT_texture_shadow_lod extension
	// for sampling cube/array shadow textures with explicit LOD.
	WriterFlagTextureShadowLod
)

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
	// Matches Rust naga's proc::BoundsCheckPolicies.
	BoundsCheckPolicies BoundsCheckPolicies

	// BindingMap maps resource bindings to flat GL binding indices.
	// Matches Rust naga's back::glsl::BindingMap.
	// When set, layout(binding = N) qualifiers are emitted.
	BindingMap map[BindingMapKey]uint8

	// PipelineConstants provides values for pipeline-overridable constants.
	// Keys are either "@id(N)" numeric IDs as strings or override names.
	// Values are float64 (NaN means "not set, use default").
	// If provided, overrides are resolved before compilation.
	PipelineConstants ir.PipelineConstants
}

// BindingMapKey identifies a resource binding for the BindingMap.
type BindingMapKey struct {
	Group   uint32
	Binding uint32
}

// BoundsCheckPolicy controls how out-of-bounds resource accesses are handled.
type BoundsCheckPolicy uint8

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

// DefaultOptions returns sensible default options for GLSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:        Version330,
		ForceHighPrecision: true,
	}
}

// TextureMapping describes a combined texture-sampler pair generated by the
// GLSL backend. Matches Rust naga's TextureMapping (glsl/mod.rs:401-406).
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
	// Matches Rust naga ReflectionInfo.texture_mapping.
	TextureMappings map[string]TextureMapping
}

// Compile generates GLSL source code from an IR module.
// Returns the GLSL source as a string, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version330
	}

	// Process overrides if pipeline constants are provided.
	// This resolves all ExprOverride to concrete Literal/Constant values.
	// Deep-clone mutable parts to avoid mutating shared state.
	if len(options.PipelineConstants) > 0 && len(module.Overrides) > 0 {
		module = ir.CloneModuleForOverrides(module)
		if err := ir.ProcessOverrides(module, options.PipelineConstants); err != nil {
			return "", TranslationInfo{}, fmt.Errorf("glsl: process overrides: %w", err)
		}
	}

	// Create writer
	w := newWriter(module, &options)

	// Generate GLSL code
	if err := w.writeModule(); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}

	// Build TextureMappings from combined sampler pairs (matches Rust naga
	// ReflectionInfo.texture_mapping built from info.sampling_set in writer.rs:4421-4502).
	textureMappings := make(map[string]TextureMapping, len(w.combinedSamplers))
	for _, cs := range w.combinedSamplers {
		tm := TextureMapping{}
		if cs.binding != nil {
			tm.TextureBinding = *cs.binding
		}
		// Look up sampler's binding from the module's global variables.
		if int(cs.samplerHandle) < len(module.GlobalVariables) {
			sampler := &module.GlobalVariables[cs.samplerHandle]
			if sampler.Binding != nil {
				binding := *sampler.Binding
				tm.SamplerBinding = &binding
			}
		}
		textureMappings[cs.glslName] = tm
	}

	info := TranslationInfo{
		EntryPointNames:     w.entryPointNames,
		UsedExtensions:      w.extensions,
		RequiredVersion:     w.requiredVersion,
		TextureSamplerPairs: w.textureSamplerPairs,
		TextureMappings:     textureMappings,
	}

	return w.String(), info, nil
}
