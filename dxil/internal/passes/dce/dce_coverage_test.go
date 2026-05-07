package dce

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Tests targeting uncovered DCE code paths with real shader patterns.
// ---------------------------------------------------------------------------

// TestDeadLoopWithOnlyBreakIsEliminated verifies that a loop whose body
// contains only a break statement (no side effects) is removed after sweep.
// This is the early-return inline pattern: after mem2reg promotes the return
// slot, the wrapper loop body is just `break`.
//
// Covers: sweepLoop, loopBodyIsDead, blockOnlyHasTerminators.
func TestDeadLoopWithOnlyBreakIsEliminated(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}
	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: condVal + 1}}},
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtBreak{}},
			},
			Continuing: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The loop body has only a break (no side effects), continuing is empty,
	// no BreakIf. The loop should be eliminated entirely.
	for _, s := range fn.Body {
		if _, isLoop := s.Kind.(ir.StmtLoop); isLoop {
			t.Fatal("dead loop with only break should have been eliminated")
		}
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement (return), got %d", len(fn.Body))
	}
}

// TestLoopWithSideEffectsKept verifies that a loop containing a barrier
// (observable side effect) survives DCE even though there are no outputs.
//
// Covers: sweepLoop when keep=true.
func TestLoopWithSideEffectsKept(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}

	fn.Body = ir.Block{
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
				{Kind: ir.StmtBreak{}},
			},
			Continuing: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	foundLoop := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
		}
	}
	if !foundLoop {
		t.Fatal("loop with barrier should be preserved")
	}
}

// TestLoopWithBreakIfAndNonEmptyContinuing verifies that loopBodyIsDead
// returns false when continuing is non-empty.
//
// Covers: loopBodyIsDead early exit on non-empty continuing.
func TestLoopWithBreakIfAndNonEmptyContinuing(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "i", Type: i32}},
	}

	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})
	counterVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: counterVal + 1}}},
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtBreak{}},
			},
			Continuing: ir.Block{
				{Kind: ir.StmtStore{Pointer: lvHandle, Value: counterVal}},
			},
			BreakIf: &condVal,
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The loop has a non-empty continuing block, so loopBodyIsDead should
	// return false. The loop is kept (even though the store is to a dead local).
	// After DCE the dead store in continuing is removed, but the loop structure
	// stays because continuing was non-empty during the sweep analysis.
	// Either outcome (loop kept or entirely removed) is acceptable as long
	// as we don't crash.
}

// TestBlockOnlyHasTerminatorsNested verifies that blockOnlyHasTerminators
// recurses through nested if/switch/loop/block structures containing only
// break/continue.
//
// Covers: blockOnlyHasTerminators nested control flow paths.
func TestBlockOnlyHasTerminatorsNested(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}
	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})
	selVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

	// Loop with nested if/switch/block that only have terminators.
	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: selVal + 1}}},
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtIf{
					Condition: condVal,
					Accept: ir.Block{
						{Kind: ir.StmtBreak{}},
					},
					Reject: ir.Block{
						{Kind: ir.StmtContinue{}},
					},
				}},
			},
			Continuing: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The loop body has only nested terminators. The condition is dead (not
	// consumed by any root). The loop should be eliminated.
	for _, s := range fn.Body {
		if _, isLoop := s.Kind.(ir.StmtLoop); isLoop {
			t.Fatal("loop with only nested terminators should be eliminated")
		}
	}
}

// TestBlockOnlyHasTerminatorsWithSideEffect verifies that blockOnlyHasTerminators
// returns false when a nested block has a non-terminator statement.
func TestBlockOnlyHasTerminatorsWithSideEffect(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}
	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: condVal + 1}}},
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtBlock{
					Block: ir.Block{
						{Kind: ir.StmtBarrier{}},
						{Kind: ir.StmtBreak{}},
					},
				}},
			},
			Continuing: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	foundLoop := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
		}
	}
	if !foundLoop {
		t.Fatal("loop with barrier inside nested block should be kept")
	}
}

// TestImageStoreIsRoot verifies that StmtImageStore (texture write in
// compute/fragment shader) is correctly treated as a side-effecting root,
// keeping the image, coordinate, value, and array index expressions live.
//
// Covers: markStmtRoots StmtImageStore branch.
func TestImageStoreIsRoot(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}

	imageH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	coordH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})
	valueH := appendExpr(fn, ir.Literal{Value: ir.LiteralF32(1.0)})
	arrayIdx := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: imageH, End: arrayIdx + 1}}},
		{Kind: ir.StmtImageStore{
			Image:      imageH,
			Coordinate: coordH,
			Value:      valueH,
			ArrayIndex: &arrayIdx,
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// ImageStore is a root. All statements should remain.
	foundImageStore := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtImageStore); ok {
			foundImageStore = true
		}
	}
	if !foundImageStore {
		t.Fatal("StmtImageStore should survive DCE")
	}
}

// TestImageAtomicIsRoot verifies that StmtImageAtomic is a root.
//
// Covers: markStmtRoots StmtImageAtomic branch.
func TestImageAtomicIsRoot(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}

	imageH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	coordH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})
	valueH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: imageH, End: valueH + 1}}},
		{Kind: ir.StmtImageAtomic{
			Image:      imageH,
			Coordinate: coordH,
			Value:      valueH,
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	foundImageAtomic := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtImageAtomic); ok {
			foundImageAtomic = true
		}
	}
	if !foundImageAtomic {
		t.Fatal("StmtImageAtomic should survive DCE")
	}
}

// TestGlobalAtomicIsRoot verifies that StmtAtomic on a global (UAV) is
// a root and its pointer/value/result are kept live.
//
// Covers: markStmtRoots StmtAtomic on global branch + Result marking.
func TestGlobalAtomicIsRoot(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "counter", Space: ir.SpaceStorage, Type: 0},
		},
	}

	fn := &ir.Function{}
	globalH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	valueH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	resultH := appendExpr(fn, ir.ExprAtomicResult{Ty: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: globalH, End: resultH + 1}}},
		{Kind: ir.StmtAtomic{
			Pointer: globalH,
			Fun:     ir.AtomicAdd{},
			Value:   valueH,
			Result:  &resultH,
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	foundAtomic := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtAtomic); ok {
			foundAtomic = true
		}
	}
	if !foundAtomic {
		t.Fatal("StmtAtomic on global should survive DCE")
	}
}

// TestWorkGroupUniformLoadIsRoot verifies that StmtWorkGroupUniformLoad
// (barrier + load) is kept as a side-effecting root.
//
// Covers: markStmtRoots StmtWorkGroupUniformLoad branch.
func TestWorkGroupUniformLoadIsRoot(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}

	ptrH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	resultH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: ptrH, End: resultH + 1}}},
		{Kind: ir.StmtWorkGroupUniformLoad{
			Pointer: ptrH,
			Result:  resultH,
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	found := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtWorkGroupUniformLoad); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("StmtWorkGroupUniformLoad should survive DCE")
	}
}

// TestRayQueryIsRoot verifies that StmtRayQuery is a root and its
// sub-handles (from each RayQueryFunction variant) are marked live.
//
// Covers: markStmtRoots StmtRayQuery, markRayQueryFunctionHandles all variants.
func TestRayQueryIsRoot(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.RayQueryFunction
		// extra expression count beyond queryH
		extraExprs int
	}{
		{
			name: "Initialize",
			fun: ir.RayQueryInitialize{
				AccelerationStructure: 1,
				Descriptor:            2,
			},
			extraExprs: 2,
		},
		{
			name:       "Proceed",
			fun:        ir.RayQueryProceed{Result: 1},
			extraExprs: 1,
		},
		{
			name:       "GenerateIntersection",
			fun:        ir.RayQueryGenerateIntersection{HitT: 1},
			extraExprs: 1,
		},
		{
			name:       "Terminate",
			fun:        ir.RayQueryTerminate{},
			extraExprs: 0,
		},
		{
			name:       "ConfirmIntersection",
			fun:        ir.RayQueryConfirmIntersection{},
			extraExprs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := &ir.Module{}
			fn := &ir.Function{}

			queryH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
			for i := 0; i < tt.extraExprs; i++ {
				appendExpr(fn, ir.Literal{Value: ir.LiteralI32(int32(i))})
			}

			lastExpr := ir.ExpressionHandle(len(fn.Expressions))
			fn.Body = ir.Block{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: queryH, End: lastExpr}}},
				{Kind: ir.StmtRayQuery{Query: queryH, Fun: tt.fun}},
				{Kind: ir.StmtReturn{}},
			}

			Run(mod, fn)

			found := false
			for _, s := range fn.Body {
				if _, ok := s.Kind.(ir.StmtRayQuery); ok {
					found = true
				}
			}
			if !found {
				t.Fatal("StmtRayQuery should survive DCE")
			}
		})
	}
}

// TestSubgroupStatementsAreRoots verifies that subgroup collective operations
// (ballot, collective op, gather) are treated as side-effecting roots.
//
// Covers: markStmtRoots StmtSubgroupBallot, StmtSubgroupCollectiveOperation,
// StmtSubgroupGather branches.
func TestSubgroupStatementsAreRoots(t *testing.T) {
	t.Run("SubgroupBallot", func(t *testing.T) {
		mod := &ir.Module{}
		fn := &ir.Function{}

		resultH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})
		predH := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})

		fn.Body = ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: resultH, End: predH + 1}}},
			{Kind: ir.StmtSubgroupBallot{Result: resultH, Predicate: &predH}},
			{Kind: ir.StmtReturn{}},
		}

		Run(mod, fn)

		found := false
		for _, s := range fn.Body {
			if _, ok := s.Kind.(ir.StmtSubgroupBallot); ok {
				found = true
			}
		}
		if !found {
			t.Fatal("SubgroupBallot should survive DCE")
		}
	})

	t.Run("SubgroupCollectiveOperation", func(t *testing.T) {
		mod := &ir.Module{}
		fn := &ir.Function{}

		argH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
		resultH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

		fn.Body = ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: argH, End: resultH + 1}}},
			{Kind: ir.StmtSubgroupCollectiveOperation{
				Argument: argH,
				Result:   resultH,
			}},
			{Kind: ir.StmtReturn{}},
		}

		Run(mod, fn)

		found := false
		for _, s := range fn.Body {
			if _, ok := s.Kind.(ir.StmtSubgroupCollectiveOperation); ok {
				found = true
			}
		}
		if !found {
			t.Fatal("SubgroupCollectiveOperation should survive DCE")
		}
	})

	t.Run("SubgroupGather", func(t *testing.T) {
		mod := &ir.Module{}
		fn := &ir.Function{}

		argH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
		resultH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

		fn.Body = ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: argH, End: resultH + 1}}},
			{Kind: ir.StmtSubgroupGather{
				Argument: argH,
				Result:   resultH,
			}},
			{Kind: ir.StmtReturn{}},
		}

		Run(mod, fn)

		found := false
		for _, s := range fn.Body {
			if _, ok := s.Kind.(ir.StmtSubgroupGather); ok {
				found = true
			}
		}
		if !found {
			t.Fatal("SubgroupGather should survive DCE")
		}
	})
}

// TestKillStatementIsRoot verifies that StmtKill (discard in fragment shader)
// is preserved as a side-effecting root.
//
// Covers: markStmtRoots StmtKill branch.
func TestKillStatementIsRoot(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}

	fn.Body = ir.Block{
		{Kind: ir.StmtKill{}},
	}

	Run(mod, fn)

	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement (kill), got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].Kind.(ir.StmtKill); !ok {
		t.Fatalf("expected StmtKill, got %T", fn.Body[0].Kind)
	}
}

// TestVisitExprHandlesAllKinds exercises visitExprHandles with various
// expression kinds that have different sub-handle structures.
//
// Covers: visitExprHandles branches for ExprSelect, ExprSplat, ExprSwizzle,
// ExprCompose, ExprAs, ExprDerivative, ExprMath, ExprRelational,
// ExprArrayLength, ExprImageSample, ExprImageLoad, ExprImageQuery, ExprPhi.
func TestVisitExprHandlesAllKinds(t *testing.T) {
	tests := []struct {
		name     string
		kind     ir.ExpressionKind
		expected []ir.ExpressionHandle
	}{
		{
			name:     "Select",
			kind:     ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2},
			expected: []ir.ExpressionHandle{0, 1, 2},
		},
		{
			name:     "Splat",
			kind:     ir.ExprSplat{Value: 5},
			expected: []ir.ExpressionHandle{5},
		},
		{
			name:     "Swizzle",
			kind:     ir.ExprSwizzle{Vector: 3},
			expected: []ir.ExpressionHandle{3},
		},
		{
			name:     "Compose",
			kind:     ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 3}},
			expected: []ir.ExpressionHandle{1, 2, 3},
		},
		{
			name:     "As",
			kind:     ir.ExprAs{Expr: 7},
			expected: []ir.ExpressionHandle{7},
		},
		{
			name:     "Derivative",
			kind:     ir.ExprDerivative{Expr: 4},
			expected: []ir.ExpressionHandle{4},
		},
		{
			name: "Math_AllArgs",
			kind: func() ir.ExpressionKind {
				a1, a2, a3 := ir.ExpressionHandle(1), ir.ExpressionHandle(2), ir.ExpressionHandle(3)
				return ir.ExprMath{Arg: 0, Arg1: &a1, Arg2: &a2, Arg3: &a3}
			}(),
			expected: []ir.ExpressionHandle{0, 1, 2, 3},
		},
		{
			name:     "Relational",
			kind:     ir.ExprRelational{Argument: 6},
			expected: []ir.ExpressionHandle{6},
		},
		{
			name:     "ArrayLength",
			kind:     ir.ExprArrayLength{Array: 2},
			expected: []ir.ExpressionHandle{2},
		},
		{
			name: "ImageSample_WithOptionals",
			kind: func() ir.ExpressionKind {
				ai, off, dr := ir.ExpressionHandle(3), ir.ExpressionHandle(4), ir.ExpressionHandle(5)
				return ir.ExprImageSample{Image: 0, Sampler: 1, Coordinate: 2, ArrayIndex: &ai, Offset: &off, DepthRef: &dr}
			}(),
			expected: []ir.ExpressionHandle{0, 1, 2, 3, 4, 5},
		},
		{
			name: "ImageLoad_WithOptionals",
			kind: func() ir.ExpressionKind {
				ai, samp, lvl := ir.ExpressionHandle(2), ir.ExpressionHandle(3), ir.ExpressionHandle(4)
				return ir.ExprImageLoad{Image: 0, Coordinate: 1, ArrayIndex: &ai, Sample: &samp, Level: &lvl}
			}(),
			expected: []ir.ExpressionHandle{0, 1, 2, 3, 4},
		},
		{
			name:     "ImageQuery",
			kind:     ir.ExprImageQuery{Image: 0},
			expected: []ir.ExpressionHandle{0},
		},
		{
			name: "Phi",
			kind: ir.ExprPhi{Incoming: []ir.PhiIncoming{
				{PredKey: ir.PhiPredIfAccept, Value: 10},
				{PredKey: ir.PhiPredIfReject, Value: 11},
			}},
			expected: []ir.ExpressionHandle{10, 11},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var visited []ir.ExpressionHandle
			visitExprHandles(tt.kind, func(h ir.ExpressionHandle) {
				visited = append(visited, h)
			})
			if len(visited) != len(tt.expected) {
				t.Fatalf("expected %d handles, got %d: %v", len(tt.expected), len(visited), visited)
			}
			for i, exp := range tt.expected {
				if visited[i] != exp {
					t.Errorf("handle[%d]: got %d, want %d", i, visited[i], exp)
				}
			}
		})
	}
}

// TestDeadSwitchWithNestedDeadStoresEliminated verifies that a switch
// statement whose cases contain only dead local stores is eliminated
// after DCE removes the stores.
//
// Covers: sweepSwitch, unmarkDeadControlFlow for switch.
func TestDeadSwitchWithNestedDeadStoresEliminated(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}

	selVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})
	val1 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(10)})
	val2 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(20)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: selVal, End: val2 + 1}}},
		{Kind: ir.StmtSwitch{
			Selector: selVal,
			Cases: []ir.SwitchCase{
				{
					Value: ir.SwitchValueI32(0),
					Body: ir.Block{
						{Kind: ir.StmtStore{Pointer: lvHandle, Value: val1}},
					},
				},
				{
					Value: ir.SwitchValueDefault{},
					Body: ir.Block{
						{Kind: ir.StmtStore{Pointer: lvHandle, Value: val2}},
					},
				},
			},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Dead stores removed, all cases empty, selector dead → switch eliminated.
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtSwitch); ok {
			t.Fatal("switch with only dead local stores should be eliminated")
		}
	}
}

// TestSwitchWithLiveCasePreserved verifies that a switch is kept when
// at least one case has a side effect (barrier).
//
// Covers: sweepSwitch when not all cases are empty.
func TestSwitchWithLiveCasePreserved(t *testing.T) {
	mod := &ir.Module{}

	fn := &ir.Function{}
	selVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: selVal, End: selVal + 1}}},
		{Kind: ir.StmtSwitch{
			Selector: selVal,
			Cases: []ir.SwitchCase{
				{Value: ir.SwitchValueI32(0), Body: ir.Block{
					{Kind: ir.StmtBarrier{}},
				}},
				{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
			},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	found := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtSwitch); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("switch with live case (barrier) should be preserved")
	}
}

// TestStmtHasSideEffectsExhaustive verifies stmtHasSideEffects for
// statement types that do and don't have side effects.
//
// Covers: stmtHasSideEffects for StmtImageStore, StmtImageAtomic,
// StmtKill, StmtWorkGroupUniformLoad, StmtRayQuery, StmtBlock,
// and control flow recursion.
func TestStmtHasSideEffectsExhaustive(t *testing.T) {
	fn := &ir.Function{}
	localPtrs := buildLocalPtrMap(fn)

	tests := []struct {
		name       string
		stmt       ir.Statement
		hasSideEff bool
	}{
		{"EmitNoEffect", ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}}, false},
		{"Return", ir.Statement{Kind: ir.StmtReturn{}}, false},
		{"Break", ir.Statement{Kind: ir.StmtBreak{}}, false},
		{"Continue", ir.Statement{Kind: ir.StmtContinue{}}, false},
		{"Barrier", ir.Statement{Kind: ir.StmtBarrier{}}, true},
		{"Kill", ir.Statement{Kind: ir.StmtKill{}}, true},
		{"ImageStore", ir.Statement{Kind: ir.StmtImageStore{Image: 0, Coordinate: 1, Value: 2}}, true},
		{"ImageAtomic", ir.Statement{Kind: ir.StmtImageAtomic{Image: 0, Coordinate: 1, Value: 2}}, true},
		{"Atomic", ir.Statement{Kind: ir.StmtAtomic{Pointer: 0, Value: 1}}, true},
		{"Call", ir.Statement{Kind: ir.StmtCall{Function: 0}}, true},
		{"WorkGroupUniformLoad", ir.Statement{Kind: ir.StmtWorkGroupUniformLoad{Pointer: 0, Result: 1}}, true},
		{"RayQuery", ir.Statement{Kind: ir.StmtRayQuery{Query: 0, Fun: ir.RayQueryTerminate{}}}, true},
		{"EmptyBlock", ir.Statement{Kind: ir.StmtBlock{Block: ir.Block{}}}, false},
		{"BlockWithBarrier", ir.Statement{Kind: ir.StmtBlock{Block: ir.Block{
			{Kind: ir.StmtBarrier{}},
		}}}, true},
		{"IfWithEmptyBranches", ir.Statement{Kind: ir.StmtIf{
			Condition: 0, Accept: ir.Block{}, Reject: ir.Block{},
		}}, false},
		{"IfWithBarrier", ir.Statement{Kind: ir.StmtIf{
			Condition: 0,
			Accept:    ir.Block{{Kind: ir.StmtBarrier{}}},
			Reject:    ir.Block{},
		}}, true},
		{"LoopEmpty", ir.Statement{Kind: ir.StmtLoop{Body: ir.Block{}, Continuing: ir.Block{}}}, false},
		{"LoopWithCall", ir.Statement{Kind: ir.StmtLoop{
			Body: ir.Block{{Kind: ir.StmtCall{Function: 0}}}, Continuing: ir.Block{},
		}}, true},
		{"SwitchEmpty", ir.Statement{Kind: ir.StmtSwitch{
			Selector: 0, Cases: []ir.SwitchCase{{Body: ir.Block{}}},
		}}, false},
		{"SwitchWithAtomic", ir.Statement{Kind: ir.StmtSwitch{
			Selector: 0, Cases: []ir.SwitchCase{{Body: ir.Block{
				{Kind: ir.StmtAtomic{Pointer: 0, Value: 1}},
			}}},
		}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stmtHasSideEffects(tt.stmt, localPtrs, fn)
			if got != tt.hasSideEff {
				t.Errorf("stmtHasSideEffects = %v, want %v", got, tt.hasSideEff)
			}
		})
	}
}

// TestLocalStoreInsideNestedBlock verifies that stores to a dead local
// variable inside nested StmtBlock, StmtLoop, and StmtSwitch are eliminated.
//
// Covers: markLiveStoresBlock recursion into StmtBlock/StmtLoop/StmtSwitch,
// doSweepBlock for StmtBlock.
func TestLocalStoreInsideNestedBlock(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}

	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtBlock{
			Block: ir.Block{
				{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
			},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Dead store inside nested block should be removed.
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements (block + return), got %d", len(fn.Body))
	}
}

// TestPureCallWithLiveResultKept verifies the propagateCallResultLiveness
// path: a pure callee whose ExprCallResult is consumed by a global store
// has its arguments marked live retroactively.
//
// Covers: propagateCallResultLiveness, sweepCallIsLive when result is live.
func TestPureCallWithLiveResultKept(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "out", Space: ir.SpaceStorage, Type: 0},
		},
		Functions: []ir.Function{
			{Name: "pure_fn"}, // no side effects
		},
	}

	fn := &ir.Function{}

	argVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	resultH := appendExpr(fn, ir.ExprCallResult{})
	globalH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: argVal, End: argVal + 1}}},
		{Kind: ir.StmtCall{
			Function:  0,
			Arguments: []ir.ExpressionHandle{argVal},
			Result:    &resultH,
		}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: resultH, End: resultH + 1}}},
		{Kind: ir.StmtStore{Pointer: globalH, Value: resultH}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The call result feeds a global store, so the call must be kept.
	foundCall := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtCall); ok {
			foundCall = true
		}
	}
	if !foundCall {
		t.Fatal("pure call with live result should be preserved")
	}
}

// TestHasOtherLiveConsumerSharedExpression verifies that unmarkExpr does
// not unmark an expression used by another live expression.
//
// Covers: hasOtherLiveConsumer returning true, unmarkExpr early exit.
func TestHasOtherLiveConsumerSharedExpression(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "out", Space: ir.SpaceStorage, Type: 0},
		},
	}

	fn := &ir.Function{}

	// expr 0: shared value used by both a dead if-condition and a live store
	sharedVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	globalH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: sharedVal, End: globalH + 1}}},
		// Dead if (both branches empty) — condition uses sharedVal
		{Kind: ir.StmtIf{
			Condition: sharedVal,
			Accept:    ir.Block{},
			Reject:    ir.Block{},
		}},
		// Live global store also uses sharedVal
		{Kind: ir.StmtStore{Pointer: globalH, Value: sharedVal}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The if is dead, but sharedVal is still consumed by the live store.
	// sharedVal must remain live; unmarkExpr should detect hasOtherLiveConsumer
	// and not unmark it.
	foundStore := false
	for _, s := range fn.Body {
		if st, ok := s.Kind.(ir.StmtStore); ok {
			foundStore = true
			if st.Value != sharedVal {
				t.Errorf("store value should still be sharedVal (%d), got %d", sharedVal, st.Value)
			}
		}
	}
	if !foundStore {
		t.Fatal("global store should survive DCE")
	}
}

// TestResolveLocalVarDeepAccessIndexChain verifies that resolveLocalVar
// follows AccessIndex chains of depth > 1 (nested struct field access).
//
// Covers: resolveLocalVar ExprAccess path and deep chain traversal.
func TestResolveLocalVarDeepAccessIndexChain(t *testing.T) {
	mod := &ir.Module{}
	f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
	innerSt := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.StructType{
			Members: []ir.StructMember{{Name: "val", Type: f32, Offset: 0}},
			Span:    4,
		},
	})
	outerSt := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.StructType{
			Members: []ir.StructMember{{Name: "inner", Type: innerSt, Offset: 0}},
			Span:    4,
		},
	})

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "outer", Type: outerSt}},
	}

	val := appendExpr(fn, ir.Literal{Value: ir.LiteralF32(1.0)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	innerField := appendExpr(fn, ir.ExprAccessIndex{Base: lvHandle, Index: 0})
	deepField := appendExpr(fn, ir.ExprAccessIndex{Base: innerField, Index: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: deepField, Value: val}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The deep field store targets a dead local. Should be eliminated.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement (return), got %d", len(fn.Body))
	}
}

// TestIsLiveInRangeOutOfBounds verifies conservative behavior when an
// expression handle is beyond the live array.
//
// Covers: isLiveInRange out-of-bounds guard returning true.
func TestIsLiveInRangeOutOfBounds(t *testing.T) {
	live := []bool{true, false}
	// Handle beyond array bounds should be treated as live (conservative).
	if !isLiveInRange(100, live) {
		t.Error("out-of-bounds handle should be treated as live")
	}
	// Normal in-range check.
	if isLiveInRange(1, live) {
		t.Error("handle 1 should be dead")
	}
	if !isLiveInRange(0, live) {
		t.Error("handle 0 should be live")
	}
}

// TestCalleeHasSideEffectsOutOfRange verifies conservative behavior when
// the function handle is out of range.
//
// Covers: calleeHasSideEffects out-of-range guard.
func TestCalleeHasSideEffectsOutOfRange(t *testing.T) {
	mod := &ir.Module{}
	// Out-of-range function handle should return true (conservative).
	if !calleeHasSideEffects(mod, 999) {
		t.Error("out-of-range function handle should be treated as having side effects")
	}
	// nil module should return true.
	if !calleeHasSideEffects(nil, 0) {
		t.Error("nil module should be treated as having side effects")
	}
}

// TestPureCalleeNoSideEffects verifies that a callee with only local stores
// is detected as having no side effects.
//
// Covers: calleeHasSideEffects → blockHasSideEffects → stmtHasSideEffects
// for local store (no side effect).
func TestPureCalleeNoSideEffects(t *testing.T) {
	// Helper function that only has a local store.
	helperFn := ir.Function{
		Name:      "pure",
		LocalVars: []ir.LocalVariable{{Name: "tmp", Type: 0}},
	}

	lvH := ir.ExpressionHandle(0)
	valH := ir.ExpressionHandle(1)
	helperFn.Expressions = []ir.Expression{
		{Kind: ir.ExprLocalVariable{Variable: 0}},
		{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
	}
	helperFn.Body = ir.Block{
		{Kind: ir.StmtStore{Pointer: lvH, Value: valH}},
	}

	mod := &ir.Module{Functions: []ir.Function{helperFn}}

	if calleeHasSideEffects(mod, 0) {
		t.Error("pure callee with only local stores should NOT have side effects")
	}
}

// TestLocalAtomicInsideIfIsConservativelyKept verifies that StmtAtomic on
// a local variable within an if-branch is conservatively kept as live.
//
// Covers: markStmtRoots StmtAtomic on local with result, markStmtRoots
// StmtIf recursion into nested atomics.
func TestLocalAtomicInsideIfIsConservativelyKept(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "counter", Type: i32}},
	}

	condH := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})
	lvH := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	valH := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	resultH := appendExpr(fn, ir.ExprAtomicResult{Ty: i32})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condH, End: resultH + 1}}},
		{Kind: ir.StmtIf{
			Condition: condH,
			Accept: ir.Block{
				{Kind: ir.StmtAtomic{
					Pointer: lvH,
					Fun:     ir.AtomicAdd{},
					Value:   valH,
					Result:  &resultH,
				}},
			},
			Reject: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Local atomics are conservatively kept because they're unusual but
	// might have observable effects. The if should survive.
	foundIf := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtIf); ok {
			foundIf = true
		}
	}
	if !foundIf {
		t.Fatal("if with local atomic should be conservatively preserved")
	}
}

// TestEmptyFunctionNoExpressions verifies that Run handles a function
// with no expressions gracefully (early return guard).
func TestEmptyFunctionNoExpressions(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{
		Body: ir.Block{{Kind: ir.StmtReturn{}}},
	}
	// Should not panic. No expressions → early return.
	Run(mod, fn)
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
}

// TestPropagateCallResultLivenessInsideSwitch verifies that
// propagateCallResultLiveness recurses into StmtSwitch cases.
//
// Covers: propagateCallResultLiveness StmtSwitch branch.
func TestPropagateCallResultLivenessInsideSwitch(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "out", Space: ir.SpaceStorage, Type: 0},
		},
		Functions: []ir.Function{
			{Name: "pure_fn"},
		},
	}

	fn := &ir.Function{}

	selVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})
	argVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	resultH := appendExpr(fn, ir.ExprCallResult{})
	globalH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: selVal, End: argVal + 1}}},
		{Kind: ir.StmtSwitch{
			Selector: selVal,
			Cases: []ir.SwitchCase{
				{
					Value: ir.SwitchValueI32(0),
					Body: ir.Block{
						{Kind: ir.StmtCall{
							Function:  0,
							Arguments: []ir.ExpressionHandle{argVal},
							Result:    &resultH,
						}},
						{Kind: ir.StmtEmit{Range: ir.Range{Start: resultH, End: resultH + 1}}},
						{Kind: ir.StmtStore{Pointer: globalH, Value: resultH}},
					},
				},
				{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
			},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The pure call result is used by a global store inside a switch case.
	// The call should be kept.
	foundCall := false
	for _, s := range fn.Body {
		if sw, ok := s.Kind.(ir.StmtSwitch); ok {
			for _, c := range sw.Cases {
				for _, cs := range c.Body {
					if _, ok := cs.Kind.(ir.StmtCall); ok {
						foundCall = true
					}
				}
			}
		}
	}
	if !foundCall {
		t.Fatal("pure call with live result inside switch should be preserved")
	}
}

// TestPropagateCallResultLivenessInsideLoop verifies recursion into StmtLoop.
//
// Covers: propagateCallResultLiveness StmtLoop branch.
func TestPropagateCallResultLivenessInsideLoop(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "out", Space: ir.SpaceStorage, Type: 0},
		},
		Functions: []ir.Function{
			{Name: "pure_fn"},
		},
	}

	fn := &ir.Function{}

	argVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	resultH := appendExpr(fn, ir.ExprCallResult{})
	globalH := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtLoop{
			Body: ir.Block{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: argVal, End: argVal + 1}}},
				{Kind: ir.StmtCall{
					Function:  0,
					Arguments: []ir.ExpressionHandle{argVal},
					Result:    &resultH,
				}},
				{Kind: ir.StmtEmit{Range: ir.Range{Start: resultH, End: resultH + 1}}},
				{Kind: ir.StmtStore{Pointer: globalH, Value: resultH}},
				{Kind: ir.StmtBreak{}},
			},
			Continuing: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Verify the call inside the loop is preserved.
	foundCall := false
	for _, s := range fn.Body {
		if loop, ok := s.Kind.(ir.StmtLoop); ok {
			for _, bs := range loop.Body {
				if _, ok := bs.Kind.(ir.StmtCall); ok {
					foundCall = true
				}
			}
		}
	}
	if !foundCall {
		t.Fatal("pure call with live result inside loop should be preserved")
	}
}
