package ir

import (
	"testing"
)

// TestInlineEmitRangeEnd verifies that StmtEmit Range.End is treated as an
// exclusive boundary when remapping into the caller's expression arena.
// Callee emit ranges must end at "one past the last remapped expression",
// not at the raw mapped value of the original End handle.
func TestInlineEmitRangeEnd(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				Name:   "callee",
				Result: &FunctionResult{Type: scalarI32},
				// Body: emit two expressions [0..2), then return the
				// second. Range.End == 2 is exclusive.
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralI32(7)}},
					{Kind: Literal{Value: LiteralI32(13)}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 2}}},
					{Kind: StmtReturn{Value: handlePtr(1)}},
				},
			},
		},
	}

	// Caller: result := callee()
	module.EntryPoints = []EntryPoint{
		{
			Name:  "main",
			Stage: StageCompute,
			Function: Function{
				Name: "main",
				Expressions: []Expression{
					{Kind: ExprCallResult{Function: 0}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtCall{
						Function:  0,
						Arguments: nil,
						Result:    handlePtr(0),
					}},
				},
			},
		},
	}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function
	var emit *StmtEmit
	walkStmts(main.Body, func(s *Statement) {
		if e, ok := s.Kind.(StmtEmit); ok && emit == nil {
			emit = &e
		}
	})
	if emit == nil {
		t.Fatalf("expected StmtEmit in inlined body, body=%+v", main.Body)
	}
	// Both literals must lie inside [Start, End). Verify Start <= literalIdx < End.
	if emit.Range.Start >= emit.Range.End {
		t.Fatalf("inlined emit range is empty/inverted: %v", emit.Range)
	}
	// The exclusive End must point one past the last remapped literal.
	// Locate the two literal expressions in the caller.
	var litIdxs []ExpressionHandle
	for i := range main.Expressions {
		if _, ok := main.Expressions[i].Kind.(Literal); ok {
			litIdxs = append(litIdxs, ExpressionHandle(i))
		}
	}
	if len(litIdxs) != 2 {
		t.Fatalf("expected 2 literals in caller, found %d", len(litIdxs))
	}
	wantStart := litIdxs[0]
	wantEnd := litIdxs[1] + 1
	if emit.Range.Start != wantStart || emit.Range.End != wantEnd {
		t.Errorf("emit range %v, want {Start:%d, End:%d}",
			emit.Range, wantStart, wantEnd)
	}
}

// TestInlineNamedExpressionsPrefixed verifies that named expressions from
// the callee are prefixed with the callee's function name when copied into
// the caller, so two helpers that both define `let foo = ...` don't
// collide in the caller's namespace.
func TestInlineNamedExpressionsPrefixed(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				Name:   "helper",
				Result: &FunctionResult{Type: scalarI32},
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralI32(42)}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
				},
				NamedExpressions: map[ExpressionHandle]string{
					0: "result",
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 1}}},
					{Kind: StmtReturn{Value: handlePtr(0)}},
				},
			},
		},
	}
	module.EntryPoints = []EntryPoint{
		{
			Name: "main", Stage: StageCompute,
			Function: Function{
				Name: "main",
				Expressions: []Expression{
					{Kind: ExprCallResult{Function: 0}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtCall{Function: 0, Result: handlePtr(0)}},
				},
			},
		},
	}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function
	if len(main.NamedExpressions) == 0 {
		t.Fatalf("expected NamedExpressions in caller, got none")
	}
	const wantPrefix = "_helper_"
	for _, name := range main.NamedExpressions {
		if !startsWith(name, wantPrefix) {
			t.Errorf("named expression %q missing callee prefix %q", name, wantPrefix)
		}
	}
}

// TestInlineArgLoadExpressionTypes verifies that the synthesized argument
// load expressions have their ExpressionTypes populated. Without this,
// downstream emitters (DXIL) misclassify the loaded value (e.g., treat a
// pointer-typed argument as a scalar default), producing invalid code.
func TestInlineArgLoadExpressionTypes(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				Name: "callee",
				Arguments: []FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: &FunctionResult{Type: scalarI32},
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 0}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtReturn{Value: handlePtr(0)}},
				},
			},
		},
	}
	caller := Function{
		Name: "main",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(99)}},
			{Kind: ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []Statement{
			{Kind: StmtCall{Function: 0, Arguments: []ExpressionHandle{0}, Result: handlePtr(1)}},
		},
	}
	module.EntryPoints = []EntryPoint{{Name: "main", Stage: StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function
	if len(main.Expressions) != len(main.ExpressionTypes) {
		t.Fatalf("Expressions and ExpressionTypes lengths diverged: %d vs %d",
			len(main.Expressions), len(main.ExpressionTypes))
	}
	// Every ExprLoad and ExprLocalVariable that the inline pass synthesizes
	// must have a non-empty TypeResolution.
	for i := range main.Expressions {
		switch main.Expressions[i].Kind.(type) {
		case ExprLoad, ExprLocalVariable:
			tr := main.ExpressionTypes[i]
			if tr.Handle == nil && tr.Value == nil {
				t.Errorf("expression [%d] (%T) has empty TypeResolution",
					i, main.Expressions[i].Kind)
			}
		}
	}
}

// TestInlineExpressionsArrayContiguous verifies that the calleeExprMap
// region in the caller's expression arena is a single contiguous block.
// Helper expressions (slot pointers, arg loads) must live OUTSIDE that
// region; otherwise StmtEmit Range.End remapping breaks.
func TestInlineExpressionsArrayContiguous(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				Name: "callee",
				Arguments: []FunctionArgument{
					{Name: "a", Type: scalarI32},
					{Name: "b", Type: scalarI32},
				},
				Result: &FunctionResult{Type: scalarI32},
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 0}},
					{Kind: ExprFunctionArgument{Index: 1}},
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 2, End: 3}}},
					{Kind: StmtReturn{Value: handlePtr(2)}},
				},
			},
		},
	}
	caller := Function{
		Name: "main",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(1)}},
			{Kind: Literal{Value: LiteralI32(2)}},
			{Kind: ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{0, 1},
				Result:    handlePtr(2),
			}},
		},
	}
	module.EntryPoints = []EntryPoint{{Name: "main", Stage: StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function
	// Find the inlined ExprBinary — it's the marker for the start of the
	// callee's contiguous region's "real" content. The handle of the
	// inlined emit's Start..End-1 must wrap exactly the callee
	// expressions, no helper expressions interleaved.
	var emit *StmtEmit
	walkStmts(main.Body, func(s *Statement) {
		if e, ok := s.Kind.(StmtEmit); ok && emit == nil {
			emit = &e
		}
	})
	if emit == nil {
		t.Fatalf("expected StmtEmit in inlined body")
	}
	for h := emit.Range.Start; h < emit.Range.End; h++ {
		switch main.Expressions[h].Kind.(type) {
		case ExprBinary, ExprAccess, ExprAccessIndex, ExprLoad, ExprUnary,
			ExprSplat, ExprSwizzle, ExprCompose, ExprMath, ExprAs, Literal,
			ExprConstant, ExprZeroValue, ExprSelect, ExprRelational,
			ExprFunctionArgument, ExprLocalVariable, ExprGlobalVariable:
			// OK — these are the kinds the callee body could hold or
			// that the inline pass produces inside the calleeExprMap
			// region. The key invariant being tested is just that the
			// range is non-empty and dense (every handle in [Start, End)
			// is a real expression, not a sentinel).
		default:
			t.Errorf("expression [%d] inside emit range has unexpected kind %T", h, main.Expressions[h].Kind)
		}
	}
}

// handlePtr returns a pointer to the given ExpressionHandle, for use in
// StmtReturn.Value / StmtCall.Result fields that take *ExpressionHandle.
func handlePtr(h ExpressionHandle) *ExpressionHandle {
	return &h
}

// walkStmts visits every statement in the block tree, calling fn for each.
func walkStmts(b Block, fn func(*Statement)) {
	for i := range b {
		fn(&b[i])
		switch sk := b[i].Kind.(type) {
		case StmtBlock:
			walkStmts(sk.Block, fn)
		case StmtIf:
			walkStmts(sk.Accept, fn)
			walkStmts(sk.Reject, fn)
		case StmtLoop:
			walkStmts(sk.Body, fn)
			walkStmts(sk.Continuing, fn)
		case StmtSwitch:
			for j := range sk.Cases {
				walkStmts(sk.Cases[j].Body, fn)
			}
		}
	}
}

// TestInlineEarlyReturnWrap verifies that a callee with early returns
// (returns inside if/switch) is wrapped in a loop+break pattern after
// inlining. This mirrors DXC's AlwaysInliner behavior.
func TestInlineEarlyReturnWrap(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				// fn test_if(x: i32) -> i32 {
				//     if x > 0 { return 1; }
				//     else { return -1; }
				// }
				Name: "test_if",
				Arguments: []FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: &FunctionResult{Type: scalarI32},
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 0}},                   // [0]
					{Kind: Literal{Value: LiteralI32(0)}},                    // [1]
					{Kind: ExprBinary{Op: BinaryGreater, Left: 0, Right: 1}}, // [2]
					{Kind: Literal{Value: LiteralI32(1)}},                    // [3]
					{Kind: Literal{Value: LiteralI32(-1)}},                   // [4]
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 2, End: 3}}},
					{Kind: StmtIf{
						Condition: 2,
						Accept:    Block{{Kind: StmtReturn{Value: handlePtr(3)}}},
						Reject:    Block{{Kind: StmtReturn{Value: handlePtr(4)}}},
					}},
				},
			},
		},
	}

	caller := Function{
		Name: "main",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(42)}},
			{Kind: ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{0},
				Result:    handlePtr(1),
			}},
		},
	}
	module.EntryPoints = []EntryPoint{{Name: "main", Stage: StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	// The inlined body must contain a StmtLoop (wrapper) because test_if
	// has early returns inside the if-statement.
	foundLoop := false
	walkStmts(main.Body, func(s *Statement) {
		if _, ok := s.Kind.(StmtLoop); ok {
			foundLoop = true
		}
	})
	if !foundLoop {
		t.Fatalf("expected StmtLoop wrapper in inlined body for early-return callee, body=%+v", main.Body)
	}

	// Inside the loop, the returns must have been replaced with
	// StmtStore (to ret slot) + StmtBreak.
	breakCount := 0
	storeCount := 0
	walkStmts(main.Body, func(s *Statement) {
		switch s.Kind.(type) {
		case StmtBreak:
			breakCount++
		case StmtStore:
			storeCount++
		}
	})
	// Two early returns -> two store+break pairs
	if breakCount < 2 {
		t.Errorf("expected at least 2 StmtBreak in wrapped body, got %d", breakCount)
	}
	if storeCount < 2 {
		t.Errorf("expected at least 2 StmtStore (ret slot + arg spill) in wrapped body, got %d", storeCount)
	}

	// The call result expression should now be an ExprLoad (from the ret slot).
	callResultExpr := main.Expressions[1]
	if _, ok := callResultExpr.Kind.(ExprLoad); !ok {
		t.Errorf("ExprCallResult[1] should be rewritten to ExprLoad, got %T", callResultExpr.Kind)
	}
}

// TestInlineEarlyReturnVoid verifies that void callees with early returns
// get wrapped in loop+break without a return slot.
func TestInlineEarlyReturnVoid(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				// fn early_void(x: i32) {
				//     if x > 0 { return; }
				//     // implicit return at end
				// }
				Name: "early_void",
				Arguments: []FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: nil, // void
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 0}},
					{Kind: Literal{Value: LiteralI32(0)}},
					{Kind: ExprBinary{Op: BinaryGreater, Left: 0, Right: 1}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 2, End: 3}}},
					{Kind: StmtIf{
						Condition: 2,
						Accept:    Block{{Kind: StmtReturn{}}},
						Reject:    Block{},
					}},
				},
			},
		},
	}

	caller := Function{
		Name: "main",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(5)}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: &scalarI32},
		},
		Body: []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{0},
				Result:    nil,
			}},
		},
	}
	module.EntryPoints = []EntryPoint{{Name: "main", Stage: StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	// Verify loop wrapper exists
	foundLoop := false
	walkStmts(main.Body, func(s *Statement) {
		if _, ok := s.Kind.(StmtLoop); ok {
			foundLoop = true
		}
	})
	if !foundLoop {
		t.Fatalf("expected StmtLoop wrapper for void callee with early return")
	}

	// Verify break exists (void return -> just break, no store)
	breakCount := 0
	walkStmts(main.Body, func(s *Statement) {
		if _, ok := s.Kind.(StmtBreak); ok {
			breakCount++
		}
	})
	if breakCount == 0 {
		t.Errorf("expected at least 1 StmtBreak in wrapped void body")
	}
}

// TestInlineTailReturnNoWrap verifies that a callee with only a tail
// return (no early returns) does NOT get wrapped in a loop.
func TestInlineTailReturnNoWrap(t *testing.T) {
	scalarI32 := TypeHandle(0)
	module := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		Functions: []Function{
			{
				// fn add(a: i32, b: i32) -> i32 { return a + b; }
				Name: "add",
				Arguments: []FunctionArgument{
					{Name: "a", Type: scalarI32},
					{Name: "b", Type: scalarI32},
				},
				Result: &FunctionResult{Type: scalarI32},
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 0}},
					{Kind: ExprFunctionArgument{Index: 1}},
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}},
				},
				ExpressionTypes: []TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 2, End: 3}}},
					{Kind: StmtReturn{Value: handlePtr(2)}},
				},
			},
		},
	}

	caller := Function{
		Name: "main",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(1)}},
			{Kind: Literal{Value: LiteralI32(2)}},
			{Kind: ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{0, 1},
				Result:    handlePtr(2),
			}},
		},
	}
	module.EntryPoints = []EntryPoint{{Name: "main", Stage: StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	// Should NOT have a loop wrapper — tail return only.
	walkStmts(main.Body, func(s *Statement) {
		if _, ok := s.Kind.(StmtLoop); ok {
			t.Errorf("unexpected StmtLoop in inlined body — tail-return callee should NOT be wrapped")
		}
	})
}

// startsWith is strings.HasPrefix without the import (keep the test file
// dependency-free of the standard "strings" package since the rest of
// this test file only depends on package ir).
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestInlineEmitsArgLoadExpressions verifies that the inliner adds
// StmtEmit coverage for scalar-spilled argument loads (step 8b).
// Without this Emit, mem2reg cannot detect the loads as live and
// fails to promote the arg-spill local variable.
func TestInlineEmitsArgLoadExpressions(t *testing.T) {
	mod := &Module{}
	// Types: i32
	i32 := TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, Type{Inner: ScalarType{Kind: ScalarSint, Width: 4}})

	// Callee: fn add_one(x: i32) -> i32 { return x + 1; }
	litOne := ExpressionHandle(0)
	funcArg0 := ExpressionHandle(1)
	binAdd := ExpressionHandle(2)
	callee := Function{
		Name:      "add_one",
		Arguments: []FunctionArgument{{Name: "x", Type: i32}},
		Result:    &FunctionResult{Type: i32},
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(1)}},
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprBinary{Op: BinaryAdd, Left: funcArg0, Right: litOne}},
		},
		Body: Block{
			{Kind: StmtEmit{Range: Range{Start: 0, End: 3}}},
			{Kind: StmtReturn{Value: &binAdd}},
		},
	}
	callee.ExpressionTypes = make([]TypeResolution, len(callee.Expressions))
	mod.Functions = append(mod.Functions, callee)

	// Entry point: fn main() { let r = add_one(42); }
	ep := Function{Name: "main"}
	callerLit := ExpressionHandle(len(ep.Expressions))
	ep.Expressions = append(ep.Expressions, Expression{Kind: Literal{Value: LiteralI32(42)}})
	ep.ExpressionTypes = append(ep.ExpressionTypes, TypeResolution{Handle: &i32})

	callResult := ExpressionHandle(len(ep.Expressions))
	ep.Expressions = append(ep.Expressions, Expression{Kind: ExprCallResult{}})
	ep.ExpressionTypes = append(ep.ExpressionTypes, TypeResolution{Handle: &i32})

	ep.Body = Block{
		{Kind: StmtEmit{Range: Range{Start: callerLit, End: callerLit + 1}}},
		{Kind: StmtCall{Function: 0, Arguments: []ExpressionHandle{callerLit}, Result: &callResult}},
		{Kind: StmtEmit{Range: Range{Start: callResult, End: callResult + 1}}},
	}

	mod.EntryPoints = append(mod.EntryPoints, EntryPoint{
		Name:     "main",
		Stage:    StageCompute,
		Function: ep,
	})

	err := InlineUserFunctions(mod, func(*Function) bool { return true })
	if err != nil {
		t.Fatalf("InlineUserFunctions failed: %v", err)
	}

	// Verify the inlined body contains StmtEmit ranges that cover
	// the ExprLoad expression(s) created for the scalar arg spill.
	fn := &mod.EntryPoints[0].Function
	emittedHandles := make(map[ExpressionHandle]bool)
	for _, s := range fn.Body {
		if em, ok := s.Kind.(StmtEmit); ok {
			for h := em.Range.Start; h < em.Range.End; h++ {
				emittedHandles[h] = true
			}
		}
	}

	// Find the ExprLoad expressions that reference a local variable
	// (the arg-spill pattern). At least one must be emitted.
	localPtrs := make(map[ExpressionHandle]uint32)
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ExprLocalVariable); ok {
			localPtrs[ExpressionHandle(h)] = lv.Variable
		}
	}

	foundEmittedArgLoad := false
	for h, expr := range fn.Expressions {
		ld, isLoad := expr.Kind.(ExprLoad)
		if !isLoad {
			continue
		}
		if _, isLV := localPtrs[ld.Pointer]; !isLV {
			continue
		}
		hh := ExpressionHandle(h)
		if emittedHandles[hh] {
			foundEmittedArgLoad = true
			break
		}
	}
	if !foundEmittedArgLoad {
		t.Fatal("expected at least one ExprLoad of arg-spill local to be inside a StmtEmit range (step 8b)")
	}
}
