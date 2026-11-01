package emit

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// visitExpressionHandles and visitStatementHandles find the uses of a local
// variable's pointer handle (liveLocalVars), so a handle they skip is a use
// that goes unseen. Each kind of package ir is built with a distinct handle
// in every ExpressionHandle field, also inside slices, pointers, structs,
// nested interfaces and nested blocks, and every handle must be visited. A
// kind added to package ir without an entry in irKindRegistry fails too (see
// ir_kinds_test.go).

type visited map[ir.ExpressionHandle]bool

func (v visited) add(h ir.ExpressionHandle) { v[h] = true }

// TestVisitExpressionHandlesEveryKind pins visitExpressionHandles.
func TestVisitExpressionHandlesEveryKind(t *testing.T) {
	b := newIRKinds(t, filepath.Join("..", "..", "..", "ir"))
	for _, kind := range b.exprKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				k := b.build(t, kind, variant).(ir.ExpressionKind)
				got := visited{}
				visitExpressionHandles(k, got.add)
				expectHandles(t, "visitExpressionHandles", collectExprHandles(reflect.ValueOf(k)), got)
			}
		})
	}
}

// TestVisitStatementHandlesEveryKind pins visitStatementHandles, also for
// the statements nested in blocks.
func TestVisitStatementHandlesEveryKind(t *testing.T) {
	b := newIRKinds(t, filepath.Join("..", "..", "..", "ir"))
	for _, kind := range b.stmtKinds() {
		t.Run(kind.Name(), func(t *testing.T) {
			for variant := 0; variant < b.variants(t, kind); variant++ {
				k := b.build(t, kind, variant).(ir.StatementKind)
				want := collectExprHandles(reflect.ValueOf(k))
				if kind == irEmitType {
					// The one deliberate exception: an Emit range describes
					// when expressions are evaluated, not a use of them, and
					// the expressions' own operands are visited through
					// visitExpressionHandles (statements.go, the StmtEmit
					// case of visitStatementHandlesOne).
					want = nil
				}
				got := visited{}
				visitStatementHandles(ir.Block{{Kind: k}}, got.add)
				expectHandles(t, "visitStatementHandles", want, got)
			}
		})
	}
}
