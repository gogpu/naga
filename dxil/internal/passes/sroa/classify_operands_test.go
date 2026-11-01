package sroa

import (
	"testing"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// sroaFunction returns a module and a function whose struct local (two i32
// members) is decomposable: one field is stored and loaded through an
// AccessIndex. Its handles: 0 the local, 1 a literal, 2 the field pointer,
// 3 the field load.
func sroaFunction() (*ir.Module, *ir.Function) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: makeTypeHandle(0)},
					{Name: "b", Type: makeTypeHandle(0), Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	fn := &ir.Function{
		Name:      "test",
		LocalVars: []ir.LocalVariable{{Name: "s", Type: makeTypeHandle(1)}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			{Kind: ir.ExprLoad{Pointer: 2}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
			{Kind: ir.StmtStore{Pointer: 2, Value: 1}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
		},
	}
	return mod, fn
}

// TestClassifyDisqualifiesEveryOtherStructUse checks the classifier's
// contract: a struct local is decomposed only if its pointer is used as the
// base of a field access, the pointer of a full load, or the pointer of a
// store of a Compose; any other use of the pointer, in any operand of any
// expression or statement, keeps the local whole. The classifier used to
// list the operands it looked at by hand and missed most of them, so the
// local was decomposed while a use of the whole struct's pointer was left
// pointing at nothing. Valid WGSL puts no pointer to a struct local in those
// places, so this is the contract, not a known miscompile.
func TestClassifyDisqualifiesEveryOtherStructUse(t *testing.T) {
	const lv, lit = ir.ExpressionHandle(0), ir.ExpressionHandle(1)
	tests := []struct {
		name string
		expr ir.ExpressionKind // added and emitted when set
		stmt ir.StatementKind  // appended to the body when set
	}{
		{name: "alias", expr: ssa.Alias{Source: lv}},
		{name: "ray-query intersection", expr: ir.ExprRayQueryGetIntersection{Query: lv}},
		{name: "array length", expr: ir.ExprArrayLength{Array: lv}},
		{name: "return value", stmt: ir.StmtReturn{Value: func() *ir.ExpressionHandle { h := lv; return &h }()}},
		{name: "stored value", stmt: ir.StmtStore{Pointer: lit, Value: lv}},
		{name: "workgroup uniform load", stmt: ir.StmtWorkGroupUniformLoad{Pointer: lv, Result: lit}},
		{name: "image store", stmt: ir.StmtImageStore{Image: lv, Coordinate: lit, Value: lit}},
		{name: "ray query", stmt: ir.StmtRayQuery{Query: lv, Fun: ir.RayQueryTerminate{}}},
	}

	mod, fn := sroaFunction()
	if n := Run(mod, fn); n != 1 {
		t.Fatalf("the base function decomposes %d locals, want 1", n)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, fn := sroaFunction()
			if tt.expr != nil {
				h := ir.ExpressionHandle(len(fn.Expressions))
				fn.Expressions = append(fn.Expressions, ir.Expression{Kind: tt.expr})
				fn.Body = append(fn.Body, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}})
			}
			if tt.stmt != nil {
				fn.Body = append(fn.Body, ir.Statement{Kind: tt.stmt})
			}
			if n := Run(mod, fn); n != 0 {
				t.Errorf("a struct local whose pointer is an operand (%s) was decomposed", tt.name)
			}
		})
	}
}
