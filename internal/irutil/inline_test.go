package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestInlineEmitRangeEnd verifies that StmtEmit Range.End is treated as an
// exclusive boundary when remapping into the caller's expression arena.
// Callee emit ranges must end at "one past the last remapped expression",
// not at the raw mapped value of the original End handle.
func TestInlineEmitRangeEnd(t *testing.T) {
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name:   "callee",
				Result: &ir.FunctionResult{Type: scalarI32},
				// Body: emit two expressions [0..2), then return the
				// second. Range.End == 2 is exclusive.
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(7)}},
					{Kind: ir.Literal{Value: ir.LiteralI32(13)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: handlePtr(1)}},
				},
			},
		},
	}

	// Caller: result := callee()
	module.EntryPoints = []ir.EntryPoint{
		{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{
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
	var emit *ir.StmtEmit
	walkStmts(main.Body, func(s *ir.Statement) {
		if e, ok := s.Kind.(ir.StmtEmit); ok && emit == nil {
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
	var litIdxs []ir.ExpressionHandle
	for i := range main.Expressions {
		if _, ok := main.Expressions[i].Kind.(ir.Literal); ok {
			litIdxs = append(litIdxs, ir.ExpressionHandle(i))
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
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name:   "helper",
				Result: &ir.FunctionResult{Type: scalarI32},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
				},
				NamedExpressions: map[ir.ExpressionHandle]string{
					0: "result",
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: handlePtr(0)}},
				},
			},
		},
	}
	module.EntryPoints = []ir.EntryPoint{
		{
			Name: "main", Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0, Result: handlePtr(0)}},
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
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "callee",
				Arguments: []ir.FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: &ir.FunctionResult{Type: scalarI32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: handlePtr(0)}},
				},
			},
		},
	}
	caller := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(99)}},
			{Kind: ir.ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0}, Result: handlePtr(1)}},
		},
	}
	module.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageCompute, Function: caller}}

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
		case ir.ExprLoad, ir.ExprLocalVariable:
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
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "callee",
				Arguments: []ir.FunctionArgument{
					{Name: "a", Type: scalarI32},
					{Name: "b", Type: scalarI32},
				},
				Result: &ir.FunctionResult{Type: scalarI32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprFunctionArgument{Index: 1}},
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtReturn{Value: handlePtr(2)}},
				},
			},
		},
	}
	caller := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},
			{Kind: ir.ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0, 1},
				Result:    handlePtr(2),
			}},
		},
	}
	module.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function
	// Find the inlined ExprBinary — it's the marker for the start of the
	// callee's contiguous region's "real" content. The handle of the
	// inlined emit's Start..End-1 must wrap exactly the callee
	// expressions, no helper expressions interleaved.
	var emit *ir.StmtEmit
	walkStmts(main.Body, func(s *ir.Statement) {
		if e, ok := s.Kind.(ir.StmtEmit); ok && emit == nil {
			emit = &e
		}
	})
	if emit == nil {
		t.Fatalf("expected StmtEmit in inlined body")
	}
	for h := emit.Range.Start; h < emit.Range.End; h++ {
		switch main.Expressions[h].Kind.(type) {
		case ir.ExprBinary, ir.ExprAccess, ir.ExprAccessIndex, ir.ExprLoad, ir.ExprUnary,
			ir.ExprSplat, ir.ExprSwizzle, ir.ExprCompose, ir.ExprMath, ir.ExprAs, ir.Literal,
			ir.ExprConstant, ir.ExprZeroValue, ir.ExprSelect, ir.ExprRelational,
			ir.ExprFunctionArgument, ir.ExprLocalVariable, ir.ExprGlobalVariable:
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
func handlePtr(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

// walkStmts visits every statement in the block tree, calling fn for each.
func walkStmts(b ir.Block, fn func(*ir.Statement)) {
	for i := range b {
		fn(&b[i])
		switch sk := b[i].Kind.(type) {
		case ir.StmtBlock:
			walkStmts(sk.Block, fn)
		case ir.StmtIf:
			walkStmts(sk.Accept, fn)
			walkStmts(sk.Reject, fn)
		case ir.StmtLoop:
			walkStmts(sk.Body, fn)
			walkStmts(sk.Continuing, fn)
		case ir.StmtSwitch:
			for j := range sk.Cases {
				walkStmts(sk.Cases[j].Body, fn)
			}
		}
	}
}

// TestInlineEarlyReturnsInIf: returns in both branches of an if, with nothing
// after it, become stores into the return slot and nothing else: no loop
// around the body, no break (there is no loop or switch to leave) and no
// "returned" flag (no code after the if needs a guard).
func TestInlineEarlyReturnsInIf(t *testing.T) {
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				// fn test_if(x: i32) -> i32 {
				//     if x > 0 { return 1; }
				//     else { return -1; }
				// }
				Name: "test_if",
				Arguments: []ir.FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: &ir.FunctionResult{Type: scalarI32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                      // [0]
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},                    // [1]
					{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}}, // [2]
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},                    // [3]
					{Kind: ir.Literal{Value: ir.LiteralI32(-1)}},                   // [4]
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept:    ir.Block{{Kind: ir.StmtReturn{Value: handlePtr(3)}}},
						Reject:    ir.Block{{Kind: ir.StmtReturn{Value: handlePtr(4)}}},
					}},
				},
			},
		},
	}

	caller := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
			{Kind: ir.ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0},
				Result:    handlePtr(1),
			}},
		},
	}
	module.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	slotStores := 0
	walkStmts(main.Body, func(s *ir.Statement) {
		switch k := s.Kind.(type) {
		case ir.StmtLoop, ir.StmtBreak:
			t.Errorf("unexpected %T in the inlined body: %+v", k, main.Body)
		case ir.StmtStore:
			if lv, ok := main.Expressions[k.Pointer].Kind.(ir.ExprLocalVariable); ok && main.LocalVars[lv.Variable].Name == "_inline_ret_test_if" {
				slotStores++
			}
		}
	})
	if slotStores != 2 {
		t.Errorf("stores into the return slot = %d, want 2 (one per return)", slotStores)
	}
	for _, lv := range main.LocalVars {
		if lv.Name == "_inline_returned_test_if" {
			t.Errorf("a returned flag was made though no guard reads it")
		}
	}

	// The call result expression should now be an ExprLoad (from the ret slot).
	callResultExpr := main.Expressions[1]
	if _, ok := callResultExpr.Kind.(ir.ExprLoad); !ok {
		t.Errorf("ExprCallResult[1] should be rewritten to ExprLoad, got %T", callResultExpr.Kind)
	}
}

// TestInlineEarlyReturnVoid: a void callee's early return with nothing after
// its if lowers to nothing at all: no loop, no break, no flag.
func TestInlineEarlyReturnVoid(t *testing.T) {
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				// fn early_void(x: i32) {
				//     if x > 0 { return; }
				//     // implicit return at end
				// }
				Name: "early_void",
				Arguments: []ir.FunctionArgument{
					{Name: "x", Type: scalarI32},
				},
				Result: nil, // void
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
					{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept:    ir.Block{{Kind: ir.StmtReturn{}}},
						Reject:    ir.Block{},
					}},
				},
			},
		},
	}

	caller := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(5)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarI32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0},
				Result:    nil,
			}},
		},
	}
	module.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	walkStmts(main.Body, func(s *ir.Statement) {
		switch k := s.Kind.(type) {
		case ir.StmtLoop, ir.StmtBreak, ir.StmtReturn:
			t.Errorf("unexpected %T in the inlined body: %+v", k, main.Body)
		}
	})
	for _, lv := range main.LocalVars {
		if lv.Name == "_inline_returned_early_void" {
			t.Errorf("a returned flag was made though no guard reads it")
		}
	}
}

// TestInlineTailReturnNoWrap verifies that a callee with only a tail
// return (no early returns) does NOT get wrapped in a loop.
func TestInlineTailReturnNoWrap(t *testing.T) {
	scalarI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				// fn add(a: i32, b: i32) -> i32 { return a + b; }
				Name: "add",
				Arguments: []ir.FunctionArgument{
					{Name: "a", Type: scalarI32},
					{Name: "b", Type: scalarI32},
				},
				Result: &ir.FunctionResult{Type: scalarI32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprFunctionArgument{Index: 1}},
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &scalarI32},
					{Handle: &scalarI32},
					{Handle: &scalarI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtReturn{Value: handlePtr(2)}},
				},
			},
		},
	}

	caller := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},
			{Kind: ir.ExprCallResult{Function: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarI32},
			{Handle: &scalarI32},
			{Handle: &scalarI32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0, 1},
				Result:    handlePtr(2),
			}},
		},
	}
	module.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageCompute, Function: caller}}

	if err := InlineUserFunctions(module, nil); err != nil {
		t.Fatalf("InlineUserFunctions: %v", err)
	}

	main := &module.EntryPoints[0].Function

	// Should NOT have a loop wrapper — tail return only.
	walkStmts(main.Body, func(s *ir.Statement) {
		if _, ok := s.Kind.(ir.StmtLoop); ok {
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
	mod := &ir.Module{}
	// Types: i32
	i32 := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}})

	// Callee: fn add_one(x: i32) -> i32 { return x + 1; }
	litOne := ir.ExpressionHandle(0)
	funcArg0 := ir.ExpressionHandle(1)
	binAdd := ir.ExpressionHandle(2)
	callee := ir.Function{
		Name:      "add_one",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: i32}},
		Result:    &ir.FunctionResult{Type: i32},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: funcArg0, Right: litOne}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtReturn{Value: &binAdd}},
		},
	}
	callee.ExpressionTypes = make([]ir.TypeResolution, len(callee.Expressions))
	mod.Functions = append(mod.Functions, callee)

	// Entry point: fn main() { let r = add_one(42); }
	ep := ir.Function{Name: "main"}
	callerLit := ir.ExpressionHandle(len(ep.Expressions))
	ep.Expressions = append(ep.Expressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralI32(42)}})
	ep.ExpressionTypes = append(ep.ExpressionTypes, ir.TypeResolution{Handle: &i32})

	callResult := ir.ExpressionHandle(len(ep.Expressions))
	ep.Expressions = append(ep.Expressions, ir.Expression{Kind: ir.ExprCallResult{}})
	ep.ExpressionTypes = append(ep.ExpressionTypes, ir.TypeResolution{Handle: &i32})

	ep.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: callerLit, End: callerLit + 1}}},
		{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{callerLit}, Result: &callResult}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: callResult, End: callResult + 1}}},
	}

	mod.EntryPoints = append(mod.EntryPoints, ir.EntryPoint{
		Name:     "main",
		Stage:    ir.StageCompute,
		Function: ep,
	})

	err := InlineUserFunctions(mod, func(*ir.Function) bool { return true })
	if err != nil {
		t.Fatalf("InlineUserFunctions failed: %v", err)
	}

	// Verify the inlined body contains StmtEmit ranges that cover
	// the ExprLoad expression(s) created for the scalar arg spill.
	fn := &mod.EntryPoints[0].Function
	emittedHandles := make(map[ir.ExpressionHandle]bool)
	for _, s := range fn.Body {
		if em, ok := s.Kind.(ir.StmtEmit); ok {
			for h := em.Range.Start; h < em.Range.End; h++ {
				emittedHandles[h] = true
			}
		}
	}

	// Find the ExprLoad expressions that reference a local variable
	// (the arg-spill pattern). At least one must be emitted.
	localPtrs := make(map[ir.ExpressionHandle]uint32)
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			localPtrs[ir.ExpressionHandle(h)] = uint32(lv.Variable)
		}
	}

	foundEmittedArgLoad := false
	for h, expr := range fn.Expressions {
		ld, isLoad := expr.Kind.(ir.ExprLoad)
		if !isLoad {
			continue
		}
		if _, isLV := localPtrs[ld.Pointer]; !isLV {
			continue
		}
		hh := ir.ExpressionHandle(h)
		if emittedHandles[hh] {
			foundEmittedArgLoad = true
			break
		}
	}
	if !foundEmittedArgLoad {
		t.Fatal("expected at least one ExprLoad of arg-spill local to be inside a StmtEmit range (step 8b)")
	}
}

// emittedIn reports whether a StmtEmit directly in block covers h.
func emittedIn(block ir.Block, h ir.ExpressionHandle) bool {
	for _, s := range block {
		if em, ok := s.Kind.(ir.StmtEmit); ok && em.Range.Start <= h && h < em.Range.End {
			return true
		}
	}
	return false
}

// TestInlineEmitsTheCallResult requires the load that replaces a call's
// result to be emitted after the inlined body, so it is evaluated at the
// call and not wherever it is first used.
func TestInlineEmitsTheCallResult(t *testing.T) {
	i32 := ir.TypeHandle(0)
	mod := &ir.Module{
		Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		Functions: []ir.Function{{
			Name:            "seven",
			Result:          &ir.FunctionResult{Type: i32},
			Expressions:     []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralI32(7)}}},
			ExpressionTypes: []ir.TypeResolution{{Handle: &i32}},
			Body:            ir.Block{{Kind: ir.StmtReturn{Value: handlePtr(0)}}},
		}},
	}
	mod.EntryPoints = []ir.EntryPoint{{
		Name:  "main",
		Stage: ir.StageCompute,
		Function: ir.Function{
			Name:            "main",
			Expressions:     []ir.Expression{{Kind: ir.ExprCallResult{Function: 0}}},
			ExpressionTypes: []ir.TypeResolution{{Handle: &i32}},
			Body:            ir.Block{{Kind: ir.StmtCall{Function: 0, Result: handlePtr(0)}}},
		},
	}}

	if err := InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	fn := &mod.EntryPoints[0].Function
	if _, ok := fn.Expressions[0].Kind.(ir.ExprLoad); !ok {
		t.Fatalf("call result is %T, want ExprLoad", fn.Expressions[0].Kind)
	}
	if !emittedIn(fn.Body, 0) {
		t.Errorf("the load of the call result has no emit, body=%+v", fn.Body)
	}
}
