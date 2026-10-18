package consteval

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestNumericOperandsCopiedLikeRust: in a function arena Rust's
// try_eval_and_append_impl runs check_and_get on every operand of Unary,
// Binary, Math, Select and Relational, deep-copying a constant operand's value
// into the arena before evaluating (constant_evaluator.rs Expression::Binary
// etc.). Pipeline-constant processing does not compact afterwards, so the
// copies decide expression numbering (_eN names) in backend output.
func TestNumericOperandsCopiedLikeRust(t *testing.T) {
	newFn := func() (*Evaluator, *[]ir.Expression, ir.ExpressionHandle) {
		mod := &ir.Module{
			Types:             []ir.Type{{Inner: tU32}},
			GlobalExpressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralU32(7)}}},
			Constants:         []ir.Constant{{Name: "n", Type: 0, Init: 0}},
		}
		var fn []ir.Expression
		a := &testArena{mod: mod, exprs: &fn}
		e := &Evaluator{Module: mod, Arena: a, Types: a, Behavior: BehaviorRuntime}
		c, err := e.TryEvalAndAppend(ir.ExprConstant{Constant: 0})
		if err != nil {
			t.Fatal(err)
		}
		return e, &fn, c
	}
	u := func(v uint32) ir.ExpressionKind { return ir.Literal{Value: ir.LiteralU32(v)} }

	t.Run("binary", func(t *testing.T) {
		e, fn, n := newFn()
		four := mustEval(t, e, u(4))
		h := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryModulo, Left: n, Right: four})
		// Constant n, 4u, copy of n's value 7u, result 3u.
		want := []ir.ExpressionKind{ir.ExprConstant{Constant: 0}, u(4), u(7), u(3)}
		if got := kinds(*fn); !reflect.DeepEqual(got, want) || h != 3 {
			t.Errorf("arena %v (result %d), want %v (result 3)", got, h, want)
		}
	})
	t.Run("select reject accept condition", func(t *testing.T) {
		mod := &ir.Module{
			Types: []ir.Type{{Inner: tU32}, {Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
			GlobalExpressions: []ir.Expression{
				{Kind: u(1)}, {Kind: u(2)}, {Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			},
			Constants: []ir.Constant{{Name: "r", Type: 0, Init: 0}, {Name: "a", Type: 0, Init: 1}, {Name: "c", Type: 1, Init: 2}},
		}
		var fn []ir.Expression
		a := &testArena{mod: mod, exprs: &fn}
		e := &Evaluator{Module: mod, Arena: a, Types: a, Behavior: BehaviorRuntime}
		var hs [3]ir.ExpressionHandle
		for i := range hs {
			hs[i] = mustEval(t, e, ir.ExprConstant{Constant: ir.ConstantHandle(i)})
		}
		mustEval(t, e, ir.ExprSelect{Reject: hs[0], Accept: hs[1], Condition: hs[2]})
		// Copies in Rust's order: reject, accept, condition; then the result.
		want := []ir.ExpressionKind{
			ir.ExprConstant{Constant: 0}, ir.ExprConstant{Constant: 1}, ir.ExprConstant{Constant: 2},
			u(1), u(2), ir.Literal{Value: ir.LiteralBool(true)}, u(2),
		}
		if got := kinds(fn); !reflect.DeepEqual(got, want) {
			t.Errorf("arena %v, want %v", got, want)
		}
	})
	t.Run("kept operation keeps original operands", func(t *testing.T) {
		// A run-time fallback appends the expression with its original
		// operands; the copies stay in the arena, as in Rust.
		mod := &ir.Module{
			Types:             []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}},
			GlobalExpressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF64(5)}}},
			Constants:         []ir.Constant{{Name: "d", Type: 0, Init: 0}},
		}
		var fn []ir.Expression
		a := &testArena{mod: mod, exprs: &fn}
		e := &Evaluator{Module: mod, Arena: a, Types: a, Behavior: BehaviorRuntime}
		d, _ := e.TryEvalAndAppend(ir.ExprConstant{Constant: 0})
		three, _ := e.TryEvalAndAppend(ir.Literal{Value: ir.LiteralF64(3)})
		bin := ir.ExprBinary{Op: ir.BinaryModulo, Left: d, Right: three}
		h, err := e.TryEvalAndAppend(bin)
		if err != nil {
			t.Fatal(err)
		}
		want := []ir.ExpressionKind{ir.ExprConstant{Constant: 0}, ir.Literal{Value: ir.LiteralF64(3)}, ir.Literal{Value: ir.LiteralF64(5)}, bin}
		if got := kinds(fn); !reflect.DeepEqual(got, want) || h != 3 {
			t.Errorf("arena %v (result %d), want %v (result 3)", got, h, want)
		}
	})
}

func kinds(exprs []ir.Expression) []ir.ExpressionKind {
	out := make([]ir.ExpressionKind, len(exprs))
	for i, e := range exprs {
		out[i] = e.Kind
	}
	return out
}
