package emit

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestImageQueryLodOperands pins the level-of-detail query's operands in both
// DXIL walkers. The query carries its sampler and coordinate in the IR, and a
// walker that returned only the image would undercount the query's uses: the
// dependency analysis and the Reassociate pass's use counting both read these
// functions, and a missed operand lets a fold remove an expression the query
// still needs.
func TestImageQueryLodOperands(t *testing.T) {
	const (
		image      = ir.ExpressionHandle(1)
		sampler    = ir.ExpressionHandle(2)
		coordinate = ir.ExpressionHandle(3)
	)
	kind := ir.ExprImageQuery{Image: image, Query: ir.ImageQueryLod{Sampler: sampler, Coordinate: coordinate}}
	want := []ir.ExpressionHandle{image, sampler, coordinate}

	assertOperandHandles(t, "expressionOperands", expressionOperands(kind), want)
	assertOperandHandles(t, "exprOperandHandles", exprOperandHandles(kind), want)

	if !imageQueryReferences(kind.Query, sampler) || !imageQueryReferences(kind.Query, coordinate) {
		t.Errorf("imageQueryReferences misses the query's own operands")
	}
	if imageQueryReferences(kind.Query, image) {
		t.Errorf("imageQueryReferences reports the image, which the query value does not carry")
	}
}

// TestImageQuerySizeLevelOperand pins the level operand a Size query has
// always carried: exprOperandHandles dropped it, which the Reassociate pass's
// use counting reads.
func TestImageQuerySizeLevelOperand(t *testing.T) {
	level := ir.ExpressionHandle(4)
	kind := ir.ExprImageQuery{Image: 1, Query: ir.ImageQuerySize{Level: &level}}
	assertOperandHandles(t, "exprOperandHandles", exprOperandHandles(kind), []ir.ExpressionHandle{1, level})
}

func assertOperandHandles(t *testing.T, name string, got, want []ir.ExpressionHandle) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
}
