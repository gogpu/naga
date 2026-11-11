package codegen

import (
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// TestPackedVec3ScalarGuards covers the defensive branches of the packed-vec3
// predicate: a bare writer, a handle outside the expression arena, an
// expression that is not an access, an access whose base has no resolved type,
// a base that is neither a pointer nor a struct, a pointer whose type handle
// is past the writer's arena, and a pointer to a non-struct type. None of
// these packs a member, and every one must report nil rather than panic or
// guess a scalar.
func TestPackedVec3ScalarGuards(t *testing.T) {
	if got := (&Writer{}).packedVec3Scalar(0); got != nil {
		t.Errorf("a writer without a current function returned %v", got)
	}

	outOfRange := &Writer{currentFunction: &ir.Function{}}
	if got := outOfRange.packedVec3Scalar(0); got != nil {
		t.Errorf("an out-of-range handle returned %v", got)
	}

	notAccess := &Writer{currentFunction: &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprGlobalVariable{Variable: 0}},
	}}}
	if got := notAccess.packedVec3Scalar(0); got != nil {
		t.Errorf("a global variable returned %v", got)
	}

	unresolved := &Writer{currentFunction: &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprGlobalVariable{Variable: 0}},
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
	}}}
	if got := unresolved.packedVec3Scalar(1); got != nil {
		t.Errorf("an access with no resolved base type returned %v", got)
	}

	// A vector-typed base is indexable but is not a struct, so no member of
	// it packs.
	vector := &ir.Module{Types: []ir.Type{{Inner: ir.VectorType{
		Size:   ir.Vec4,
		Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
	}}}}
	vectorFn := &ir.Function{
		Arguments: []ir.FunctionArgument{{Type: 0}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
	}
	vectorTypes, err := typifier.Function(vector, vectorFn)
	if err != nil {
		t.Fatalf("typifier.Function (vector): %v", err)
	}
	vectorWriter := &Writer{module: vector, currentFunction: vectorFn, types: vectorTypes}
	if got := vectorWriter.packedVec3Scalar(1); got != nil {
		t.Errorf("an access into a vector returned %v", got)
	}

	// The typifier resolves a global to ptr<space, type>; the writer's module
	// holds no types at all, so the pointer's base handle is past its arena.
	typed := &ir.Module{
		Types:           []ir.Type{{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}},
		GlobalVariables: []ir.GlobalVariable{{Space: ir.SpaceWorkGroup, Type: 0}},
	}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprGlobalVariable{Variable: 0}},
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},
	}}
	types, err := typifier.Function(typed, fn)
	if err != nil {
		t.Fatalf("typifier.Function: %v", err)
	}
	pastArena := &Writer{module: &ir.Module{}, currentFunction: fn, types: types}
	if got := pastArena.packedVec3Scalar(1); got != nil {
		t.Errorf("a pointer past the type arena returned %v", got)
	}

	// A private vector's pointer resolves in a consistent module, but its
	// base is not a struct, so there is no member to pack.
	consistent := &Writer{module: typed, currentFunction: fn, types: types}
	if got := consistent.packedVec3Scalar(1); got != nil {
		t.Errorf("a pointer to a vector returned %v", got)
	}
}
