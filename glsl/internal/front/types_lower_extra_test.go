package front

// Contracts of GLSL type resolution, Rust naga's parser/types.rs:19-134 and the
// std140/std430 layout of offset.rs:43-190: array specifiers, struct member
// arrays, the layouts std140 cannot express, and the uint constant rule of
// parse_uint_constant (parser.rs:203-229).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestArrayOfArraysStrides nests two array specifiers. The specifiers are
// applied in source order, each wrapped around the previous type, exactly as
// Rust's parse_array_specifier_single does (parser/types.rs:32-77): the last
// written specifier is the outermost array.
func TestArrayOfArraysStrides(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
float a[2][3];
void main() { o = vec4(a[0][0]); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	outer, ok := mod.Types[findGlobal(t, mod, "a").Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("a's type = %T, want an array", mod.Types[findGlobal(t, mod, "a").Type].Inner)
	}
	if outer.Size.Constant == nil || *outer.Size.Constant != 3 {
		t.Fatalf("outer size = %+v, want the last written specifier's 3", outer.Size)
	}
	if outer.Stride != 8 {
		t.Fatalf("outer stride = %d, want the inner array's 8 bytes", outer.Stride)
	}
	inner, ok := mod.Types[outer.Base].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("outer base = %T, want the inner array", mod.Types[outer.Base].Inner)
	}
	if inner.Stride != 4 || inner.Size.Constant == nil || *inner.Size.Constant != 2 {
		t.Fatalf("inner array = %+v, want stride 4 and size 2", inner)
	}
}

// TestStructMemberArrayKeepsItsStride is the member arm of
// parse_struct_declaration_list (declarations.rs:625-685) under the std140
// layout a plain struct definition uses (parser/types.rs:86-92): an array
// member strides by a whole vec4, and the next member is placed after it.
func TestStructMemberArrayKeepsItsStride(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
struct S { float a[2]; float b; };
void main() { S s; s.b = 1.0; o = vec4(s.a[0]); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	var structTy ir.StructType
	for _, local := range main.LocalVars {
		if st, ok := mod.Types[local.Type].Inner.(ir.StructType); ok {
			structTy = st
		}
	}
	if len(structTy.Members) != 2 {
		t.Fatalf("the struct has %d members, want 2", len(structTy.Members))
	}
	array, ok := mod.Types[structTy.Members[0].Type].Inner.(ir.ArrayType)
	if !ok || array.Stride != 16 {
		t.Fatalf("member 0 = %v, want the float array with the std140 stride 16", mod.Types[structTy.Members[0].Type].Inner)
	}
	if structTy.Members[1].Offset != 32 {
		t.Fatalf("member b offset = %d, want 32 after the padded array", structTy.Members[1].Offset)
	}
}

// TestStd140RefusesMatrixOfTwoRows is offset.rs:125-144: the IR's matrix stride
// cannot express a two-row matrix under std140, so the block reports it.
func TestStd140RefusesMatrixOfTwoRows(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(set = 0, binding = 0) uniform U { mat3x2 m; } u;
layout(location = 0) out vec4 o;
void main() { o = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
	want := "unsupported matrix of the form matCx2 (in this case mat3x2) in std140 block layout."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// TestStd140RefusesF16Matrix is the other shape offset.rs:125-144 refuses: an
// f16 matrix cannot be expressed by the IR's stride either.
func TestStd140RefusesF16Matrix(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(set = 0, binding = 0) uniform U { f16mat2 m; } u;
layout(location = 0) out vec4 o;
void main() { o = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "unsupported matrix of the form f16matCxR") {
		t.Fatalf("error = %v, want the f16 matrix refusal", err)
	}
}

// TestArraySizeConstantErrors covers parse_uint_constant (parser.rs:203-229):
// a negative value overflows, a value that is not an integer is not a uint, an
// expression is folded first, and zero is refused by the array rule.
func TestArraySizeConstantErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "negative constant",
			source: "const int N = -1;\nfloat a[N];",
			want:   "int constant overflows",
		},
		{
			name:   "negative literal",
			source: "float a[-2];",
			want:   "int constant overflows",
		},
		{
			name:   "float literal",
			source: "float a[2.5];",
			want:   "Expected a uint constant",
		},
		{
			name:   "boolean",
			source: "float a[true];",
			want:   "Expected a uint constant",
		},
		{
			name:   "zero",
			source: "float a[0];",
			want:   "Array size must be greater than zero",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "#version 450\n" + tt.source + "\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(1.0); }\n"
			_, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestArraySizeExpressionIsFolded is the other half of the same rule: a
// constant expression is evaluated, not refused.
func TestArraySizeExpressionIsFolded(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
float a[1 + 1];
void main() { o = vec4(a[0]); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	array, ok := mod.Types[findGlobal(t, mod, "a").Type].Inner.(ir.ArrayType)
	if !ok || array.Size.Constant == nil {
		t.Fatalf("a's type = %T, want a constant-sized array", mod.Types[findGlobal(t, mod, "a").Type].Inner)
	}
	if *array.Size.Constant != 2 {
		t.Fatalf("array size = %d, want the folded 2", *array.Size.Constant)
	}
}

// TestU32FromLiteral is the literal conversion of proc/mod.rs:242-254: unsigned
// and non-negative signed values convert, a negative one is its own error, and
// anything else is not an integer constant.
func TestU32FromLiteral(t *testing.T) {
	tests := []struct {
		name    string
		lit     ir.LiteralValue
		want    uint32
		wantErr string
	}{
		{"u16", ir.LiteralU16(3), 3, ""},
		{"u32", ir.LiteralU32(4), 4, ""},
		{"i16", ir.LiteralI16(5), 5, ""},
		{"i32", ir.LiteralI32(6), 6, ""},
		{"negative i16", ir.LiteralI16(-1), 0, "negative constant"},
		{"negative i32", ir.LiteralI32(-2), 0, "negative constant"},
		{"float", ir.LiteralF32(1), 0, "not an integer constant"},
		{"bool", ir.LiteralBool(true), 0, "not an integer constant"},
		{"u64", ir.LiteralU64(7), 0, "not an integer constant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := u32FromLiteral(tt.lit)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("u32FromLiteral: %v", err)
			}
			if got != tt.want {
				t.Fatalf("value = %d, want %d", got, tt.want)
			}
		})
	}
}
