package wgsl

import (
	"strings"
	"testing"
)

// notEnabled is Rust naga's Error::EnableExtensionNotEnabled message
// (naga/src/front/wgsl/error.rs).
func notEnabled(ext string) string {
	return "the `" + ext + "` enable extension is not enabled"
}

// TestEnableExtensionGates: every feature Rust naga gates behind an `enable`
// directive is an error without it, with Rust's message and a label on the
// word that needs it; the same shader with the enable gets past the gate.
// Sites: naga/src/front/wgsl/parse/conv.rs (map_address_space, map_ray_flag,
// map_built_in, map_interpolation, map_predeclared_type), parse/mod.rs
// (number literals, blend_src, per_primitive, entry point attributes) and
// lower/mod.rs (debugPrintf).
func TestEnableExtensionGates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ext    string // the extension the shader needs
		source string // the shader without its enable
		word   string // the source under the error's label
		// implemented: with the enable the shader parses. Otherwise the
		// feature is past the gate but not implemented by this frontend, and
		// only the gate is checked.
		implemented bool
	}{
		// map_predeclared_type: f16 types.
		{"f16 scalar", "f16", "var<private> x: f16;", "f16", true},
		{"f16 vector alias", "f16", "var<private> v: vec2h;", "vec2h", true},
		{"f16 matrix alias", "f16", "var<private> m: mat2x3h;", "mat2x3h", true},
		{"f16 template argument", "f16", "var<private> v: vec3<f16>;", "f16", true},
		{"f16 constructor", "f16", "fn f() { let x = f16(1.0); }", "f16", true},
		{"f16 vector constructor", "f16", "fn f() { let x = vec4h(); }", "vec4h", true},
		{"f16 bitcast target", "f16", "fn f() { let x = bitcast<vec2<f16>>(1u); }", "f16", true},
		// number.rs requires_enable_extension: the f16 literal suffix.
		{"f16 literal", "f16", "const c = 1.5h;", "1.5h", true},
		{"f16 literal in function", "f16", "fn f() -> f32 { return f32(2h); }", "2h", true},
		// map_predeclared_type: 16-bit integers.
		{"i16 scalar", "wgpu_int16", "var<private> x: i16;", "i16", false},
		{"u16 scalar", "wgpu_int16", "var<private> x: u16;", "u16", false},
		// map_predeclared_type: ray query types need wgpu_ray_query or
		// wgpu_ray_tracing_pipeline; the error names the first.
		{"acceleration_structure", "wgpu_ray_query", "@group(0) @binding(0) var a: acceleration_structure;", "acceleration_structure", true},
		{"ray_query", "wgpu_ray_query", "fn f() { var q: ray_query; }", "ray_query", true},
		{"RayDesc", "wgpu_ray_query", "var<private> d: RayDesc;", "RayDesc", true},
		{"RayIntersection", "wgpu_ray_query", "var<private> r: RayIntersection;", "RayIntersection", true},
		// map_predeclared_type: cooperative matrices and binding arrays.
		{"coop_mat8x8", "wgpu_cooperative_matrix", "var<private> m: coop_mat8x8<f32, A>;", "coop_mat8x8", false},
		{"binding_array", "wgpu_binding_array", "@group(0) @binding(0) var t: binding_array<texture_2d<f32>, 2>;", "binding_array", true},
		// map_built_in.
		{"clip_distances", "clip_distances",
			"struct O { @builtin(position) p: vec4f, @builtin(clip_distances) c: array<f32, 1>, }\n@vertex fn vs() -> O { return O(vec4f(), array<f32, 1>()); }",
			"clip_distances", true},
		{"primitive_index", "primitive_index",
			"@fragment fn fs(@builtin(primitive_index) i: u32) -> @location(0) vec4f { return vec4f(); }",
			"primitive_index", true},
		{"draw_index", "draw_index",
			"@vertex fn vs(@builtin(draw_index) i: u32) -> @builtin(position) vec4f { return vec4f(); }",
			"draw_index", true},
		{"cull_primitive", "wgpu_mesh_shader", "struct P { @builtin(cull_primitive) c: bool, }", "cull_primitive", true},
		{"point_index", "wgpu_mesh_shader", "struct P { @builtin(point_index) c: u32, }", "point_index", true},
		{"line_indices", "wgpu_mesh_shader", "struct P { @builtin(line_indices) c: vec2u, }", "line_indices", true},
		{"triangle_indices", "wgpu_mesh_shader", "struct P { @builtin(triangle_indices) c: vec3u, }", "triangle_indices", true},
		{"vertex_count", "wgpu_mesh_shader", "struct M { @builtin(vertex_count) c: u32, }", "vertex_count", true},
		{"primitive_count", "wgpu_mesh_shader", "struct M { @builtin(primitive_count) c: u32, }", "primitive_count", true},
		// map_interpolation.
		{"per_vertex", "wgpu_per_vertex",
			"@fragment fn fs(@location(0) @interpolate(per_vertex) x: array<f32, 3>) -> @location(0) vec4f { return vec4f(); }",
			"per_vertex", true},
		// BindingParser: blend_src and per_primitive.
		{"blend_src", "dual_source_blending",
			"struct O { @location(0) @blend_src(0) a: vec4f, @location(0) @blend_src(1) b: vec4f, }\n@fragment fn fs() -> O { return O(vec4f(), vec4f()); }",
			"blend_src", true},
		{"per_primitive", "wgpu_mesh_shader", "struct P { @location(0) @per_primitive x: f32, }", "per_primitive", true},
		// map_address_space.
		{"task_payload", "wgpu_mesh_shader", "var<task_payload> p: u32;", "task_payload", true},
		{"ray_payload", "wgpu_ray_tracing_pipeline", "var<ray_payload> p: u32;", "ray_payload", false},
		{"incoming_ray_payload", "wgpu_ray_tracing_pipeline", "var<incoming_ray_payload> p: u32;", "incoming_ray_payload", false},
		{"pointer to task_payload", "wgpu_mesh_shader", "fn f(p: ptr<task_payload, u32>) {}", "task_payload", true},
		// Entry point attributes.
		{"task stage", "wgpu_mesh_shader",
			"@task @workgroup_size(1) fn ts() -> @builtin(mesh_task_size) vec3u { return vec3u(1u); }",
			"task", true},
		{"mesh stage", "wgpu_mesh_shader", "@mesh(out) @workgroup_size(1) fn ms() {}", "mesh", false},
		{"payload", "wgpu_mesh_shader", "@payload(p) fn f() {}", "payload", true},
		{"ray_generation stage", "wgpu_ray_tracing_pipeline", "@ray_generation fn f() {}", "ray_generation", false},
		{"any_hit stage", "wgpu_ray_tracing_pipeline", "@any_hit fn f() {}", "any_hit", false},
		{"closest_hit stage", "wgpu_ray_tracing_pipeline", "@closest_hit fn f() {}", "closest_hit", false},
		{"miss stage", "wgpu_ray_tracing_pipeline", "@miss fn f() {}", "miss", false},
		{"incoming_payload", "wgpu_ray_tracing_pipeline", "@incoming_payload(p) fn f() {}", "incoming_payload", false},
		// lower/mod.rs: debugPrintf is checked before its arguments.
		{"debugPrintf", "wgpu_debug_printf", "fn f() { debugPrintf(); }", "debugPrintf", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pe := parseError(t, tc.source)
			if want := notEnabled(tc.ext); pe.Message != want {
				t.Fatalf("error %q, want %q", pe.Message, want)
			}
			if len(pe.Labels) == 0 {
				t.Fatalf("error %q has no label", pe.Message)
			}
			span := pe.Labels[0].Span
			if got := tc.source[span.Start:span.End]; got != tc.word {
				t.Errorf("label covers %q, want %q", got, tc.word)
			}

			enabled := "enable " + tc.ext + ";\n" + tc.source
			_, _, err := Parse(enabled, ParseOptions{})
			if tc.implemented {
				if err != nil {
					t.Errorf("with `enable %s;`: %v", tc.ext, err)
				}
			} else if err != nil && strings.Contains(err.Error(), "enable extension is not enabled") {
				t.Errorf("with `enable %s;` the gate still fails: %v", tc.ext, err)
			}
		})
	}
}

// TestEnableExtensionAnyOf: the ray query types need either of two
// extensions (map_predeclared_type's extensions_needed list).
func TestEnableExtensionAnyOf(t *testing.T) {
	for _, src := range []string{
		"enable wgpu_ray_tracing_pipeline;\n@group(0) @binding(0) var a: acceleration_structure;",
		"enable wgpu_ray_tracing_pipeline;\nfn f() { var q: ray_query; }",
	} {
		if _, _, err := Parse(src, ParseOptions{}); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

// TestEnableExtensionVertexReturn: map_ray_flag gates vertex_return on its
// own extension, past the ray query one.
func TestEnableExtensionVertexReturn(t *testing.T) {
	const src = "enable wgpu_ray_query;\n@group(0) @binding(0) var a: acceleration_structure<vertex_return>;"
	pe := parseError(t, src)
	if want := notEnabled("wgpu_ray_query_vertex_return"); pe.Message != want {
		t.Fatalf("error %q, want %q", pe.Message, want)
	}
	if span := pe.Labels[0].Span; src[span.Start:span.End] != "vertex_return" {
		t.Errorf("label covers %q, want %q", src[span.Start:span.End], "vertex_return")
	}
}

// TestEnableDirectiveUnknown: a name that is no enable-extension is Rust's
// UnknownEnableExtension, labeled at the name (parse/directive/
// enable_extension.rs from_ident); a typo is not silently ignored.
func TestEnableDirectiveUnknown(t *testing.T) {
	for _, tc := range []struct{ source, word string }{
		{"enable no_such_ext;", "no_such_ext"},
		{"enable f61;\nvar<private> x: f32;", "f61"},
		{"enable f16, clip_distance;", "clip_distance"},
		{"enable F16;", "F16"},
	} {
		pe := parseError(t, tc.source)
		if want := "unknown enable-extension `" + tc.word + "`"; pe.Message != want {
			t.Errorf("%q: error %q, want %q", tc.source, pe.Message, want)
			continue
		}
		if len(pe.Labels) == 0 {
			t.Errorf("%q: no label", tc.source)
			continue
		}
		if span := pe.Labels[0].Span; tc.source[span.Start:span.End] != tc.word {
			t.Errorf("%q: label covers %q, want %q", tc.source, tc.source[span.Start:span.End], tc.word)
		}
	}
}

// TestEnableDirectiveKnown: every name Rust naga knows is accepted, and so
// are swizzle_assignment (W3C gpuweb#5268, implemented here, not in Rust)
// and the WGSL spec's subgroup_size_control.
func TestEnableDirectiveKnown(t *testing.T) {
	src := "enable f16, clip_distances, dual_source_blending, wgpu_mesh_shader, wgpu_ray_query,\n" +
		"    wgpu_ray_query_vertex_return, wgpu_ray_tracing_pipeline, wgpu_cooperative_matrix,\n" +
		"    primitive_index, draw_index, wgpu_per_vertex, wgpu_binding_array, wgpu_int16,\n" +
		"    wgpu_debug_printf, subgroups, subgroup_size_control;\n" +
		"enable swizzle_assignment;\n" +
		"var<private> x: f16;"
	if _, _, err := Parse(src, ParseOptions{}); err != nil {
		t.Fatal(err)
	}
}

// TestEnableSubgroups: the WGSL spec gates subgroup built-in values,
// subgroup built-in functions and quad built-in functions on `enable
// subgroups;` (enable-extensions table: "Otherwise, any use will result in
// a shader-creation error"), and subgroup_size_control enables subgroups.
// Deviation from Rust naga, whose enable_extension.rs maps `subgroups` to
// Unimplemented (it rejects the directive as EnableExtensionNotYetImplemented,
// gfx-rs/wgpu#5555) and gates none of these built-ins; the spec decides.
func TestEnableSubgroups(t *testing.T) {
	for _, tc := range []struct{ name, source, word string }{
		{"built-in value subgroup_size",
			"@compute @workgroup_size(64) fn main(@builtin(subgroup_size) size: u32) {}", "subgroup_size"},
		{"built-in value subgroup_invocation_id",
			"@compute @workgroup_size(64) fn main(@builtin(subgroup_invocation_id) id: u32) {}", "subgroup_invocation_id"},
		// The subgroup_id language extension: "when the subgroups extension
		// is enabled".
		{"built-in value subgroup_id",
			"@compute @workgroup_size(64) fn main(@builtin(subgroup_id) id: u32) {}", "subgroup_id"},
		{"built-in value num_subgroups",
			"@compute @workgroup_size(64) fn main(@builtin(num_subgroups) n: u32) {}", "num_subgroups"},
		{"subgroup function",
			"@compute @workgroup_size(64) fn main() { _ = subgroupAdd(1u); }", "subgroupAdd"},
		{"subgroupBallot",
			"@compute @workgroup_size(64) fn main() { _ = subgroupBallot(true); }", "subgroupBallot"},
		{"quad function",
			"@compute @workgroup_size(64) fn main() { _ = quadSwapX(1u); }", "quadSwapX"},
		{"quadBroadcast",
			"@compute @workgroup_size(64) fn main() { _ = quadBroadcast(1u, 0u); }", "quadBroadcast"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pe := parseError(t, tc.source)
			if want := notEnabled("subgroups"); pe.Message != want {
				t.Fatalf("error %q, want %q", pe.Message, want)
			}
			if span := pe.Labels[0].Span; tc.source[span.Start:span.End] != tc.word {
				t.Errorf("label covers %q, want %q", tc.source[span.Start:span.End], tc.word)
			}
			for _, ext := range []string{"subgroups", "subgroup_size_control"} {
				if _, _, err := Parse("enable "+ext+";\n"+tc.source, ParseOptions{}); err != nil {
					t.Errorf("with `enable %s;`: %v", ext, err)
				}
			}
		})
	}
}

// TestDirectiveAfterDeclaration: directives come before every global
// declaration (WGSL grammar: translation_unit is global_directive* then
// global_decl*), as Rust's parser requires
// (Error::DirectiveAfterFirstGlobalDecl).
func TestDirectiveAfterDeclaration(t *testing.T) {
	for _, src := range []string{
		"const c = 1;\nenable f16;\n",
		"fn f() {}\nenable subgroups;\n",
		"struct S { x: f32 }\ndiagnostic(off, derivative_uniformity);\n",
	} {
		pe := parseError(t, src)
		if want := "expected global declaration, but found a global directive"; pe.Message != want {
			t.Errorf("%q: error %q, want %q", src, pe.Message, want)
		}
	}
	if _, _, err := Parse("enable f16;\ndiagnostic(off, derivative_uniformity);\nconst c = 1h;\n", ParseOptions{}); err != nil {
		t.Errorf("directives first: %v", err)
	}
}

// TestEnableSubgroupsNotGated: subgroupBarrier, a wgpu function the WGSL
// spec does not define, needs no enable, as in Rust.
func TestEnableSubgroupsNotGated(t *testing.T) {
	const src = "@compute @workgroup_size(64)\n" +
		"fn main() {\n" +
		"    subgroupBarrier();\n" +
		"}\n"
	if _, _, err := Parse(src, ParseOptions{}); err != nil {
		t.Fatal(err)
	}
}

// TestEnableShadowedPredeclaredType: a module-scope declaration named like a
// gated predeclared type is the declaration, and needs no enable (Rust's
// lowerer looks up module-scope declarations before predeclared types).
func TestEnableShadowedPredeclaredType(t *testing.T) {
	const src = "struct RayDesc { x: f32, }\nvar<private> d: RayDesc;"
	if _, _, err := Parse(src, ParseOptions{}); err != nil {
		t.Fatal(err)
	}
}
