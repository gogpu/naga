package codegen

import (
	"strings"
	"testing"
)

// WGSL puts the columns of a mat2x2<f32> 8 bytes apart in a uniform buffer;
// a std140 block puts them 16 apart. Declared as is, the matrix read its
// second column from the vec2 after it and that vec2 from the padding beyond.
// A uniform buffer declares such a matrix as its vec2 columns, which std140
// packs like WGSL, and the load rebuilds the matrix. Storage buffers (std430)
// keep the regular struct.
func TestUniformMatCx2IsStd140Compatible(t *testing.T) {
	module := lowerWGSL(t, `
struct U {
    m: mat2x2<f32>,
    v: vec2<f32>,
}
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<uniform> ms: array<mat2x2<f32>, 2>;
@group(0) @binding(2) var<storage, read> s: U;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let whole = u;
    return vec4<f32>(u.m * u.v + ms[1][0] + s.m[1], whole.v);
}
`)
	out, _, err := Compile(module, Options{LangVersion: Version430})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"struct U_std140_ {\n    vec2 m_0;\n    vec2 m_1;\n    vec2 v;\n};",
		"struct mat2x2_std140_ {\n    vec2 col0;\n    vec2 col1;\n};",
		"uniform U_block_0Fragment { U_std140_ _group_0_binding_0_fs; };",
		"uniform type_2_block_1Fragment { mat2x2_std140_ _group_0_binding_1_fs[2]; };",
		"readonly buffer U_block_2Fragment { U _group_0_binding_2_fs; };",
		"U whole = U_from_std140_(_group_0_binding_0_fs);",
		"mat2x2(_group_0_binding_0_fs.m_0, _group_0_binding_0_fs.m_1)",
		"mat2x2_from_std140_(_group_0_binding_1_fs[1])[0]",
		"_group_0_binding_2_fs.m[1]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// A member whose name ends in a digit is named with an underscore after it,
// m2_; its columns are m2_0 and m2_1, as GLSL reserves m2__0.
func TestStd140ColumnNamesHaveNoDoubleUnderscore(t *testing.T) {
	module := lowerWGSL(t, `
struct U {
    m2: mat2x2<f32>,
    v: vec2<f32>,
}
@group(0) @binding(0) var<uniform> u: U;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(u.m2 * u.v, 0.0, 1.0);
}
`)
	out, _, err := Compile(module, Options{LangVersion: Version430})
	if err != nil {
		t.Fatal(err)
	}
	if want := "struct U_std140_ {\n    vec2 m2_0;\n    vec2 m2_1;\n    vec2 v;\n};"; !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "__") {
		t.Errorf("output has a name with two underscores in a row:\n%s", out)
	}
}
