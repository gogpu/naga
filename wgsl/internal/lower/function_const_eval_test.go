package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// Function bodies evaluate constant operands with the shared evaluator
// (internal/consteval, ADR-007), as Rust naga's lowerer does through
// try_eval_and_append. The WGSL spec makes a const-expression that overflows,
// leaves a function's domain or divides by zero a shader-creation error;
// before the evaluator the lowerer emitted NaN/Inf literals (invalid IR) or
// kept 1 / 0 as a run-time division. Messages are Rust v30's.
func TestFunctionBodyConstEvalErrors(t *testing.T) {
	tests := []struct {
		stmt string
		want string
	}{
		{"let x = sqrt(-1.0);", "sqrt built-in function argument is out of valid range"},
		{"let x = log(0.0);", "log built-in function argument is out of valid range"},
		{"let x = acos(2.0);", "acos built-in function argument is out of valid range"},
		{"let x = exp(1000.0f);", "exp operation overflowed"},
		{"let x = 1e38f * 10.0f;", "Float literal is infinite"},
		{"let x = 1 / 0;", "Division by zero"},
		{"let x = 7u % 0u;", "Remainder by zero"},
		{"let x = i32(-2147483648) / -1i;", "division operation overflowed"},
		{"let x = vec2(1.0, -4.0); let y = sqrt(vec2(1.0, -4.0));", "sqrt built-in function argument is out of valid range"},
		// Spec errors Rust misses (it keeps these as run-time expressions).
		{"let x = smoothstep(1.0, 1.0, 0.5);", "smoothstep built-in function argument is out of valid range"},
	}
	for _, tt := range tests {
		t.Run(tt.stmt, func(t *testing.T) {
			src := "@compute @workgroup_size(1) fn main() { " + tt.stmt + " }"
			_, err := compileWGSL(t, src)
			if err == nil {
				t.Fatalf("compiled, want error %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// fnExprs lowers a compute shader with the given body and returns main's
// expressions and named expressions.
func fnExprs(t *testing.T, body string) (*ir.Module, *ir.Function) {
	t.Helper()
	m, err := compileWGSL(t, "@compute @workgroup_size(1) fn main() { "+body+" }")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(m.EntryPoints) != 1 {
		t.Fatalf("%d entry points", len(m.EntryPoints))
	}
	return m, &m.EntryPoints[0].Function
}

// namedExpr returns the expression a let binding names.
func namedExpr(t *testing.T, f *ir.Function, name string) ir.ExpressionKind {
	t.Helper()
	for h, n := range f.NamedExpressions {
		if n == name {
			return f.Expressions[h].Kind
		}
	}
	t.Fatalf("no named expression %q", name)
	return nil
}

func TestFunctionBodyConstEvalValues(t *testing.T) {
	tests := []struct {
		body string
		want ir.LiteralValue
	}{
		{"let x = sqrt(4.0);", ir.LiteralF32(2)},
		{"let x = 2147483647i + 1i;", ir.LiteralI32(-2147483648)}, // concrete ints wrap
		{"let x = u32(-1.5);", ir.LiteralU32(0)},                  // spec float->int saturation
		{"let x = i32(3e9f);", ir.LiteralI32(2147483520)},         // largest f32 below 2^31
		{"let x = max(3, 7) + 1;", ir.LiteralI32(8)},
		{"let x = all(vec2(true, false));", ir.LiteralBool(false)},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			_, f := fnExprs(t, tt.body)
			got, ok := namedExpr(t, f, "x").(ir.Literal)
			if !ok || got.Value != tt.want {
				t.Errorf("x = %#v, want Literal %#v", namedExpr(t, f, "x"), tt.want)
			}
		})
	}
}

// Rust naga's IR shape is kept where Rust does not evaluate: the @const
// built-ins it does not implement and expressions on let-bound values (WGSL:
// a let-bound value is not a const-expression).
func TestFunctionBodyRustShape(t *testing.T) {
	t.Run("mix", func(t *testing.T) {
		_, f := fnExprs(t, "let x = mix(1f, 3f, 0.5f);")
		if m, ok := namedExpr(t, f, "x").(ir.ExprMath); !ok || m.Fun != ir.MathMix {
			t.Errorf("x = %#v, want Math(mix)", namedExpr(t, f, "x"))
		}
	})
	// Better than Rust: Rust v30 rejects this valid shader ("failed to
	// convert expression to a concrete type"); the call is a const-expression.
	t.Run("mix of abstract values", func(t *testing.T) {
		_, f := fnExprs(t, "let x = mix(1.0, 3.0, 0.5);")
		if l, ok := namedExpr(t, f, "x").(ir.Literal); !ok || l.Value != ir.LiteralF32(2) {
			t.Errorf("x = %#v, want Literal 2.0", namedExpr(t, f, "x"))
		}
	})
	t.Run("let-bound operand", func(t *testing.T) {
		_, f := fnExprs(t, "let a = 2.0; let x = a * 3.0;")
		if b, ok := namedExpr(t, f, "x").(ir.ExprBinary); !ok || b.Op != ir.BinaryMultiply {
			t.Errorf("x = %#v, want Binary(*)", namedExpr(t, f, "x"))
		}
	})
	t.Run("function-local const operand", func(t *testing.T) {
		_, f := fnExprs(t, "const a = 2.0; let x = a * 3.0;")
		if l, ok := namedExpr(t, f, "x").(ir.Literal); !ok || l.Value != ir.LiteralF32(6) {
			t.Errorf("x = %#v, want Literal 6.0", namedExpr(t, f, "x"))
		}
	})
}
