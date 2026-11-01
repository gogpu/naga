package dxil

import (
	"slices"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestBlockUses pins what the reachability scan and the signature-usage
// liveness count as a use, on top of the operand walker
// (irutil.StmtExprHandles, pinned against every kind in internal/irutil):
// every expression of an Emit range, the operands of every other statement,
// and the statements nested in control flow.
func TestBlockUses(t *testing.T) {
	breakIf := ir.ExpressionHandle(8)
	ret := ir.ExpressionHandle(11)
	block := ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
		{Kind: ir.StmtIf{
			Condition: 1,
			Accept:    ir.Block{{Kind: ir.StmtStore{Pointer: 6, Value: 7}}},
		}},
		{Kind: ir.StmtLoop{
			Body:    ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: 9, End: 10}}}},
			BreakIf: &breakIf,
		}},
		{Kind: ir.StmtSwitch{Selector: 2, Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtReturn{Value: &ret}}}},
		}}},
		{Kind: ir.StmtBlock{Block: ir.Block{{Kind: ir.StmtCall{Arguments: []ir.ExpressionHandle{12, 13}}}}}},
	}

	var got []ir.ExpressionHandle
	blockUses(block, func(h ir.ExpressionHandle) { got = append(got, h) })
	slices.Sort(got)
	want := []ir.ExpressionHandle{1, 2, 3, 4, 6, 7, 8, 9, 11, 12, 13}
	if !slices.Equal(got, want) {
		t.Errorf("blockUses visited %v, want %v", got, want)
	}

	// One statement: an Emit is its range, anything else its operands.
	got = nil
	stmtUses(ir.StmtImageStore{Image: 20, Coordinate: 21, Value: 22}, func(h ir.ExpressionHandle) { got = append(got, h) })
	if want := []ir.ExpressionHandle{20, 21, 22}; !slices.Equal(got, want) {
		t.Errorf("stmtUses(image store) visited %v, want %v", got, want)
	}
}
