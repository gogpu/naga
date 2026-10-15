package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// A var<private> initializer that depends on overrides gets its literal
// operands in the variable's type, as in Rust naga (gfx-rs/wgpu 14328f49):
//
//	var<private> x: u32 = cnt + 1;   → Binary(Add, Override(cnt), Literal(U32(1)))
//	var<private> y: i32 = -s * 4;    → Binary(Multiply, Unary(Negate, Override(s)), Literal(I32(4)))
//
// Negating an override stays a Unary; only a negated literal is folded.
func TestOverrideDependentVarInitLiteralTypes(t *testing.T) {
	src := `override cnt: u32 = 3u;
override s: i32 = 2;
var<private> x: u32 = cnt + 1;
var<private> y: i32 = -s * 4;
@compute @workgroup_size(1) fn main() { x = x + 1u; y = y + 1; }
`
	m := helperCompileWGSL(t, src)
	ge := m.GlobalExpressions
	initOf := func(name string) ir.ExprBinary {
		t.Helper()
		for _, g := range m.GlobalVariables {
			if g.Name == name && g.InitExpr != nil {
				if b, ok := ge[*g.InitExpr].Kind.(ir.ExprBinary); ok {
					return b
				}
				t.Fatalf("%s init = %T, want Binary", name, ge[*g.InitExpr].Kind)
			}
		}
		t.Fatalf("%s: no InitExpr", name)
		return ir.ExprBinary{}
	}

	x := initOf("x")
	if _, ok := ge[x.Left].Kind.(ir.ExprOverride); !ok || x.Op != ir.BinaryAdd {
		t.Errorf("x: op %v left %T, want Add with Override", x.Op, ge[x.Left].Kind)
	}
	if lit, ok := ge[x.Right].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralValue(ir.LiteralU32(1)) {
		t.Errorf("x right = %#v, want Literal(U32(1))", ge[x.Right].Kind)
	}

	y := initOf("y")
	neg, ok := ge[y.Left].Kind.(ir.ExprUnary)
	if !ok || neg.Op != ir.UnaryNegate || y.Op != ir.BinaryMultiply {
		t.Fatalf("y: op %v left %#v, want Multiply with Unary(Negate)", y.Op, ge[y.Left].Kind)
	}
	if _, ok := ge[neg.Expr].Kind.(ir.ExprOverride); !ok {
		t.Errorf("y: negate operand = %T, want Override (not folded)", ge[neg.Expr].Kind)
	}
	if lit, ok := ge[y.Right].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralValue(ir.LiteralI32(4)) {
		t.Errorf("y right = %#v, want Literal(I32(4))", ge[y.Right].Kind)
	}
}

// An override's default value must be a single literal of the override's
// declared type, as in Rust naga: the abstract literal is converted to the
// declared type and a negated literal is folded. Expected values are Rust naga
// IR dumps of the same declarations (gfx-rs/wgpu 14328f49).
//
// Before the fix (BUG-GE-001): `u32 = 0` was lowered as F32(0.0), `i32 = -1`
// as Unary(Negate, ...), and suffixed float defaults (`2.3f`, `1.5h`) were
// dropped entirely — Override.Init was nil, so the default was lost.
func TestOverrideDefaultIsLiteralOfDeclaredType(t *testing.T) {
	tests := []struct {
		decl string
		want ir.LiteralValue
	}{
		{"override a: u32 = 0;", ir.LiteralU32(0)},
		{"override b: i32 = -1;", ir.LiteralI32(-1)},
		{"override c: f32 = 2.3f;", ir.LiteralF32(2.3)},
		{"override d: f32 = 2.3;", ir.LiteralF32(2.3)},
		{"override e = 1.5;", ir.LiteralF32(1.5)},
		{"override f: u32 = 7u;", ir.LiteralU32(7)},
		{"override g: bool = true;", ir.LiteralBool(true)},
		{"override h: f32 = 2;", ir.LiteralF32(2)},
		{"override k: f16 = 1.5h;", ir.LiteralF16(1.5)},
	}
	for _, tt := range tests {
		t.Run(tt.decl, func(t *testing.T) {
			src := "enable f16;\n" + tt.decl + "\n@compute @workgroup_size(1) fn main() {}\n"
			m := helperCompileWGSL(t, src)
			if len(m.Overrides) != 1 {
				t.Fatalf("overrides: got %d, want 1", len(m.Overrides))
			}
			o := m.Overrides[0]
			if o.Init == nil {
				t.Fatalf("Override.Init is nil — default value lost")
			}
			lit, ok := m.GlobalExpressions[*o.Init].Kind.(ir.Literal)
			if !ok {
				t.Fatalf("init = %T, want a single Literal", m.GlobalExpressions[*o.Init].Kind)
			}
			if lit.Value != tt.want {
				t.Errorf("init literal = %T(%v), want %T(%v)", lit.Value, lit.Value, tt.want, tt.want)
			}
		})
	}
}
