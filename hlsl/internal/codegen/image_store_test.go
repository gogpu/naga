package codegen

// Storage image stores into arrayed textures: HLSL accepts a single int3, so
// the coordinate and the array index are composed as int3(coordinate, index),
// exactly as Rust naga writes it (back/hlsl/writer.rs Statement::ImageStore).

import (
	"regexp"
	"strings"
	"testing"
)

// TestCompile_TextureStore2DArray checks the int3 composition of a store into
// texture_storage_2d_array.
func TestCompile_TextureStore2DArray(t *testing.T) {
	src := `
@group(0) @binding(0) var img: texture_storage_2d_array<rgba8unorm, write>;

@compute @workgroup_size(1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    textureStore(img, vec2<i32>(i32(gid.x), i32(gid.y)), 3, vec4<f32>(2.0, 2.0, 2.0, 2.0));
}
`
	code := compileWGSLToHLSL(t, src, nil)

	// The store must be a single int3 index, never a comma-separated pair.
	store := regexp.MustCompile(`(?m)^\s*img\[(.*)\] = (.*);$`)
	m := store.FindStringSubmatch(code)
	if m == nil {
		t.Fatalf("no image store line in output:\n%s", code)
	}
	if !strings.HasPrefix(m[1], "int3(") || !strings.HasSuffix(m[1], ")") {
		t.Errorf("store index = %q, want a single int3(coordinate, index)", m[1])
	}
	if strings.Contains(m[1], "],") {
		t.Errorf("store index %q composes the array index outside int3", m[1])
	}
}

// TestCompile_TextureLoad2DArray pins the load form that the store now
// matches: HLSL's RWTexture2DArray.Load also takes one int3.
func TestCompile_TextureLoad2DArray(t *testing.T) {
	src := `
@group(0) @binding(0) var img: texture_storage_2d_array<rgba8unorm, read>;

@compute @workgroup_size(1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let c = textureLoad(img, vec2<i32>(i32(gid.x), i32(gid.y)), 4);
    let f = c.x + c.y + c.z + c.w;
    _ = f;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	load := regexp.MustCompile(`(?m)^\s*float4 c = img\.Load\(int3\(.*\)\);$`)
	if !load.MatchString(code) {
		t.Errorf("expected img.Load(int3(coordinate, index)) in output:\n%s", code)
	}
}
