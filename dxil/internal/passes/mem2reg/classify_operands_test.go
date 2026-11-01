package mem2reg

import (
	"testing"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// TestClassifyDisqualifiesEveryOtherUse checks the classifier's contract:
// a local is promotable only if its pointer is used as the pointer of a load
// or of a store, so any other use, in any operand of any expression or
// statement, disqualifies it. The classifier used to list the operands it
// looked at by hand and missed some (the query of a ray-query intersection,
// the level of a size query, a phi's incomings, and every statement operand
// but an atomic's pointer, an image store's image and a call's arguments),
// leaving a local promotable while a use of its pointer would dangle.
// Valid WGSL puts no pointer to a scalar local in those places, so this is
// the contract, not a known miscompile.
func TestClassifyDisqualifiesEveryOtherUse(t *testing.T) {
	type build func(fn *ir.Function, lv, val ir.ExpressionHandle)
	exprUse := func(kind func(lv, val ir.ExpressionHandle) ir.ExpressionKind) build {
		return func(fn *ir.Function, lv, val ir.ExpressionHandle) {
			h := appendExpr(fn, kind(lv, val))
			fn.Body = append(fn.Body, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}})
		}
	}
	stmtUse := func(kind func(lv, val ir.ExpressionHandle) ir.StatementKind) build {
		return func(fn *ir.Function, lv, val ir.ExpressionHandle) {
			fn.Body = append(fn.Body, ir.Statement{Kind: kind(lv, val)})
		}
	}
	tests := []struct {
		name string
		use  build
	}{
		{"ray-query intersection", exprUse(func(lv, _ ir.ExpressionHandle) ir.ExpressionKind {
			return ir.ExprRayQueryGetIntersection{Query: lv}
		})},
		{"size query level", exprUse(func(lv, val ir.ExpressionHandle) ir.ExpressionKind {
			return ir.ExprImageQuery{Image: val, Query: ir.ImageQuerySize{Level: &lv}}
		})},
		{"phi incoming", exprUse(func(lv, val ir.ExpressionHandle) ir.ExpressionKind {
			return ssa.Phi{Incoming: []ssa.Incoming{{Value: val}, {PredKey: ssa.PredIfReject, Value: lv}}}
		})},
		{"return value", stmtUse(func(lv, _ ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtReturn{Value: &lv}
		})},
		{"stored value", stmtUse(func(lv, val ir.ExpressionHandle) ir.StatementKind {
			// The pointer of the local stored somewhere else.
			return ir.StmtStore{Pointer: val, Value: lv}
		})},
		{"workgroup uniform load", stmtUse(func(lv, val ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtWorkGroupUniformLoad{Pointer: lv, Result: val}
		})},
		{"image atomic", stmtUse(func(lv, val ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtImageAtomic{Image: lv, Coordinate: val, Fun: ir.AtomicAdd{}, Value: val}
		})},
		{"subgroup operation", stmtUse(func(lv, val ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtSubgroupCollectiveOperation{Argument: lv, Result: val}
		})},
		{"if condition", stmtUse(func(lv, _ ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtIf{Condition: lv}
		})},
		{"nested in a loop", stmtUse(func(lv, _ ir.ExpressionHandle) ir.StatementKind {
			return ir.StmtLoop{Body: ir.Block{{Kind: ir.StmtReturn{Value: &lv}}}}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := &ir.Module{}
			f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
			fn := &ir.Function{LocalVars: []ir.LocalVariable{{Name: "x", Type: f32}}}
			lv := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
			val := appendExpr(fn, ir.Literal{Value: ir.LiteralF32(1)})
			fn.Body = ir.Block{{Kind: ir.StmtStore{Pointer: lv, Value: val}}}
			tt.use(fn, lv, val)

			if uses := classifyLocals(mod, fn); uses[0].promotable {
				t.Errorf("a local whose pointer is an operand (%s) stays promotable", tt.name)
			}
		})
	}
}

// TestClassifyKeepsLoadsAndStores checks the other half of the contract: a
// local used only through loads and stores stays promotable, with every
// emitted load and every store counted.
func TestClassifyKeepsLoadsAndStores(t *testing.T) {
	mod := &ir.Module{}
	f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
	fn := &ir.Function{LocalVars: []ir.LocalVariable{{Name: "x", Type: f32}}}
	lv := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralF32(1)})
	load := appendExpr(fn, ir.ExprLoad{Pointer: lv})
	fn.Body = ir.Block{
		{Kind: ir.StmtStore{Pointer: lv, Value: val}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: load, End: load + 1}}},
		{Kind: ir.StmtIf{Condition: val, Accept: ir.Block{{Kind: ir.StmtStore{Pointer: lv, Value: load}}}}},
	}

	uses := classifyLocals(mod, fn)
	if !uses[0].promotable {
		t.Fatal("a local used only by loads and stores is not promotable")
	}
	if uses[0].storeCount != 2 {
		t.Errorf("storeCount = %d, want 2", uses[0].storeCount)
	}
	if len(uses[0].loadHandles) != 1 || uses[0].loadHandles[0] != load {
		t.Errorf("loadHandles = %v, want [%d]", uses[0].loadHandles, load)
	}
}
