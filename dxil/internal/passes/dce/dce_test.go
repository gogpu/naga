package dce

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// scalarTypeHandle appends a scalar type to mod.Types and returns its handle.
func scalarTypeHandle(mod *ir.Module, kind ir.ScalarKind, width uint8) ir.TypeHandle {
	h := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.ScalarType{Kind: kind, Width: width},
	})
	return h
}

// appendExpr appends an expression and returns its handle.
func appendExpr(fn *ir.Function, kind ir.ExpressionKind) ir.ExpressionHandle {
	h := ir.ExpressionHandle(len(fn.Expressions))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
	return h
}

// TestDeadLocalOnlyCompute verifies that a compute shader with only
// local variable writes (no UAV store, no return value) has all its
// stores eliminated.
//
// IR shape (models bits.wgsl / select.wgsl pattern):
//
//	var x: i32
//	emit(Literal(42))
//	x = 42
//	emit(ExprLoad(x))
//	ret void
//
// Expected: after DCE, body contains only the return statement
// (stores and emits for dead locals removed).
func TestDeadLocalOnlyCompute(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: i32},
		},
	}

	// Expressions: 0=Literal(42), 1=ExprLocalVariable(0), 2=ExprLoad(1)
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	loadHandle := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadHandle, End: loadHandle + 1}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// After DCE: store to dead local removed, emit ranges for dead
	// expressions removed. Only return remains.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].Kind.(ir.StmtReturn); !ok {
		t.Fatalf("expected StmtReturn, got %T", fn.Body[0].Kind)
	}
}

// TestLiveLocalUsedByGlobalStore verifies that a local variable whose
// load feeds into a global store is NOT eliminated.
//
// IR shape:
//
//	var x: i32
//	x = 42
//	global_out = load(x)
func TestLiveLocalUsedByGlobalStore(t *testing.T) {
	mod := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "out", Space: ir.SpaceStorage, Type: 0},
		},
	}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: i32},
		},
	}

	// Expressions: 0=Literal(42), 1=ExprLocalVariable(0), 2=ExprLoad(1),
	// 3=ExprGlobalVariable(0)
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	loadHandle := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})
	globalHandle := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadHandle, End: loadHandle + 1}}},
		{Kind: ir.StmtStore{Pointer: globalHandle, Value: loadHandle}},
		{Kind: ir.StmtReturn{}},
	}

	bodyLenBefore := len(fn.Body)
	Run(mod, fn)

	// Nothing should be eliminated — the local feeds into a global store.
	if len(fn.Body) != bodyLenBefore {
		t.Fatalf("expected %d statements (nothing dead), got %d", bodyLenBefore, len(fn.Body))
	}
}

// TestMultipleDeadLocals verifies that multiple dead locals are all
// eliminated in a single pass.
func TestMultipleDeadLocals(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "a", Type: i32},
			{Name: "b", Type: i32},
		},
	}

	valA := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	valB := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(2)})
	lvA := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	lvB := appendExpr(fn, ir.ExprLocalVariable{Variable: 1})
	loadA := appendExpr(fn, ir.ExprLoad{Pointer: lvA})
	loadB := appendExpr(fn, ir.ExprLoad{Pointer: lvB})
	// Use loadB in binary expr with loadA (still dead).
	_ = appendExpr(fn, ir.ExprBinary{Op: ir.BinaryAdd, Left: loadA, Right: loadB})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: valA, End: valB + 1}}},
		{Kind: ir.StmtStore{Pointer: lvA, Value: valA}},
		{Kind: ir.StmtStore{Pointer: lvB, Value: valB}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadA, End: loadB + 1}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// After DCE: both stores removed, emits for dead ranges removed.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
}

// TestMixedLiveAndDeadLocals verifies that only the dead local is
// eliminated when another local is live (feeds into a return value).
func TestMixedLiveAndDeadLocals(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "live_var", Type: i32},
			{Name: "dead_var", Type: i32},
		},
		Result: &ir.FunctionResult{Type: i32},
	}

	valLive := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(10)})
	valDead := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(20)})
	lvLive := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	lvDead := appendExpr(fn, ir.ExprLocalVariable{Variable: 1})
	loadLive := appendExpr(fn, ir.ExprLoad{Pointer: lvLive})
	loadDead := appendExpr(fn, ir.ExprLoad{Pointer: lvDead})
	_ = loadDead // used in emit range but not by any root

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: valLive, End: valDead + 1}}},
		{Kind: ir.StmtStore{Pointer: lvLive, Value: valLive}},
		{Kind: ir.StmtStore{Pointer: lvDead, Value: valDead}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadLive, End: loadLive + 1}}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadDead, End: loadDead + 1}}},
		{Kind: ir.StmtReturn{Value: &loadLive}},
	}

	Run(mod, fn)

	// dead_var's store and emit should be removed.
	// live_var's store, emit, and the return must remain.
	// Count remaining statements.
	storeCount := 0
	emitCount := 0
	returnCount := 0
	for _, s := range fn.Body {
		switch s.Kind.(type) {
		case ir.StmtStore:
			storeCount++
		case ir.StmtEmit:
			emitCount++
		case ir.StmtReturn:
			returnCount++
		}
	}

	if storeCount != 1 {
		t.Errorf("expected 1 live store, got %d", storeCount)
	}
	if returnCount != 1 {
		t.Errorf("expected 1 return, got %d", returnCount)
	}
	// The dead_var's emit range should be removed; the live_var's kept.
	if emitCount < 1 {
		t.Errorf("expected at least 1 live emit, got %d", emitCount)
	}
}

// TestBarrierKeptEvenIfSurroundingCodeDead verifies that StmtBarrier
// is preserved even when there's no other live code.
func TestBarrierKeptEvenIfSurroundingCodeDead(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: i32},
		},
	}

	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
		{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Barrier and return must stay; dead store and its emit removed.
	hasBarrier := false
	hasReturn := false
	for _, s := range fn.Body {
		switch s.Kind.(type) {
		case ir.StmtBarrier:
			hasBarrier = true
		case ir.StmtReturn:
			hasReturn = true
		case ir.StmtStore:
			t.Error("dead store should have been eliminated")
		}
	}
	if !hasBarrier {
		t.Error("barrier should not be eliminated")
	}
	if !hasReturn {
		t.Error("return should not be eliminated")
	}
}

// TestNilInputs verifies that Run handles nil module/function gracefully.
func TestNilInputs(t *testing.T) {
	// Should not panic.
	Run(nil, nil)
	Run(&ir.Module{}, nil)
	Run(nil, &ir.Function{})
}

// TestNoLocals verifies that a function with no local variables
// passes through unchanged.
func TestNoLocals(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{
		Body: ir.Block{
			{Kind: ir.StmtReturn{}},
		},
	}
	Run(mod, fn)
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
}

// TestDeadLocalInsideIf verifies that dead locals inside control
// flow are eliminated AND the if statement itself is removed when
// both branches become empty (ADCE-style dead control flow elimination).
func TestDeadLocalInsideIf(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: i32},
		},
	}

	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: condVal + 1}}},
		{Kind: ir.StmtIf{
			Condition: condVal,
			Accept: ir.Block{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
				{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
			},
			Reject: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The dead store is removed, both branches become empty, and the
	// condition has no side effects. The entire if is eliminated.
	// Only the return statement remains.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].Kind.(ir.StmtReturn); !ok {
		t.Fatalf("expected StmtReturn, got %T", fn.Body[0].Kind)
	}
}

// TestCallWithSideEffectsIsLive verifies that StmtCall to a function
// with side effects is kept even if its result is unused.
func TestCallWithSideEffectsIsLive(t *testing.T) {
	mod := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "helper_with_effects",
				Body: ir.Block{
					{Kind: ir.StmtBarrier{}},
				},
			},
		},
	}

	fn := &ir.Function{}

	argVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: argVal, End: argVal + 1}}},
		{Kind: ir.StmtCall{
			Function:  0,
			Arguments: []ir.ExpressionHandle{argVal},
		}},
		{Kind: ir.StmtReturn{}},
	}

	bodyLenBefore := len(fn.Body)
	Run(mod, fn)

	if len(fn.Body) != bodyLenBefore {
		t.Fatalf("expected %d statements (impure call is live), got %d", bodyLenBefore, len(fn.Body))
	}
}

// TestPureCallWithUnusedResultIsDead verifies that a void-return call
// to a pure function (no side effects) is eliminated.
func TestPureCallWithUnusedResultIsDead(t *testing.T) {
	mod := &ir.Module{
		Functions: []ir.Function{
			{Name: "pure_helper"},
		},
	}

	fn := &ir.Function{}

	argVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: argVal, End: argVal + 1}}},
		{Kind: ir.StmtCall{
			Function:  0,
			Arguments: []ir.ExpressionHandle{argVal},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Pure void call should be eliminated, leaving only StmtReturn.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement (pure call eliminated), got %d", len(fn.Body))
	}
}

// TestWriteOnlyLocalNeverRead verifies that a local variable that
// is written to but never loaded is treated as dead.
func TestWriteOnlyLocalNeverRead(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "writeonly", Type: i32},
		},
	}

	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(99)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	// No ExprLoad for this variable.

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Store to write-only local should be eliminated.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
}

// TestDeadIfEliminated verifies that an if statement with empty
// accept+reject blocks and a dead condition is entirely removed
// (ADCE-style dead control flow elimination). This matches DXC's
// behavior for shaders like empty-if.wgsl where the if body is empty.
func TestDeadIfEliminated(t *testing.T) {
	mod := &ir.Module{}

	fn := &ir.Function{}
	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: condVal + 1}}},
		{Kind: ir.StmtIf{
			Condition: condVal,
			Accept:    ir.Block{},
			Reject:    ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// Both branches empty, condition has no side effects → if eliminated.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].Kind.(ir.StmtReturn); !ok {
		t.Fatalf("expected StmtReturn, got %T", fn.Body[0].Kind)
	}
}

// TestIfWithSideEffectsPreserved verifies that an if statement
// containing a side-effecting root (barrier) is preserved even when
// one branch is empty.
func TestIfWithSideEffectsPreserved(t *testing.T) {
	mod := &ir.Module{}

	fn := &ir.Function{}
	condVal := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: condVal, End: condVal + 1}}},
		{Kind: ir.StmtIf{
			Condition: condVal,
			Accept: ir.Block{
				{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
			},
			Reject: ir.Block{},
		}},
		{Kind: ir.StmtReturn{}},
	}

	bodyLenBefore := len(fn.Body)
	Run(mod, fn)

	// Barrier is a root — if must be preserved.
	if len(fn.Body) != bodyLenBefore {
		t.Fatalf("expected %d statements (if with barrier is live), got %d", bodyLenBefore, len(fn.Body))
	}
	foundIf := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtIf); ok {
			foundIf = true
		}
	}
	if !foundIf {
		t.Error("if with side-effecting body should be preserved")
	}
}

// TestDeadAccessIndexLocalStore verifies that stores through
// AccessIndex chains (struct field stores) to dead locals are
// correctly recognized and eliminated.
func TestDeadAccessIndexLocalStore(t *testing.T) {
	mod := &ir.Module{}
	f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
	// Struct type with 2 float fields.
	structTH := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "x", Type: f32, Offset: 0},
				{Name: "y", Type: f32, Offset: 4},
			},
			Span: 8,
		},
	})

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: structTH},
		},
	}

	// Expressions: literal, local var, access-index to field 0
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralF32(1.0)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	fieldHandle := appendExpr(fn, ir.ExprAccessIndex{Base: lvHandle, Index: 0})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		// Store to struct field via AccessIndex(LocalVariable, 0).
		{Kind: ir.StmtStore{Pointer: fieldHandle, Value: val}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// The field store targets a dead local — should be eliminated.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].Kind.(ir.StmtReturn); !ok {
		t.Fatalf("expected StmtReturn, got %T", fn.Body[0].Kind)
	}
}

// TestDeadSwitchEliminated verifies that a switch statement with all
// empty case bodies and a dead selector is eliminated.
func TestDeadSwitchEliminated(t *testing.T) {
	mod := &ir.Module{}

	fn := &ir.Function{}
	selVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(0)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: selVal, End: selVal + 1}}},
		{Kind: ir.StmtSwitch{
			Selector: selVal,
			Cases: []ir.SwitchCase{
				{Value: ir.SwitchValueI32(0), Body: ir.Block{}},
				{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
			},
		}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	// All case bodies empty, selector dead → switch eliminated.
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement after DCE (just return), got %d", len(fn.Body))
	}
}
