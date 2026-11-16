package front

// This file is the version/profile matrix of design §15: what each dialect the
// frontend accepts allows. Every rule here is pinned by a glslangValidator
// (11:16.2.0) run against the same source and by the GLSL 4.60 / GLSL ES 3.20
// specification; the comment on each predicate names the glslang evidence. The
// tests in dialect_test.go carry the oracle sources.

import (
	"fmt"
	"strconv"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// dialect is the language of a source: its #version number and profile. The
// preprocessor accepts only the four combinations of design §15 (440, 450 and
// 460 core, 330 core, 300 es), so a dialect is always one of them.
type dialect struct {
	version uint16
	profile ppProfile
}

// String names the dialect in diagnostics, the way the specifications write the
// version: "GLSL 3.30 core", "GLSL ES 3.00".
func (d dialect) String() string {
	if d.es() {
		return "GLSL ES " + formatVersion(d.version)
	}
	return "GLSL " + formatVersion(d.version) + " core"
}

// formatVersion renders a #version number as the major.minor form the
// specifications use: 330 is 3.30, 300 is 3.00.
func formatVersion(version uint16) string {
	return strconv.Itoa(int(version)/100) + "." + fmt.Sprintf("%02d", int(version)%100)
}

// es reports the ES profile.
func (d dialect) es() bool { return d.profile == ppProfileES }

// vulkan is Vulkan GLSL (440/450/460): the dialect every rule of Rust naga's
// frontend is written for. Its behavior must not change with the GL/ES
// support (design §15: "the 440/450/460 path identical").
func (d dialect) vulkan() bool { return !d.es() && d.version >= 440 }

// gl330 is GL 3.30 core: no binding qualifiers, no compute, but explicit
// locations on vertex inputs and fragment outputs.
func (d dialect) gl330() bool { return !d.es() && d.version == 330 }

// supportsCompute reports whether the compute stage exists in the dialect.
// glslang: "compute shaders require es profile with version 310 or above, or
// non-es profile with version 420 or above" (comp330.comp, comp300.comp).
func (d dialect) supportsCompute() bool { return d.vulkan() }

// hasLayoutBindings reports whether a resource declaration may carry
// layout(binding=)/layout(set=). glslang on `layout(binding = 0) uniform
// sampler2D tex;` in 330 core: "'binding' : not supported for this version or
// the enabled extensions"; the same in 300 es. The qualifier is available from
// GLSL 4.20 (or through GL_ARB_shading_language_420pack, which this frontend
// does not implement) and in Vulkan GLSL always.
func (d dialect) hasLayoutBindings() bool { return d.vulkan() }

// supportsStorageBuffers reports whether a `buffer` block is a storage block.
// glslang rejects the declaration in both dialects with a syntax error at the
// `buffer` token (`buffer B { float x; } b;` in 330 core and 300 es): the
// qualifier is GLSL 4.30 / ES 3.10 and later.
func (d dialect) supportsStorageBuffers() bool { return d.vulkan() }

// supportsStorageImages reports whether image* types exist. glslang prints
// "'image2D' : Reserved word" for a bare declaration and "'image load store' :
// not supported for this version or the enabled extensions" when a layout
// format qualifier is present (img330bare.frag, img330.frag, img300.frag).
func (d dialect) supportsStorageImages() bool { return d.vulkan() }

// supportsCubeArraySamplers reports whether the samplerCubeArray family exists.
// glslang treats the names as reserved words in both dialects ("'samplerCubeArray'
// : Reserved word", cubearr330.frag and cubearr300.frag); cube array textures
// are GLSL 4.00 / ES 3.20.
func (d dialect) supportsCubeArraySamplers() bool { return d.vulkan() }

// supportsPushConstants reports whether layout(push_constant) is accepted.
// glslang on a push_constant block in 330 core: "'push_constant' : only
// allowed when using GLSL for Vulkan".
func (d dialect) supportsPushConstants() bool { return d.vulkan() }

// supportsAtomicMemory reports whether the atomicAdd family of atomic memory
// functions exists. They are GLSL 4.30 core and ES 3.10 core. Reproduced with
// glslangValidator 11:16.2.0 (2026-11-16): a `#version 430` compute shader with
// `shared uint c; ... atomicAdd(c, 1u);` compiles; the same source at
// `#version 420` needs GL_ARB_compute_shader first, and then gets "'atomicAdd' :
// required extension not requested: GL_ARB_shader_storage_buffer_object" — so
// 4.20 reaches them only through that extension, which this frontend does not
// implement; `#version 330 core` gets the same "required extension" sentence; a
// `#version 310 es` compute shader with the same body compiles, and `#version
// 300 es` is refused on the compute stage itself ("compute shaders require es
// profile with version 310 or above, or non-es profile with version 420 or
// above"). Every dialect this frontend accepts reaches them only through Vulkan
// GLSL, because the compute stage, shared variables and buffer blocks are
// Vulkan GLSL alone here.
func (d dialect) supportsAtomicMemory() bool { return d.vulkan() }

// supportsImageAtomics reports whether the imageAtomic* family exists. They are
// GLSL 4.20 core, not ES 3.10 core. Reproduced with glslangValidator 11:16.2.0
// (2026-11-16): a `#version 420` fragment shader with an r32ui storage image and
// `imageAtomicAdd(img, ivec2(0), 1u);` compiles with no extension requested; the
// same source at `#version 410` is refused with "'image load store' : not
// supported for this version or the enabled extensions" unless
// GL_ARB_shader_image_load_store is required, which makes it compile; `#version
// 330 core` is refused the same way. In ES, the functions need
// GL_OES_shader_image_atomic even at ES 3.10: the `#version 310 es` form of the
// probe gets "'imageAtomicAdd' : required extension not requested:
// GL_OES_shader_image_atomic". This frontend's storage images are Vulkan GLSL
// only, and it implements no ES extension.
func (d dialect) supportsImageAtomics() bool { return d.vulkan() }

// supportsDouble reports whether the double scalar type exists. glslang on
// `double d = 1.0lf;` in 330 core: "'double' : not supported for this version
// or the enabled extensions"; in 300 es: "'double' : not supported with this
// profile: es".
func (d dialect) supportsDouble() bool { return d.vulkan() }

// supportsTextureGather reports whether textureGather/textureGatherOffset
// exist. glslang: "'textureGather(...)' : not supported for this version or
// the enabled extensions" (330 core and 300 es); GLSL 4.00 and ES 3.10
// introduced them.
func (d dialect) supportsTextureGather() bool { return d.vulkan() }

// supportsTextureQueryLevels reports whether textureQueryLevels exists.
// glslang: "'textureQueryLevels' : no matching overloaded function found" (330
// core and 300 es); GLSL 4.30 and ES 3.10 introduced it.
func (d dialect) supportsTextureQueryLevels() bool { return d.vulkan() }

// supportsTextureQueryLod reports whether textureQueryLod exists. glslang:
// "'textureQueryLod' : no matching overloaded function found" in 330 core
// without an extension and in 300, 310 and 320 es; GLSL 4.00 introduced it
// (desktop 1.50 up through GL_ARB_texture_query_lod, which glslang refuses
// below 1.50), and no GLSL ES version has it.
func (d dialect) supportsTextureQueryLod() bool { return d.vulkan() }

// allowsIOLocation reports whether layout(location=N) may be written on an in
// or out variable of the given stage. GL 3.30 core and ES 3.00 give a location
// to vertex shader inputs; a varying (a vertex output or a fragment input)
// needs GLSL 4.10 / ES 3.10. glslang prints "'location qualifier on output' :
// not supported for this version or the enabled extensions" for a vertex output
// in either dialect (vary330.vert, vary300_loc.vert); for a fragment input in
// 330 core it prints only "'location qualifier on input' : not supported for
// this version or the enabled extensions" (frag330_inloc.frag), while the 300 es
// case adds "'location qualifier on input' : not supported in this stage:
// fragment" (frag300_inloc.frag). A fragment shader output takes a location in
// GL 3.30 core, but not in ES 3.00: design §15 gives it "no", and the ES 3.00
// specification has no location on fragment outputs (ES 3.10 introduced it).
// glslang 16.2 is lax here and accepts `layout(location = 0) out vec4 color;` in
// a 300 es shader (frag300_loc.frag); the specification decides, so the
// qualifier is refused, and an unlocated output keeps its sequential slot
// (design §10).
func (d dialect) allowsIOLocation(stage ir.ShaderStage, storage storageQualifier) bool {
	if d.vulkan() {
		return true
	}
	if storage == storageInput {
		return stage == ir.StageVertex
	}
	if stage != ir.StageFragment {
		return false
	}
	return d.gl330()
}

// ioLocationRequirement names the version rule a refused IO location needs, for
// the diagnostic of allowsIOLocation.
func (d dialect) ioLocationRequirement(stage ir.ShaderStage, storage storageQualifier) string {
	if d.es() && stage == ir.StageFragment && storage == storageOutput {
		return "fragment output locations require ES 3.10"
	}
	return "a varying location requires GLSL 4.10 or ES 3.10"
}

// unsupported is the message shape glslang uses for a feature the version or
// profile does not have, completed with the rule that introduces it.
func (d dialect) unsupported(what, requirement string) string {
	return fmt.Sprintf("'%s' : not supported by %s%s", what, d, requirement)
}

// builtinRefusal returns the version rule a texture builtin family needs that
// the dialect does not meet, or the empty string. The builtins are injected
// for every dialect (the injection is Rust naga's, written for Vulkan GLSL), so
// the gate sits at the call: glslang, for textureGather in both 330 core and
// 300 es, "'textureGather(...)' : not supported for this version or the
// enabled extensions", and for textureQueryLevels, "'textureQueryLevels' : no
// matching overloaded function found".
func (d dialect) builtinRefusal(name string) string {
	switch name {
	case "textureGather", "textureGatherOffset":
		if !d.supportsTextureGather() {
			return d.unsupported(name, " (texture gather requires GLSL 4.00 or ES 3.10)")
		}
	case "textureQueryLevels":
		if !d.supportsTextureQueryLevels() {
			return d.unsupported(name, " (textureQueryLevels requires GLSL 4.30 or ES 3.10)")
		}
	case "textureQueryLod":
		if !d.supportsTextureQueryLod() {
			return d.unsupported(name, " (textureQueryLod requires GLSL 4.00; no GLSL ES version has it)")
		}
	case "atomicAdd", "atomicMin", "atomicMax", "atomicAnd", "atomicOr", "atomicXor",
		"atomicExchange", "atomicCompSwap":
		if !d.supportsAtomicMemory() {
			return d.unsupported(name, " (atomic memory functions require GLSL 4.30 or ES 3.10)")
		}
	case "imageAtomicAdd", "imageAtomicMin", "imageAtomicMax", "imageAtomicAnd",
		"imageAtomicOr", "imageAtomicXor", "imageAtomicExchange", "imageAtomicCompSwap":
		if !d.supportsImageAtomics() {
			return d.unsupported(name, " (image atomic functions require GLSL 4.20, or ES 3.10 with "+
				"GL_OES_shader_image_atomic)")
		}
	}
	return ""
}

// extensionNames is the set of extension names the dialect gates consult (today
// GL_EXT_shader_explicit_arithmetic_types_float16, which lets float16_t into GL
// 3.30 core, and the eight GL_KHR_shader_subgroup_* families, whose builtins
// subgroup.go gates). It is not a claim of support: the frontend implements no
// extension-gated syntax beyond the types, precision and builtins it already
// has. The names are the ones the GLSL corpus and design §15 use, spelled out
// exactly, so a typo such as GL_KHR_shader_subgroup_balot is not mistaken for a
// real extension. The list plays no part in the Vulkan GLSL path, which examines
// no #extension directive at all (checkExtensions).
var extensionNames = map[string]bool{
	"GL_AMD_gpu_shader_half_float":                    true,
	"GL_ARB_compute_shader":                           true,
	"GL_ARB_conservative_depth":                       true,
	"GL_ARB_geometry_shader4":                         true,
	"GL_ARB_gpu_shader_fp64":                          true,
	"GL_ARB_gpu_shader_int64":                         true,
	"GL_ARB_shader_draw_parameters":                   true,
	"GL_ARB_shader_image_load_store":                  true,
	"GL_ARB_shader_storage_buffer_object":             true,
	"GL_ARB_shader_texture_image_samples":             true,
	"GL_ARB_texture_cube_map_array":                   true,
	"GL_ARB_texture_query_levels":                     true,
	"GL_EXT_blend_func_extended":                      true,
	"GL_EXT_clip_cull_distance":                       true,
	"GL_EXT_conservative_depth":                       true,
	"GL_EXT_debug_printf":                             true,
	"GL_EXT_fragment_shader_barycentric":              true,
	"GL_EXT_mesh_shader":                              true,
	"GL_EXT_multiview":                                true,
	"GL_EXT_nonuniform_qualifier":                     true,
	"GL_EXT_ray_query":                                true,
	"GL_EXT_ray_tracing":                              true,
	"GL_EXT_samplerless_texture_functions":            true,
	"GL_EXT_shader_16bit_storage":                     true,
	"GL_EXT_shader_8bit_storage":                      true,
	"GL_EXT_shader_atomic_float":                      true,
	"GL_EXT_shader_atomic_int64":                      true,
	"GL_EXT_shader_explicit_arithmetic_types_float16": true,
	"GL_EXT_shader_explicit_arithmetic_types_int16":   true,
	"GL_EXT_shader_explicit_arithmetic_types_int8":    true,
	"GL_EXT_shader_image_int64":                       true,
	"GL_EXT_shader_subgroup_extended_types_int16":     true,
	"GL_EXT_spirv_intrinsics":                         true,
	"GL_EXT_texture_cube_map_array":                   true,
	"GL_EXT_texture_shadow_lod":                       true,
	"GL_KHR_cooperative_matrix":                       true,
	"GL_KHR_memory_scope_semantics":                   true,
	"GL_KHR_shader_subgroup_arithmetic":               true,
	"GL_KHR_shader_subgroup_ballot":                   true,
	"GL_KHR_shader_subgroup_basic":                    true,
	"GL_KHR_shader_subgroup_clustered":                true,
	"GL_KHR_shader_subgroup_quad":                     true,
	"GL_KHR_shader_subgroup_shuffle":                  true,
	"GL_KHR_shader_subgroup_shuffle_relative":         true,
	"GL_KHR_shader_subgroup_vote":                     true,
	"GL_OES_geometry_shader":                          true,
	"GL_OES_sample_variables":                         true,
	"GL_OES_shader_image_atomic":                      true,
	"GL_OES_texture_storage_multisample_2d_array":     true,
	"GL_OVR_multiview2":                               true,
}

// extensionKnown reports whether name is exactly one of the names the dialect
// gates consult. A prefix comparison would let a typo through, so the match is
// exact.
func extensionKnown(name string) bool {
	return extensionNames[name]
}

// extensionEnabled reports whether the recognized extension name is enabled by
// the source's #extension directives: the last directive naming it decides
// (require, enable and warn enable it; disable turns it off), and
// `#extension all : disable` turns every extension off, as the GLSL 4.60 §3.3
// behavior rules say. A name outside the recognized list enables nothing.
func extensionEnabled(exts []ppExtension, name string) bool {
	if !extensionKnown(name) {
		return false
	}
	enabled := false
	for _, ext := range exts {
		switch {
		case ext.Name == name:
			enabled = ext.Behavior != ppExtensionDisable
		case ext.Name == "all" && ext.Behavior == ppExtensionDisable:
			enabled = false
		}
	}
	return enabled
}

// checkExtensions applies the #extension behavior rules the frontend keeps.
//
// Vulkan GLSL (440/450/460) gets no diagnostics at all: Rust naga never
// examines the directive there, and the 440/450/460 path stays identical.
//
// In GL 3.30 core and ES 3.00 (#extension-unsupported.frag, both dialects):
//   - `#extension all : require` (or enable) is an error, glslang's "extension
//     'all' cannot have 'require' or 'enable' behavior"; `all : disable` and
//     `all : warn` are silent (ext_all_req.frag, ext_all_warn.frag);
//   - a directive naming an extension outside extensionNames is accepted as a
//     no-op with a warning, whatever its behavior (glslang warns for enable,
//     warn and disable and errors for require: ext_unknown_330_*.frag).
//     Warning even for `require` is a deliberate deviation from GLSL 4.60 §3.3,
//     which makes an unsupported required extension an error: the frontend
//     cannot enumerate the Khronos extension registry, so a hard list would
//     reject valid shaders (GL_EXT_control_flow_attributes is one) while still
//     not catching every typo reliably. Card P2-GLSLIN-022.
//   - a recognized extension is silent; the dialect gates decide where its
//     feature is used.
func checkExtensions(d dialect, exts []ppExtension) ([]frontError, []frontend.Diagnostic) {
	if d.vulkan() {
		return nil, nil
	}
	var (
		errors   []frontError
		warnings []frontend.Diagnostic
	)
	for _, ext := range exts {
		if ext.Name == "all" {
			if ext.Behavior == ppExtensionRequire || ext.Behavior == ppExtensionEnable {
				errors = append(errors, *errMessage(ext.Span,
					"'#extension' : extension 'all' cannot have 'require' or 'enable' behavior"))
			}
			continue
		}
		if extensionKnown(ext.Name) {
			continue
		}
		warnings = append(warnings, frontend.Diagnostic{
			Message: "'#extension' : extension not supported: " + ext.Name +
				" (accepted as a no-op; the frontend cannot enumerate the Khronos extension registry, " +
				"a deliberate deviation from GLSL 4.60 §3.3, card P2-GLSLIN-022)",
			Span: ext.Span,
		})
	}
	return errors, warnings
}
