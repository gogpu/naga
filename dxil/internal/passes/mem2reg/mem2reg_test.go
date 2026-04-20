package mem2reg

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

// TestClassifyScalarSingleStore verifies that a scalar local variable
// with a single store and a single load passes classification.
//
// IR shape:
//
//	var x: i32          (LocalVars[0])
//	x = 42              (StmtStore)
//	let y = x           (StmtEmit{ExprLoad{LocalVar(0)}})
func TestClassifyScalarSingleStore(t *testing.T) {
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
	}

	uses := classifyLocals(mod, fn)
	if len(uses) != 1 {
		t.Fatalf("expected 1 use entry, got %d", len(uses))
	}
	if !uses[0].promotable {
		t.Fatalf("expected scalar local with load+store to be promotable")
	}
	if !uses[0].promotableType {
		t.Fatalf("expected promotable type detection")
	}
	if len(uses[0].loadHandles) != 1 || uses[0].loadHandles[0] != loadHandle {
		t.Fatalf("expected exactly one load handle %d, got %v", loadHandle, uses[0].loadHandles)
	}
	if uses[0].storeCount != 1 {
		t.Fatalf("expected one store, got %d", uses[0].storeCount)
	}
}

// TestClassifyVectorNotPromotable verifies that a vector local variable
// is NOT promotable by mem2reg (vector locals require emit-layer changes
// to handle ExprAlias chains for vector values).
func TestClassifyVectorNotPromotable(t *testing.T) {
	mod := &ir.Module{}
	f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
	vec3Handle := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	})
	_ = f32

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "v", Type: vec3Handle}},
	}
	uses := classifyLocals(mod, fn)
	if uses[0].promotableType {
		t.Fatalf("expected vector local to have promotableType=false")
	}
}

// TestClassifyStructNotPromotable verifies that a struct local is NOT
// promotable by mem2reg (requires SROA decomposition first).
func TestClassifyStructNotPromotable(t *testing.T) {
	mod := &ir.Module{}
	f32 := scalarTypeHandle(mod, ir.ScalarFloat, 4)
	structHandle := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.StructType{
			Members: []ir.StructMember{{Name: "x", Type: f32}},
			Span:    4,
		},
	})

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "s", Type: structHandle}},
	}
	uses := classifyLocals(mod, fn)
	if uses[0].promotableType {
		t.Fatalf("expected struct local to have promotableType=false")
	}
	if uses[0].promotable {
		t.Fatalf("expected struct local to be non-promotable")
	}
}

// TestClassifyAccessIndexDisqualifies verifies that a scalar local
// referenced via ExprAccessIndex (rather than ExprLoad) is rejected.
func TestClassifyAccessIndexDisqualifies(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	// Even though AccessIndex on a scalar is meaningless, the
	// classifier should reject ANY non-load reference.
	_ = appendExpr(fn, ir.ExprAccessIndex{Base: lvHandle, Index: 0})
	fn.Body = ir.Block{}

	uses := classifyLocals(mod, fn)
	if uses[0].promotable {
		t.Fatalf("expected AccessIndex use to disqualify promotion")
	}
}

// TestClassifyStmtAtomicDisqualifies verifies that using a scalar
// local as an atomic pointer disqualifies it.
func TestClassifyStmtAtomicDisqualifies(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	fn.Body = ir.Block{
		{Kind: ir.StmtAtomic{
			Pointer: lvHandle,
			Fun:     ir.AtomicAdd{},
			Value:   val,
		}},
	}

	uses := classifyLocals(mod, fn)
	if uses[0].promotable {
		t.Fatalf("expected StmtAtomic use to disqualify promotion")
	}
}

// TestPromoteSingleStoreSingleLoad verifies that the rewrite walks
// the body, replaces the load with ExprAlias{Source: stored value},
// and removes the store from the statement stream.
func TestPromoteSingleStoreSingleLoad(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	val := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(42)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	loadHandle := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})
	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val, End: val + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadHandle, End: loadHandle + 1}}},
	}

	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Load should now be ExprAlias{Source: val}.
	alias, ok := fn.Expressions[loadHandle].Kind.(ir.ExprAlias)
	if !ok {
		t.Fatalf("expected load handle %d to be ExprAlias, got %T", loadHandle, fn.Expressions[loadHandle].Kind)
	}
	if alias.Source != val {
		t.Fatalf("expected alias source = %d (val), got %d", val, alias.Source)
	}

	// StmtStore should have been removed; only the two StmtEmit
	// statements remain.
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements after promotion, got %d (%+v)", len(fn.Body), fn.Body)
	}
	for i, s := range fn.Body {
		if _, isStore := s.Kind.(ir.StmtStore); isStore {
			t.Fatalf("StmtStore still present at index %d", i)
		}
	}
}

// TestPromoteUsesInitWhenLoadPrecedesStore verifies that a load
// emitted BEFORE any store resolves to the variable's Init handle.
func TestPromoteUsesInitWhenLoadPrecedesStore(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	// Build the IR step by step so we control expression order.
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	initVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(7)})
	storeVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(99)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	earlyLoad := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})
	lateLoad := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})

	// Assign Init AFTER the LocalVariable is declared (Init is
	// evaluated at declaration time per WGSL semantics; mem2reg
	// honors it as the dominating value pre-store).
	fn.LocalVars[0].Init = &initVal

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: initVal, End: storeVal + 1}}},
		// Load BEFORE the store.
		{Kind: ir.StmtEmit{Range: ir.Range{Start: earlyLoad, End: earlyLoad + 1}}},
		// Store.
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: storeVal}},
		// Load AFTER the store.
		{Kind: ir.StmtEmit{Range: ir.Range{Start: lateLoad, End: lateLoad + 1}}},
	}

	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	earlyAlias, ok := fn.Expressions[earlyLoad].Kind.(ir.ExprAlias)
	if !ok {
		t.Fatalf("early load not rewritten to ExprAlias: %T", fn.Expressions[earlyLoad].Kind)
	}
	if earlyAlias.Source != initVal {
		t.Fatalf("early load should alias Init (%d), got %d", initVal, earlyAlias.Source)
	}

	lateAlias, ok := fn.Expressions[lateLoad].Kind.(ir.ExprAlias)
	if !ok {
		t.Fatalf("late load not rewritten to ExprAlias: %T", fn.Expressions[lateLoad].Kind)
	}
	if lateAlias.Source != storeVal {
		t.Fatalf("late load should alias storeVal (%d), got %d", storeVal, lateAlias.Source)
	}
}

// TestPromoteSynthesizesZeroValueWhenUninitialized verifies that a
// scalar var with no Init causes a synthesized ExprZeroValue to be
// appended and used as the initial value for pre-store loads.
func TestPromoteSynthesizesZeroValueWhenUninitialized(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}}, // no Init
	}
	storeVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(5)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	earlyLoad := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: storeVal, End: storeVal + 1}}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: earlyLoad, End: earlyLoad + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: storeVal}},
	}

	exprCountBefore := len(fn.Expressions)
	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// A new ExprZeroValue should have been appended.
	if len(fn.Expressions) <= exprCountBefore {
		t.Fatalf("expected a synthesized ExprZeroValue to be appended (had %d, now %d)",
			exprCountBefore, len(fn.Expressions))
	}
	// The early load should alias that new ExprZeroValue.
	earlyAlias, ok := fn.Expressions[earlyLoad].Kind.(ir.ExprAlias)
	if !ok {
		t.Fatalf("early load not rewritten: %T", fn.Expressions[earlyLoad].Kind)
	}
	zv, ok := fn.Expressions[earlyAlias.Source].Kind.(ir.ExprZeroValue)
	if !ok {
		t.Fatalf("alias source should be ExprZeroValue, got %T", fn.Expressions[earlyAlias.Source].Kind)
	}
	if zv.Type != i32 {
		t.Fatalf("synthesized ExprZeroValue type mismatch: got %d, want %d", zv.Type, i32)
	}
}

// TestPromoteMultipleStoresInOneBlock verifies that a variable with
// multiple stores in the SAME block is promoted: each load gets the
// most recent preceding store.
func TestPromoteMultipleStoresInOneBlock(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	val1 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	val2 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(2)})
	val3 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(3)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	load1 := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})
	load2 := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})
	load3 := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: val1, End: val3 + 1}}},
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val1}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: load1, End: load1 + 1}}}, // sees val1
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val2}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: load2, End: load2 + 1}}}, // sees val2
		{Kind: ir.StmtStore{Pointer: lvHandle, Value: val3}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: load3, End: load3 + 1}}}, // sees val3
	}

	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	checkAlias := func(loadH, want ir.ExpressionHandle) {
		t.Helper()
		alias, ok := fn.Expressions[loadH].Kind.(ir.ExprAlias)
		if !ok {
			t.Fatalf("load %d not rewritten: %T", loadH, fn.Expressions[loadH].Kind)
		}
		if alias.Source != want {
			t.Fatalf("load %d aliases %d, want %d", loadH, alias.Source, want)
		}
	}
	checkAlias(load1, val1)
	checkAlias(load2, val2)
	checkAlias(load3, val3)

	// All three stores should be removed; only the four StmtEmit
	// statements remain.
	if len(fn.Body) != 4 {
		t.Fatalf("expected 4 statements after promotion, got %d", len(fn.Body))
	}
}

// TestPhaseBIfMergePromotion verifies the if-merge phi placement:
// a variable stored on both branches of an if-statement is promoted
// by Phase B, the load after the if becomes ExprAlias pointing to
// an ExprPhi that merges the two stored values via PhiPredIfAccept
// / PhiPredIfReject incomings.
func TestPhaseBIfMergePromotion(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x", Type: i32}},
	}
	cond := appendExpr(fn, ir.Literal{Value: ir.LiteralBool(true)})
	val1 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	val2 := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(2)})
	lvHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	loadH := appendExpr(fn, ir.ExprLoad{Pointer: lvHandle})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: cond, End: val2 + 1}}},
		{Kind: ir.StmtIf{
			Condition: cond,
			Accept: ir.Block{
				{Kind: ir.StmtStore{Pointer: lvHandle, Value: val1}},
			},
			Reject: ir.Block{
				{Kind: ir.StmtStore{Pointer: lvHandle, Value: val2}},
			},
		}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadH, End: loadH + 1}}},
	}

	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	alias, isAlias := fn.Expressions[loadH].Kind.(ir.ExprAlias)
	if !isAlias {
		t.Fatalf("expected ExprAlias after Phase B, got %T", fn.Expressions[loadH].Kind)
	}
	phi, isPhi := fn.Expressions[alias.Source].Kind.(ir.ExprPhi)
	if !isPhi {
		t.Fatalf("expected ExprPhi at alias source %d, got %T", alias.Source, fn.Expressions[alias.Source].Kind)
	}
	if len(phi.Incoming) != 2 {
		t.Fatalf("expected 2 incomings, got %d", len(phi.Incoming))
	}
	hasAccept, hasReject := false, false
	for _, inc := range phi.Incoming {
		switch inc.PredKey {
		case ir.PhiPredIfAccept:
			hasAccept = true
			if inc.Value != val1 {
				t.Errorf("IfAccept incoming should be val1=%d, got %d", val1, inc.Value)
			}
		case ir.PhiPredIfReject:
			hasReject = true
			if inc.Value != val2 {
				t.Errorf("IfReject incoming should be val2=%d, got %d", val2, inc.Value)
			}
		}
	}
	if !hasAccept || !hasReject {
		t.Fatalf("phi missing predecessor: accept=%v reject=%v", hasAccept, hasReject)
	}
}

// TestRunEmptyFunction verifies that a function with no local
// variables is a no-op.
func TestRunEmptyFunction(t *testing.T) {
	mod := &ir.Module{}
	fn := &ir.Function{}
	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run on empty fn returned error: %v", err)
	}
}

// TestRunNilModule verifies safe handling of nil arguments.
func TestRunNilModule(t *testing.T) {
	if err := Run(nil, nil); err != nil {
		t.Fatalf("Run(nil, nil) returned error: %v", err)
	}
}

// TestPromoteInlineArgSpillPattern mimics the exact pattern the IR-level
// inline pass produces for scalar arguments: TWO ExprLocalVariable handles
// (same variable index, different expression handles) — one for the store
// pointer (prefix), one for the load pointer (body). The load is inside a
// StmtEmit range (added by inline step 8b). Verifies that mem2reg promotes
// the variable despite the dual-handle pattern.
func TestPromoteInlineArgSpillPattern(t *testing.T) {
	mod := &ir.Module{}
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "_inline_arg", Type: i32}},
	}

	// Caller's literal value (the constant arg passed to the inlined function).
	litVal := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})

	// Step 2 produces: ExprLocalVariable for the STORE pointer.
	storePtrHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	// Step 3 produces: ANOTHER ExprLocalVariable for the LOAD pointer
	// (different expression handle, same variable index).
	loadPtrHandle := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})

	// The load of the spilled arg value.
	loadHandle := appendExpr(fn, ir.ExprLoad{Pointer: loadPtrHandle})

	// Body: StmtStore (prefix), StmtEmit for load (step 8b), then
	// some other emit that references the loaded value indirectly.
	fn.Body = ir.Block{
		{Kind: ir.StmtStore{Pointer: storePtrHandle, Value: litVal}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: loadHandle, End: loadHandle + 1}}},
	}

	if err := Run(mod, fn); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// The load should now be ExprAlias{Source: litVal}.
	alias, ok := fn.Expressions[loadHandle].Kind.(ir.ExprAlias)
	if !ok {
		t.Fatalf("expected load handle %d to be ExprAlias after promotion, got %T",
			loadHandle, fn.Expressions[loadHandle].Kind)
	}
	if alias.Source != litVal {
		t.Fatalf("expected alias source = %d (literal), got %d", litVal, alias.Source)
	}

	// StmtStore should have been removed.
	for i, s := range fn.Body {
		if _, isStore := s.Kind.(ir.StmtStore); isStore {
			t.Fatalf("StmtStore still present at index %d after promotion", i)
		}
	}
}
