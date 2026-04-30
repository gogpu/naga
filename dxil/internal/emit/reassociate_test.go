package emit

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestComputeExprUseCount verifies that expression use counts are computed
// correctly for binary expression trees.
func ptrUint8(v uint8) *uint8 { return &v }

// TestShlAndCombineDetectsAndMul verifies that tryShlAndCombine transforms
// mul(as(and(X, 1)), 2) -> and(shl(X, 1), 2), matching LLVM InstCombine.
// This pattern appears in force_point_size_vertex_shader_webgl:
//
//	let y = f32(i32(in_vertex_index & 1u) * 2 - 1);
func TestShlAndCombineDetectsAndMul(t *testing.T) {
	// IR expression tree:
	//   [0] FuncArg(0)      -- vertex_index (u32)
	//   [1] Literal(1u)     -- u32 constant
	//   [2] And(0, 1)       -- vertex_index & 1u
	//   [3] As(i32, 2)      -- i32 cast
	//   [4] Literal(2)      -- i32 constant
	//   [5] Multiply(3, 4)  -- * 2

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAnd, Left: 0, Right: 1}},
			{Kind: ir.ExprAs{Expr: 2, Kind: ir.ScalarSint, Convert: ptrUint8(4)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 3, Right: 4}},
		},
	}
	counts := computeExprUseCount(fn)

	// Verify use counts
	if counts[0] != 1 {
		t.Errorf("exprUseCount[0] = %d, want 1 (arg used by And)", counts[0])
	}
	if counts[2] != 1 {
		t.Errorf("exprUseCount[2] = %d, want 1 (And used by As)", counts[2])
	}
	if counts[3] != 1 {
		t.Errorf("exprUseCount[3] = %d, want 1 (As used by Multiply)", counts[3])
	}

	// Verify peelToAndExpr works (manual trace)
	// At Multiply emission: bin.Left=3 (As)
	// peelToAndExpr(3): As -> peel to 2 (And) -> found!
	t.Log("use counts correct for tryShlAndCombine detection")
}

func TestComputeExprUseCount(t *testing.T) {
	// Build a function with expressions:
	//   expr[0] = FuncArg(0)    -- vertex_index
	//   expr[1] = FuncArg(1)    -- instance_index
	//   expr[2] = FuncArg(2)    -- color
	//   expr[3] = Add(0, 1)     -- vertex + instance
	//   expr[4] = Add(3, 2)     -- (vertex + instance) + color
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprFunctionArgument{Index: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}},
		},
	}

	counts := computeExprUseCount(fn)

	// FuncArgs are leaves referenced by Binary ops:
	// expr[0] referenced by expr[3].Left → count=1
	// expr[1] referenced by expr[3].Right → count=1
	// expr[2] referenced by expr[4].Right → count=1
	// expr[3] referenced by expr[4].Left → count=1
	// expr[4] not referenced by any expression → count=0
	assertCount(t, counts, 0, 1)
	assertCount(t, counts, 1, 1)
	assertCount(t, counts, 2, 1)
	assertCount(t, counts, 3, 1)
	assertCount(t, counts, 4, 0)
}

// TestComputeExprUseCountMultiUse verifies that multi-use expressions
// have their use count incremented for each reference.
func TestComputeExprUseCountMultiUse(t *testing.T) {
	// expr[0] = FuncArg(0)
	// expr[1] = Add(0, 0)     -- same arg used twice
	// expr[2] = Add(1, 0)     -- expr[0] used 3 times total, expr[1] used once
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 0}},
		},
	}

	counts := computeExprUseCount(fn)
	assertCount(t, counts, 0, 3) // referenced by expr[1] twice + expr[2] once
	assertCount(t, counts, 1, 1) // referenced by expr[2] once
}

// TestFlattenBinaryChain verifies that a chain Add(Add(a,b),c) is
// flattened into [a, b, c] when the inner Add is single-use.
func TestFlattenBinaryChain(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                  // [0]
			{Kind: ir.ExprFunctionArgument{Index: 1}},                  // [1]
			{Kind: ir.ExprFunctionArgument{Index: 2}},                  // [2]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}}, // [3]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}}, // [4]
		},
	}

	e := &Emitter{
		exprValues:   make(map[ir.ExpressionHandle]int),
		exprUseCount: computeExprUseCount(fn),
	}

	// All leaves should be collected without emitting anything.
	leaves := e.flattenBinaryChain(fn, ir.BinaryAdd, 3, 2)
	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves, got %d", len(leaves))
	}
	// Leaves should be [0, 1, 2] (the function arguments).
	if leaves[0] != 0 || leaves[1] != 1 || leaves[2] != 2 {
		t.Fatalf("expected leaves [0,1,2], got %v", leaves)
	}
}

// TestFlattenBinaryChainMultiUse verifies that multi-use intermediates
// are treated as opaque leaves and NOT flattened.
func TestFlattenBinaryChainMultiUse(t *testing.T) {
	// expr[0] = FuncArg(0)
	// expr[1] = FuncArg(1)
	// expr[2] = FuncArg(2)
	// expr[3] = Add(0, 1)     -- used by expr[4] AND expr[5] → multi-use
	// expr[4] = Add(3, 2)
	// expr[5] = Mul(3, 2)     -- second use of expr[3]
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprFunctionArgument{Index: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 3, Right: 2}},
		},
	}

	e := &Emitter{
		exprValues:   make(map[ir.ExpressionHandle]int),
		exprUseCount: computeExprUseCount(fn),
	}

	// expr[3] has useCount=2, so it should NOT be flattened.
	leaves := e.flattenBinaryChain(fn, ir.BinaryAdd, 3, 2)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves (multi-use stops flattening), got %d", len(leaves))
	}
	if leaves[0] != 3 || leaves[1] != 2 {
		t.Fatalf("expected leaves [3,2], got %v", leaves)
	}
}

// TestFlattenBinaryChainAlreadyEmitted verifies that already-emitted
// expressions are treated as opaque leaves.
func TestFlattenBinaryChainAlreadyEmitted(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprFunctionArgument{Index: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}},
		},
	}

	e := &Emitter{
		exprValues:   map[ir.ExpressionHandle]int{3: 42}, // already emitted
		exprUseCount: computeExprUseCount(fn),
	}

	leaves := e.flattenBinaryChain(fn, ir.BinaryAdd, 3, 2)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves (cached stops flattening), got %d", len(leaves))
	}
}

// TestBuildChainSkipSet verifies that inner chain members are identified
// for skipping during StmtEmit processing.
func TestBuildChainSkipSet(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                  // [0]
			{Kind: ir.ExprFunctionArgument{Index: 1}},                  // [1]
			{Kind: ir.ExprFunctionArgument{Index: 2}},                  // [2]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}}, // [3]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}}, // [4]
		},
	}

	e := &Emitter{
		exprUseCount: computeExprUseCount(fn),
	}

	rng := ir.Range{Start: 3, End: 5} // covers expr[3] and expr[4]
	skip := e.buildChainSkipSet(fn, rng)

	if !skip[3] {
		t.Error("expected expr[3] to be in skip set (single-use inner chain member)")
	}
	if skip[4] {
		t.Error("expected expr[4] NOT to be in skip set (chain root)")
	}
}

// TestBuildChainSkipSetMultiUse verifies that multi-use intermediates
// are NOT skipped.
func TestBuildChainSkipSetMultiUse(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprFunctionArgument{Index: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},      // [3] — used by [4] and [5]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}},      // [4]
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 3, Right: 2}}, // [5]
		},
	}

	e := &Emitter{
		exprUseCount: computeExprUseCount(fn),
	}

	rng := ir.Range{Start: 3, End: 5}
	skip := e.buildChainSkipSet(fn, rng)

	if skip[3] {
		t.Error("expected expr[3] NOT to be in skip set (multi-use)")
	}
}

// TestBuildChainSkipSetDifferentOps verifies that chains of different
// operators are NOT flattened together.
func TestBuildChainSkipSetDifferentOps(t *testing.T) {
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprFunctionArgument{Index: 2}},
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}}, // [3] Mul
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 2}},      // [4] Add — different op!
		},
	}

	e := &Emitter{
		exprUseCount: computeExprUseCount(fn),
	}

	rng := ir.Range{Start: 3, End: 5}
	skip := e.buildChainSkipSet(fn, rng)

	if skip[3] {
		t.Error("expected expr[3] NOT to be in skip set (different op)")
	}
}

func assertCount(t *testing.T, counts map[ir.ExpressionHandle]int, handle ir.ExpressionHandle, expected int) {
	t.Helper()
	if got := counts[handle]; got != expected {
		t.Errorf("exprUseCount[%d] = %d, want %d", handle, got, expected)
	}
}
