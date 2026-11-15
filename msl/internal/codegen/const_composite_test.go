package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// The expected text of these tests is Rust naga 29.0.0 MSL output for the
// equivalent modules and WGSL sources (naga-cli, ReadZeroSkipWrite policies).

// TestGlobalArrayConstantDoubleBraces: a constant of a fixed-size array type is
// written with two brace levels, because a naga array is a Metal array wrapped
// in a struct: `constant type_1 array = type_1 {{1.0, 2.0}};`
// (back/msl/writer.rs, put_possibly_const_expression, Expression::Compose).
func TestGlobalArrayConstantDoubleBraces(t *testing.T) {
	size := uint32(2)
	code := compileModule(t, &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size}, Stride: 4}},
		},
		Constants: []ir.Constant{
			{Name: "array", Type: 1, Init: 2},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
		},
	})
	mustContainMSL(t, code, "constant type_1 array = type_1 {{1.0, 2.0}};")
}

// TestGlobalStructConstantWithArrayMember: the struct level takes one brace
// pair and its array member keeps its own two: `constant S s = S {type_1 {{3.0,
// 4.0}}, 5.0};`
func TestGlobalStructConstantWithArrayMember(t *testing.T) {
	size := uint32(2)
	code := compileModule(t, &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size}, Stride: 4}},
			{Name: "S", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 1, Offset: 0},
					{Name: "b", Type: 0, Offset: 8},
				},
				Span: 12,
			}},
		},
		Constants: []ir.Constant{
			{Name: "s", Type: 2, Init: 4},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(4.0)}},
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
			{Kind: ir.Literal{Value: ir.LiteralF32(5.0)}},
			{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{2, 3}}},
		},
	})
	mustContainMSL(t, code, "constant S s = S {type_1 {{3.0, 4.0}}, 5.0};")
}

// TestVectorComposeKeepsRepeatedComponents: a vector Compose is written out
// component by component even when every component is the same expression; it
// is not collapsed to a one-argument constructor. Rust naga writes
// `metal::float2(1.0, 1.0)` here (long-form-matrix.frag).
func TestVectorComposeKeepsRepeatedComponents(t *testing.T) {
	code := compileModule(t, &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants: []ir.Constant{
			{Name: "v", Type: 1, Init: 1},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 0}}},
		},
	})
	mustContainMSL(t, code, "constant metal::float2 v = metal::float2(1.0, 1.0);")
	mustNotContainMSL(t, code, "metal::float2(1.0);")
}

// TestLocalArrayComposeDoubleBraces: the same two brace levels in a function
// body, where the Compose is a function expression written by writeCompose:
// `type_N local = type_N {{6.0, 7.0}};`
func TestLocalArrayComposeDoubleBraces(t *testing.T) {
	const src = `@group(0) @binding(0) var<storage, read_write> o: array<f32>;

@compute @workgroup_size(1)
fn main() {
    var local = array<f32, 2>(6.0, 7.0);
    o[0] = local[0];
}
`
	code := compileWGSL(t, src)
	re := regexp.MustCompile(`\btype_\d+ local = type_\d+ \{\{6\.0, 7\.0\}\};`)
	if !re.MatchString(code) {
		t.Errorf("missing local array initializer with two brace levels (%s):\n%s", re, code)
	}
}

// TestImageSampleLevelOmittedFor1D: MSL's texture1d has no sample overload
// taking a level, bias or gradient (Tint's MSL intrinsic table, `sample`), so a
// 1D sample drops it, as Rust naga's put_image_sample_level does. A 2D sample
// keeps it.
func TestImageSampleLevelOmittedFor1D(t *testing.T) {
	const src = `@group(0) @binding(0) var t1: texture_1d<f32>;
@group(0) @binding(1) var t2: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;

@fragment fn main() -> @location(0) vec4<f32> {
    let a = textureSampleLevel(t1, s, 0.5, 3.0);
    let b = textureSampleGrad(t1, s, 0.5, 4.0, 4.0);
    let c = textureSampleLevel(t2, s, vec2<f32>(0.5), 3.0);
    let d = textureSampleGrad(t2, s, vec2<f32>(0.5), vec2<f32>(4.0), vec2<f32>(4.0));
    return a + b + c + d;
}
`
	code := compileWGSL(t, src)
	if got := strings.Count(code, Namespace+"level("); got != 1 {
		t.Errorf("metal::level written %d times, want 1 (2D only)", got)
	}
	if got := strings.Count(code, Namespace+"gradient2d("); got != 1 {
		t.Errorf("metal::gradient2d written %d times, want 1 (2D only)", got)
	}
}
