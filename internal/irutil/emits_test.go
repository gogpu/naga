package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestDeduplicateEmits_RemovesEmptyEmits(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "f",
				Expressions: []ir.Expression{
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 0}}}, // empty
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}}, // non-empty with non-pre-emit expr
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements after dedup (empty emit removed), got %d", len(body))
	}
}

func TestDeduplicateEmits_RemovesCoveredEmits(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "f",
				Expressions: []ir.Expression{
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}}, // first emit
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}}, // duplicate — covered
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements (duplicate removed), got %d", len(body))
	}
}

func TestDeduplicateEmits_SkipsPreEmitOnlyRanges(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "f",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // pre-emit
					{Kind: ir.ExprConstant{Constant: 0}},                       // pre-emit
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}}, // not pre-emit
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}}, // all pre-emit -> removed
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}}, // has non-pre-emit -> kept
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements (pre-emit only removed), got %d", len(body))
	}
}
