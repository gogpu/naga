package dce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// visitExprHandles carries liveness from an expression to the expressions it
// reads, so a handle it skips is an operand the sweep may remove while the
// emitter still needs it. Each kind of package ir is built with a distinct
// handle in every ExpressionHandle field, also inside slices, pointers,
// structs and nested interfaces, and every handle must be visited. A kind
// added to package ir without an entry in irKindRegistry fails too (see
// ir_kinds_test.go).

type visited map[ir.ExpressionHandle]bool

func (v visited) add(h ir.ExpressionHandle) { v[h] = true }

// TestVisitExprHandlesEveryKind pins visitExprHandles.
func TestVisitExprHandlesEveryKind(t *testing.T) {
	b := newIRKinds(t, filepath.Join("..", "..", "..", "..", "ir"))
	for _, kind := range b.exprKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				k := b.build(t, kind, variant).(ir.ExpressionKind)
				got := visited{}
				visitExprHandles(k, got.add)
				expectHandles(t, "visitExprHandles", collectExprHandles(reflect.ValueOf(k)), got)
			}
		})
	}
}

// TestMarkStmtRootsEveryKind pins markStmtRoots, which marks the operands of
// the statements that are side-effecting roots: a handle it skips is an
// operand the sweep may remove while the emitter still needs it. With no
// module the callee of a StmtCall counts as side-effecting, and nothing in
// the empty function is a local variable, so a store is a root.
func TestMarkStmtRootsEveryKind(t *testing.T) {
	b := newIRKinds(t, filepath.Join("..", "..", "..", "..", "ir"))
	for _, kind := range b.stmtKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				k := b.build(t, kind, variant).(ir.StatementKind)
				want := collectExprHandles(reflect.ValueOf(k))
				if kind == irEmitType {
					// The one deliberate exception: an Emit range defines
					// when expressions are evaluated and is not a root; its
					// expressions become live when a root reaches them
					// (dce.go, the StmtEmit case of markStmtRoots).
					want = nil
				}
				got := visited{}
				markStmtRoots(nil, &ir.Function{}, k, map[ir.ExpressionHandle]uint32{}, got.add)
				expectHandles(t, "markStmtRoots", want, got)
			}
		})
	}
}
