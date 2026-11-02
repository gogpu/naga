package valid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestUnaryOperatorNames: every unary operator has its Rust Debug name, so a
// new operator cannot reach an error message as UnaryOperator(n).
func TestUnaryOperatorNames(t *testing.T) {
	want := map[ir.UnaryOperator]string{
		ir.UnaryNegate:     "Negate",
		ir.UnaryLogicalNot: "LogicalNot",
		ir.UnaryBitwiseNot: "BitwiseNot",
	}
	for op := ir.UnaryNegate; op <= ir.UnaryBitwiseNot; op++ {
		if got := unaryOperatorName(op); got != want[op] {
			t.Errorf("unaryOperatorName(%d) = %q, want %q", op, got, want[op])
		}
	}
	if len(unaryOperatorNames) != int(ir.UnaryBitwiseNot)+1 {
		t.Errorf("unaryOperatorNames has %d names for %d operators", len(unaryOperatorNames), int(ir.UnaryBitwiseNot)+1)
	}
}
