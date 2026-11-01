package dxil

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// The read-only visitors of this package (the expression and statement
// walkers of the reachability scan, the scalarizer's pointer check and the
// signature-usage liveness) each carry their own type switch over the IR's
// kinds. These tests pin every one of them: each kind of package ir is built
// with a distinct handle in every ExpressionHandle field, also inside slices,
// pointers, structs, nested interfaces and nested blocks, and every handle
// must be visited. A kind a visitor has no case for, or a field a case
// skips, fails here instead of leaving a use unseen. A kind added to package
// ir without an entry in irKindRegistry fails too (see ir_kinds_test.go).
//
// An operand a visitor skips on purpose is listed in the test of that
// visitor as an explicit exception, with the reason.

// visited collects the handles a visitor reports.
type visited map[ir.ExpressionHandle]bool

func (v visited) add(h ir.ExpressionHandle) { v[h] = true }

// handleCount is the length of a dense table that has a slot for every
// handle in hs and the one after the largest.
func handleCount(hs []ir.ExpressionHandle) int {
	n := ir.ExpressionHandle(0)
	for _, h := range hs {
		n = max(n, h)
	}
	return int(n) + 2
}

// aliveSet turns a liveness table into the set of live handles.
func aliveSet(alive []bool) visited {
	out := visited{}
	for h, a := range alive {
		if a {
			out.add(ir.ExpressionHandle(h))
		}
	}
	return out
}

// eachExprKind runs check for every variant of every expression kind.
func eachExprKind(t *testing.T, check func(t *testing.T, kind ir.ExpressionKind)) {
	t.Helper()
	b := newIRKinds(t, filepath.Join("..", "ir"))
	for _, kind := range b.exprKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				check(t, b.build(t, kind, variant).(ir.ExpressionKind))
			}
		})
	}
}

// eachStmtKind runs check for every variant of every statement kind.
func eachStmtKind(t *testing.T, check func(t *testing.T, kind ir.StatementKind)) {
	t.Helper()
	b := newIRKinds(t, filepath.Join("..", "ir"))
	for _, kind := range b.stmtKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				check(t, b.build(t, kind, variant).(ir.StatementKind))
			}
		})
	}
}

// wantStmtHandles is what a visitor that follows every operand of a
// statement, and the handles of an Emit range one by one, reports.
func wantStmtHandles(kind ir.StatementKind) []ir.ExpressionHandle {
	if emit, ok := kind.(ir.StmtEmit); ok {
		return emitRange(emit.Range)
	}
	return collectExprHandles(reflect.ValueOf(kind))
}

// TestVisitExprSubHandlesEveryKind pins visitExprSubHandles, which the
// reachability scan and the workgroup scalarizer's check for a leftover
// pointer rely on to see every use of an expression.
func TestVisitExprSubHandlesEveryKind(t *testing.T) {
	eachExprKind(t, func(t *testing.T, kind ir.ExpressionKind) {
		got := visited{}
		visitExprSubHandles(kind, got.add)
		expectHandles(t, "visitExprSubHandles", collectExprHandles(reflect.ValueOf(kind)), got)
	})
}

// TestWalkStmtExprsEveryKind pins walkStmtExprs, the statement walker of the
// reachability scan, which follows the handles of an Emit range one by one.
func TestWalkStmtExprsEveryKind(t *testing.T) {
	eachStmtKind(t, func(t *testing.T, kind ir.StatementKind) {
		got := visited{}
		walkStmtExprs(&ir.Function{}, kind, func(_ *ir.Function, h ir.ExpressionHandle) { got.add(h) })
		expectHandles(t, "walkStmtExprs", wantStmtHandles(kind), got)
	})
}

// TestPropagateExprLivenessEveryKind pins propagateExprLiveness, the
// expression half of the signature-usage liveness: an alive expression
// makes every expression it reads alive.
func TestPropagateExprLivenessEveryKind(t *testing.T) {
	eachExprKind(t, func(t *testing.T, kind ir.ExpressionKind) {
		want := collectExprHandles(reflect.ValueOf(kind))
		// The kind sits at handle 0; the builder hands out handles from 1.
		fn := &ir.Function{Expressions: []ir.Expression{{Kind: kind}}}
		alive := make([]bool, handleCount(want))
		propagateExprLiveness(fn, 0, alive)
		expectHandles(t, "propagateExprLiveness", want, aliveSet(alive))
	})
}

// TestMarkAliveFromStmtEveryKind pins markAliveFromStmt, the statement half
// of the signature-usage liveness: a statement makes every expression it
// reads alive, and an Emit its whole range.
func TestMarkAliveFromStmtEveryKind(t *testing.T) {
	eachStmtKind(t, func(t *testing.T, kind ir.StatementKind) {
		want := wantStmtHandles(kind)
		alive := make([]bool, handleCount(want))
		markAliveFromStmt(&ir.Statement{Kind: kind}, &ir.Function{}, alive)
		expectHandles(t, "markAliveFromStmt", want, aliveSet(alive))
	})
}
