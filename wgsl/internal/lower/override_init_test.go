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

// && and || become If-blocks only in a function body (Rust
// front/wgsl/lower/mod.rs, binary_logical: ctx.is_runtime()). In the override
// and const contexts a left operand that is not a constant (an override) gives
// a plain Binary that pipeline creation evaluates; a constant left operand
// short-circuits. Expected IR: Rust naga v30.
func TestLogicalOperatorsInOverrideContext(t *testing.T) {
	src := `override o: bool = true;
var<private> v: bool = o && false;
override p = o || true;
const Q = (1 < 2) && (2 < 3);
fn f() -> bool { return v || p || Q; }
@compute @workgroup_size(1) fn main() { _ = f(); }
`
	m := helperCompileWGSL(t, src)
	ge := m.GlobalExpressions
	wantBinary := func(what string, h ir.ExpressionHandle, op ir.BinaryOperator, right bool) {
		t.Helper()
		b, ok := ge[h].Kind.(ir.ExprBinary)
		if !ok || b.Op != op {
			t.Fatalf("%s = %#v, want Binary %v", what, ge[h].Kind, op)
		}
		if _, ok := ge[b.Left].Kind.(ir.ExprOverride); !ok {
			t.Errorf("%s left = %#v, want Override", what, ge[b.Left].Kind)
		}
		if lit, ok := ge[b.Right].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralBool(right) {
			t.Errorf("%s right = %#v, want Literal(%v)", what, ge[b.Right].Kind, right)
		}
	}
	for _, g := range m.GlobalVariables {
		if g.Name == "v" {
			if g.InitExpr == nil {
				t.Fatal("v: no initializer")
			}
			wantBinary("v init", *g.InitExpr, ir.BinaryLogicalAnd, false)
		}
	}
	for _, o := range m.Overrides {
		if o.Name == "p" {
			if o.Init == nil {
				t.Fatal("p: no initializer")
			}
			wantBinary("p init", *o.Init, ir.BinaryLogicalOr, true)
		}
	}
	q := constByName(t, m, "Q")
	if lit, ok := ge[q.Init].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralBool(true) {
		t.Errorf("Q init = %#v, want Literal(true)", ge[q.Init].Kind)
	}
	if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
		t.Errorf("validation: %v %v", err, errs)
	}
}

// In a function body && and || always become an If-block, even with a
// constant left operand (Rust front/wgsl/lower/mod.rs, binary_logical: the
// ctx.is_runtime() branch does not look at the operand's value), so
// `true || q()` keeps the call in an untaken branch and binds a local.
func TestLogicalOperatorsInFunctionBodyAreIfBlocks(t *testing.T) {
	src := `fn q() -> bool { return false; }
@compute @workgroup_size(1) fn main() {
    let a = true || q();
    let b = false && q();
    _ = a; _ = b;
}
`
	m := helperCompileWGSL(t, src)
	fn := &m.EntryPoints[0].Function
	ifs := 0
	for _, s := range fn.Body {
		st, ok := s.Kind.(ir.StmtIf)
		if !ok {
			continue
		}
		ifs++
		calls := 0
		for _, a := range st.Accept {
			if _, ok := a.Kind.(ir.StmtCall); ok {
				calls++
			}
		}
		if calls != 1 {
			t.Errorf("If %d: accept block has %d calls of q, want 1", ifs, calls)
		}
	}
	if ifs != 2 {
		t.Errorf("entry point body has %d If statements, want 2", ifs)
	}
	if len(fn.LocalVars) != 2 {
		t.Errorf("%d local variables, want 2 (one per operator)", len(fn.LocalVars))
	}
}
