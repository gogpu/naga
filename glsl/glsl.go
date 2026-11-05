// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl/internal/codegen"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/unsupported"
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

// Options configures GLSL code generation.
type Options struct {
	// LangVersion is the target GLSL version.
	// Defaults to Version330 if zero.
	LangVersion Version

	// EntryPoint selects the entry point to write: GLSL holds one, as main.
	// nil is accepted only for a module with exactly one entry point. An
	// entry point the module does not have, or nil with several, is an error
	// wrapping backend.ErrEntryPointNotFound.
	EntryPoint *backend.EntryPointSelector

	// SkipAdjustCoordinateSpace leaves gl_Position as the entry point writes
	// it. By default a vertex entry point ends by converting WebGPU's clip
	// space to OpenGL's, y negated and z from [0, 1] to [-1, 1]:
	// gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
	// as Rust naga's ADJUST_COORDINATE_SPACE, which its default flags and
	// wgpu-hal's GLES backend set.
	SkipAdjustCoordinateSpace bool

	// ForcePointSize writes gl_PointSize = 1.0 in vertex entry points, which
	// GL ES and WebGL need to draw a point list (Rust's FORCE_POINT_SIZE;
	// wgpu-hal sets it for point-list pipelines).
	ForcePointSize bool

	// TextureShadowLod samples cube and array shadow textures at an explicit
	// level with GL_EXT_texture_shadow_lod (Rust's TEXTURE_SHADOW_LOD); the
	// device needs the extension. Without it, such a sample at level zero is
	// written with textureGrad.
	TextureShadowLod bool

	// DrawParameters says the device has GL_ARB_shader_draw_parameters
	// (Rust's DRAW_PARAMETERS): instance_index adds gl_BaseInstanceARB
	// instead of the naga_vs_first_instance uniform the host sets, and
	// draw_index is gl_DrawIDARB, so it works below GLSL 4.60. Without it,
	// draw_index needs GLSL 4.60; GLSL ES has no draw index.
	DrawParameters bool

	// BoundsCheckPolicies selects how out-of-bounds indices and image loads
	// are handled. The zero value is backend.BoundsCheckRestrict everywhere,
	// which WebGPU requires; Rust naga's default is Unchecked. Per field:
	//   - Index and Buffer: Restrict clamps the index with min(); Unchecked
	//     adds nothing; ReadZeroSkipWrite is not supported and makes Compile
	//     fail.
	//   - ImageLoad: Restrict clamps the coordinates, level and sample to
	//     the image's size; ReadZeroSkipWrite returns zero for a load out of
	//     bounds; Unchecked adds nothing. The checks need textureSize and
	//     imageSize, as in Rust naga.
	//   - BindingArray: GLSL has no binding arrays (Rust naga writes none
	//     either), so it has no effect.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// BindingMap maps resource bindings to flat GL binding indices, written
	// as layout(binding = N) on versions that have explicit bindings
	// (desktop 4.20, ES 3.10). A nil map writes none: the caller binds the
	// resources by the names TranslationInfo reports. A non-nil map, empty
	// too, must list every resource the entry point uses when the version
	// has explicit bindings: one without an entry would get the driver's
	// binding 0, so it is an error wrapping backend.ErrMissingBinding unless
	// FakeMissingBindings is set. Rust naga writes such a resource without a
	// binding silently.
	BindingMap map[ir.ResourceBinding]uint8

	// FakeMissingBindings writes a resource a non-nil BindingMap leaves out
	// without a binding qualifier, as Rust naga does, instead of failing.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero the
	// shared variables they use, as WebGPU requires. The zero value,
	// backend.ZeroInitWorkgroupPolyfill, stores zero to them from the first
	// invocation and adds a barrier, as Rust naga's default
	// zero_initialize_workgroup_memory: true does. ZeroInitWorkgroupNone
	// leaves them as they are; ZeroInitWorkgroupNative is an error, OpenGL
	// has no way to ask for it.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// ClampFragDepth, when set, clamps the depth a fragment entry point
	// writes to the range in two f32, min then max, at byte offset
	// ClampFragDepth.Offset of the entry point's immediate data. The shader
	// declares them in its immediates uniform, after the entry point's own
	// immediate data, which it then holds in a struct with them, or alone
	// when the entry point has none, and TranslationInfo.ImmediateItems
	// lists them as immediate data. WebGPU clamps frag_depth to the
	// viewport's depth range; the caller puts the viewport's minDepth and
	// maxDepth there, where Dawn's GL backend keeps them next to the
	// immediates, and as the spirv and msl options of the same name read
	// them. Offset has to be past the entry point's own immediate data.
	// Rust naga has no such option.
	ClampFragDepth *backend.ClampFragDepth
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

	// ImmediateItems lists the GL uniforms that carry the entry point's
	// immediate data, the range of Options.ClampFragDepth included, in
	// declaration order; empty when there is none. Matches Rust naga
	// ReflectionInfo.immediates_items.
	ImmediateItems []ImmediateItem

	// ClipDistanceCount is the number of clip distances a vertex entry point
	// writes: the N of the `out float gl_ClipDistance[N];` re-declaration in
	// the GLSL, 0 for a fragment or compute entry point and for a vertex one
	// without @builtin(clip_distances). The runtime enables
	// GL_CLIP_DISTANCE0 to GL_CLIP_DISTANCE(N-1) for the program (glEnable);
	// GLSL itself does not. Matches Rust naga ReflectionInfo.clip_distance_count.
	ClipDistanceCount uint32
}

// ImmediateItem is one GL uniform that carries part of the immediate data.
//
// OpenGL has no immediate data, so the writer declares the immediate
// variable as a plain (non-block) uniform, and GL gives each of its leaf
// scalars, vectors and matrices a location of its own. After linking, the
// runtime looks each item up with glGetUniformLocation(program, AccessPath)
// and, whenever the application sets immediate data, uploads the SizeBytes
// bytes at Offset with the glUniform* call that fits Type. GL cannot update
// part of a uniform, so the runtime keeps a copy of the whole immediate data
// and re-sends every item a write overlaps.
type ImmediateItem struct {
	// AccessPath is the GLSL name of the uniform, as a shader would access
	// it: "_immediates_binding_vs.light.color" for a member of a nested
	// struct, "_immediates_binding_fs" when the variable is not a struct.
	AccessPath string

	// Type is the uniform's type: always an ir.ScalarType, ir.VectorType or
	// ir.MatrixType.
	Type ir.TypeInner

	// Offset is the item's byte offset in the immediate data, the WGSL
	// layout of the immediate variable (StructMember.Offset).
	Offset uint32

	// SizeBytes is the item's size in bytes; a matrix counts its columns
	// padded to their alignment (mat3x3<f32> is 48 bytes).
	SizeBytes uint32
}

// DefaultOptions returns the zero Options, the safe defaults: GLSL 3.30,
// Restrict bounds checks, gl_Position converted to GL clip space.
func DefaultOptions() Options {
	return Options{}
}

// Compile generates GLSL source code from an IR module.
// Returns the GLSL source as a string, translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	if options.LangVersion == (Version{}) {
		options.LangVersion = Version330
	}
	if err := unsupported.Capabilities("glsl", module, SupportedCapabilities()); err != nil {
		return "", TranslationInfo{}, err
	}
	if err := irutil.CheckWorkgroupSizesResolved(module); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}
	module, err := irutil.ResolvePendingArraySizes(module)
	if err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}
	// The writer names identifiers over the whole module and writes the
	// selected entry point only, as Rust naga does.
	if _, err = entrypoint.Index(module, options.EntryPoint, true); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("glsl: %w", err)
	}
	copts := toCodegenOptions(options)
	src, cinfo, err := codegen.Compile(module, copts)
	if err != nil {
		var missing *codegen.MissingFeaturesError
		if errors.As(err, &missing) {
			return "", TranslationInfo{}, fmt.Errorf("glsl: %w", &MissingFeaturesError{
				Version:            options.LangVersion,
				Features:           missing.Features.Names(),
				ForImageLoadChecks: missing.ForImageLoadChecks,
			})
		}
		return "", TranslationInfo{}, err
	}
	return src, fromCodegenTranslationInfo(cinfo), nil
}

// MissingFeaturesError reports that the entry point needs GLSL features the
// target version lacks, as Rust naga's Error::MissingFeatures does: writing
// them anyway gives GLSL the driver rejects. Features holds Rust's names
// (TEXTURE_LEVELS, COMPUTE_SHADER, ...). ForImageLoadChecks reports that the
// ImageLoad bounds checks asked for some of them: GLSL ES cannot query a
// texture's levels or samples, so it cannot check the level or sample of a
// textureLoad, and setting BoundsCheckPolicies.ImageLoad to
// backend.BoundsCheckUnchecked for such a target avoids the error, as
// wgpu-hal does below desktop GLSL 4.30. A caller can match it with
// errors.As.
type MissingFeaturesError struct {
	Version            Version
	Features           []string
	ForImageLoadChecks bool
}

// Error returns Rust naga's message, with the policy to set when the
// ImageLoad checks need the features.
func (e *MissingFeaturesError) Error() string {
	msg := fmt.Sprintf("GLSL %s doesn't support the required feature(s): %s", e.Version, strings.Join(e.Features, " | "))
	if e.ForImageLoadChecks {
		msg += " (the ImageLoad bounds checks need them; set BoundsCheckPolicies.ImageLoad to Unchecked " +
			"for this target, as wgpu-hal does below desktop GLSL 4.30)"
	}
	return msg
}

// Unwrap returns backend.ErrUnsupported: the version cannot express the
// features.
func (e *MissingFeaturesError) Unwrap() error { return backend.ErrUnsupported }

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
		SkipAdjustCoordinateSpace: o.SkipAdjustCoordinateSpace,
		ForcePointSize:            o.ForcePointSize,
		TextureShadowLod:          o.TextureShadowLod,
		DrawParameters:            o.DrawParameters,
		BoundsCheckPolicies:       o.BoundsCheckPolicies,
		BindingMap:                bindingMap,
		FakeMissingBindings:       o.FakeMissingBindings,
		ClampFragDepth:            o.ClampFragDepth,
		EntryPoint:                o.EntryPoint,

		ZeroInitializeWorkgroupMemory: o.ZeroInitializeWorkgroupMemory,
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
	var immediates []ImmediateItem
	if len(ci.ImmediateItems) > 0 {
		immediates = make([]ImmediateItem, len(ci.ImmediateItems))
		for i, item := range ci.ImmediateItems {
			immediates[i] = ImmediateItem{
				AccessPath: item.AccessPath,
				Type:       item.Type,
				Offset:     item.Offset,
				SizeBytes:  item.SizeBytes,
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
		ImmediateItems:      immediates,
		ClipDistanceCount:   ci.ClipDistanceCount,
	}
}
