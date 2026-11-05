// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/internal/unsupported"
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

// supportsStd430Layout returns true if std430 buffer blocks (storage
// buffers) are available: desktop GLSL 400+ through
// GL_ARB_shader_storage_buffer_object, ES 310+. Matches Rust naga
// Version::supports_std430_layout.
func (v Version) supportsStd430Layout() bool {
	if v.ES {
		return v.Major > 3 || (v.Major == 3 && v.Minor >= 10)
	}
	return v.Major >= 4
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

// Options configures GLSL code generation.
type Options struct {
	// LangVersion is the target GLSL version.
	// Defaults to Version330 if zero.
	LangVersion Version

	// EntryPoint selects the entry point to compile; nil selects the first.
	EntryPoint *backend.EntryPointSelector

	// SkipAdjustCoordinateSpace leaves gl_Position as written; by default a
	// vertex entry point ends with
	// gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
	// (Rust naga's WriterFlags::ADJUST_COORDINATE_SPACE).
	SkipAdjustCoordinateSpace bool

	// ForcePointSize writes gl_PointSize = 1.0 in vertex entry points
	// (FORCE_POINT_SIZE).
	ForcePointSize bool

	// TextureShadowLod enables GL_EXT_texture_shadow_lod (TEXTURE_SHADOW_LOD).
	TextureShadowLod bool

	// DrawParameters uses GL_ARB_shader_draw_parameters (DRAW_PARAMETERS).
	DrawParameters bool

	// BoundsCheckPolicies controls bounds checking for resource accesses.
	// Matches Rust naga's proc::BoundsCheckPolicies.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// BindingMap maps resource bindings to flat GL binding indices.
	// Matches Rust naga's back::glsl::BindingMap.
	// When set, layout(binding = N) qualifiers are emitted.
	BindingMap map[BindingMapKey]uint8

	// FakeMissingBindings writes a resource a non-nil BindingMap leaves out
	// without a binding qualifier instead of failing.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// shared variables they use: Polyfill (zero) or None; Native is an
	// error.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// ClampFragDepth clamps the depth a fragment entry point writes to the
	// range at byte ClampFragDepth.Offset of its immediate data (see the
	// glsl package).
	ClampFragDepth *backend.ClampFragDepth
}

// BindingMapKey identifies a resource binding for the BindingMap.
type BindingMapKey struct {
	Group   uint32
	Binding uint32
}

// DefaultOptions returns sensible default options for GLSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion: Version330,
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
	// Matches Rust naga ReflectionInfo.texture_mapping.
	TextureMappings map[string]TextureMapping

	// Uniforms maps global variable block names to their binding info.
	// Used by GLES HAL for runtime binding fallback on GL < 4.2 where
	// layout(binding=N) is unavailable. After glLinkProgram, the HAL
	// queries block indices by name and assigns bindings via GL calls.
	// Matches Rust naga ReflectionInfo.uniforms.
	Uniforms []UniformInfo

	// ImmediateItems lists the GL uniforms of the immediate data.
	// Matches Rust naga ReflectionInfo.immediates_items.
	ImmediateItems []ImmediateItem

	// ClipDistanceCount is the size of the gl_ClipDistance re-declaration the
	// vertex entry point writes, 0 otherwise.
	// Matches Rust naga ReflectionInfo.clip_distance_count.
	ClipDistanceCount uint32
}

// Compile generates GLSL source code from an IR module.
// Returns the GLSL source as a string, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version330
	}
	p := options.BoundsCheckPolicies
	for _, policy := range []backend.BoundsCheckPolicy{p.Index, p.Buffer, p.ImageLoad, p.BindingArray} {
		if policy > backend.BoundsCheckUnchecked {
			return "", TranslationInfo{}, unsupported.Errorf("glsl: BoundsCheckPolicies: %v is not supported", policy)
		}
	}
	if p.Index == backend.BoundsCheckReadZeroSkipWrite || p.Buffer == backend.BoundsCheckReadZeroSkipWrite {
		return "", TranslationInfo{}, fmt.Errorf("glsl: the index and buffer bounds checks support Restrict only")
	}
	if m := options.ZeroInitializeWorkgroupMemory; m != backend.ZeroInitWorkgroupPolyfill && m != backend.ZeroInitWorkgroupNone {
		return "", TranslationInfo{}, unsupported.Errorf("glsl: ZeroInitializeWorkgroupMemory %v is not supported", m)
	}

	if _, err := entrypoint.Index(module, options.EntryPoint, false); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}

	// Rust's writer reads expression types from the validator's ModuleInfo;
	// Compile takes no such info, so it types the module it writes.
	types, err := typifier.ForModule(module)
	if err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}

	// Create writer
	w := newWriter(module, &options)
	w.typifiers = types
	if ep := w.getSelectedEntryPoint(); ep != nil {
		if err := checkVaryingInterpolation(module, ep); err != nil {
			return "", TranslationInfo{}, err
		}
	}

	// Generate GLSL code
	if err := w.writeModule(); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}
	if w.float16 {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", errNoFloat16)
	}
	if w.int64 {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", errNoInt64)
	}
	if w.int16 {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", errNoInt16)
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
		Uniforms:            w.uniformInfos,
		ImmediateItems:      w.immediateItems,
		ClipDistanceCount:   w.clipDistanceCount,
	}

	return w.String(), info, nil
}
