package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestInt16Types: under `enable wgpu_int16;` the predeclared types i16 and
// u16 are 16-bit integers (Rust naga front/wgsl/parse/conv.rs,
// map_predeclared_type: "i16" => Scalar::I16, "u16" => Scalar::U16), usable
// as scalars and vector components, and their constructors fold to
// Literal::I16 / Literal::U16 in the constant evaluator.
func TestInt16Types(t *testing.T) {
	m := helperCompileWGSL(t, `enable wgpu_int16;
enable f16;
var<private> p: i16 = i16(1);
const c: u16 = u16(20);
const v = vec2<i16>(i16(-3), i16(4));
// The spec's float to integral conversion: the largest f16 not above
// i16's maximum is 32752. Rust naga clamps to 32767 (i16::MAX as f32,
// constant_evaluator.rs cast), which an f16 cannot represent.
const clamped: i16 = i16(f16(33000.0));
const wrapped: u16 = u16(70000u);
const low: i16 = i16(-5i);
`)
	scalar := func(kind ir.ScalarKind) ir.ScalarType { return ir.ScalarType{Kind: kind, Width: 2} }
	consts := map[string]ir.LiteralValue{
		"c":       ir.LiteralU16(20),
		"clamped": ir.LiteralI16(32752),
		"wrapped": ir.LiteralU16(70000 % 65536),
		"low":     ir.LiteralI16(-5),
	}
	for name, want := range consts {
		c := constByName(t, m, name)
		lit, ok := m.GlobalExpressions[c.Init].Kind.(ir.Literal)
		if !ok {
			t.Errorf("%s: init %T, want a literal", name, m.GlobalExpressions[c.Init].Kind)
			continue
		}
		if lit.Value != want {
			t.Errorf("%s = %#v, want %#v", name, lit.Value, want)
		}
	}
	if got := m.Types[constByName(t, m, "v").Type].Inner; got != (ir.VectorType{Size: ir.Vec2, Scalar: scalar(ir.ScalarSint)}) {
		t.Errorf("v: type %#v, want vec2<i16>", got)
	}
	for _, gv := range m.GlobalVariables {
		if gv.Name == "p" {
			if got := m.Types[gv.Type].Inner; got != scalar(ir.ScalarSint) {
				t.Errorf("p: type %#v, want i16", got)
			}
		}
	}

	// An abstract value out of range is an error (WGSL: a conversion of an
	// abstract value must be exact). Rust converts with `v as i16`, wrapping.
	expectError(t, "enable wgpu_int16;\nconst big = i16(40000);", "cannot represent the abstract value `40000`")
}

// TestInt16IncrementLiteral: `x++` on an i16 or u16 adds a 16-bit one, as
// Rust's Literal::one(scalar) (proc/mod.rs Literal::new) builds it; naga
// refused the statement.
func TestInt16IncrementLiteral(t *testing.T) {
	m := helperCompileWGSL(t, `enable wgpu_int16;
fn f() { var a: i16; var b: u16; a++; b--; }`)
	var got []ir.LiteralValue
	for _, e := range m.Functions[0].Expressions {
		if lit, ok := e.Kind.(ir.Literal); ok {
			got = append(got, lit.Value)
		}
	}
	if len(got) != 2 || got[0] != ir.LiteralI16(1) || got[1] != ir.LiteralU16(1) {
		t.Fatalf("literals %#v, want i16 1 and u16 1", got)
	}
}
