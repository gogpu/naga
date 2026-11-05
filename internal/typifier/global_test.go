package typifier

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestModuleGlobalTypes: ForModule also types the module's global (constant)
// expressions, with no arguments and no local variables, as Rust's
// validator does (ModuleInfo.const_expression_types). A backend writes a
// constant Splat with the type of its value from here.
func TestModuleGlobalTypes(t *testing.T) {
	tF16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	m := &ir.Module{
		Types:     []ir.Type{{Inner: tI32}},
		Constants: []ir.Constant{{Name: "c", Type: 0, Init: 0}},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(7)}},
			{Kind: ir.ExprConstant{Constant: 0}},
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 1}},
			{Kind: ir.Literal{Value: ir.LiteralF16(1.5)}},
			{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 3}},
		},
	}
	mt, err := ForModule(m)
	if err != nil {
		t.Fatalf("ForModule: %v", err)
	}
	g, err := mt.Global()
	if err != nil {
		t.Fatalf("Global: %v", err)
	}
	if g.Len() != len(m.GlobalExpressions) {
		t.Fatalf("Global() typed %v expressions, want %d", g, len(m.GlobalExpressions))
	}
	for h, want := range map[ir.ExpressionHandle]ir.TypeInner{
		1: tI32,
		2: ir.VectorType{Size: ir.Vec3, Scalar: tI32},
		4: ir.VectorType{Size: ir.Vec2, Scalar: tF16},
	} {
		if got := g.Inner(m, h); got != want {
			t.Errorf("global expression [%d]: %#v, want %#v", h, got, want)
		}
	}
}

// TestModuleGlobalTypesRefuseLocals: a global expression that reads a
// function argument has no type, as in Rust, where the const-expression
// resolve context has no arguments. Only Global reports it: ForModule still
// types the functions, so a backend that never reads global types keeps
// its own error for such a module.
func TestModuleGlobalTypesRefuseLocals(t *testing.T) {
	m := &ir.Module{GlobalExpressions: []ir.Expression{{Kind: ir.ExprFunctionArgument{Index: 0}}}}
	mt, err := ForModule(m)
	if err != nil {
		t.Fatalf("ForModule: %v", err)
	}
	if _, err := mt.Global(); err == nil {
		t.Errorf("Global typed a global expression that reads a function argument")
	}
}
