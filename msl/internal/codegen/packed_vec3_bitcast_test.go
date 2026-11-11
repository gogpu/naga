package codegen

import "testing"

// BUG-MSL-PACKED-BITCAST-001: the wrapping-arithmetic path reinterprets its
// operands with as_type, but a packed_int3 struct member is 12 bytes where
// metal::uint3 is 16, so `as_type<metal::uint3>(in.chunk)` does not compile.
// Rust naga's put_bitcasted_expression converts a packed vec3 to the regular
// vector first: as_type<metal::uint3>(metal::int3(in.chunk)).
//
// Reference: back/msl/writer.rs put_bitcasted_expression (get_packed_vec_kind
// branch). Expected line below is Rust naga's output for the same source.
func TestPackedVec3IsUnpackedBeforeBitcast(t *testing.T) {
	src := `struct VertexInput {
    @location(0) chunk : vec3<i32>,
    @location(1) texture_index : u32,
}

struct VertexOutput {
    @builtin(position) clip_position : vec4<f32>,
}

@vertex
fn vs_main(in : VertexInput) -> VertexOutput {
    var out : VertexOutput;
    let position = vec3<f32>(in.chunk - vec3<i32>(5));
    return out;
}
`
	code := compileWGSL(t, src)

	want := "metal::float3 position = static_cast<metal::float3>(as_type<metal::int3>(as_type<metal::uint3>(metal::int3(in.chunk)) - as_type<metal::uint3>(metal::int3(5))));"
	if !containsLine(code, want) {
		t.Errorf("missing line (Rust naga output):\n  %s\ngot:\n%s", want, linesContaining(code, "position ="))
	}
	bad := "as_type<metal::uint3>(in.chunk)"
	if got := linesContaining(code, bad); got != "" {
		t.Errorf("bitcast applies as_type directly to the packed member:\n%s", got)
	}
}
