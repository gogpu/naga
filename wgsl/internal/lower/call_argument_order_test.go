package lower

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// describeExpr is a compact, comparable spelling of a function-arena
// expression for order tests.
func describeExpr(e ir.Expression) string {
	switch k := e.Kind.(type) {
	case ir.Literal:
		return fmt.Sprintf("Literal(%T(%v))", k.Value, k.Value)
	case ir.ExprCompose:
		return fmt.Sprintf("Compose%v", k.Components)
	case ir.ExprZeroValue:
		return "ZeroValue"
	default:
		return fmt.Sprintf("%T", e.Kind)
	}
}

// User function call arguments are lowered and converted to the parameter
// type one at a time, as in Rust naga (front/wgsl/lower/mod.rs, call(): the
// arguments map lowers each argument with expression_for_abstract and
// immediately applies try_automatic_conversions). An abstract argument is
// therefore concretized before the next argument is lowered, which fixes the
// arena order. Expected order taken from the Rust naga v30 oracle for the same
// source.
func TestCallArgumentsConvertedInOrder(t *testing.T) {
	src := `fn f(a: i32, b: vec4<f32>) {}
fn g(a: f32, b: i32) {}
@compute @workgroup_size(1) fn main() {
    f(1, vec4f(2, 3, 4, 5));
    g(1, 2);
    f(3, vec4<f32>());
}
`
	m := helperCompileWGSL(t, src)
	if len(m.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(m.EntryPoints))
	}
	exprs := m.EntryPoints[0].Function.Expressions
	got := make([]string, len(exprs))
	for i, e := range exprs {
		got[i] = describeExpr(e)
	}
	want := []string{
		"Literal(ir.LiteralI32(1))", // f: a, concretized before b is lowered
		"Literal(ir.LiteralF32(2))",
		"Literal(ir.LiteralF32(3))",
		"Literal(ir.LiteralF32(4))",
		"Literal(ir.LiteralF32(5))",
		"Compose[1 2 3 4]",
		"Literal(ir.LiteralF32(1))", // g: a
		"Literal(ir.LiteralI32(2))", // g: b
		"Literal(ir.LiteralI32(3))", // f: a, before the ZeroValue of b
		"ZeroValue",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("entry point expressions:\n got: %v\nwant: %v", got, want)
	}
}
