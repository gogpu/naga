package lower

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// Module-scope initializers are appended to GlobalExpressions when their
// declaration is lowered, in declaration (dependency) order, as in Rust naga
// (front/wgsl/lower/mod.rs: the visit_ordered loop lowers GlobalDeclKind::Var
// and GlobalDeclKind::Override with type_and_init in ctx.as_override()). An
// override initializer must not be deferred behind a later var<private>
// initializer. Expected arena taken from the Rust naga v30 oracle for the same
// source.
func TestGlobalExpressionsInDeclarationOrder(t *testing.T) {
	src := `override a: f32 = 1.5;
override b: f32;
override c = 2 * b;
var<private> v: f32 = a * 10.;
override d: u32 = 0;
@compute @workgroup_size(1) fn main() { v = v + c + f32(d); }
`
	m := helperCompileWGSL(t, src)
	got := make([]string, len(m.GlobalExpressions))
	for i, e := range m.GlobalExpressions {
		switch k := e.Kind.(type) {
		case ir.ExprOverride:
			got[i] = fmt.Sprintf("Override(%d)", k.Override)
		case ir.ExprBinary:
			got[i] = fmt.Sprintf("Binary(%v, %d, %d)", k.Op, k.Left, k.Right)
		default:
			got[i] = describeExpr(e)
		}
	}
	want := []string{
		"Literal(ir.LiteralF32(1.5))", // a
		"Override(1)",                 // c = 2 * b
		"Literal(ir.LiteralF32(2))",
		fmt.Sprintf("Binary(%v, 2, 1)", ir.BinaryMultiply),
		"Override(0)", // v = a * 10.
		"Literal(ir.LiteralF32(10))",
		fmt.Sprintf("Binary(%v, 4, 5)", ir.BinaryMultiply),
		"Literal(ir.LiteralU32(0))", // d
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("global expressions:\n got: %v\nwant: %v", got, want)
	}

	initOf := func(o *ir.Override) string {
		if o.Init == nil {
			return "none"
		}
		return fmt.Sprint(*o.Init)
	}
	if got, want := []string{initOf(&m.Overrides[0]), initOf(&m.Overrides[1]), initOf(&m.Overrides[2]), initOf(&m.Overrides[3])},
		[]string{"0", "none", "3", "7"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("override inits = %v, want %v", got, want)
	}
	if gv := m.GlobalVariables[0]; gv.InitExpr == nil || *gv.InitExpr != 6 {
		t.Errorf("v init = %v, want 6", gv.InitExpr)
	}
}
