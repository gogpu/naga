package consteval

// This file covers LocalConstResolver, the optional arena capability behind the
// resolving view: a load of a const-qualified local is a constant expression
// (GLSL 4.50 §4.3.3). The strict view used by the lowering path must keep every
// load run-time, which is the IR shape the parity goldens record, so the two
// views are separate queries and TestTryEvalAndAppendKeepsConstLocalLoad pins
// the strict half.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// constLocalArena is a testArena that also implements LocalConstResolver.
type constLocalArena struct {
	testArena
	locals map[ir.LocalVariableHandle]ir.ExpressionHandle
}

func (a *constLocalArena) LocalConstInit(variable ir.LocalVariableHandle) (ir.ExpressionHandle, bool) {
	init, ok := a.locals[variable]
	return init, ok
}

func newConstLocalEval() (*Evaluator, *constLocalArena, *[]ir.Expression) {
	mod := &ir.Module{}
	var fn []ir.Expression
	a := &constLocalArena{
		testArena: testArena{mod: mod, exprs: &fn},
		locals:    map[ir.LocalVariableHandle]ir.ExpressionHandle{},
	}
	return &Evaluator{Module: mod, Arena: a, Types: a, Behavior: BehaviorRuntime}, a, &fn
}

// appendLoad appends a pointer and a load of it the way the lowering does,
// without the evaluator (a load is not sent through TryEvalAndAppend in these
// tests, which is the point).
func appendLoad(a Arena, local ir.LocalVariableHandle) ir.ExpressionHandle {
	pointer := a.Append(ir.ExprLocalVariable{Variable: local})
	return a.Append(ir.ExprLoad{Pointer: pointer})
}

// TestLoadOfConstLocalIsConst checks that the resolving view gives a load of a
// recorded const local the initializer's stage and value, including through a
// binary operation and a chain of two const locals, while the strict view
// leaves the same load run-time.
func TestLoadOfConstLocalIsConst(t *testing.T) {
	e, a, _ := newConstLocalEval()

	// const int a = 1;
	aInit := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(1)})
	aLoc := ir.LocalVariableHandle(0)
	a.locals[aLoc] = aInit
	aLoad := appendLoad(a, aLoc)

	if e.IsConst(aLoad) {
		t.Fatal("the strict view made a load const, want run-time")
	}
	if !e.IsConstWithConstLocals(aLoad) {
		t.Fatal("the resolving view left a load run-time")
	}
	if got, ok := e.LiteralWithConstLocals(aLoad); !ok || got != ir.LiteralI32(1) {
		t.Fatalf("LiteralWithConstLocals(load(a)) = %v, %v, want 1", got, ok)
	}

	// const int b = a + 1; the lowering keeps the operation run-time because a
	// load is one of its operands, and the resolving view computes its value
	// through the chain.
	one := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(1)})
	bInit, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: aLoad, Right: one})
	if err != nil {
		t.Fatalf("TryEvalAndAppend(a + 1): %v", err)
	}
	if e.IsConst(bInit) {
		t.Fatal("the strict view folded a + 1, want the operation kept run-time")
	}
	bLoc := ir.LocalVariableHandle(1)
	a.locals[bLoc] = bInit
	bLoad := appendLoad(a, bLoc)

	if got, ok := e.LiteralWithConstLocals(bLoad); !ok || got != ir.LiteralI32(2) {
		t.Fatalf("LiteralWithConstLocals(load(b)) = %v, %v, want 2", got, ok)
	}
	if v, ok := e.ValueWithConstLocals(bLoad); !ok || !v.IsScalar() || v.Component(0) != ir.LiteralI32(2) {
		t.Fatalf("ValueWithConstLocals(load(b)) = %v, %v, want 2", v, ok)
	}
}

// TestConstExprWithConstLocals checks that the materializing query returns a
// value-shaped handle the backends can read as a constant, for the initializer
// itself and for a value the lowering kept unevaluated.
func TestConstExprWithConstLocals(t *testing.T) {
	e, a, exprs := newConstLocalEval()

	// const vec2 v = vec2(1, 2);
	x := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(1)})
	y := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(2)})
	vec2 := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}})
	init := mustEval(t, e, ir.ExprCompose{Type: vec2, Components: []ir.ExpressionHandle{x, y}})
	a.locals[0] = init
	load := appendLoad(a, 0)

	h, ok := e.ConstExprWithConstLocals(load)
	if !ok {
		t.Fatal("ConstExprWithConstLocals(load) reported no constant")
	}
	if _, isCompose := (*exprs)[h].Kind.(ir.ExprCompose); !isCompose {
		t.Fatalf("the const expression is %T, want the initializer's Compose", (*exprs)[h].Kind)
	}

	// A scalar value that only the resolving view knows (a + 1 over a const
	// local) has no value shape in the arena, so it is materialized as a fresh
	// literal.
	aScalar := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(1)})
	a.locals[1] = aScalar
	one := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(1)})
	sum, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: appendLoad(a, 1), Right: one})
	if err != nil {
		t.Fatalf("TryEvalAndAppend(load + 1): %v", err)
	}
	a.locals[2] = sum
	h, ok = e.ConstExprWithConstLocals(appendLoad(a, 2))
	if !ok {
		t.Fatal("ConstExprWithConstLocals(load(a + 1)) reported no constant")
	}
	if lit, isLit := (*exprs)[h].Kind.(ir.Literal); !isLit || lit.Value != ir.LiteralI32(2) {
		t.Fatalf("the materialized const expression is %#v, want Literal 2", (*exprs)[h].Kind)
	}
}

// TestLoadOfUnrecordedLocalStaysRuntime is the negative half: a mutable local,
// or any local when the arena cannot resolve const locals, is run-time in both
// views.
func TestLoadOfUnrecordedLocalStaysRuntime(t *testing.T) {
	t.Run("mutable local", func(t *testing.T) {
		e, a, _ := newConstLocalEval()
		load := appendLoad(a, 0)
		if e.IsConstWithConstLocals(load) {
			t.Fatal("the resolving view made an unrecorded load const")
		}
	})

	t.Run("arena without the resolver", func(t *testing.T) {
		e, a, _ := newEval(BehaviorRuntime)
		load := appendLoad(a, 0)
		if e.IsConstWithConstLocals(load) {
			t.Fatal("the resolving view made a load const without a resolver")
		}
	})
}

// TestTryEvalAndAppendKeepsConstLocalLoad pins the strict path: a load of a
// recorded const local is appended as the load itself, never folded to its
// initializer. Folding it changed the IR shape and broke the Rust parity
// goldens (bevy-pbr.frag): the design keeps the fold to the resolving queries.
func TestTryEvalAndAppendKeepsConstLocalLoad(t *testing.T) {
	e, a, exprs := newConstLocalEval()
	init := mustEval(t, e, ir.Literal{Value: ir.LiteralI32(2)})
	a.locals[0] = init
	pointer := a.Append(ir.ExprLocalVariable{Variable: 0})

	before := len(*exprs)
	load, err := e.TryEvalAndAppend(ir.ExprLoad{Pointer: pointer})
	if err != nil {
		t.Fatalf("TryEvalAndAppend(load): %v", err)
	}
	if load != ir.ExpressionHandle(before) {
		t.Fatalf("the load was replaced by expression %d, want the appended load at %d", load, before)
	}
	if _, ok := (*exprs)[load].Kind.(ir.ExprLoad); !ok {
		t.Fatalf("expression %d is %T, want the load", load, (*exprs)[load].Kind)
	}
	if e.IsConst(load) {
		t.Fatal("the strict path folded the load to its initializer")
	}
}
