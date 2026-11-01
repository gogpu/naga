package dce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// The operands of an expression and of a statement are walked by
// internal/irutil's walkers, pinned against every kind by its
// TestWalkersCoverEveryKind. The tests here pin what this package decides on
// top of them, for every statement kind of package ir: which statements are
// liveness roots, and which have side effects a caller observes. A kind
// added to package ir without an entry in irKindRegistry fails both (see
// ir_kinds_test.go).

type visited map[ir.ExpressionHandle]bool

func (v visited) add(h ir.ExpressionHandle) { v[h] = true }

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

// sideEffects is what stmtHasSideEffects reports for each statement kind
// built by irKinds: in an empty function, where no pointer is a local and
// every statement nested in a block is a store to such a pointer. A kind
// without an entry fails TestStmtHasSideEffectsEveryKind: decide whether it
// has a side effect, give stmtHasSideEffects a case for it, and list it.
var sideEffects = map[string]bool{
	"StmtEmit": false, "StmtReturn": false, "StmtBreak": false, "StmtContinue": false,
	"StmtSubgroupBallot": false, "StmtSubgroupCollectiveOperation": false, "StmtSubgroupGather": false,
	"StmtStore": true, "StmtImageStore": true, "StmtAtomic": true, "StmtImageAtomic": true,
	"StmtKill": true, "StmtControlBarrier": true, "StmtMemoryBarrier": true,
	"StmtCall": true, "StmtWorkGroupUniformLoad": true, "StmtRayQuery": true,
	// Through the store nested in them.
	"StmtBlock": true, "StmtIf": true, "StmtSwitch": true, "StmtLoop": true,
}

// TestStmtHasSideEffectsEveryKind pins stmtHasSideEffects, which decides
// whether a call to a function can be dropped when its result is unused.
func TestStmtHasSideEffectsEveryKind(t *testing.T) {
	b := newIRKinds(t, filepath.Join("..", "..", "..", "..", "ir"))
	for _, kind := range b.stmtKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			want, listed := sideEffects[kind.Name()]
			if !listed {
				t.Fatalf("%s has no entry in sideEffects", kind.Name())
			}
			for variant := 0; variant < b.variants(t, kind); variant++ {
				k := b.build(t, kind, variant).(ir.StatementKind)
				got := stmtHasSideEffects(ir.Statement{Kind: k}, map[ir.ExpressionHandle]uint32{}, &ir.Function{})
				if got != want {
					t.Errorf("stmtHasSideEffects(%#v) = %v, want %v", k, got, want)
				}
			}
		})
	}

	// An empty construct, and a store to a local, have none.
	lv := ir.ExpressionHandle(0)
	fn := &ir.Function{
		LocalVars:   []ir.LocalVariable{{Name: "x"}},
		Expressions: []ir.Expression{{Kind: ir.ExprLocalVariable{Variable: 0}}},
	}
	localPtrs := map[ir.ExpressionHandle]uint32{lv: 0}
	for _, k := range []ir.StatementKind{
		ir.StmtIf{}, ir.StmtSwitch{Cases: []ir.SwitchCase{{}}}, ir.StmtLoop{}, ir.StmtBlock{},
		ir.StmtStore{Pointer: lv, Value: lv},
	} {
		if stmtHasSideEffects(ir.Statement{Kind: k}, localPtrs, fn) {
			t.Errorf("stmtHasSideEffects(%#v) = true, want false", k)
		}
	}
}
