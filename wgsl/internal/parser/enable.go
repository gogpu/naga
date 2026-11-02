package parser

import "unicode/utf8"

// Enable-extension names, as Rust naga's EnableExtension
// (naga/src/front/wgsl/parse/directive/enable_extension.rs).
const (
	EnableF16                      = "f16"
	EnableClipDistances            = "clip_distances"
	EnableDualSourceBlending       = "dual_source_blending"
	EnableWgpuMeshShader           = "wgpu_mesh_shader"
	EnableWgpuRayQuery             = "wgpu_ray_query"
	EnableWgpuRayQueryVertexReturn = "wgpu_ray_query_vertex_return"
	EnableWgpuRayTracingPipeline   = "wgpu_ray_tracing_pipeline"
	EnableWgpuCooperativeMatrix    = "wgpu_cooperative_matrix"
	EnableSubgroups                = "subgroups"
	EnablePrimitiveIndex           = "primitive_index"
	EnableDrawIndex                = "draw_index"
	EnableWgpuPerVertex            = "wgpu_per_vertex"
	EnableWgpuBindingArray         = "wgpu_binding_array"
	EnableWgpuInt16                = "wgpu_int16"
	EnableWgpuDebugPrintf          = "wgpu_debug_printf"
	EnableSwizzleAssignment        = "swizzle_assignment"
	EnableSubgroupSizeControl      = "subgroup_size_control"
)

// knownEnableExtensions is every name an enable directive may list: Rust
// naga's EnableExtension::from_ident plus swizzle_assignment (W3C
// gpuweb#5268), which this frontend implements and Rust does not, and the
// WGSL spec's subgroup_size_control.
//
// subgroups is accepted although Rust rejects it as not yet implemented
// (EnableExtensionNotYetImplemented): the WGSL spec defines it, and this
// frontend implements and gates what it enables. subgroup_size_control
// enables subgroups too (WGSL spec, enable-extensions table); its
// @subgroup_size attribute is not implemented here.
var knownEnableExtensions = map[string]bool{
	EnableSubgroupSizeControl:      true,
	EnableF16:                      true,
	EnableClipDistances:            true,
	EnableDualSourceBlending:       true,
	EnableWgpuMeshShader:           true,
	EnableWgpuRayQuery:             true,
	EnableWgpuRayQueryVertexReturn: true,
	EnableWgpuRayTracingPipeline:   true,
	EnableWgpuCooperativeMatrix:    true,
	EnableSubgroups:                true,
	EnablePrimitiveIndex:           true,
	EnableDrawIndex:                true,
	EnableWgpuPerVertex:            true,
	EnableWgpuBindingArray:         true,
	EnableWgpuInt16:                true,
	EnableWgpuDebugPrintf:          true,
	EnableSwizzleAssignment:        true,
}

// IsEnableExtension reports whether name is an enable-extension the
// frontend knows.
func IsEnableExtension(name string) bool {
	return knownEnableExtensions[name]
}

// WordSpan is the span of word written at start, for a name the parser
// recorded by its start alone. A span without a start stays empty.
func WordSpan(start Position, word string) Span {
	if start.Line == 0 {
		return Span{}
	}
	end := start
	end.Column += utf8.RuneCountInString(word)
	return Span{Start: start, End: end}
}
