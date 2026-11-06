// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dce

import (
	"testing"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// loops returns the loops of block, not looking into nested blocks.
func loops(block ir.Block) []ir.StmtLoop {
	var out []ir.StmtLoop
	for _, st := range block {
		if k, ok := st.Kind.(ir.StmtLoop); ok {
			out = append(out, k)
		}
	}
	return out
}

// counterLoop is the IR of a promoted counter loop:
//
//	loop {
//	    let i = phi(0, i + 1);   // header phi, emitted first in the body
//	    let c = i < 10u;
//	    if c {} else { break; }
//	    <extra>
//	    continuing { let n = i + 1u; }
//	}
//
// The fields are the handles a test needs to look at or to build on.
type counterLoop struct {
	fn   *ir.Function
	phi  ir.ExpressionHandle
	cond ir.ExpressionHandle
	next ir.ExpressionHandle
	loop ir.Statement
}

// newCounterLoop appends the expressions of a counter loop to fn; prepare
// adds the expressions <extra> needs and returns its statements, built after
// the loop's own expressions exist so they may read i.
func newCounterLoop(fn *ir.Function, prepare func(i ir.ExpressionHandle) ir.Block) counterLoop {
	zero := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(0)})
	ten := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(10)})
	one := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(1)})
	phi := appendExpr(fn, ssa.Phi{})
	cond := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryLess, Left: phi, Right: ten})
	next := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryAdd, Left: phi, Right: one})
	fn.Expressions[phi].Kind = ssa.Phi{Incoming: []ssa.Incoming{
		{PredKey: ssa.PredLoopInit, Value: zero},
		{PredKey: ssa.PredLoopBackEdge, Value: next},
	}}
	var extra ir.Block
	if prepare != nil {
		extra = prepare(phi)
	}
	body := ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: phi, End: cond + 1}}},
		{Kind: ir.StmtIf{Condition: cond, Reject: ir.Block{{Kind: ir.StmtBreak{}}}}},
	}
	body = append(body, extra...)
	return counterLoop{fn: fn, phi: phi, cond: cond, next: next, loop: ir.Statement{Kind: ir.StmtLoop{
		Body:       body,
		Continuing: ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: next, End: next + 1}}}},
	}}}
}

// storageOut adds a read-write storage global of type u32 to mod and
// returns its type.
func storageOut(mod *ir.Module) ir.TypeHandle {
	u32 := scalarTypeHandle(mod, ir.ScalarUint, 4)
	mod.GlobalVariables = append(mod.GlobalVariables, ir.GlobalVariable{
		Name: "out", Space: ir.SpaceStorage, Access: ir.StorageAccessLoad | ir.StorageAccessStore, Type: u32,
	})
	return u32
}

// TestDeadCounterLoopRemoved: a loop that only counts, with nothing read
// after it, goes entirely, as LLVM's LoopDeletion removes it in DXC (the
// HLSL change there drops the trip-count check: HLSL loops make forward
// progress). The loop's exit condition used to be a root, which kept the
// counter and the loop alive.
func TestDeadCounterLoopRemoved(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}
	cl := newCounterLoop(fn, nil)
	fn.Body = ir.Block{cl.loop, {Kind: ir.StmtReturn{}}}

	Run(mod, fn)

	if n := len(loops(fn.Body)); n != 0 {
		t.Fatalf("loops after DCE = %d, want 0: the loop has no observable effect", n)
	}
	if len(fn.Body) != 1 {
		t.Errorf("body = %d statements, want only the return", len(fn.Body))
	}
}

// TestDeadLoopInsideDeadIf: a dead loop nested in an if goes, and then the
// if, whose condition no longer controls anything live.
func TestDeadLoopInsideDeadIf(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{Arguments: []ir.FunctionArgument{{Name: "c"}}}
	c := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	cl := newCounterLoop(fn, nil)
	fn.Body = ir.Block{
		{Kind: ir.StmtIf{Condition: c, Accept: ir.Block{cl.loop}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	if len(fn.Body) != 1 {
		t.Fatalf("body = %d statements, want only the return", len(fn.Body))
	}
}

// TestLoopResultStoredToGlobalKept: the loop's counter is read after the
// loop (through its header phi) and stored to a storage buffer, so the loop
// and its exit condition stay.
func TestLoopResultStoredToGlobalKept(t *testing.T) {
	mod := &ir.Module{}
	storageOut(mod)
	fn := &ir.Function{}
	cl := newCounterLoop(fn, nil)
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	fn.Body = ir.Block{
		cl.loop,
		{Kind: ir.StmtStore{Pointer: out, Value: cl.phi}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	got := loops(fn.Body)
	if len(got) != 1 {
		t.Fatalf("loops after DCE = %d, want 1: its counter is stored after it", len(got))
	}
	if !emitted(got[0].Body, cl.cond) {
		t.Errorf("exit condition [%d] is not emitted, but it decides the stored value", cl.cond)
	}
	if !emitted(got[0].Continuing, cl.next) {
		t.Errorf("increment [%d] is not emitted, but the header phi reads it", cl.next)
	}
	if len(ifs(got[0].Body)) != 1 {
		t.Errorf("the exit if is gone from the live loop")
	}
}

// TestEarlyReturnKeepsCondition: `if c { return; } out = 7;` — the return
// decides whether the store runs, so c and the if stay.
func TestEarlyReturnKeepsCondition(t *testing.T) {
	mod := &ir.Module{}
	u32 := storageOut(mod)
	fn := &ir.Function{Arguments: []ir.FunctionArgument{{Name: "li", Type: u32}}}
	li := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	two := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(2)})
	c := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryGreater, Left: li, Right: two})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(7)})
	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: c, End: c + 1}}},
		{Kind: ir.StmtIf{Condition: c, Accept: ir.Block{{Kind: ir.StmtReturn{}}}}},
		{Kind: ir.StmtStore{Pointer: out, Value: seven}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	if len(ifs(fn.Body)) != 1 {
		t.Fatalf("the early-return if is gone")
	}
	if !emitted(fn.Body, c) {
		t.Errorf("condition [%d] is not emitted, but the early return reads it", c)
	}
}

// TestBreakInLiveSwitchCaseKeepsCondition: `switch s { default: { if c
// { break; } out = 7; } }` — the break decides whether the store runs.
func TestBreakInLiveSwitchCaseKeepsCondition(t *testing.T) {
	mod := &ir.Module{}
	u32 := storageOut(mod)
	fn := &ir.Function{Arguments: []ir.FunctionArgument{{Name: "s", Type: u32}, {Name: "c"}}}
	s := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	c := appendExpr(fn, ir.ExprFunctionArgument{Index: 1})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(7)})
	fn.Body = ir.Block{
		{Kind: ir.StmtSwitch{Selector: s, Cases: []ir.SwitchCase{{Value: ir.SwitchValueDefault{}, Body: ir.Block{
			{Kind: ir.StmtIf{Condition: c, Accept: ir.Block{{Kind: ir.StmtBreak{}}}}},
			{Kind: ir.StmtStore{Pointer: out, Value: seven}},
		}}}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	if len(fn.Body) != 2 {
		t.Fatalf("body = %d statements, want the switch and the return", len(fn.Body))
	}
	sw, ok := fn.Body[0].Kind.(ir.StmtSwitch)
	if !ok {
		t.Fatalf("statement 0 is %T, want the switch", fn.Body[0].Kind)
	}
	if len(ifs(sw.Cases[0].Body)) != 1 {
		t.Errorf("the if with the break is gone from the live switch case")
	}
}

// TestDeadInnerLoopInLiveLoopRemoved: an inner counter loop with no effect
// goes even though the loop around it stores to a storage buffer.
func TestDeadInnerLoopInLiveLoopRemoved(t *testing.T) {
	mod := &ir.Module{}
	storageOut(mod)
	fn := &ir.Function{}
	inner := newCounterLoop(fn, nil)
	var out ir.ExpressionHandle
	outer := newCounterLoop(fn, func(i ir.ExpressionHandle) ir.Block {
		out = appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
		return ir.Block{inner.loop, {Kind: ir.StmtStore{Pointer: out, Value: i}}}
	})
	fn.Body = ir.Block{outer.loop, {Kind: ir.StmtReturn{}}}

	Run(mod, fn)

	got := loops(fn.Body)
	if len(got) != 1 {
		t.Fatalf("top-level loops = %d, want the outer one", len(got))
	}
	if n := len(loops(got[0].Body)); n != 0 {
		t.Errorf("inner loops = %d, want 0: the inner loop has no effect", n)
	}
	if !emitted(got[0].Body, outer.cond) {
		t.Errorf("outer exit condition [%d] is not emitted", outer.cond)
	}
}

// TestLoopWithObservableEffectKept: a loop is kept, with its exit
// condition, whenever its body holds something observable. Each case puts
// one such statement into the body of a counter loop whose counter is read
// by nothing else.
func TestLoopWithObservableEffectKept(t *testing.T) {
	tests := []struct {
		name  string
		extra func(mod *ir.Module, fn *ir.Function, i ir.ExpressionHandle) ir.Block
	}{
		{"store to storage", func(mod *ir.Module, fn *ir.Function, i ir.ExpressionHandle) ir.Block {
			storageOut(mod)
			out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
			return ir.Block{{Kind: ir.StmtStore{Pointer: out, Value: i}}}
		}},
		{"atomic", func(mod *ir.Module, fn *ir.Function, i ir.ExpressionHandle) ir.Block {
			storageOut(mod)
			out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
			return ir.Block{{Kind: ir.StmtAtomic{Pointer: out, Fun: ir.AtomicAdd{}, Value: i}}}
		}},
		{"control barrier", func(*ir.Module, *ir.Function, ir.ExpressionHandle) ir.Block {
			return ir.Block{{Kind: ir.StmtControlBarrier{Flags: ir.BarrierWorkGroup}}}
		}},
		{"memory barrier", func(*ir.Module, *ir.Function, ir.ExpressionHandle) ir.Block {
			return ir.Block{{Kind: ir.StmtMemoryBarrier{Flags: ir.BarrierStorage}}}
		}},
		{"discard", func(*ir.Module, *ir.Function, ir.ExpressionHandle) ir.Block {
			return ir.Block{{Kind: ir.StmtKill{}}}
		}},
		{"return of a value", func(_ *ir.Module, _ *ir.Function, i ir.ExpressionHandle) ir.Block {
			return ir.Block{{Kind: ir.StmtReturn{Value: &i}}}
		}},
		{"bare return", func(*ir.Module, *ir.Function, ir.ExpressionHandle) ir.Block {
			return ir.Block{{Kind: ir.StmtReturn{}}}
		}},
		{"image store", func(mod *ir.Module, fn *ir.Function, i ir.ExpressionHandle) ir.Block {
			mod.GlobalVariables = append(mod.GlobalVariables, ir.GlobalVariable{Name: "img", Space: ir.SpaceHandle})
			img := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
			return ir.Block{{Kind: ir.StmtImageStore{Image: img, Coordinate: i, Value: i}}}
		}},
		{"derivative stored to storage", func(mod *ir.Module, fn *ir.Function, i ir.ExpressionHandle) ir.Block {
			storageOut(mod)
			d := appendExpr(fn, ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeNone, Expr: i})
			out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
			return ir.Block{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: d, End: d + 1}}},
				{Kind: ir.StmtStore{Pointer: out, Value: d}},
			}
		}},
		{"subgroup ballot", func(_ *ir.Module, fn *ir.Function, _ ir.ExpressionHandle) ir.Block {
			r := appendExpr(fn, ir.ExprSubgroupBallotResult{})
			return ir.Block{{Kind: ir.StmtSubgroupBallot{Result: r}}}
		}},
		{"call with side effects", func(mod *ir.Module, _ *ir.Function, _ ir.ExpressionHandle) ir.Block {
			callee := ir.Function{Name: "effect"}
			callee.Body = ir.Block{{Kind: ir.StmtControlBarrier{Flags: ir.BarrierWorkGroup}}}
			mod.Functions = append(mod.Functions, callee)
			return ir.Block{{Kind: ir.StmtCall{Function: ir.FunctionHandle(len(mod.Functions) - 1)}}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := &ir.Module{}
			fn := &ir.Function{}
			cl := newCounterLoop(fn, func(i ir.ExpressionHandle) ir.Block { return tt.extra(mod, fn, i) })
			fn.Body = ir.Block{cl.loop, {Kind: ir.StmtReturn{}}}

			Run(mod, fn)

			got := loops(fn.Body)
			if len(got) != 1 {
				t.Fatalf("loops after DCE = %d, want 1: the body has an observable effect", len(got))
			}
			if len(ifs(got[0].Body)) != 1 {
				t.Errorf("the exit if is gone: the loop would not terminate")
			}
			if !emitted(got[0].Body, cl.cond) {
				t.Errorf("exit condition [%d] is not emitted", cl.cond)
			}
			if !emitted(got[0].Continuing, cl.next) {
				t.Errorf("increment [%d] is not emitted, but the exit condition depends on it", cl.next)
			}
		})
	}
}

// TestLoopKeptByLivePhiKeepsItsExit: a loop whose body holds nothing live
// but which a live phi right after it keeps (feedsLivePhi) is walked as a
// live loop, so the condition of the if that leaves it stays. It used to
// stay unmarked, and the sweep then dropped the exit if: the loop would
// never end. mem2reg puts no phi after a loop today (ssa.PredKey has no
// loop-exit edge), so this guards the shape before a pass creates it.
func TestLoopKeptByLivePhiKeepsItsExit(t *testing.T) {
	mod := &ir.Module{}
	storageOut(mod)
	fn := &ir.Function{Arguments: []ir.FunctionArgument{{Name: "x"}}}
	x := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	ten := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(10)})
	cond := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryLess, Left: x, Right: ten})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(7)})
	phi := appendExpr(fn, ssa.Phi{Incoming: []ssa.Incoming{{PredKey: ssa.PredFallThrough, Value: seven}}})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	fn.Body = ir.Block{
		{Kind: ir.StmtLoop{Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: cond, End: cond + 1}}},
			{Kind: ir.StmtIf{Condition: cond, Reject: ir.Block{{Kind: ir.StmtBreak{}}}}},
		}}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: phi, End: phi + 1}}},
		{Kind: ir.StmtStore{Pointer: out, Value: phi}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	got := loops(fn.Body)
	if len(got) != 1 {
		t.Fatalf("loops after DCE = %d, want 1: the phi after it is stored", len(got))
	}
	if len(ifs(got[0].Body)) != 1 {
		t.Fatalf("the exit if is gone: the loop would never end")
	}
	if !emitted(got[0].Body, cond) {
		t.Errorf("exit condition [%d] is not emitted", cond)
	}
}
