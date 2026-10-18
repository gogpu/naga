package overrides

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestFilterEmits: an Emit is split around expressions that need no Emit
// (Rust filter_emits_in_block), inside nested blocks too.
func TestFilterEmits(t *testing.T) {
	lit := ir.Expression{Kind: ir.Literal{Value: ir.LiteralU32(1)}}
	load := ir.Expression{Kind: ir.ExprLoad{Pointer: 0}}
	// 0 load, 1 literal, 2 load, 3 load, 4 literal
	exprs := []ir.Expression{load, lit, load, load, lit}
	emit := func(s, e ir.ExpressionHandle) ir.Statement {
		return ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: s, End: e}}}
	}
	block := ir.Block{
		emit(0, 5),
		{Kind: ir.StmtIf{Condition: 0, Accept: ir.Block{emit(1, 2)}, Reject: ir.Block{emit(2, 4)}}},
	}
	got := filterEmits(block, exprs)
	want := ir.Block{
		emit(0, 1), emit(2, 4),
		{Kind: ir.StmtIf{Condition: 0, Accept: ir.Block{}, Reject: ir.Block{emit(2, 4)}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterEmits:\n got %+v\nwant %+v", got, want)
	}
}
