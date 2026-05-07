package sroa

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Tests targeting uncovered SROA code paths with real shader patterns.
// ---------------------------------------------------------------------------

// TestNestedStoreInIfDisqualifies verifies that a struct local with
// field stores inside an if-branch is disqualified from SROA. This is
// necessary because SROA-decomposed per-field loads might create
// phi-insertion problems for mem2reg Phase B.
//
// Real pattern: vertex shader conditionally writing to struct output.
//
// Covers: disqualifyNestedStores, markNestedUses for StmtIf,
// resolveStructLocal.
func TestNestedStoreInIfDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
					{Name: "y", Type: makeTypeHandle(0)},
				},
				Span: 8,
			}},
		},
	}

	fn := &ir.Function{
		Name: "vs",
		LocalVars: []ir.LocalVariable{
			{Name: "out", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},       // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},   // [1] &out.x
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // [2]
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [3] condition
			{Kind: ir.ExprLoad{Pointer: 0}},                 // [4] full load
		},
		ExpressionTypes: make([]ir.TypeResolution, 5),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			// Store to field inside if → disqualifies
			{Kind: ir.StmtIf{
				Condition: 3,
				Accept: ir.Block{
					{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
				},
				Reject: ir.Block{},
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (nested store in if), got %d", count)
	}
}

// TestNestedStoreInLoopDisqualifies verifies that a struct local with
// stores inside a loop body is disqualified from SROA.
//
// Real pattern: compute shader writing struct fields in a loop.
//
// Covers: disqualifyNestedStores StmtLoop path, markNestedUses StmtLoop.
func TestNestedStoreInLoopDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "sum", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "compute",
		LocalVars: []ir.LocalVariable{
			{Name: "accum", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] &accum.sum
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [2] value
			{Kind: ir.ExprLoad{Pointer: 0}},               // [3] full load
		},
		ExpressionTypes: make([]ir.TypeResolution, 4),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
					{Kind: ir.StmtBreak{}},
				},
				Continuing: ir.Block{},
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (store in loop), got %d", count)
	}
}

// TestNestedStoreInSwitchDisqualifies verifies that struct locals stored
// inside switch cases are disqualified from SROA.
//
// Covers: disqualifyNestedStores StmtSwitch path,
// markNestedUses StmtSwitch recursion.
func TestNestedStoreInSwitchDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [2]
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},   // [3] selector
			{Kind: ir.ExprLoad{Pointer: 0}},               // [4]
		},
		ExpressionTypes: make([]ir.TypeResolution, 5),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtSwitch{
				Selector: 3,
				Cases: []ir.SwitchCase{
					{Value: ir.SwitchValueI32(0), Body: ir.Block{
						{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
					}},
					{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
				},
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (store in switch), got %d", count)
	}
}

// TestNestedStoreInBlockDisqualifies verifies that struct locals with
// stores inside a nested StmtBlock are disqualified.
//
// Covers: disqualifyNestedStores StmtBlock path,
// markNestedUses StmtBlock recursion.
func TestNestedStoreInBlockDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [2]
			{Kind: ir.ExprLoad{Pointer: 0}},               // [3]
		},
		ExpressionTypes: make([]ir.TypeResolution, 4),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtBlock{Block: ir.Block{
				{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
			}}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (store in nested block), got %d", count)
	}
}

// TestResolveStructLocalAccessChain verifies that resolveStructLocal
// follows AccessIndex chains to the root local variable.
//
// Covers: resolveStructLocal with depth > 1.
func TestResolveStructLocalAccessChain(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}}, // [2] nested field
		},
	}

	lvHandleMap := map[ir.ExpressionHandle]uint32{0: 0}

	varIdx, ok := resolveStructLocal(fn, 2, lvHandleMap)
	if !ok {
		t.Fatal("expected resolveStructLocal to find the root local")
	}
	if varIdx != 0 {
		t.Errorf("expected variable index 0, got %d", varIdx)
	}
}

// TestResolveStructLocalNonLocal verifies that resolveStructLocal returns
// false for non-local pointer chains.
//
// Covers: resolveStructLocal default return path.
func TestResolveStructLocalNonLocal(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [0] global, not local
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]
		},
	}

	lvHandleMap := map[ir.ExpressionHandle]uint32{}

	_, ok := resolveStructLocal(fn, 1, lvHandleMap)
	if ok {
		t.Fatal("expected resolveStructLocal to return false for global-based chain")
	}
}

// TestResolveStructLocalOutOfBounds verifies that out-of-bounds handles
// return false safely.
//
// Covers: resolveStructLocal out-of-bounds guard.
func TestResolveStructLocalOutOfBounds(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
	}
	lvHandleMap := map[ir.ExpressionHandle]uint32{0: 0}

	_, ok := resolveStructLocal(fn, 999, lvHandleMap)
	if ok {
		t.Fatal("expected false for out-of-bounds handle")
	}
}

// TestClassifyStmtsAtomicDisqualifies verifies that StmtAtomic using a
// struct local pointer disqualifies it.
//
// Covers: classifyStmts StmtAtomic branch.
func TestClassifyStmtsAtomicDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "counter", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 2),
		Body: ir.Block{
			{Kind: ir.StmtAtomic{Pointer: 0, Fun: ir.AtomicAdd{}, Value: 1}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (atomic use), got %d", count)
	}
}

// TestClassifyStmtsRecursesIntoControlFlow verifies that classifyStmts
// recurses into if/loop/switch/block.
//
// Covers: classifyStmts StmtIf, StmtLoop, StmtSwitch, StmtBlock paths.
func TestClassifyStmtsRecursesIntoControlFlow(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},       // [0]
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [1]
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},     // [2]
		},
		ExpressionTypes: make([]ir.TypeResolution, 3),
		Body: ir.Block{
			// Call with local handle inside nested control flow.
			{Kind: ir.StmtIf{
				Condition: 1,
				Accept: ir.Block{
					{Kind: ir.StmtLoop{
						Body: ir.Block{
							{Kind: ir.StmtSwitch{
								Selector: 2,
								Cases: []ir.SwitchCase{
									{Value: ir.SwitchValueDefault{}, Body: ir.Block{
										{Kind: ir.StmtBlock{Block: ir.Block{
											{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0}}},
										}}},
									}},
								},
							}},
							{Kind: ir.StmtBreak{}},
						},
						Continuing: ir.Block{},
					}},
				},
				Reject: ir.Block{},
			}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (call argument deep in control flow), got %d", count)
	}
}

// TestFullStructStoreNonComposeDisqualifies verifies that storing a
// non-Compose value to the full struct local disqualifies it.
//
// Covers: classifyStmts StmtStore full struct non-compose path.
func TestFullStructStoreNonComposeDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}}, // [0]
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [1] non-compose value
		},
		ExpressionTypes: make([]ir.TypeResolution, 2),
		Body: ir.Block{
			// Full struct store with non-compose value.
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (non-compose full struct store), got %d", count)
	}
}

// TestFullStructStoreWithComposeIsAllowed verifies that storing a Compose
// value to the full struct local is acceptable for SROA.
//
// Covers: classifyStmts StmtStore compose-check path (positive).
func TestFullStructStoreWithComposeIsAllowed(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
					{Name: "y", Type: makeTypeHandle(0)},
				},
				Span: 8,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},                       // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                   // [1] &s.x
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},                   // [2] &s.y
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                   // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                   // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4}}}, // [5]
			{Kind: ir.ExprLoad{Pointer: 0}},                                 // [6] full load
		},
		ExpressionTypes: make([]ir.TypeResolution, 7),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			// Full struct store with Compose — allowed.
			{Kind: ir.StmtStore{Pointer: 0, Value: 5}},
			{Kind: ir.StmtStore{Pointer: 1, Value: 3}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 6, End: 7}}},
		},
	}

	count := Run(mod, fn)
	if count != 1 {
		t.Errorf("expected 1 decomposition (compose store is allowed), got %d", count)
	}
}

// TestClassifyFieldIndexOutOfBounds verifies that AccessIndex with an
// out-of-bounds field index disqualifies the candidate.
//
// Covers: classifyExprs out-of-bounds field index path.
func TestClassifyFieldIndexOutOfBounds(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},      // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 99}}, // [1] out of bounds!
		},
		ExpressionTypes: make([]ir.TypeResolution, 2),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (out-of-bounds field index), got %d", count)
	}
}

// TestF16MemberDisqualifies verifies that struct members with f16 type
// prevent SROA decomposition.
//
// Covers: allMembersDecomposable f16 rejection (Width != 4).
func TestF16MemberDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, // f16
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 2,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (f16 member), got %d", count)
	}
}

// TestF16VectorMemberDisqualifies verifies that vector members with f16
// scalar width prevent decomposition.
//
// Covers: allMembersDecomposable vector width != 4.
func TestF16VectorMemberDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}}, // vec3<f16>
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "v", Type: makeTypeHandle(0)},
				},
				Span: 6,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (f16 vector member), got %d", count)
	}
}

// TestNestedStructMemberDisqualifies verifies that struct members which
// are themselves structs prevent SROA.
//
// Covers: allMembersDecomposable default case (matrix/struct/array).
func TestNestedStructMemberDisqualifies(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "inner_x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "nested", Type: makeTypeHandle(1)}, // struct member
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(2)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (nested struct member), got %d", count)
	}
}

// TestMemberTypeOutOfBounds verifies that a struct member with an
// out-of-bounds type handle prevents decomposition.
//
// Covers: allMembersDecomposable type-bounds check.
func TestMemberTypeOutOfBounds(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(99)}, // out of bounds
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (member type out of bounds), got %d", count)
	}
}

// TestAllUsesInTopLevelRejectsNestedLoad verifies that a full-struct load
// inside a nested if-branch causes allUsesInTopLevel to return false.
//
// Covers: allUsesInTopLevel rejection, emitRangeInTopLevel false path.
func TestAllUsesInTopLevelRejectsNestedLoad(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},       // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},   // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // [2]
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [3]
			{Kind: ir.ExprLoad{Pointer: 0}},                 // [4] full load
		},
		ExpressionTypes: make([]ir.TypeResolution, 5),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
			// Full load inside if → not top-level.
			{Kind: ir.StmtIf{
				Condition: 3,
				Accept: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
				},
				Reject: ir.Block{},
			}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (full load not in top-level), got %d", count)
	}
}

// TestNoUsesAtAllSkipped verifies that a struct local with no accesses
// and no loads is skipped (would be dead anyway).
//
// Covers: classify step 4 no-uses path.
func TestNoUsesAtAllSkipped(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}}, // declared but never used
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (no uses), got %d", count)
	}
}

// TestLocalVarIndexOutOfBounds verifies that an ExprLocalVariable with
// a variable index beyond LocalVars is handled gracefully.
//
// Covers: classify step 1 bounds check on lv.Variable.
func TestLocalVarIndexOutOfBounds(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	fn := &ir.Function{
		Name:      "test",
		LocalVars: []ir.LocalVariable{}, // empty
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 99}}, // out of bounds
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	// Should not panic.
	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions, got %d", count)
	}
}

// TestLocalTypeOutOfBounds verifies that a local with an out-of-bounds
// type handle is skipped.
//
// Covers: classify step 1 type-bounds check.
func TestLocalTypeOutOfBounds(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(99)}, // out of bounds
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions, got %d", count)
	}
}

// TestMarkNestedUsesRecursesDeep verifies that markNestedUses recurses
// through nested if-inside-loop-inside-switch patterns.
//
// Covers: markNestedUses StmtIf→StmtLoop→StmtSwitch recursion.
func TestMarkNestedUsesRecursesDeep(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},       // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},   // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // [2]
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [3]
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},     // [4]
			{Kind: ir.ExprLoad{Pointer: 0}},                 // [5]
		},
		ExpressionTypes: make([]ir.TypeResolution, 6),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			// Store deeply nested.
			{Kind: ir.StmtIf{
				Condition: 3,
				Accept: ir.Block{
					{Kind: ir.StmtLoop{
						Body: ir.Block{
							{Kind: ir.StmtSwitch{
								Selector: 4,
								Cases: []ir.SwitchCase{
									{Value: ir.SwitchValueDefault{}, Body: ir.Block{
										{Kind: ir.StmtStore{Pointer: 1, Value: 2}},
									}},
								},
							}},
							{Kind: ir.StmtBreak{}},
						},
						Continuing: ir.Block{},
					}},
				},
				Reject: ir.Block{},
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 6}}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions (deeply nested store), got %d", count)
	}
}

// TestResolveStructLocalExprAccess verifies that resolveStructLocal
// follows ExprAccess (dynamic index) chains.
//
// Covers: resolveStructLocal ExprAccess case.
func TestResolveStructLocalExprAccess(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},   // [0]
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // [1] index
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // [2] dynamic access
		},
	}

	lvHandleMap := map[ir.ExpressionHandle]uint32{0: 0}

	varIdx, ok := resolveStructLocal(fn, 2, lvHandleMap)
	if !ok {
		t.Fatal("expected resolveStructLocal to follow ExprAccess chain")
	}
	if varIdx != 0 {
		t.Errorf("expected variable 0, got %d", varIdx)
	}
}
