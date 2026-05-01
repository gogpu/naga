package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// emitFunctionBody emits the statements in the function body.
//
// This walks the naga IR statement tree and emits:
// - StmtEmit: evaluates expressions in the emit range
// - StmtReturn: stores outputs and returns
// - StmtStore: stores values through pointers (simplified)
// - StmtIf: conditional branching (not yet implemented)
// - StmtLoop: loop constructs (not yet implemented)
// - StmtBlock: nested statement blocks
//
// Reference: Mesa nir_to_dxil.c emit_module()
func (e *Emitter) emitFunctionBody(fn *ir.Function) error {
	// Pre-allocate all local variable allocas in the entry block.
	// LLVM requires allocas in the entry block for correct semantics
	// (allocas in loop bodies create new stack frames per iteration).
	if err := e.preAllocateLocalVars(fn); err != nil {
		return fmt.Errorf("local var allocation: %w", err)
	}
	return e.emitBlock(fn, fn.Body)
}

// preAllocateLocalVars creates allocas for all local variables in the function's
// entry block. This ensures local variables used inside loops have stable
// alloca pointers that persist across iterations.
//
// Variables that have been promoted to SSA values by the mem2reg pass
// (BUG-DXIL-039) no longer have any expression or statement reference
// in the function and are skipped here. Skipping the alloca avoids
// emitting dead-store stack allocations that DXC's output would have
// eliminated via LLVM's PromoteMemoryToRegister + DCE pipeline.
//
// Additionally, non-scalar locals with a constant Init that are never stored
// to in the body are registered as "init-only" — their loads resolve directly
// to the Init value without alloca. This matches DXC's SROA + constant
// propagation for the pattern `var x = const_expr; return x`.
func (e *Emitter) preAllocateLocalVars(fn *ir.Function) error {
	// Identify struct locals that serve exclusively as output staging.
	// Must run before alloca creation so promoted locals can be skipped.
	e.analyzeOutputPromotableLocals(fn)

	// Identify vector/scalar locals stored to exactly once in straight-line
	// code. These bypass alloca -- loads resolve to the stored value directly.
	singleStores := analyzeSingleStoreLocals(fn, e.ir)

	live := liveLocalVars(fn)
	stored := localsStoredTo(fn)
	for i := range fn.LocalVars {
		if !live[uint32(i)] {
			continue
		}
		// Output-promoted locals bypass the alloca entirely.
		// Their field stores record expression handles, and the return
		// emits storeOutput directly from those values.
		if e.outputPromotedLocals[uint32(i)] {
			continue
		}
		// Single-store vector/scalar local: skip alloca, record the stored
		// value so loads resolve directly to it.
		if valueH, ok := singleStores[uint32(i)]; ok {
			e.singleStoreLocals[uint32(i)] = valueH
			continue
		}
		// Init-only optimization: if the local has a non-nil Init and is
		// never stored to, register it so loads resolve to Init directly.
		lv := &fn.LocalVars[i]
		if lv.Init != nil && !stored[uint32(i)] {
			e.initOnlyLocals[uint32(i)] = *lv.Init
			continue
		}
		// Zero-store optimization: if the local has no Init (nil) and is
		// never stored to, it is zero-initialized by definition (WGSL spec).
		// For scalar/vector types, loads resolve to the zero constant of
		// the type, eliminating an unnecessary alloca+load chain. This
		// handles the SROA pattern where unassigned struct members become
		// separate locals that are read but never written.
		if lv.Init == nil && !stored[uint32(i)] {
			if int(lv.Type) < len(e.ir.Types) {
				switch e.ir.Types[lv.Type].Inner.(type) {
				case ir.ScalarType, ir.VectorType:
					e.zeroStoreLocals[uint32(i)] = lv.Type
					continue
				}
			}
		}
		lvExpr := ir.ExprLocalVariable{Variable: uint32(i)}
		if _, err := e.emitLocalVariable(fn, lvExpr); err != nil {
			return fmt.Errorf("local var %d (%s): %w", i, fn.LocalVars[i].Name, err)
		}
	}
	return nil
}

// localsStoredTo returns the set of local variable indices that have at
// least one StmtStore targeting them (directly or via AccessIndex/Access
// chains) in the function body. Locals NOT in this set are never written
// to after their Init (if any) — their value is immutable.
func localsStoredTo(fn *ir.Function) map[uint32]bool {
	// Build ExprLocalVariable handle -> var index map.
	lvHandles := make(map[ir.ExpressionHandle]uint32)
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			lvHandles[ir.ExpressionHandle(h)] = lv.Variable
		}
	}
	stored := make(map[uint32]bool)
	localsStoredToBlock(fn, fn.Body, lvHandles, stored)
	return stored
}

func localsStoredToBlock(fn *ir.Function, block ir.Block, lvHandles map[ir.ExpressionHandle]uint32, stored map[uint32]bool) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtStore:
			if v, ok := lvHandles[sk.Pointer]; ok {
				stored[v] = true
			} else if v, ok := resolveLocalVarEmit(fn, sk.Pointer); ok {
				stored[v] = true
			}
		case ir.StmtIf:
			localsStoredToBlock(fn, sk.Accept, lvHandles, stored)
			localsStoredToBlock(fn, sk.Reject, lvHandles, stored)
		case ir.StmtLoop:
			localsStoredToBlock(fn, sk.Body, lvHandles, stored)
			localsStoredToBlock(fn, sk.Continuing, lvHandles, stored)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				localsStoredToBlock(fn, sk.Cases[ci].Body, lvHandles, stored)
			}
		case ir.StmtBlock:
			localsStoredToBlock(fn, sk.Block, lvHandles, stored)
		}
	}
}

// analyzeSingleStoreLocals identifies vector/scalar local variables that
// are stored to exactly once in straight-line code (top-level function body).
// For such locals, the alloca/store/load chain can be bypassed: loads
// resolve directly to the stored value expression.
//
// This fills the gap left by mem2reg (which only handles scalar locals)
// for the common SROA decomposition pattern: after struct SROA, per-member
// vector locals are stored once and loaded once in the return's Compose.
// DXC's LLVM mem2reg handles both scalar and vector promotions; ours
// does not, so this targeted analysis handles the vector case.
//
// A local is eligible when ALL of:
//  1. It is scalar or vector typed (not struct/array/matrix)
//  2. It has exactly one whole-variable Store in the top-level body
//  3. It has no Init expression (Init locals are handled by initOnlyLocals)
//  4. No stores to it exist inside control flow (if/loop/switch)
//  5. No AccessIndex/Access chains reference it (only direct Load/Store)
func analyzeSingleStoreLocals(fn *ir.Function, mod *ir.Module) map[uint32]ir.ExpressionHandle {
	lvHandles := buildLocalVarHandleMap(fn)
	hasChainRef := findChainReferencedLocals(fn, lvHandles)
	storeCount, storeValue := countTopLevelStores(fn, lvHandles)
	nestedStored := findNestedStoredLocals(fn, lvHandles)

	result := make(map[uint32]ir.ExpressionHandle)
	for varIdx, count := range storeCount {
		if !isSingleStoreEligible(fn, mod, varIdx, count, nestedStored, hasChainRef) {
			continue
		}
		result[varIdx] = storeValue[varIdx]
	}
	return result
}

// buildLocalVarHandleMap maps ExprLocalVariable expression handles to
// their local variable indices.
func buildLocalVarHandleMap(fn *ir.Function) map[ir.ExpressionHandle]uint32 {
	lvHandles := make(map[ir.ExpressionHandle]uint32)
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			lvHandles[ir.ExpressionHandle(h)] = lv.Variable
		}
	}
	return lvHandles
}

// findChainReferencedLocals identifies locals referenced via AccessIndex
// or Access chains, which need pointer semantics and cannot be promoted.
func findChainReferencedLocals(fn *ir.Function, lvHandles map[ir.ExpressionHandle]uint32) map[uint32]bool {
	hasChainRef := make(map[uint32]bool)
	for _, expr := range fn.Expressions {
		switch k := expr.Kind.(type) {
		case ir.ExprAccessIndex:
			if v, ok := lvHandles[k.Base]; ok {
				hasChainRef[v] = true
			}
		case ir.ExprAccess:
			if v, ok := lvHandles[k.Base]; ok {
				hasChainRef[v] = true
			}
		}
	}
	return hasChainRef
}

// countTopLevelStores counts whole-variable stores per local in the
// top-level function body only (not inside control flow).
func countTopLevelStores(fn *ir.Function, lvHandles map[ir.ExpressionHandle]uint32) (map[uint32]int, map[uint32]ir.ExpressionHandle) {
	storeCount := make(map[uint32]int)
	storeValue := make(map[uint32]ir.ExpressionHandle)
	for _, stmt := range fn.Body {
		if sk, ok := stmt.Kind.(ir.StmtStore); ok {
			if v, ok := lvHandles[sk.Pointer]; ok {
				storeCount[v]++
				storeValue[v] = sk.Value
			}
		}
	}
	return storeCount, storeValue
}

// findNestedStoredLocals identifies locals that have stores inside
// control flow blocks (if/loop/switch), which cannot be promoted.
func findNestedStoredLocals(fn *ir.Function, lvHandles map[ir.ExpressionHandle]uint32) map[uint32]bool {
	nestedStored := make(map[uint32]bool)
	for _, stmt := range fn.Body {
		switch sk := stmt.Kind.(type) {
		case ir.StmtIf:
			markNestedStores(sk.Accept, lvHandles, nestedStored)
			markNestedStores(sk.Reject, lvHandles, nestedStored)
		case ir.StmtLoop:
			markNestedStores(sk.Body, lvHandles, nestedStored)
			markNestedStores(sk.Continuing, lvHandles, nestedStored)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				markNestedStores(sk.Cases[ci].Body, lvHandles, nestedStored)
			}
		case ir.StmtBlock:
			markNestedStores(sk.Block, lvHandles, nestedStored)
		}
	}
	return nestedStored
}

// isSingleStoreEligible checks if a local variable with the given store
// count is eligible for single-store promotion.
func isSingleStoreEligible(fn *ir.Function, mod *ir.Module, varIdx uint32, count int,
	nestedStored, hasChainRef map[uint32]bool) bool {
	if count != 1 || nestedStored[varIdx] || hasChainRef[varIdx] {
		return false
	}
	if int(varIdx) >= len(fn.LocalVars) {
		return false
	}
	lv := &fn.LocalVars[varIdx]
	if lv.Init != nil {
		return false // handled by initOnlyLocals
	}
	if int(lv.Type) >= len(mod.Types) {
		return false
	}
	inner := mod.Types[lv.Type].Inner
	switch inner.(type) {
	case ir.ScalarType, ir.VectorType:
		return true
	}
	return false
}

// markNestedStores marks local variables that have stores inside a block.
func markNestedStores(block ir.Block, lvHandles map[ir.ExpressionHandle]uint32, stored map[uint32]bool) {
	for _, stmt := range block {
		switch sk := stmt.Kind.(type) {
		case ir.StmtStore:
			if v, ok := lvHandles[sk.Pointer]; ok {
				stored[v] = true
			}
		case ir.StmtIf:
			markNestedStores(sk.Accept, lvHandles, stored)
			markNestedStores(sk.Reject, lvHandles, stored)
		case ir.StmtLoop:
			markNestedStores(sk.Body, lvHandles, stored)
			markNestedStores(sk.Continuing, lvHandles, stored)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				markNestedStores(sk.Cases[ci].Body, lvHandles, stored)
			}
		case ir.StmtBlock:
			markNestedStores(sk.Block, lvHandles, stored)
		}
	}
}

// analyzeOutputPromotableLocals identifies struct-typed local variables
// that can bypass the alloca/GEP/store/load chain and emit storeOutput
// directly. A local is promotable when ALL of the following hold:
//
//  1. The function has a struct result type (entry point returns a struct)
//  2. The local variable has the same type as the result
//  3. The return statement loads from exactly this local variable
//  4. All stores to the local are field-level (AccessIndex) in straight-line
//     code (not inside if/loop/switch) -- each member stored exactly once
//  5. The local is not read mid-body (only loaded in the return)
//
// DXC's LLVM SROA + mem2reg eliminates the alloca entirely for this pattern,
// producing direct storeOutput calls. We replicate that at the emitter level.
func (e *Emitter) analyzeOutputPromotableLocals(fn *ir.Function) {
	// Must be an entry point with a struct return type.
	if fn.Result == nil || e.emittingHelperFunction {
		return
	}
	resultType := e.ir.Types[fn.Result.Type]
	st, isSt := resultType.Inner.(ir.StructType)
	if !isSt {
		return
	}

	// Find the return statement and its load source in the top-level body.
	retVarIdx, retFound := findReturnLoadVar(fn)
	if !retFound {
		return
	}

	// Verify the local variable is a struct of the same type as the result.
	if int(retVarIdx) >= len(fn.LocalVars) {
		return
	}
	localVar := &fn.LocalVars[retVarIdx]
	if localVar.Type != fn.Result.Type {
		return
	}

	// Find the ExprLocalVariable handle for this variable.
	var lvHandle ir.ExpressionHandle
	foundLV := false
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok && lv.Variable == retVarIdx {
			lvHandle = ir.ExpressionHandle(h)
			foundLV = true
			break
		}
	}
	if !foundLV {
		return
	}

	// Verify all stores to this local are field-level AccessIndex stores
	// in straight-line code (top-level body only, not inside control flow).
	// Also verify no other reads happen (no Load of this local except in return).
	if !isOutputOnlyLocal(fn, lvHandle, retVarIdx, &st) {
		return
	}

	e.outputPromotedLocals[retVarIdx] = true
}

// findReturnLoadVar finds the local variable index loaded in the return
// statement of a function. Returns (varIdx, true) if the function body
// ends with StmtReturn whose value is an ExprLoad of an ExprLocalVariable.
func findReturnLoadVar(fn *ir.Function) (uint32, bool) {
	for i := len(fn.Body) - 1; i >= 0; i-- {
		ret, ok := fn.Body[i].Kind.(ir.StmtReturn)
		if !ok {
			continue
		}
		if ret.Value == nil {
			return 0, false
		}
		retExpr := fn.Expressions[*ret.Value]
		load, ok := retExpr.Kind.(ir.ExprLoad)
		if !ok {
			return 0, false
		}
		lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
		if !ok {
			return 0, false
		}
		return lv.Variable, true
	}
	return 0, false
}

// isOutputOnlyLocal checks that a struct local variable is only used for
// field-level stores in straight-line code (the top-level function body)
// and is never read except via the return-time Load. Returns false if any
// usage pattern would make output promotion unsafe:
//   - Store inside control flow (if/loop/switch)
//   - Whole-variable store (Store to the LocalVariable directly)
//   - Dynamic access (Access instead of AccessIndex)
//   - Load mid-body (any Load except the final return)
//   - Passed as a function call argument
func isOutputOnlyLocal(fn *ir.Function, _ ir.ExpressionHandle, varIdx uint32, st *ir.StructType) bool {
	aiHandles := collectFieldAccessHandles(fn, varIdx)
	memberStored := make(map[int]bool)

	for _, stmt := range fn.Body {
		switch sk := stmt.Kind.(type) {
		case ir.StmtStore:
			if ok := classifyOutputStore(fn, sk, varIdx, aiHandles, memberStored); !ok {
				return false
			}
		case ir.StmtReturn, ir.StmtEmit:
			continue
		case ir.StmtIf:
			if blockStoresTo(fn, sk.Accept, varIdx) || blockStoresTo(fn, sk.Reject, varIdx) {
				return false
			}
		case ir.StmtLoop:
			if blockStoresTo(fn, sk.Body, varIdx) || blockStoresTo(fn, sk.Continuing, varIdx) {
				return false
			}
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				if blockStoresTo(fn, sk.Cases[ci].Body, varIdx) {
					return false
				}
			}
		case ir.StmtBlock:
			if blockStoresTo(fn, sk.Block, varIdx) {
				return false
			}
		}
	}

	// Verify all members with bindings were stored to.
	for idx, member := range st.Members {
		if member.Binding != nil && !memberStored[idx] {
			return false
		}
	}
	return true
}

// collectFieldAccessHandles finds all ExprAccessIndex handles rooted at
// a local variable, returning a map from expression handle to member index.
func collectFieldAccessHandles(fn *ir.Function, varIdx uint32) map[ir.ExpressionHandle]int {
	aiHandles := make(map[ir.ExpressionHandle]int)
	for h, expr := range fn.Expressions {
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			if inner, ok2 := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable); ok2 {
				if inner.Variable == varIdx {
					aiHandles[ir.ExpressionHandle(h)] = int(ai.Index)
				}
			}
		}
	}
	return aiHandles
}

// classifyOutputStore checks a single store statement against the output-only
// local variable rules. Returns false if the store disqualifies the local.
func classifyOutputStore(fn *ir.Function, sk ir.StmtStore, varIdx uint32,
	aiHandles map[ir.ExpressionHandle]int, memberStored map[int]bool) bool {
	// Whole-variable store -> not promotable.
	if lv, ok := fn.Expressions[sk.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if lv.Variable == varIdx {
			return false
		}
	}
	// Field-level store via AccessIndex.
	if memberIdx, ok := aiHandles[sk.Pointer]; ok {
		if memberStored[memberIdx] {
			return false // stored more than once
		}
		memberStored[memberIdx] = true
		return true
	}
	// Two-level AccessIndex -> component-level store, bail.
	if ai, ok := fn.Expressions[sk.Pointer].Kind.(ir.ExprAccessIndex); ok {
		if _, ok2 := aiHandles[ai.Base]; ok2 {
			return false
		}
	}
	return true
}

// blockStoresTo checks if any statement in a block stores to the given
// local variable (directly or via AccessIndex/Access chains).
func blockStoresTo(fn *ir.Function, block ir.Block, varIdx uint32) bool {
	for _, stmt := range block {
		if stmtStoresTo(fn, stmt, varIdx) {
			return true
		}
	}
	return false
}

// stmtStoresTo checks a single statement and its children for stores
// to the given local variable.
func stmtStoresTo(fn *ir.Function, stmt ir.Statement, varIdx uint32) bool {
	switch sk := stmt.Kind.(type) {
	case ir.StmtStore:
		v, ok := resolveLocalVarEmit(fn, sk.Pointer)
		return ok && v == varIdx
	case ir.StmtIf:
		return blockStoresTo(fn, sk.Accept, varIdx) || blockStoresTo(fn, sk.Reject, varIdx)
	case ir.StmtLoop:
		return blockStoresTo(fn, sk.Body, varIdx) || blockStoresTo(fn, sk.Continuing, varIdx)
	case ir.StmtSwitch:
		for ci := range sk.Cases {
			if blockStoresTo(fn, sk.Cases[ci].Body, varIdx) {
				return true
			}
		}
	case ir.StmtBlock:
		return blockStoresTo(fn, sk.Block, varIdx)
	}
	return false
}

// resolveLocalVarEmit follows AccessIndex/Access chains to find the root
// local variable. Same logic as DCE's resolveLocalVar but local to the
// emit package to avoid import cycles.
func resolveLocalVarEmit(fn *ir.Function, h ir.ExpressionHandle) (uint32, bool) {
	const maxDepth = 16
	for depth := 0; depth < maxDepth; depth++ {
		if int(h) >= len(fn.Expressions) {
			return 0, false
		}
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprLocalVariable:
			return k.Variable, true
		case ir.ExprAccessIndex:
			h = k.Base
		case ir.ExprAccess:
			h = k.Base
		default:
			return 0, false
		}
	}
	return 0, false
}

// liveLocalVars returns the set of LocalVariable indices that still
// have at least one live use after the mem2reg pass.
//
// A use is "live" iff some expression OTHER than the ExprLocalVariable
// definition itself references the variable's pointer handle, OR some
// statement (StmtStore, StmtAtomic, StmtCall, ...) references it.
//
// Variables not in the returned set are dead-stack allocations that
// would otherwise generate orphan allocas with no readers — those are
// safe to skip in the bitcode output (DXC achieves the same effect via
// the post-mem2reg DCE pass).
func liveLocalVars(fn *ir.Function) map[uint32]bool {
	// Collect every ExpressionHandle that names a LocalVariable
	// pointer, plus the variable index it points at.
	lvHandle := make(map[ir.ExpressionHandle]uint32)
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			lvHandle[ir.ExpressionHandle(h)] = lv.Variable
		}
	}
	live := make(map[uint32]bool)
	// Mark live: only check expressions within StmtEmit ranges in
	// the (post-DCE) body. Checking ALL expressions would find dead
	// references whose emit ranges were removed by the IR-level DCE
	// pass, causing the emitter to generate orphan allocas.
	emitted := collectEmittedExprs(fn.Body)
	for h := range emitted {
		expr := fn.Expressions[h]
		if _, isLV := expr.Kind.(ir.ExprLocalVariable); isLV {
			continue
		}
		visitExpressionHandles(expr.Kind, func(eh ir.ExpressionHandle) {
			if v, ok := lvHandle[eh]; ok {
				live[v] = true
			}
		})
	}
	// Mark live: any statement that references one of the handles.
	visitStatementHandles(fn.Body, func(h ir.ExpressionHandle) {
		if v, ok := lvHandle[h]; ok {
			live[v] = true
		}
	})
	return live
}

// visitExpressionHandles invokes f for every ExpressionHandle field
// inside the given expression kind. Used by liveLocalVars to find
// references to LocalVariable pointer handles.
//
//nolint:gocyclo,cyclop,funlen // exhaustive expression-kind dispatch mirrors classify.go's referencedLocalVar
func visitExpressionHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle)) {
	switch k := kind.(type) {
	case ir.ExprLoad:
		f(k.Pointer)
	case ir.ExprAlias:
		f(k.Source)
	case ir.ExprAccess:
		f(k.Base)
		f(k.Index)
	case ir.ExprAccessIndex:
		f(k.Base)
	case ir.ExprBinary:
		f(k.Left)
		f(k.Right)
	case ir.ExprUnary:
		f(k.Expr)
	case ir.ExprSelect:
		f(k.Condition)
		f(k.Accept)
		f(k.Reject)
	case ir.ExprSplat:
		f(k.Value)
	case ir.ExprSwizzle:
		f(k.Vector)
	case ir.ExprCompose:
		for _, c := range k.Components {
			f(c)
		}
	case ir.ExprAs:
		f(k.Expr)
	case ir.ExprDerivative:
		f(k.Expr)
	case ir.ExprMath:
		f(k.Arg)
		if k.Arg1 != nil {
			f(*k.Arg1)
		}
		if k.Arg2 != nil {
			f(*k.Arg2)
		}
		if k.Arg3 != nil {
			f(*k.Arg3)
		}
	case ir.ExprRelational:
		f(k.Argument)
	case ir.ExprArrayLength:
		f(k.Array)
	case ir.ExprImageSample:
		f(k.Image)
		f(k.Sampler)
		f(k.Coordinate)
		if k.ArrayIndex != nil {
			f(*k.ArrayIndex)
		}
		if k.Offset != nil {
			f(*k.Offset)
		}
		if k.DepthRef != nil {
			f(*k.DepthRef)
		}
	case ir.ExprImageLoad:
		f(k.Image)
		f(k.Coordinate)
		if k.ArrayIndex != nil {
			f(*k.ArrayIndex)
		}
		if k.Sample != nil {
			f(*k.Sample)
		}
		if k.Level != nil {
			f(*k.Level)
		}
	case ir.ExprImageQuery:
		f(k.Image)
	}
}

// visitStatementHandles invokes f for every ExpressionHandle field
// inside any statement of the block (recursively into nested blocks).
func visitStatementHandles(blk ir.Block, f func(ir.ExpressionHandle)) {
	for i := range blk {
		visitStatementHandlesOne(blk[i].Kind, f)
	}
}

// visitStatementHandlesOne invokes f for every ExpressionHandle
// directly referenced by a single statement kind, and recurses into
// nested control-flow blocks via visitStatementHandles.
//
// Split out from visitStatementHandles purely to keep the parent
// dispatcher's cognitive-complexity score under the linter threshold;
// the two functions together cover the full statement-kind arena.
//
//nolint:gocyclo,cyclop // exhaustive statement-kind dispatch
func visitStatementHandlesOne(kind ir.StatementKind, f func(ir.ExpressionHandle)) {
	switch sk := kind.(type) {
	case ir.StmtStore:
		f(sk.Pointer)
		f(sk.Value)
	case ir.StmtAtomic:
		f(sk.Pointer)
		f(sk.Value)
		if sk.Result != nil {
			f(*sk.Result)
		}
	case ir.StmtImageStore:
		f(sk.Image)
		f(sk.Coordinate)
		f(sk.Value)
		if sk.ArrayIndex != nil {
			f(*sk.ArrayIndex)
		}
	case ir.StmtImageAtomic:
		f(sk.Image)
		f(sk.Coordinate)
		f(sk.Value)
		if sk.ArrayIndex != nil {
			f(*sk.ArrayIndex)
		}
	case ir.StmtCall:
		for _, a := range sk.Arguments {
			f(a)
		}
		if sk.Result != nil {
			f(*sk.Result)
		}
	case ir.StmtReturn:
		if sk.Value != nil {
			f(*sk.Value)
		}
	case ir.StmtIf:
		f(sk.Condition)
		visitStatementHandles(sk.Accept, f)
		visitStatementHandles(sk.Reject, f)
	case ir.StmtLoop:
		visitStatementHandles(sk.Body, f)
		visitStatementHandles(sk.Continuing, f)
		if sk.BreakIf != nil {
			f(*sk.BreakIf)
		}
	case ir.StmtSwitch:
		f(sk.Selector)
		for ci := range sk.Cases {
			visitStatementHandles(sk.Cases[ci].Body, f)
		}
	case ir.StmtBlock:
		visitStatementHandles(sk.Block, f)
	case ir.StmtWorkGroupUniformLoad:
		f(sk.Pointer)
		f(sk.Result)
	case ir.StmtRayQuery:
		f(sk.Query)
	case ir.StmtSubgroupBallot:
		f(sk.Result)
		if sk.Predicate != nil {
			f(*sk.Predicate)
		}
	case ir.StmtSubgroupCollectiveOperation:
		f(sk.Argument)
		f(sk.Result)
	case ir.StmtSubgroupGather:
		f(sk.Argument)
		f(sk.Result)
	case ir.StmtEmit:
		// StmtEmit ranges describe expression evaluation timing,
		// not data flow. Expression bodies are visited separately
		// by visitExpressionHandles — don't double-count the
		// range as a use.
	}
}

// emitBlock emits all statements in a block.
//
// When consecutive StmtEmit statements appear with no intervening
// non-emit statements, they are merged into a single combined emission.
// This allows the reassociate pass (buildChainSkipSet) and the
// evalRightFirst scheduling in emitBinaryOperands to see the full
// expression tree across what the WGSL lowerer split into separate
// emit ranges. DXC's LLVM sees all instructions in one basic block
// and reorders freely; merging emit ranges gives our emitter the same
// visibility.
//
// Expression handles that fall in gaps between the original ranges
// (e.g., handles emitted as side effects of other statements like
// StmtCall) are excluded via a membership set -- only handles that
// belong to at least one of the original ranges are emitted.
func (e *Emitter) emitBlock(fn *ir.Function, block ir.Block) error {
	i := 0
	for i < len(block) {
		// Check if this is the start of a consecutive StmtEmit run.
		firstEmit, isEmit := block[i].Kind.(ir.StmtEmit)
		if !isEmit {
			if err := e.emitStatement(fn, &block[i]); err != nil {
				return err
			}
			i++
			continue
		}

		// Check if the next statement is also StmtEmit.
		if i+1 >= len(block) {
			if err := e.emitStatement(fn, &block[i]); err != nil {
				return err
			}
			i++
			continue
		}
		if _, nextIsEmit := block[i+1].Kind.(ir.StmtEmit); !nextIsEmit {
			if err := e.emitStatement(fn, &block[i]); err != nil {
				return err
			}
			i++
			continue
		}

		// Found consecutive StmtEmit statements -- collect them all.
		ranges := []ir.Range{firstEmit.Range}
		j := i + 1
		for j < len(block) {
			nextEmit, nextOk := block[j].Kind.(ir.StmtEmit)
			if !nextOk {
				break
			}
			ranges = append(ranges, nextEmit.Range)
			j++
		}

		if err := e.emitMergedStmtEmit(fn, ranges); err != nil {
			return err
		}
		i = j
	}
	return nil
}

// mergeConsecutiveEmitRanges coalesces directly adjacent StmtEmit
// statements into a single StmtEmit covering the union of their
// expression handle ranges. Only StmtEmit statements that appear
// consecutively with no other statement kinds between them are merged.
//
// This is safe because within a flat statement list, consecutive
// StmtEmit ranges belong to the same basic block -- there is no
// intervening control flow. The merged range simply evaluates more
// expressions before proceeding, which does not change semantics for
// side-effect-free expressions.
//
// When no merging occurs, the original block is returned unmodified
// (zero allocation).
func mergeConsecutiveEmitRanges(block ir.Block) ir.Block {
	if len(block) < 2 {
		return block
	}

	// Quick scan: is there anything to merge?
	hasMerge := false
	for i := 1; i < len(block); i++ {
		_, prevIsEmit := block[i-1].Kind.(ir.StmtEmit)
		_, curIsEmit := block[i].Kind.(ir.StmtEmit)
		if prevIsEmit && curIsEmit {
			hasMerge = true
			break
		}
	}
	if !hasMerge {
		return block
	}

	result := make(ir.Block, 0, len(block))
	i := 0
	for i < len(block) {
		emit, ok := block[i].Kind.(ir.StmtEmit)
		if !ok {
			result = append(result, block[i])
			i++
			continue
		}

		// Start of a run of consecutive StmtEmit statements.
		// Merge all directly adjacent ones into a single range.
		merged := emit.Range
		j := i + 1
		for j < len(block) {
			nextEmit, nextOk := block[j].Kind.(ir.StmtEmit)
			if !nextOk {
				break
			}
			if nextEmit.Range.End > merged.End {
				merged.End = nextEmit.Range.End
			}
			if nextEmit.Range.Start < merged.Start {
				merged.Start = nextEmit.Range.Start
			}
			j++
		}

		result = append(result, ir.Statement{
			Kind: ir.StmtEmit{Range: merged},
		})
		i = j
	}
	return result
}

// emitStatement emits a single statement.
//
//nolint:gocyclo,cyclop // type-switch dispatch over all IR statement kinds
func (e *Emitter) emitStatement(fn *ir.Function, stmt *ir.Statement) error {
	switch sk := stmt.Kind.(type) {
	case ir.StmtEmit:
		return e.emitStmtEmit(fn, sk)

	case ir.StmtReturn:
		return e.emitStmtReturn(fn, sk)

	case ir.StmtStore:
		return e.emitStmtStore(fn, sk)

	case ir.StmtImageStore:
		return e.emitStmtImageStore(fn, sk)

	case ir.StmtBlock:
		return e.emitBlock(fn, sk.Block)

	case ir.StmtIf:
		return e.emitIfStatement(fn, sk)

	case ir.StmtLoop:
		return e.emitLoopStatement(fn, sk)

	case ir.StmtBreak:
		return e.emitBreak()

	case ir.StmtContinue:
		return e.emitContinue()

	case ir.StmtAtomic:
		return e.emitStmtAtomic(fn, sk)

	case ir.StmtBarrier:
		return e.emitStmtBarrier(sk)

	case ir.StmtWorkGroupUniformLoad:
		return e.emitStmtWorkGroupUniformLoad(fn, sk)

	case ir.StmtSwitch:
		return e.emitSwitchStatement(fn, sk)

	case ir.StmtKill:
		// Fragment shader discard — not yet implemented.
		return nil

	case ir.StmtCall:
		// Function calls.
		return e.emitStmtCall(fn, sk)

	case ir.StmtImageAtomic:
		return e.emitStmtImageAtomic(fn, sk)

	case ir.StmtRayQuery:
		return e.emitStmtRayQuery(fn, sk)

	case ir.StmtSubgroupBallot:
		return e.emitStmtSubgroupBallot(fn, sk)

	case ir.StmtSubgroupCollectiveOperation:
		return e.emitStmtSubgroupCollectiveOp(fn, sk)

	case ir.StmtSubgroupGather:
		return e.emitStmtSubgroupGather(fn, sk)

	default:
		return fmt.Errorf("unsupported statement kind: %T", sk)
	}
}

// emitStmtEmit evaluates expressions in the emit range.
// In naga IR, StmtEmit marks when expressions should be materialized.
func (e *Emitter) emitStmtEmit(fn *ir.Function, emit ir.StmtEmit) error {
	// Identify single-use Binary intermediates that will be flattened
	// by the Reassociate pass when their parent chain root is emitted.
	// Skip them here so their value IDs are not pre-materialized.
	skipChainIntermediate := e.buildChainSkipSet(fn, emit.Range)

	for h := emit.Range.Start; h < emit.Range.End; h++ {
		if skipChainIntermediate[h] {
			continue
		}
		if _, err := e.emitExpression(fn, h); err != nil {
			return fmt.Errorf("emit range [%d]: %w", h, err)
		}
	}
	return nil
}

// emitMergedStmtEmit evaluates expressions from multiple consecutive
// StmtEmit ranges as a single combined emission. This allows
// buildChainSkipSet to detect reassociation chains that span across
// what the WGSL lowerer split into separate emit ranges, and lets
// emitBinaryOperands evaluate resource reads before ALU when both
// appear within the combined range.
//
// Expression handles that fall in gaps between the original ranges
// are skipped -- only handles covered by at least one original range
// are emitted. This prevents emitting ExprCallResult or other handles
// that are materialized by side-effect statements (StmtCall etc.).
func (e *Emitter) emitMergedStmtEmit(fn *ir.Function, ranges []ir.Range) error {
	// Build a membership set of handles that belong to original ranges.
	inRange := make(map[ir.ExpressionHandle]bool)
	var minStart, maxEnd ir.ExpressionHandle
	minStart = ranges[0].Start
	maxEnd = ranges[0].End
	for _, r := range ranges {
		for h := r.Start; h < r.End; h++ {
			inRange[h] = true
		}
		if r.Start < minStart {
			minStart = r.Start
		}
		if r.End > maxEnd {
			maxEnd = r.End
		}
	}

	// Build the chain skip set over the full combined range so it can
	// detect chains that span across original range boundaries.
	combinedRange := ir.Range{Start: minStart, End: maxEnd}
	skipChainIntermediate := e.buildChainSkipSetFiltered(fn, combinedRange, inRange)

	// Emit handles in order, skipping gaps and chain intermediates.
	for h := minStart; h < maxEnd; h++ {
		if !inRange[h] {
			continue
		}
		if skipChainIntermediate[h] {
			continue
		}
		if _, err := e.emitExpression(fn, h); err != nil {
			return fmt.Errorf("emit range [%d]: %w", h, err)
		}
	}
	return nil
}

// buildChainSkipSetFiltered is like buildChainSkipSet but only considers
// handles that are in the provided membership set. This is used when
// merging multiple StmtEmit ranges to avoid inspecting expression
// handles in gaps between the original ranges.
func (e *Emitter) buildChainSkipSetFiltered(fn *ir.Function, rng ir.Range, inRange map[ir.ExpressionHandle]bool) map[ir.ExpressionHandle]bool {
	type binInfo struct {
		handle ir.ExpressionHandle
		op     ir.BinaryOperator
		left   ir.ExpressionHandle
		right  ir.ExpressionHandle
	}
	var bins []binInfo
	for h := rng.Start; h < rng.End; h++ {
		if !inRange[h] {
			continue
		}
		expr := fn.Expressions[h]
		if bin, ok := expr.Kind.(ir.ExprBinary); ok && isCommutativeBinOp(bin.Op) {
			bins = append(bins, binInfo{h, bin.Op, bin.Left, bin.Right})
		}
	}
	if len(bins) < 2 {
		return nil
	}

	skip := map[ir.ExpressionHandle]bool{}
	for _, b := range bins {
		for _, child := range [2]ir.ExpressionHandle{b.left, b.right} {
			if !inRange[child] {
				continue
			}
			childExpr := fn.Expressions[child]
			cbin, ok := childExpr.Kind.(ir.ExprBinary)
			if !ok || cbin.Op != b.op {
				continue
			}
			if e.exprUseCount[child] == 1 {
				skip[child] = true
			}
		}
	}
	return skip
}

// buildChainSkipSet identifies expression handles within the emit range
// that are single-use Binary intermediates of a commutative+associative
// chain. These should NOT be emitted individually — the Reassociate pass
// in emitBinary will flatten them into the chain root's emission.
//
// A handle is skipped when ALL of the following hold:
//   - It is an ExprBinary with a commutative+associative operator.
//   - It has exactly 1 expression-level use (exprUseCount == 1).
//   - Its sole consumer is another ExprBinary with the same operator
//     within the same emit range.
func (e *Emitter) buildChainSkipSet(fn *ir.Function, rng ir.Range) map[ir.ExpressionHandle]bool {
	// Quick scan: collect all Binary handles in the range.
	type binInfo struct {
		handle ir.ExpressionHandle
		op     ir.BinaryOperator
		left   ir.ExpressionHandle
		right  ir.ExpressionHandle
	}
	var bins []binInfo
	for h := rng.Start; h < rng.End; h++ {
		expr := fn.Expressions[h]
		if bin, ok := expr.Kind.(ir.ExprBinary); ok && isCommutativeBinOp(bin.Op) {
			bins = append(bins, binInfo{h, bin.Op, bin.Left, bin.Right})
		}
	}
	if len(bins) < 2 {
		return nil
	}

	// For each Binary in the range, check if its left or right child is
	// a same-op Binary that is single-use. If so, the child is an inner
	// chain member and should be skipped.
	skip := map[ir.ExpressionHandle]bool{}
	for _, b := range bins {
		for _, child := range [2]ir.ExpressionHandle{b.left, b.right} {
			if child < rng.Start || child >= rng.End {
				continue
			}
			childExpr := fn.Expressions[child]
			cbin, ok := childExpr.Kind.(ir.ExprBinary)
			if !ok || cbin.Op != b.op {
				continue
			}
			if e.exprUseCount[child] == 1 {
				skip[child] = true
			}
		}
	}
	return skip
}

// emitStmtReturn handles the return statement.
//
// For entry point functions with a result binding, the return value is
// stored via dx.op.storeOutput before the void return. This matches how
// DXIL entry points work: they are void functions that store outputs
// via intrinsics.
//
// For struct returns, the struct must be decomposed into per-member output stores.
// DXIL does not support struct-typed values — everything is scalarized.
func (e *Emitter) emitStmtReturn(fn *ir.Function, ret ir.StmtReturn) error {
	// Inline expansion: capture the return value into inline state instead of
	// emitting a ret instruction. The caller's emitStmtCallInline consumes it.
	if e.inInlineExpansion {
		return e.captureInlineReturn(fn, ret)
	}

	if ret.Value == nil || fn.Result == nil {
		// Void return for helper functions.
		if e.emittingHelperFunction {
			e.currentBB.AddInstruction(module.NewRetVoidInstr())
		}
		return nil
	}

	// Helper function: emit an LLVM ret instruction with the return value.
	if e.emittingHelperFunction {
		valueID, err := e.emitExpression(fn, *ret.Value)
		if err != nil {
			return fmt.Errorf("helper return value: %w", err)
		}

		// For vector returns (helperReturnComps > 1), pack components into a struct
		// using insertvalue instructions: {scalar, scalar, ...}.
		if e.helperReturnComps > 1 {
			retTy := e.mainFn.FuncType.RetType
			// Start with undef of the struct type.
			aggID := e.getTypedUndefConstID(retTy)
			for c := 0; c < e.helperReturnComps; c++ {
				compID := e.getComponentID(*ret.Value, c)
				insertID := e.allocValue()
				insertInstr := &module.Instruction{
					Kind:       module.InstrInsertVal,
					HasValue:   true,
					ResultType: retTy,
					Operands:   []int{aggID, compID, c},
					ValueID:    insertID,
				}
				e.currentBB.AddInstruction(insertInstr)
				aggID = insertID
			}
			valueID = aggID
		}

		instr := &module.Instruction{
			Kind:        module.InstrRet,
			HasValue:    false,
			ReturnValue: valueID,
		}
		e.currentBB.AddInstruction(instr)
		return nil
	}

	resultType := e.ir.Types[fn.Result.Type]

	// Handle struct return types specially to avoid loading entire structs.
	if st, ok := resultType.Inner.(ir.StructType); ok {
		return e.emitStructReturn(fn, ret, &st)
	}

	// Evaluate the return value expression.
	if _, err := e.emitExpression(fn, *ret.Value); err != nil {
		return fmt.Errorf("return value: %w", err)
	}

	// Store the return value as output(s). Non-struct return = single
	// output with signature-element ID 0 regardless of which @location or
	// @builtin the return binding declares: there's only one output, so
	// it's the first (and only) element in collectGraphicsSignatures.
	// The semantic NAME and KIND come from the binding, but the DXIL
	// storeOutput outputID refers to the sig element index, not the WGSL
	// location.
	scalar, ok := scalarOfType(resultType.Inner)
	if !ok {
		// Vector type — handle via component access.
		if vt, isVec := resultType.Inner.(ir.VectorType); isVec {
			ol := overloadForScalar(vt.Scalar)
			for comp := 0; comp < int(vt.Size); comp++ {
				compValueID := e.getComponentID(*ret.Value, comp)
				e.emitOutputStore(0, comp, compValueID, ol)
			}
			return nil
		}
		return fmt.Errorf("unsupported return type: %T", resultType.Inner)
	}

	numComps := componentCount(resultType.Inner)
	ol := overloadForScalar(scalar)

	for comp := 0; comp < numComps; comp++ {
		compValueID := e.getComponentID(*ret.Value, comp)
		e.emitOutputStore(0, comp, compValueID, ol)
	}

	return nil
}

// emitStructReturn handles returning a struct from an entry point.
// Each struct member with a binding becomes a separate output via storeOutput.
//
// The return value can be:
//   - ExprLoad of a local variable (struct alloca) — decompose via per-member GEP + load
//   - ExprCompose — sub-expressions are already available as flattened components
//   - Any other expression that produces per-component values via pendingComponents
//
// DXIL does not support struct-typed SSA values, so we must decompose the struct
// before emitting output stores.
func (e *Emitter) emitStructReturn(fn *ir.Function, ret ir.StmtReturn, st *ir.StructType) error {
	retExpr := fn.Expressions[*ret.Value]

	// Case 1: Load of a struct-typed local variable.
	// We must NOT emit a struct load. Instead, GEP + load each member individually.
	if load, ok := retExpr.Kind.(ir.ExprLoad); ok {
		return e.emitStructReturnFromLoad(fn, load, st)
	}

	// Case 2: Compose or other expression that produces flattened components.
	// Emit the expression — this populates pendingComponents via emitCompose.
	if _, err := e.emitExpression(fn, *ret.Value); err != nil {
		return fmt.Errorf("return value: %w", err)
	}

	// Walk members in graphics output order — locations first, builtins last.
	// Same convention as buildSignatures (dxil/dxil.go) so storeOutput sigID
	// stays in lockstep with the OSG1 register slot for each member.
	//
	// Compose flattens its sub-expressions in WGSL declaration order, so we
	// precompute the per-member flat scalar offset once and index into it from
	// the sorted iteration order.
	memberCompOffsets := make([]int, len(st.Members))
	cumComps := 0
	for i, member := range st.Members {
		memberCompOffsets[i] = cumComps
		cumComps += totalScalarCount(e.ir, e.ir.Types[member.Type].Inner)
	}

	order := backend.SortedMemberIndices(st.Members)
	sigID := 0
	for _, idx := range order {
		member := st.Members[idx]
		memberType := e.ir.Types[member.Type]
		scalar, ok := scalarOfType(memberType.Inner)
		numComps := componentCount(memberType.Inner)
		if !ok {
			if arr, isArr := memberType.Inner.(ir.ArrayType); isArr {
				elemInner := e.ir.Types[arr.Base].Inner
				scalar, ok = scalarOfType(elemInner)
				numComps = totalScalarCount(e.ir, memberType.Inner)
			}
		}
		if member.Binding == nil || !ok {
			continue
		}
		ol := overloadForScalar(scalar)
		baseComp := memberCompOffsets[idx]
		for comp := 0; comp < numComps; comp++ {
			valueID := e.getComponentID(*ret.Value, baseComp+comp)
			e.emitOutputStore(sigID, comp, valueID, ol)
		}
		sigID++
	}

	return nil
}

// emitStructReturnFromLoad handles returning a struct loaded from a local variable.
// Instead of emitting a single struct load (which DXIL doesn't support), we emit
// per-field GEP + load + storeOutput for each struct member.
//
// The struct is flattened in DXIL: vector members become multiple scalar fields.
// For a struct {vec4<f32>, vec4<f32>}, the DXIL type is {f32, f32, f32, f32, f32, f32, f32, f32}.
// So member 0 (vec4) uses flattened indices 0..3, member 1 uses indices 4..7.
func (e *Emitter) emitStructReturnFromLoad(fn *ir.Function, load ir.ExprLoad, st *ir.StructType) error {
	// Output-promoted path: skip alloca/GEP/load entirely and emit
	// storeOutput directly from the recorded value expressions.
	if lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if e.outputPromotedLocals[lv.Variable] {
			return e.emitPromotedStructReturn(fn, lv.Variable, st)
		}
	}

	// Ensure the pointer expression (local variable) is emitted.
	ptr, err := e.emitExpression(fn, load.Pointer)
	if err != nil {
		return fmt.Errorf("struct return pointer: %w", err)
	}

	// Get the LLVM struct type from the local variable.
	var structTy *module.Type
	if lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if cachedTy, hasCached := e.localVarStructTypes[lv.Variable]; hasCached {
			structTy = cachedTy
		}
	}
	if structTy == nil {
		var resolveErr error
		structTy, resolveErr = typeToDXILFull(e.mod, e.ir, st)
		if resolveErr != nil {
			return fmt.Errorf("struct return type resolution: %w", resolveErr)
		}
	}

	// Walk members in graphics output order (locations first, builtins last)
	// so storeOutput sigID lines up with the OSG1 register slot. Same
	// convention as buildSignatures and emitStructReturn.
	//
	// flatIdx walks the alloca which is laid out in WGSL declaration order
	// (the alloca is built from the original struct, not the sorted view).
	// Precompute per-member offset so sorted iteration indexes correctly.
	flatOffsets := make([]int, len(st.Members))
	cum := 0
	for i, member := range st.Members {
		flatOffsets[i] = cum
		mty := e.ir.Types[member.Type].Inner
		if _, isArr := mty.(ir.ArrayType); isArr {
			cum++ // arrays bump flat index by 1 (handled specially by emitArrayMemberOutput)
			continue
		}
		cum += componentCount(mty)
	}

	order := backend.SortedMemberIndices(st.Members)
	sigID := 0
	for _, idx := range order {
		member := st.Members[idx]
		_, err := e.emitStructMemberOutput(fn, structTy, ptr, member, flatOffsets[idx], sigID)
		if err != nil {
			return err
		}
		if member.Binding != nil {
			sigID++
		}
	}

	return nil
}

// emitPromotedStructReturn emits storeOutput calls for an output-promoted
// struct local variable. Instead of GEP+load from an alloca, this evaluates
// each member's stored value expression (recorded during statement emission)
// and emits storeOutput directly. Produces the same output as DXC's
// SROA+mem2reg pipeline for the "var out: S; out.x = a; return out;" pattern.
//
// Members are walked in graphics output order (locations first, builtins last)
// to match the signature element ID assignment in buildSignatures and
// collectGraphicsSignatures.
func (e *Emitter) emitPromotedStructReturn(fn *ir.Function, varIdx uint32, st *ir.StructType) error {
	order := backend.SortedMemberIndices(st.Members)
	sigID := 0
	for _, idx := range order {
		member := st.Members[idx]
		if member.Binding == nil {
			continue
		}

		// Look up the recorded value expression for this member.
		key := outputStoreKey{varIdx: varIdx, memberIdx: idx}
		valueHandle, ok := e.outputPromotedStores[key]
		if !ok {
			return fmt.Errorf("output-promoted struct: no stored value for member %d", idx)
		}

		// Evaluate the value expression.
		if _, err := e.emitExpression(fn, valueHandle); err != nil {
			return fmt.Errorf("output-promoted struct member %d value: %w", idx, err)
		}

		memberType := e.ir.Types[member.Type]
		scalar, ok := scalarOfType(memberType.Inner)
		numComps := componentCount(memberType.Inner)
		if !ok {
			if arr, isArr := memberType.Inner.(ir.ArrayType); isArr {
				elemInner := e.ir.Types[arr.Base].Inner
				scalar, ok = scalarOfType(elemInner)
				numComps = totalScalarCount(e.ir, memberType.Inner)
				_ = arr
			}
		}
		if !ok {
			return fmt.Errorf("output-promoted struct member %d: unsupported type", idx)
		}

		ol := overloadForScalar(scalar)
		for comp := 0; comp < numComps; comp++ {
			compID := e.getComponentID(valueHandle, comp)
			e.emitOutputStore(sigID, comp, compID, ol)
		}
		sigID++
	}
	return nil
}

// emitStructMemberOutput emits output stores for a single struct member during return.
// Returns the flat index delta for the member. sigID is the signature
// element ID assigned to this member — the caller's running counter of
// members with bindings (matches collectGraphicsSignatures ordering).
func (e *Emitter) emitStructMemberOutput(_ *ir.Function, structTy *module.Type, ptr int, member ir.StructMember, flatIdx, sigID int) (int, error) {
	memberType := e.ir.Types[member.Type]

	// Resolve scalar type and component count.
	scalar, ok := scalarOfType(memberType.Inner)
	numComps := componentCount(memberType.Inner)
	isArray := false
	var arrType ir.ArrayType

	if !ok {
		if arr, isArr := memberType.Inner.(ir.ArrayType); isArr {
			elemInner := e.ir.Types[arr.Base].Inner
			scalar, ok = scalarOfType(elemInner)
			if arr.Size.Constant != nil {
				numComps = int(*arr.Size.Constant)
			}
			isArray = true
			arrType = arr
		}
	}

	if member.Binding == nil || !ok {
		if isArray {
			return 1, nil
		}
		return numComps, nil
	}

	ol := overloadForScalar(scalar)
	outputID := sigID
	scalarTy := e.overloadReturnType(ol)
	scalarPtrTy := e.mod.GetPointerType(scalarTy)
	zeroVal := e.getIntConstID(0)

	if isArray {
		return e.emitArrayMemberOutput(structTy, ptr, arrType, flatIdx, numComps, outputID, ol, scalarTy, scalarPtrTy)
	}

	for comp := 0; comp < numComps; comp++ {
		fieldIdx := e.getIntConstID(int64(flatIdx + comp))
		gepID := e.addGEPInstr(structTy, scalarPtrTy, ptr, []int{zeroVal, fieldIdx})
		loadID := e.emitScalarLoad(scalarTy, gepID)
		e.emitOutputStore(outputID, comp, loadID, ol)
	}
	return numComps, nil
}

// emitArrayMemberOutput emits output stores for an array-typed struct member.
func (e *Emitter) emitArrayMemberOutput(
	structTy *module.Type, ptr int, arrType ir.ArrayType,
	flatIdx, numComps, outputID int, ol overloadType,
	scalarTy, scalarPtrTy *module.Type,
) (int, error) {
	zeroVal := e.getIntConstID(0)
	fieldIdx := e.getIntConstID(int64(flatIdx))

	arrDxilTy, err := typeToDXIL(e.mod, e.ir, arrType)
	if err != nil {
		return 0, fmt.Errorf("array member type: %w", err)
	}
	arrPtrTy := e.mod.GetPointerType(arrDxilTy)
	arrGepID := e.addGEPInstr(structTy, arrPtrTy, ptr, []int{zeroVal, fieldIdx})

	for comp := 0; comp < numComps; comp++ {
		elemIdx := e.getIntConstID(int64(comp))
		elemGepID := e.addGEPInstr(arrDxilTy, scalarPtrTy, arrGepID, []int{zeroVal, elemIdx})
		loadID := e.emitScalarLoad(scalarTy, elemGepID)
		e.emitOutputStore(outputID, comp, loadID, ol)
	}
	return 1, nil // Arrays are 1 field in the flattened struct
}

// emitScalarLoad emits an LLVM load instruction for a scalar value from a pointer.
func (e *Emitter) emitScalarLoad(scalarTy *module.Type, ptrID int) int {
	loadID := e.allocValue()
	align := e.alignForType(scalarTy)
	instr := &module.Instruction{
		Kind:       module.InstrLoad,
		HasValue:   true,
		ResultType: scalarTy,
		Operands:   []int{ptrID, scalarTy.ID, align, 0},
		ValueID:    loadID,
	}
	e.currentBB.AddInstruction(instr)
	return loadID
}

// tryOutputPromotedStore checks if a store targets a field of an
// output-promoted struct local. If so, records the value expression
// handle for later storeOutput emission and returns true (handled).
// Handles both aggregate fields (vec4, vec3) and scalar fields (f32, u32).
// Pattern: Store(AccessIndex(LocalVariable(V), memberIdx), value)
func (e *Emitter) tryOutputPromotedStore(fn *ir.Function, store ir.StmtStore) bool {
	ai, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false
	}
	lv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable)
	if !ok {
		return false
	}
	if !e.outputPromotedLocals[lv.Variable] {
		return false
	}
	e.outputPromotedStores[outputStoreKey{
		varIdx:    lv.Variable,
		memberIdx: int(ai.Index),
	}] = store.Value
	return true
}

// emitStmtStore handles store statements.
//
// In naga IR, stores write a value through a pointer expression.
// For UAV (storage buffers), this emits dx.op.bufferStore.
// For local variables, this emits an LLVM store instruction.
// For vector local variables, each component is stored separately.
//
// Reference: Mesa nir_to_dxil.c dxil_emit_store(), emit_bufferstore_call()
func (e *Emitter) emitStmtStore(fn *ir.Function, store ir.StmtStore) error {
	// Check if this store targets a mesh output variable.
	// Mesh output stores are converted to dx.op mesh shader intrinsics.
	if e.meshCtx != nil {
		if handled, err := e.tryEmitMeshOutputStore(fn, store); err != nil {
			return err
		} else if handled {
			return nil
		}
	}

	// Check if this store targets a UAV (storage buffer).
	// UAV stores use dx.op.bufferStore instead of LLVM store instructions.
	if chain, ok := e.resolveUAVPointerChain(fn, store.Pointer); ok {
		return e.emitUAVStore(fn, chain, store.Value)
	}

	// Output-promoted struct field store: record the value expression
	// for later direct storeOutput emission, skip alloca/GEP/store.
	if handled := e.tryOutputPromotedStore(fn, store); handled {
		return nil
	}

	// Single-store local: the value is already recorded in singleStoreLocals
	// by the analysis. Skip the store -- the load will resolve directly.
	if lv, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if _, isSingle := e.singleStoreLocals[lv.Variable]; isSingle {
			return nil
		}
	}

	// Check for whole-local-variable stores: vector/struct/array via dedicated
	// per-component / per-field / per-element decompose paths.
	if handled, err := e.tryEmitLocalVariableStore(fn, store); handled || err != nil {
		return err
	}

	// Handle AccessIndex chain to struct member's vector component:
	// Pattern: AccessIndex(AccessIndex(LocalVariable, fieldIdx), compIdx) → store to alloca
	if handled, hErr := e.tryStructMemberComponentStore(fn, store); hErr != nil {
		return hErr
	} else if handled {
		return nil
	}

	// Handle whole-aggregate store to a struct member of a flat-per-scalar local
	// alloca. Pattern: Store(AccessIndex(LocalVariable, fieldIdx), value) where
	// the field is a vector/matrix/struct/array (N scalar components). The local
	// alloca is laid out as flat scalars, so a single LLVM store of the aggregate
	// value would write only the first component (because the GEP for the field
	// resolves to a float* at the first scalar index). Decompose into N scalar
	// stores at sequential GEPs starting at the field's flat scalar offset.
	//
	// Triggered by WGSL like `out.position = vec4<f32>(...)` where `out` is a
	// `var` of struct type with vector/aggregate members. Without this path,
	// uninitialized stack memory leaks into SV_Position w/z and varyings,
	// producing chaotic vertex placement at runtime.
	if handled, hErr := e.tryStructMemberAggregateStore(fn, store); hErr != nil {
		return hErr
	} else if handled {
		return nil
	}

	ptr, err := e.emitExpression(fn, store.Pointer)
	if err != nil {
		return fmt.Errorf("store pointer: %w", err)
	}
	value, err := e.emitExpression(fn, store.Value)
	if err != nil {
		return fmt.Errorf("store value: %w", err)
	}

	// Resolve the stored value's type for alignment.
	storedTy, err := e.resolveLoadType(fn, store.Pointer)
	if err != nil {
		return fmt.Errorf("store type resolution: %w", err)
	}

	// BUG-DXIL-026 (remaining 4 tilecompute shaders): DXIL forbids aggregate
	// (struct/array) load/store instructions — the validator rejects them
	// with 0x80aa0009 "Explicit load/store type does not match pointee type
	// of pointer operand" since the explicit type operand must be a "first
	// class" scalar/vector type that matches the GEP-computed pointee.
	// Decompose aggregate stores into per-field / per-element scalar stores,
	// propagating the source pointer's addrspace so workgroup (addrspace 3)
	// stores stay in TGSM.
	if storedTy.Kind == module.TypeStruct {
		addrSpace := e.resolvePointerAddrSpace(fn, store.Pointer)
		return e.emitStructStoreDecomposed(ptr, storedTy, store.Value, addrSpace)
	}
	if storedTy.Kind == module.TypeArray && storedTy.ElemType != nil {
		addrSpace := e.resolvePointerAddrSpace(fn, store.Pointer)
		return e.emitArrayStoreDecomposed(ptr, storedTy, store.Value, addrSpace)
	}

	align := e.alignForType(storedTy)

	// Emit LLVM store instruction (no value produced).
	instr := &module.Instruction{
		Kind:        module.InstrStore,
		HasValue:    false,
		Operands:    []int{ptr, value, align, 0}, // ptr, value, align, isVolatile
		ReturnValue: -1,
	}
	e.currentBB.AddInstruction(instr)
	return nil
}

// emitStructStoreDecomposed fans a struct-typed store out into one scalar
// store per field, each through a per-field GEP that inherits the base
// pointer's addrspace. Callers must have already emitted the value
// expression — per-component IDs are sourced via getComponentID.
//
// TGSM origin handling: when basePtrID came from a single-step array GEP
// off a workgroup global (workgroupElemPtrs), each field GEP is re-rooted
// directly at the global with indices [0, elemIdx, fieldN]. Same reason
// as emitStructLoadAS — validator's TGSM-origin analysis rejects GEP-of-GEP.
func (e *Emitter) emitStructStoreDecomposed(basePtrID int, dxilStructTy *module.Type, valueHandle ir.ExpressionHandle, addrSpace uint8) error {
	if len(dxilStructTy.StructElems) == 0 {
		return nil
	}
	elemOrigin, hasElemOrigin := e.workgroupElemRoot(basePtrID)
	zeroID := e.getIntConstID(0)
	for i, elemTy := range dxilStructTy.StructElems {
		indexID := e.getIntConstID(int64(i))
		resultPtrTy := e.mod.GetPointerTypeAS(elemTy, addrSpace)
		var gepID int
		if hasElemOrigin {
			gepID = e.addGEPInstr(
				elemOrigin.arrayTy,
				resultPtrTy,
				elemOrigin.globalAllocaID,
				[]int{zeroID, elemOrigin.elemIndexID, indexID},
			)
		} else {
			gepID = e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID})
		}

		compID := e.getComponentID(valueHandle, i)
		align := e.alignForType(elemTy)
		instr := &module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{gepID, compID, align, 0},
			ReturnValue: -1,
		}
		e.currentBB.AddInstruction(instr)
	}
	return nil
}

// emitArrayStoreDecomposed fans an array-typed store out into one scalar
// store per element through per-element GEPs that inherit the base pointer's
// addrspace. Mirrors emitStructStoreDecomposed but uses ElemCount and the
// array's element type.
func (e *Emitter) emitArrayStoreDecomposed(basePtrID int, arrayTy *module.Type, valueHandle ir.ExpressionHandle, addrSpace uint8) error {
	elemTy := arrayTy.ElemType
	if elemTy == nil {
		return nil
	}
	numElems := int(arrayTy.ElemCount) //nolint:gosec // ElemCount bounded by shader array size
	if numElems == 0 {
		return nil
	}
	elemOrigin, hasElemOrigin := e.workgroupElemRoot(basePtrID)
	zeroID := e.getIntConstID(0)
	resultPtrTy := e.mod.GetPointerTypeAS(elemTy, addrSpace)
	align := e.alignForType(elemTy)
	for i := 0; i < numElems; i++ {
		indexID := e.getIntConstID(int64(i))
		var gepID int
		if hasElemOrigin {
			gepID = e.addGEPInstr(
				elemOrigin.arrayTy,
				resultPtrTy,
				elemOrigin.globalAllocaID,
				[]int{zeroID, elemOrigin.elemIndexID, indexID},
			)
		} else {
			gepID = e.addGEPInstr(arrayTy, resultPtrTy, basePtrID, []int{zeroID, indexID})
		}

		compID := e.getComponentID(valueHandle, i)
		instr := &module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{gepID, compID, align, 0},
			ReturnValue: -1,
		}
		e.currentBB.AddInstruction(instr)
	}
	return nil
}

// tryStructMemberComponentStore handles stores to a local struct variable's
// vector member component. Pattern:
//
//	store(AccessIndex(AccessIndex(LocalVariable(var), fieldIdx), compIdx), value)
//
// This pattern occurs when WGSL code does: output.vec_field.x = value;
// The struct alloca has per-component allocas for vector fields, so we need
// to resolve the chain to the specific component alloca and store there.
func (e *Emitter) tryStructMemberComponentStore(fn *ir.Function, store ir.StmtStore) (bool, error) {
	// Match: AccessIndex(base, compIdx) where base -> AccessIndex(LocalVariable, fieldIdx)
	outerAI, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false, nil
	}
	innerAI, ok := fn.Expressions[outerAI.Base].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false, nil
	}
	lv, ok := fn.Expressions[innerAI.Base].Kind.(ir.ExprLocalVariable)
	if !ok {
		return false, nil
	}

	// Resolve the struct type and field.
	if int(lv.Variable) >= len(fn.LocalVars) {
		return false, nil
	}
	localVar := &fn.LocalVars[lv.Variable]
	irType := e.ir.Types[localVar.Type]
	st, isSt := irType.Inner.(ir.StructType)
	if !isSt {
		return false, nil
	}
	fieldIdx := int(innerAI.Index)
	if fieldIdx >= len(st.Members) {
		return false, nil
	}
	member := st.Members[fieldIdx]
	memberType := e.ir.Types[member.Type]
	vt, isVec := memberType.Inner.(ir.VectorType)
	if !isVec {
		return false, nil
	}

	compIdx := int(outerAI.Index)
	if compIdx >= int(vt.Size) {
		return false, nil
	}

	// Ensure the struct alloca exists.
	if _, err := e.emitLocalVariable(fn, ir.ExprLocalVariable{Variable: lv.Variable}); err != nil {
		return false, fmt.Errorf("struct component store alloca: %w", err)
	}

	// Compute the flat component index within the struct's alloca.
	// Count scalar components of all preceding fields, then add compIdx.
	flatIdx := 0
	for fi := 0; fi < fieldIdx; fi++ {
		memType := e.ir.Types[st.Members[fi].Type]
		flatIdx += componentCount(memType.Inner)
	}
	flatIdx += compIdx

	// Look up the struct's per-component alloca pointers.
	dxilStructTy, hasTy := e.localVarStructTypes[lv.Variable]
	if !hasTy {
		return false, nil
	}

	// Ensure the local var alloca was created.
	allocaID, hasAlloca := e.localVarPtrs[lv.Variable]
	if !hasAlloca {
		return false, nil
	}

	// Emit GEP to the specific component.
	scalarTy, _ := typeToDXIL(e.mod, e.ir, vt.Scalar)
	ptrTy := e.mod.GetPointerType(scalarTy)
	i32Ty := e.mod.GetIntType(32)
	zeroID := e.getIntConstID(0)
	fieldIdxID := e.getIntConstID(int64(flatIdx))

	gepID := e.allocValue()
	gepInstr := &module.Instruction{
		Kind:       module.InstrGEP,
		HasValue:   true,
		ResultType: ptrTy,
		Operands:   []int{1, dxilStructTy.ID, allocaID, zeroID, fieldIdxID},
		ValueID:    gepID,
	}
	_ = i32Ty
	e.currentBB.AddInstruction(gepInstr)

	// Emit the value to store.
	valueID, err := e.emitExpression(fn, store.Value)
	if err != nil {
		return false, fmt.Errorf("struct component store value: %w", err)
	}

	// Store to the component pointer.
	storeInstr := &module.Instruction{
		Kind:        module.InstrStore,
		HasValue:    false,
		Operands:    []int{gepID, valueID, 4, 0},
		ReturnValue: -1,
	}
	e.currentBB.AddInstruction(storeInstr)
	return true, nil
}

// tryEmitLocalVariableStore dispatches whole-local-variable stores to the
// matching decompose path: vector via per-component allocas, struct via
// per-field GEP+store, array via per-element GEP+store. Returns (handled,
// err) — handled=true means the store was fully emitted (caller must return).
//
// Reference parity: matches DXC HLOperationLower's pattern of decomposing
// aggregate writes at the front-end before reaching the LLVM bitcode emitter,
// since DXIL forbids aggregate store instructions (validator 0x80aa0009).
// Mesa's NIR pre-pass does the equivalent flattening.
func (e *Emitter) tryEmitLocalVariableStore(fn *ir.Function, store ir.StmtStore) (bool, error) {
	lv, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return false, nil
	}

	if compPtrs, hasComps := e.localVarComponentPtrs[lv.Variable]; hasComps {
		localVar := &fn.LocalVars[lv.Variable]
		elemDXILTy, resolveErr := typeToDXIL(e.mod, e.ir, e.ir.Types[localVar.Type].Inner)
		if resolveErr != nil {
			return true, fmt.Errorf("vector store type resolution: %w", resolveErr)
		}
		return true, e.emitVectorStore(fn, compPtrs, store.Value, elemDXILTy)
	}

	if int(lv.Variable) < len(fn.LocalVars) {
		localVar := &fn.LocalVars[lv.Variable]
		irType := e.ir.Types[localVar.Type]
		if st, isSt := irType.Inner.(ir.StructType); isSt {
			return true, e.emitStructStore(fn, lv.Variable, irType, st, store.Value)
		}
	}

	if arrayTy, hasArr := e.localVarArrayTypes[lv.Variable]; hasArr {
		return true, e.emitArrayStore(fn, lv.Variable, arrayTy, store.Value)
	}

	return false, nil
}

// tryStructMemberAggregateStore handles whole-aggregate stores to a struct
// member of a flat-per-scalar local alloca:
//
//	store(AccessIndex(LocalVariable(out), fieldIdx), value)
//
// where the field type holds N scalar components (vector, matrix, struct, array).
// The local alloca is laid out by emitLocalVariable as one DXIL slot per scalar
// (vectors/aggregates are pre-flattened). The naive single-store would only
// write the first scalar because emitExpression on a Compose returns only the
// first component ID; the rest would land in pendingComponents and be lost.
//
// Decomposes into N scalar stores at GEPs [0, fieldFlatIdx + i] for i in 0..N-1.
// Mirrors storeStructFields layout but rooted at a single field rather than
// the whole struct.
//
// WGSL trigger example:
//
//	struct VertexOutput { @builtin(position) p: vec4<f32>, @location(0) c: vec3<f32> }
//	var out: VertexOutput;
//	out.p = vec4<f32>(x, y, 0.0, 1.0);  // 4 scalar stores at flat indices 0..3
//	out.c = some_vec3;                  // 3 scalar stores at flat indices 4..6
//
// Without this path the SV_Position w/z come from uninitialized stack memory →
// runtime perspective divide produces chaotic vertex placement (BUG-DXIL-032).
func (e *Emitter) tryStructMemberAggregateStore(fn *ir.Function, store ir.StmtStore) (bool, error) {
	ai, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false, nil
	}
	lv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable)
	if !ok {
		return false, nil
	}
	if int(lv.Variable) >= len(fn.LocalVars) {
		return false, nil
	}
	localVar := &fn.LocalVars[lv.Variable]
	st, isSt := e.ir.Types[localVar.Type].Inner.(ir.StructType)
	if !isSt || int(ai.Index) >= len(st.Members) {
		return false, nil
	}

	// Output-promoted local: record the store for later storeOutput emission
	// at return time, skip the alloca/GEP/store chain entirely.
	if e.outputPromotedLocals[lv.Variable] {
		e.outputPromotedStores[outputStoreKey{
			varIdx:    lv.Variable,
			memberIdx: int(ai.Index),
		}] = store.Value
		return true, nil
	}

	member := st.Members[ai.Index]
	memberInner := e.ir.Types[member.Type].Inner

	// Only fire when the field is an aggregate (vector/matrix/struct/array).
	// Scalar fields go through the generic single-store path and don't need
	// component decomposition.
	numScalars := totalScalarCount(e.ir, memberInner)
	if numScalars <= 1 {
		return false, nil
	}

	// Compute the flat scalar index of this field within the alloca layout.
	flatIdx := 0
	for fi := 0; fi < int(ai.Index); fi++ {
		flatIdx += totalScalarCount(e.ir, e.ir.Types[st.Members[fi].Type].Inner)
	}

	// Ensure the struct alloca + cached DXIL type are available.
	allocaID, err := e.emitLocalVariable(fn, ir.ExprLocalVariable{Variable: lv.Variable})
	if err != nil {
		return false, fmt.Errorf("struct member aggregate store alloca: %w", err)
	}
	dxilStructTy, hasTy := e.localVarStructTypes[lv.Variable]
	if !hasTy {
		return false, nil
	}

	// Materialize the value's component IDs (Compose etc. populates pendingComponents).
	if _, err := e.emitExpression(fn, store.Value); err != nil {
		return false, fmt.Errorf("struct member aggregate store value: %w", err)
	}

	zeroID := e.getIntConstID(0)
	for i := 0; i < numScalars; i++ {
		elemIdx := flatIdx + i
		if elemIdx >= len(dxilStructTy.StructElems) {
			break
		}
		memberDXILTy := dxilStructTy.StructElems[elemIdx]
		resultPtrTy := e.mod.GetPointerType(memberDXILTy)
		indexID := e.getIntConstID(int64(elemIdx))
		gepID := e.addGEPInstr(dxilStructTy, resultPtrTy, allocaID, []int{zeroID, indexID})

		compID := e.getComponentID(store.Value, i)
		align := e.alignForType(memberDXILTy)
		e.currentBB.AddInstruction(&module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{gepID, compID, align, 0},
			ReturnValue: -1,
		})
	}

	return true, nil
}

// emitArrayStore decomposes an array value into per-element stores using GEP.
// For each array element, emits GEP [N x T]*, i32 0, i32 idx → store.
func (e *Emitter) emitArrayStore(fn *ir.Function, varIdx uint32, arrayTy *module.Type, valueHandle ir.ExpressionHandle) error {
	allocaID, err := e.emitLocalVariable(fn, ir.ExprLocalVariable{Variable: varIdx})
	if err != nil {
		return fmt.Errorf("array store alloca: %w", err)
	}

	_, err = e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("array store value: %w", err)
	}

	elemTy := arrayTy.ElemType
	if elemTy == nil {
		return fmt.Errorf("array store: nil element type")
	}
	resultPtrTy := e.mod.GetPointerType(elemTy)
	zeroID := e.getIntConstID(0)
	align := e.alignForType(elemTy)

	numElems := int(arrayTy.ElemCount) //nolint:gosec // ElemCount bounded by shader array size
	for i := 0; i < numElems; i++ {
		indexID := e.getIntConstID(int64(i))
		gepID := e.addGEPInstr(arrayTy, resultPtrTy, allocaID, []int{zeroID, indexID})

		compID := e.getComponentID(valueHandle, i)
		instr := &module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{gepID, compID, align, 0},
			ReturnValue: -1,
		}
		e.currentBB.AddInstruction(instr)
	}
	return nil
}

// emitStructStore decomposes a struct value into per-scalar-field stores using GEP.
// Recursively flattens nested structs and vectors into individual scalar stores.
func (e *Emitter) emitStructStore(fn *ir.Function, varIdx uint32, _ ir.Type, st ir.StructType, valueHandle ir.ExpressionHandle) error {
	// Ensure the alloca exists.
	allocaID, err := e.emitLocalVariable(fn, ir.ExprLocalVariable{Variable: varIdx})
	if err != nil {
		return fmt.Errorf("struct store alloca: %w", err)
	}

	// Emit the value expression.
	_, err = e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("struct store value: %w", err)
	}

	// Use cached DXIL struct type to match the alloca's type.
	dxilStructTy, ok := e.localVarStructTypes[varIdx]
	if !ok {
		return fmt.Errorf("struct store: no cached DXIL type for local var %d", varIdx)
	}

	// Recursively store each scalar field, tracking flat component index.
	flatIdx := 0
	e.storeStructFields(dxilStructTy, allocaID, st, valueHandle, &flatIdx)
	return nil
}

// storeStructFields stores each scalar value from a flattened compose into
// struct fields via GEP. Since struct types are fully flattened (vectors
// become N scalars), each GEP targets a single scalar element.
// flatIdx tracks the position in both the value's component list and the
// DXIL struct element list.
func (e *Emitter) storeStructFields(dxilStructTy *module.Type, basePtrID int, st ir.StructType, valueHandle ir.ExpressionHandle, flatIdx *int) {
	zeroID := e.getIntConstID(0)

	for _, member := range st.Members {
		memberIRType := e.ir.Types[member.Type]
		numScalars := totalScalarCount(e.ir, memberIRType.Inner)

		// Store each scalar component via separate GEP + store.
		for j := 0; j < numScalars; j++ {
			elemIdx := *flatIdx + j
			if elemIdx >= len(dxilStructTy.StructElems) {
				break
			}
			memberDXILTy := dxilStructTy.StructElems[elemIdx]
			resultPtrTy := e.mod.GetPointerType(memberDXILTy)
			indexID := e.getIntConstID(int64(elemIdx))
			gepID := e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID})

			compID := e.getComponentID(valueHandle, elemIdx)
			align := e.alignForType(memberDXILTy)

			instr := &module.Instruction{
				Kind:        module.InstrStore,
				HasValue:    false,
				Operands:    []int{gepID, compID, align, 0},
				ReturnValue: -1,
			}
			e.currentBB.AddInstruction(instr)
		}

		*flatIdx += numScalars
	}
}

// emitVectorStore stores each component of a vector value to separate allocas.
// scalarDXILTy is the DXIL scalar element type of the vector (e.g. f16 for vec2<f16>).
func (e *Emitter) emitVectorStore(fn *ir.Function, compPtrs []int, valueHandle ir.ExpressionHandle, scalarDXILTy *module.Type) error {
	// Emit the value expression to get component tracking.
	_, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("store value: %w", err)
	}

	scalarTy := scalarDXILTy
	if comps, ok := e.exprComponents[valueHandle]; ok {
		// We have per-component tracking — use it for value IDs.
		for i := 0; i < len(compPtrs) && i < len(comps); i++ {
			align := e.alignForType(scalarTy)
			instr := &module.Instruction{
				Kind:        module.InstrStore,
				HasValue:    false,
				Operands:    []int{compPtrs[i], comps[i], align, 0},
				ReturnValue: -1,
			}
			e.currentBB.AddInstruction(instr)
		}
	} else {
		// Scalar value — store to first component only.
		valueID := e.exprValues[valueHandle]
		align := e.alignForType(scalarTy)
		instr := &module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{compPtrs[0], valueID, align, 0},
			ReturnValue: -1,
		}
		e.currentBB.AddInstruction(instr)
	}
	return nil
}

// emitStmtCall handles function call statements.
// If the helper function has been emitted, generates an LLVM call instruction.
// Otherwise, evaluates arguments (for side effects) and skips the call.
func (e *Emitter) emitStmtCall(fn *ir.Function, call ir.StmtCall) error {
	// Evaluate arguments (always needed for side effects).
	for _, arg := range call.Arguments {
		if _, err := e.emitExpression(fn, arg); err != nil {
			return fmt.Errorf("call argument: %w", err)
		}
	}

	// If helper function was emitted, generate an actual LLVM call.
	dxilFn, ok := e.helperFunctions[call.Function]
	if !ok {
		// Helper function was NOT emitted as a standalone LLVM function —
		// either the function uses DXIL-forbidden constructs (aggregate
		// args/returns, global access, complex locals) OR we deliberately
		// route it through inline expansion. Try to inline the callee body
		// into the caller's basic block, matching DXC's AlwaysInliner
		// semantics. See emitStmtCallInline for the full rationale.
		if int(call.Function) < len(e.ir.Functions) {
			callee := &e.ir.Functions[call.Function]
			if e.canInlineCallee(callee) {
				return e.emitStmtCallInline(fn, call, callee)
			}
		}
		// Final fallback: zero-valued result so the shader still compiles
		// (other code paths may still produce correct output).
		if call.Result != nil {
			zeroID := e.getZeroValueForResult(fn, call)
			e.callResultValues[*call.Result] = zeroID
			e.exprValues[*call.Result] = zeroID
		}
		return nil
	}

	// Build flattened argument list (expanding vectors to per-component).
	var argIDs []int
	for _, arg := range call.Arguments {
		argTy := e.resolveExprTypeInner(fn, arg)
		numComps := 1
		if argTy != nil {
			numComps = componentCount(argTy)
		}
		if numComps > 1 {
			for c := 0; c < numComps; c++ {
				argIDs = append(argIDs, e.getComponentID(arg, c))
			}
		} else {
			argIDs = append(argIDs, e.exprValues[arg])
		}
	}

	resultTy := dxilFn.FuncType.RetType
	valueID := e.addCallInstr(dxilFn, resultTy, argIDs)

	if call.Result != nil {
		// Check if the called function returns a vector (packed as struct).
		// If so, extract per-component values with extractvalue.
		calledFn := &e.ir.Functions[call.Function]
		if calledFn.Result != nil {
			retIRType := e.ir.Types[calledFn.Result.Type].Inner
			retComps := componentCount(retIRType)
			if retComps > 1 {
				scalarTy, _ := typeToDXIL(e.mod, e.ir, retIRType)
				comps := make([]int, retComps)
				for c := 0; c < retComps; c++ {
					extractID := e.allocValue()
					extractInstr := &module.Instruction{
						Kind:       module.InstrExtractVal,
						HasValue:   true,
						ResultType: scalarTy,
						Operands:   []int{valueID, c},
						ValueID:    extractID,
					}
					e.currentBB.AddInstruction(extractInstr)
					comps[c] = extractID
				}
				e.callResultValues[*call.Result] = comps[0]
				e.callResultComponents[*call.Result] = comps
				e.exprValues[*call.Result] = comps[0]
				e.exprComponents[*call.Result] = comps
				return nil
			}
		}

		e.callResultValues[*call.Result] = valueID
		e.exprValues[*call.Result] = valueID
	}
	return nil
}

// getZeroValueForResult returns a zero-valued constant for the given call's
// return type. Used as fallback when a helper function cannot be emitted.
//
//nolint:nestif // handles scalar, vector, and struct return types
func (e *Emitter) getZeroValueForResult(_ *ir.Function, call ir.StmtCall) int {
	// Look up the called function's return type.
	if int(call.Function) < len(e.ir.Functions) {
		calledFn := &e.ir.Functions[call.Function]
		if calledFn.Result != nil {
			resultType := e.ir.Types[calledFn.Result.Type].Inner

			// For struct returns, flatten all members into a component array.
			// This allows AccessIndex on the zero-fallback result to find the
			// correct sub-range of components for each struct field.
			if st, isSt := resultType.(ir.StructType); isSt {
				return e.getZeroValueForStruct(call, st)
			}

			numComps := componentCount(resultType)
			if numComps > 1 {
				// For vector/matrix returns, set up zero components.
				zeroID := e.getZeroForType(resultType)
				comps := make([]int, numComps)
				for i := range comps {
					comps[i] = zeroID
				}
				e.callResultComponents[*call.Result] = comps
				e.exprComponents[*call.Result] = comps
				return zeroID
			}
			return e.getZeroForType(resultType)
		}
	}
	return e.getIntConstID(0)
}

// getZeroValueForStruct creates a flattened zero component array for a struct return type.
// Each struct member gets cbvComponentCount(member) zero values, concatenated together.
// This uses cbvComponentCount (not componentCount) to match extractComponentsForAccessIndex
// which also uses cbvComponentCount for struct member offset computation.
func (e *Emitter) getZeroValueForStruct(call ir.StmtCall, st ir.StructType) int {
	var allComps []int
	for _, member := range st.Members {
		if int(member.Type) >= len(e.ir.Types) {
			continue
		}
		memberInner := e.ir.Types[member.Type].Inner
		zeroID := e.getZeroForType(memberInner)
		numComps := cbvComponentCount(e.ir, memberInner)
		for c := 0; c < numComps; c++ {
			allComps = append(allComps, zeroID)
		}
	}
	if len(allComps) == 0 {
		return e.getIntConstID(0)
	}
	e.callResultComponents[*call.Result] = allComps
	e.exprComponents[*call.Result] = allComps
	return allComps[0]
}

// getZeroForType returns a zero constant ID for the given type's scalar.
// Recurses into arrays and structs to find the base scalar type, ensuring
// the zero constant has the correct type (e.g., f32(0) for float arrays).
func (e *Emitter) getZeroForType(inner ir.TypeInner) int {
	scalar, ok := scalarOfType(inner)
	if ok {
		if scalar.Kind == ir.ScalarFloat {
			if scalar.Width != 4 {
				// Non-f32 floats (f16, f64) need their own typed constant.
				return e.addFloatConstID(e.mod.GetFloatType(uint(scalar.Width)*8), 0.0)
			}
			return e.getFloatConstID(0.0)
		}
		if scalar.Width == 8 {
			// i64/u64 needs a 64-bit zero constant.
			c := e.mod.AddIntConst(e.mod.GetIntType(64), 0)
			id := e.allocValue()
			e.constMap[id] = c
			return id
		}
		return e.getIntConstID(0)
	}
	// Recurse into array/struct to find the base scalar element type.
	switch t := inner.(type) {
	case ir.ArrayType:
		if int(t.Base) < len(e.ir.Types) {
			return e.getZeroForType(e.ir.Types[t.Base].Inner)
		}
	case ir.StructType:
		if len(t.Members) > 0 && int(t.Members[0].Type) < len(e.ir.Types) {
			return e.getZeroForType(e.ir.Types[t.Members[0].Type].Inner)
		}
	}
	return e.getIntConstID(0)
}

// emitStmtAtomic handles atomic operations on UAV (storage buffer) elements.
//
// For most atomic operations (add, and, or, xor, min, max, exchange), emits:
//
//	i32 @dx.op.atomicBinOp(i32 78, %handle, i32 atomicOp, i32 coord0, i32 coord1, i32 coord2, i32 value)
//
// For compare-and-exchange (AtomicExchange with Compare), emits:
//
//	i32 @dx.op.atomicCompareExchange(i32 79, %handle, i32 coord0, i32 coord1, i32 coord2, i32 cmpVal, i32 newVal)
//
// For AtomicLoad, emits dx.op.bufferLoad; for AtomicStore, emits dx.op.bufferStore.
//
// Reference: Mesa nir_to_dxil.c emit_atomic_binop() ~949, emit_atomic_cmpxchg() ~973
func (e *Emitter) emitStmtAtomic(fn *ir.Function, atomic ir.StmtAtomic) error {
	// Check if this is a workgroup (groupshared) atomic — uses LLVM atomicrmw.
	if e.isWorkgroupPointer(fn, atomic.Pointer) {
		return e.emitWorkgroupAtomic(fn, atomic)
	}

	// Resolve the pointer to a UAV handle + index.
	chain, ok := e.resolveUAVPointerChain(fn, atomic.Pointer)
	if !ok {
		return fmt.Errorf("atomic: pointer does not resolve to UAV")
	}

	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return fmt.Errorf("atomic: UAV handle not found for global variable %d", chain.varHandle)
	}

	// Resolve the buffer index (handles constant/dynamic/strided patterns).
	indexID, err := e.resolveUAVIndex(fn, chain)
	if err != nil {
		return fmt.Errorf("atomic index: %w", err)
	}

	ol := overloadForScalar(chain.scalar)

	switch af := atomic.Fun.(type) {
	case ir.AtomicLoad:
		return e.emitAtomicLoad(fn, atomic, handleID, indexID, chain)

	case ir.AtomicStore:
		return e.emitAtomicStore(fn, atomic, handleID, indexID, chain)

	case ir.AtomicExchange:
		if af.Compare != nil {
			return e.emitAtomicCmpXchg(fn, atomic, af, handleID, indexID, ol)
		}
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicExchange, handleID, indexID, ol)

	case ir.AtomicAdd:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicAdd, handleID, indexID, ol)

	case ir.AtomicSubtract:
		// DXIL has no subtract atomic — negate the value and use ADD.
		return e.emitAtomicSubtract(fn, atomic, handleID, indexID, ol)

	case ir.AtomicAnd:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicAnd, handleID, indexID, ol)

	case ir.AtomicInclusiveOr:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicOr, handleID, indexID, ol)

	case ir.AtomicExclusiveOr:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicXor, handleID, indexID, ol)

	case ir.AtomicMin:
		op := e.atomicMinMaxOp(chain.scalar, true)
		return e.emitAtomicBinOp(fn, atomic, op, handleID, indexID, ol)

	case ir.AtomicMax:
		op := e.atomicMinMaxOp(chain.scalar, false)
		return e.emitAtomicBinOp(fn, atomic, op, handleID, indexID, ol)

	default:
		return fmt.Errorf("unsupported atomic function: %T", atomic.Fun)
	}
}

// atomicMinMaxOp selects the correct DXIL atomic min/max opcode based on
// whether the scalar is signed or unsigned.
func (e *Emitter) atomicMinMaxOp(scalar ir.ScalarType, isMin bool) DXILAtomicOp {
	if scalar.Kind == ir.ScalarUint {
		if isMin {
			return DXILAtomicUMin
		}
		return DXILAtomicUMax
	}
	// Signed int (or default).
	if isMin {
		return DXILAtomicIMin
	}
	return DXILAtomicIMax
}

// emitAtomicBinOp emits a dx.op.atomicBinOp call with the appropriate type overload.
//
// Signature: T @dx.op.atomicBinOp.T(i32 78, %handle, i32 atomicOp,
//
//	i32 coord0, i32 coord1, i32 coord2, T value) → T
//
// where T is i32, i64, or f32 depending on the atomic's scalar type.
//
// Reference: Mesa nir_to_dxil.c emit_atomic_binop() ~949
func (e *Emitter) emitAtomicBinOp(fn *ir.Function, atomic ir.StmtAtomic, atomicOp DXILAtomicOp, handleID, indexID int, ol overloadType) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic value: %w", err)
	}

	atomicFn := e.getDxOpAtomicBinOpFuncTyped(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpAtomicBinOp))
	atomicOpVal := e.getIntConstID(int64(atomicOp))
	undefVal := e.getUndefConstID()

	// dx.op.atomicBinOp.T(i32 78, %handle, i32 atomicOp, i32 coord0, i32 coord1, i32 coord2, T value)
	resultID := e.addCallInstr(atomicFn, retTy, []int{
		opcodeVal, handleID, atomicOpVal,
		indexID, undefVal, undefVal, // coord0, coord1, coord2
		valueID,
	})

	// Store the result for the AtomicResult expression.
	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
	}

	return nil
}

// emitAtomicSubtract emits an atomic subtract by negating the value and using ADD.
// DXIL does not have a native subtract atomic operation.
func (e *Emitter) emitAtomicSubtract(fn *ir.Function, atomic ir.StmtAtomic, handleID, indexID int, ol overloadType) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic subtract value: %w", err)
	}

	retTy := e.overloadReturnType(ol)

	// Negate: 0 - value.
	var negatedID int
	switch ol {
	case overloadF32:
		zeroVal := e.getFloatConstID(0.0)
		negatedID = e.addBinOpInstr(retTy, BinOpFSub, zeroVal, valueID)
	case overloadF64, overloadF16:
		zeroVal := e.addFloatConstID(retTy, 0.0)
		negatedID = e.addBinOpInstr(retTy, BinOpFSub, zeroVal, valueID)
	case overloadI64:
		c := e.mod.AddIntConst(e.mod.GetIntType(64), 0)
		id := e.allocValue()
		e.constMap[id] = c
		negatedID = e.addBinOpInstr(retTy, BinOpSub, id, valueID)
	default:
		zeroVal := e.getIntConstID(0)
		negatedID = e.addBinOpInstr(retTy, BinOpSub, zeroVal, valueID)
	}

	atomicFn := e.getDxOpAtomicBinOpFuncTyped(ol)
	opcodeVal := e.getIntConstID(int64(OpAtomicBinOp))
	atomicOpVal := e.getIntConstID(int64(DXILAtomicAdd))
	undefVal := e.getUndefConstID()

	resultID := e.addCallInstr(atomicFn, retTy, []int{
		opcodeVal, handleID, atomicOpVal,
		indexID, undefVal, undefVal,
		negatedID,
	})

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
	}

	return nil
}

// emitAtomicCmpXchg emits a dx.op.atomicCompareExchange call with the appropriate type overload.
//
// Signature: T @dx.op.atomicCompareExchange.T(i32 79, %handle,
//
//	i32 coord0, i32 coord1, i32 coord2, T cmpVal, T newVal) → T
//
// Reference: Mesa nir_to_dxil.c emit_atomic_cmpxchg() ~973
func (e *Emitter) emitAtomicCmpXchg(fn *ir.Function, atomic ir.StmtAtomic, exchange ir.AtomicExchange, handleID, indexID int, ol overloadType) error {
	newValID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic cmpxchg new value: %w", err)
	}

	cmpValID, err := e.emitExpression(fn, *exchange.Compare)
	if err != nil {
		return fmt.Errorf("atomic cmpxchg compare value: %w", err)
	}

	cmpXchgFn := e.getDxOpAtomicCmpXchgFuncTyped(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpAtomicCmpXchg))
	undefVal := e.getUndefConstID()

	// dx.op.atomicCompareExchange.T(i32 79, %handle, i32 coord0, coord1, coord2, T cmpVal, T newVal)
	resultID := e.addCallInstr(cmpXchgFn, retTy, []int{
		opcodeVal, handleID,
		indexID, undefVal, undefVal, // coord0, coord1, coord2
		cmpValID, newValID,
	})

	if atomic.Result != nil {
		// atomicCompareExchangeWeak returns a struct {old_value, exchanged}.
		// Component 0 = old_value (the T result of dx.op.atomicCompareExchange).
		// Component 1 = exchanged (bool: old_value == cmpVal).
		var exchangedID int
		if ol == overloadF32 || ol == overloadF64 {
			exchangedID = e.addCmpInstr(FCmpOEQ, resultID, cmpValID)
		} else {
			exchangedID = e.addCmpInstr(ICmpEQ, resultID, cmpValID)
		}

		e.exprValues[*atomic.Result] = resultID
		e.exprComponents[*atomic.Result] = []int{resultID, exchangedID}
	}

	return nil
}

// emitAtomicLoad handles AtomicLoad by emitting a dx.op.bufferLoad.
// Atomic loads on UAV buffers use the same bufferLoad intrinsic as regular loads.
func (e *Emitter) emitAtomicLoad(_ *ir.Function, atomic ir.StmtAtomic, handleID, indexID int, chain *uavPointerChain) error {
	ol := overloadForScalar(chain.scalar)
	resRetTy := e.getDxResRetType(ol)
	bufLoadFn := e.getDxOpBufferLoadFunc(ol)

	opcodeVal := e.getIntConstID(int64(OpBufferLoad))
	undefVal := e.getUndefConstID()

	retID := e.addCallInstr(bufLoadFn, resRetTy, []int{opcodeVal, handleID, indexID, undefVal})

	// Extract the first component as the atomic result (scalar).
	scalarTy := e.overloadReturnType(ol)
	extractID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrExtractVal,
		HasValue:   true,
		ResultType: scalarTy,
		Operands:   []int{retID, 0},
		ValueID:    extractID,
	}
	e.currentBB.AddInstruction(instr)

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = extractID
	}

	return nil
}

// emitAtomicStore handles AtomicStore by emitting a dx.op.bufferStore.
// Atomic stores on UAV buffers use the same bufferStore intrinsic as regular stores.
func (e *Emitter) emitAtomicStore(fn *ir.Function, atomic ir.StmtAtomic, handleID, indexID int, chain *uavPointerChain) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic store value: %w", err)
	}

	ol := overloadForScalar(chain.scalar)
	bufStoreFn := e.getDxOpBufferStoreFunc(ol)

	opcodeVal := e.getIntConstID(int64(OpBufferStore))
	undefVal := e.getUndefConstID()
	maskVal := e.getI8ConstID(1) // single component write mask

	// dx.op.bufferStore(i32 69, %handle, i32 index, i32 undef, val, undef, undef, undef, i8 mask)
	e.addCallInstr(bufStoreFn, e.mod.GetVoidType(), []int{
		opcodeVal, handleID, indexID, undefVal,
		valueID, undefVal, undefVal, undefVal,
		maskVal,
	})

	return nil
}

// isWorkgroupPointer returns true if the given pointer expression refers to a
// workgroup (groupshared) global variable, either directly or through access chains.
func (e *Emitter) isWorkgroupPointer(fn *ir.Function, ptrHandle ir.ExpressionHandle) bool {
	if int(ptrHandle) >= len(fn.Expressions) {
		return false
	}
	expr := fn.Expressions[ptrHandle]
	switch ek := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(ek.Variable) < len(e.ir.GlobalVariables) {
			return e.ir.GlobalVariables[ek.Variable].Space == ir.SpaceWorkGroup
		}
	case ir.ExprAccessIndex:
		return e.isWorkgroupPointer(fn, ek.Base)
	case ir.ExprAccess:
		return e.isWorkgroupPointer(fn, ek.Base)
	}
	return false
}

// resolveWorkgroupPointer resolves a pointer to a workgroup variable to its alloca ID.
// Handles direct GlobalVariable, AccessIndex(field, GV), and Access(index, GV).
func (e *Emitter) resolveWorkgroupPointer(fn *ir.Function, ptrHandle ir.ExpressionHandle) (int, error) {
	// Just emit the expression — ExprGlobalVariable triggers emitGlobalVarAlloca,
	// and AccessIndex/Access on globals emit GEP instructions from the alloca.
	return e.emitExpression(fn, ptrHandle)
}

// emitWorkgroupAtomic emits LLVM atomicrmw/cmpxchg instructions for workgroup
// (groupshared) variable atomic operations. Unlike UAV atomics (which use
// dx.op.atomicBinOp intrinsics), workgroup atomics use native LLVM atomic
// instructions operating on alloca pointers.
func (e *Emitter) emitWorkgroupAtomic(fn *ir.Function, atomic ir.StmtAtomic) error {
	ptrID, err := e.resolveWorkgroupPointer(fn, atomic.Pointer)
	if err != nil {
		return fmt.Errorf("workgroup atomic pointer: %w", err)
	}

	// Resolve the actual scalar width of the atomic (i32, i64, etc.).
	atomicScalar := e.resolveAtomicScalar(fn, atomic.Pointer)
	bitWidth := uint(atomicScalar.Width) * 8
	atomicTy := e.mod.GetIntType(bitWidth)
	align := 3 // log2(4)+1 for i32
	if bitWidth == 64 {
		align = 4 // log2(8)+1 for i64
	}

	switch af := atomic.Fun.(type) {
	case ir.AtomicLoad:
		// atomicLoad on workgroup = plain load (DXIL doesn't need special atomic load for groupshared)
		loadID := e.allocValue()
		loadInstr := &module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: atomicTy,
			Operands:   []int{ptrID, atomicTy.ID, align, 0},
			ValueID:    loadID,
		}
		e.currentBB.AddInstruction(loadInstr)
		if atomic.Result != nil {
			e.exprValues[*atomic.Result] = loadID
		}
		return nil

	case ir.AtomicStore:
		// atomicStore on workgroup = plain store
		valueID, err2 := e.emitExpression(fn, atomic.Value)
		if err2 != nil {
			return fmt.Errorf("workgroup atomic store value: %w", err2)
		}
		storeInstr := &module.Instruction{
			Kind:     module.InstrStore,
			HasValue: false,
			Operands: []int{ptrID, valueID, align, 0},
		}
		e.currentBB.AddInstruction(storeInstr)
		return nil

	case ir.AtomicAdd:
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWAdd, ptrID)

	case ir.AtomicSubtract:
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWSub, ptrID)

	case ir.AtomicAnd:
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWAnd, ptrID)

	case ir.AtomicInclusiveOr:
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWOr, ptrID)

	case ir.AtomicExclusiveOr:
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWXor, ptrID)

	case ir.AtomicMin:
		op := AtomicRMWMin
		if e.isUnsignedAtomicPointer(fn, atomic.Pointer) {
			op = AtomicRMWUMin
		}
		return e.emitWorkgroupAtomicRMW(fn, atomic, op, ptrID)

	case ir.AtomicMax:
		op := AtomicRMWMax
		if e.isUnsignedAtomicPointer(fn, atomic.Pointer) {
			op = AtomicRMWUMax
		}
		return e.emitWorkgroupAtomicRMW(fn, atomic, op, ptrID)

	case ir.AtomicExchange:
		if af.Compare != nil {
			return e.emitWorkgroupCmpXchg(fn, atomic, af, ptrID)
		}
		return e.emitWorkgroupAtomicRMW(fn, atomic, AtomicRMWXchg, ptrID)

	default:
		return fmt.Errorf("unsupported workgroup atomic function: %T", atomic.Fun)
	}
}

// emitWorkgroupAtomicRMW emits an LLVM atomicrmw instruction.
// The type is resolved from the atomic pointer's scalar (i32, i64, etc.).
func (e *Emitter) emitWorkgroupAtomicRMW(fn *ir.Function, atomic ir.StmtAtomic, op AtomicRMWOp, ptrID int) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("workgroup atomic value: %w", err)
	}

	atomicScalar := e.resolveAtomicScalar(fn, atomic.Pointer)
	bitWidth := uint(atomicScalar.Width) * 8
	atomicTy := e.mod.GetIntType(bitWidth)

	resultID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrAtomicRMW,
		HasValue:   true,
		ResultType: atomicTy,
		Operands:   []int{ptrID, valueID, int(op), 0, int(AtomicOrderingSeqCst), 1},
		ValueID:    resultID,
	}
	e.currentBB.AddInstruction(instr)

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
	}
	return nil
}

// emitWorkgroupCmpXchg emits an LLVM cmpxchg instruction for workgroup variables.
func (e *Emitter) emitWorkgroupCmpXchg(fn *ir.Function, atomic ir.StmtAtomic, af ir.AtomicExchange, ptrID int) error {
	cmpID, err := e.emitExpression(fn, *af.Compare)
	if err != nil {
		return fmt.Errorf("workgroup cmpxchg compare: %w", err)
	}
	newID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("workgroup cmpxchg new value: %w", err)
	}

	atomicScalar := e.resolveAtomicScalar(fn, atomic.Pointer)
	bitWidth := uint(atomicScalar.Width) * 8
	atomicTy := e.mod.GetIntType(bitWidth)

	resultID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrCmpXchg,
		HasValue:   true,
		ResultType: atomicTy,
		Operands:   []int{ptrID, cmpID, newID, 0, int(AtomicOrderingSeqCst), 1},
		ValueID:    resultID,
	}
	e.currentBB.AddInstruction(instr)

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
	}
	return nil
}

// resolveAtomicScalar walks the expression/type chain from a pointer to an atomic
// and returns the scalar type of the atomic. This handles direct GlobalVariable,
// AccessIndex into structs/arrays, and Access into arrays.
func (e *Emitter) resolveAtomicScalar(fn *ir.Function, ptrHandle ir.ExpressionHandle) ir.ScalarType {
	// Try ExpressionTypes first — the pointer type contains the base type.
	if baseInner := e.resolveAtomicBaseType(fn, ptrHandle); baseInner != nil {
		if at, ok := baseInner.(ir.AtomicType); ok {
			return at.Scalar
		}
	}
	// Fallback: walk expression chain to find the global variable + type.
	return e.resolveAtomicScalarFromExpr(fn, ptrHandle)
}

// resolveAtomicBaseType extracts the base type from a pointer's ExpressionTypes.
func (e *Emitter) resolveAtomicBaseType(fn *ir.Function, ptrHandle ir.ExpressionHandle) ir.TypeInner {
	if int(ptrHandle) >= len(fn.ExpressionTypes) {
		return nil
	}
	tr := fn.ExpressionTypes[ptrHandle]
	if tr.Handle != nil {
		inner := e.ir.Types[*tr.Handle].Inner
		if pt, ok := inner.(ir.PointerType); ok {
			return e.ir.Types[pt.Base].Inner
		}
		return inner
	}
	if tr.Value != nil {
		if pt, ok := tr.Value.(ir.PointerType); ok {
			return e.ir.Types[pt.Base].Inner
		}
	}
	return nil
}

// resolveAtomicScalarFromExpr walks expression kinds recursively to find the atomic scalar.
func (e *Emitter) resolveAtomicScalarFromExpr(fn *ir.Function, ptrHandle ir.ExpressionHandle) ir.ScalarType {
	if int(ptrHandle) >= len(fn.Expressions) {
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	}
	expr := fn.Expressions[ptrHandle]
	switch ek := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(ek.Variable) < len(e.ir.GlobalVariables) {
			return e.resolveAtomicScalarFromType(e.ir.GlobalVariables[ek.Variable].Type)
		}
	case ir.ExprAccessIndex:
		// Walk through the base's type to find the member/element type.
		baseInner := e.resolveExprTypeInner(fn, ek.Base)
		switch bt := baseInner.(type) {
		case ir.StructType:
			if int(ek.Index) < len(bt.Members) {
				return e.resolveAtomicScalarFromType(bt.Members[ek.Index].Type)
			}
		case ir.ArrayType:
			return e.resolveAtomicScalarFromType(bt.Base)
		}
		// Fall through to base.
		return e.resolveAtomicScalarFromExpr(fn, ek.Base)
	case ir.ExprAccess:
		baseInner := e.resolveExprTypeInner(fn, ek.Base)
		if at, ok := baseInner.(ir.ArrayType); ok {
			return e.resolveAtomicScalarFromType(at.Base)
		}
		return e.resolveAtomicScalarFromExpr(fn, ek.Base)
	}
	return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
}

// resolveAtomicScalarFromType resolves an atomic scalar from a type handle,
// walking through arrays to find the atomic.
func (e *Emitter) resolveAtomicScalarFromType(th ir.TypeHandle) ir.ScalarType {
	if int(th) >= len(e.ir.Types) {
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	}
	inner := e.ir.Types[th].Inner
	switch t := inner.(type) {
	case ir.AtomicType:
		return t.Scalar
	case ir.ArrayType:
		return e.resolveAtomicScalarFromType(t.Base)
	case ir.StructType:
		// Can't pick a member without an index — default.
		if len(t.Members) > 0 {
			return e.resolveAtomicScalarFromType(t.Members[0].Type)
		}
	}
	return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
}

// isUnsignedAtomicPointer determines if the atomic variable's scalar type is unsigned.
func (e *Emitter) isUnsignedAtomicPointer(fn *ir.Function, ptrHandle ir.ExpressionHandle) bool {
	return e.resolveAtomicScalar(fn, ptrHandle).Kind == ir.ScalarUint
}

// emitStmtBarrier emits a dx.op.barrier call.
//
// Signature: void @dx.op.barrier(i32 80, i32 flags)
//
// Maps naga BarrierFlags to DXIL barrier mode flags:
//   - BarrierStorage  → UAV_FENCE_GLOBAL (2)
//   - BarrierWorkGroup → SYNC_THREAD_GROUP (1) | GROUPSHARED_MEM_FENCE (8)
//   - BarrierSubGroup  → SYNC_THREAD_GROUP (1) | GROUPSHARED_MEM_FENCE (8) |
//     UAV_FENCE_THREAD_GROUP (4)
//
// DXIL has no dedicated wave/subgroup barrier intrinsic: the closest HLSL
// equivalent is GroupMemoryBarrierWithGroupSync, which is what DXC lowers
// such semantics to. The DXIL validator also rejects a Sync-only flag
// combination with "sync must include some form of memory barrier" — so
// for any SubGroup/WorkGroup barrier we MUST include at least one memory
// fence bit (TGSM and/or UAV).
//
// Reference: Mesa nir_to_dxil.c emit_barrier_impl() ~3082;
// DxilValidation.cpp SyncThreadGroup memory-fence requirement.
func (e *Emitter) emitStmtBarrier(barrier ir.StmtBarrier) error {
	var flags DXILBarrierMode

	if barrier.Flags&ir.BarrierStorage != 0 {
		// WGSL storageBarrier() = DeviceMemoryBarrierWithGroupSync in HLSL.
		// DXC emits SYNC + UAV_FENCE_GLOBAL (flag 3).
		flags |= BarrierModeSyncThreadGroup | BarrierModeUAVFenceGlobal
	}

	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		// WGSL workgroupBarrier() = GroupMemoryBarrierWithGroupSync in HLSL.
		// DXC emits SYNC + GROUPSHARED_MEM_FENCE (flag 9).
		flags |= BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence
	}

	if barrier.Flags&ir.BarrierTexture != 0 {
		// WGSL textureBarrier() — no direct HLSL equivalent.
		// DXC treats texture UAVs as global device memory, so the fence
		// is SYNC + UAV_FENCE_GLOBAL (flag 3), same as storageBarrier.
		flags |= BarrierModeSyncThreadGroup | BarrierModeUAVFenceGlobal
	}

	if barrier.Flags&ir.BarrierSubGroup != 0 {
		flags |= BarrierModeSyncThreadGroup |
			BarrierModeGroupSharedMemFence |
			BarrierModeUAVFenceThreadGroup
	}

	// If no flags are set, use a default thread-group sync WITH a memory
	// fence (TGSM): a bare sync flag is rejected by DxilValidation.cpp.
	if flags == 0 {
		flags = BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence
	}

	barrierFn := e.getDxOpBarrierFunc()
	opcodeVal := e.getIntConstID(int64(OpBarrier))
	flagsVal := e.getIntConstID(int64(flags))

	e.addCallInstr(barrierFn, e.mod.GetVoidType(), []int{opcodeVal, flagsVal})

	return nil
}

// emitStmtWorkGroupUniformLoad emits a workgroup uniform load: barrier + load + barrier.
// Semantics: all invocations in the workgroup must execute the load uniformly.
// DXIL pattern: GroupMemoryBarrierWithGroupSync(); val = *ptr; GroupMemoryBarrierWithGroupSync();
// Reference: HLSL GroupMemoryBarrierWithGroupSync + load.
func (e *Emitter) emitStmtWorkGroupUniformLoad(fn *ir.Function, wul ir.StmtWorkGroupUniformLoad) error {
	// First barrier: SYNC_THREAD_GROUP | GROUPSHARED_MEM_FENCE
	barrierFlags := BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence
	barrierFn := e.getDxOpBarrierFunc()
	opcodeVal := e.getIntConstID(int64(OpBarrier))
	flagsVal := e.getIntConstID(int64(barrierFlags))
	e.addCallInstr(barrierFn, e.mod.GetVoidType(), []int{opcodeVal, flagsVal})

	// Load the value from the workgroup pointer.
	loadExpr := ir.ExprLoad{Pointer: wul.Pointer}
	loadID, err := e.emitLoad(fn, loadExpr)
	if err != nil {
		return fmt.Errorf("workgroup uniform load: %w", err)
	}

	// Track the result expression so it can be referenced downstream.
	e.exprValues[wul.Result] = loadID
	if comps := e.pendingComponents; len(comps) > 0 {
		e.exprComponents[wul.Result] = comps
		e.pendingComponents = nil
	}

	// Second barrier.
	e.addCallInstr(barrierFn, e.mod.GetVoidType(), []int{opcodeVal, flagsVal})

	return nil
}

// getDxOpAtomicBinOpFuncTyped creates a typed dx.op.atomicBinOp function declaration.
// Signature: T @dx.op.atomicBinOp.T(i32, %dx.types.Handle, i32, i32, i32, i32, T)
// where T is i32, i64, or f32 depending on the overload.
func (e *Emitter) getDxOpAtomicBinOpFuncTyped(ol overloadType) *module.Function {
	name := "dx.op.atomicBinOp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	handleTy := e.getDxHandleType()
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)

	// (i32 opcode, %handle, i32 atomicOp, i32 coord0, i32 coord1, i32 coord2, T value) → T
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, i32Ty, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpAtomicCmpXchgFuncTyped creates a typed dx.op.atomicCompareExchange function declaration.
// Signature: T @dx.op.atomicCompareExchange.T(i32, %handle, i32, i32, i32, T, T)
func (e *Emitter) getDxOpAtomicCmpXchgFuncTyped(ol overloadType) *module.Function {
	name := "dx.op.atomicCompareExchange"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	handleTy := e.getDxHandleType()
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)

	// (i32 opcode, %handle, i32 coord0, i32 coord1, i32 coord2, T cmpVal, T newVal) → T
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, retTy, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpBarrierFunc creates the dx.op.barrier function declaration.
// Signature: void @dx.op.barrier(i32, i32)
func (e *Emitter) getDxOpBarrierFunc() *module.Function {
	name := "dx.op.barrier"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)

	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	fn.AttrSetID = module.AttrSetNoDuplicate
	e.dxOpFuncs[key] = fn
	return fn
}

// emitIfStatement emits an if/else construct using LLVM basic blocks.
//
// DXIL uses basic block branching for control flow:
//
//	current_bb:
//	  br i1 %cond, label %then, label %else_or_merge
//	then_bb:
//	  <accept block>
//	  br label %merge
//	else_bb: (only if reject block is non-empty)
//	  <reject block>
//	  br label %merge
//	merge_bb:
//	  <continues>
//
// Reference: Mesa nir_to_dxil.c emit_cf_list / emit_if
func (e *Emitter) emitIfStatement(fn *ir.Function, stmt ir.StmtIf) error {
	// Emit the condition expression.
	condID, err := e.emitExpression(fn, stmt.Condition)
	if err != nil {
		return fmt.Errorf("if condition: %w", err)
	}

	hasReject := len(stmt.Reject) > 0

	// Snapshot the BB the conditional branch is emitted FROM. When
	// the if has no else branch, the false-target of the conditional
	// br goes directly to the merge BB — so the phi's reject-incoming
	// flows from THIS entry BB, not from any synthesized else block.
	// Capturing the index now (before AddBasicBlock alters anything)
	// gives us a stable handle for emitPhi to reference via
	// branchBBs.rejectEndBB when !hasReject.
	entryBBIndex := bbIndexOf(e.mainFn, e.currentBB)

	// Create basic blocks. We add them to mainFn now so we can
	// reference their indices for branch instructions.
	thenBB := e.mainFn.AddBasicBlock("if.then")
	thenBBIndex := len(e.mainFn.BasicBlocks) - 1

	var elseBBIndex int
	if hasReject {
		elseBB := e.mainFn.AddBasicBlock("if.else")
		elseBBIndex = len(e.mainFn.BasicBlocks) - 1
		_ = elseBB // used below via index
	}

	mergeBB := e.mainFn.AddBasicBlock("if.merge")
	mergeBBIndex := len(e.mainFn.BasicBlocks) - 1

	// Emit conditional branch from the current BB.
	falseBBIndex := mergeBBIndex
	if hasReject {
		falseBBIndex = elseBBIndex
	}
	e.currentBB.AddInstruction(module.NewBrCondInstr(thenBBIndex, falseBBIndex, condID))

	// Emit accept (then) block.
	e.currentBB = thenBB
	if err := e.emitBlock(fn, stmt.Accept); err != nil {
		return fmt.Errorf("if accept: %w", err)
	}
	// Capture the accept-end BB BEFORE adding the terminator branch
	// so the BB index reflects where the phi's accept-incoming flows
	// from. Indexing in mainFn.BasicBlocks: walk to find current.
	acceptEndIdx := bbIndexOf(e.mainFn, e.currentBB)
	// Branch from then to merge (unless block already ends with a terminator).
	if !e.blockHasTerminator(e.currentBB) {
		e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
	}

	// Emit reject (else) block if present. When absent, the
	// reject-incoming for any post-if phi flows from entryBBIndex
	// (the BB where the conditional branch was emitted) — its false
	// edge goes directly to merge.
	rejectEndIdx := entryBBIndex
	if hasReject {
		e.currentBB = e.mainFn.BasicBlocks[elseBBIndex]
		if err := e.emitBlock(fn, stmt.Reject); err != nil {
			return fmt.Errorf("if reject: %w", err)
		}
		rejectEndIdx = bbIndexOf(e.mainFn, e.currentBB)
		if !e.blockHasTerminator(e.currentBB) {
			e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
		}
	}

	// Snapshot the BB indices for any ExprPhi that may immediately
	// follow this StmtIf in the parent block (placed there by the
	// DXIL mem2reg Phase B walker).
	e.lastBranchBBs = &branchBBs{
		kind:        branchKindIf,
		acceptEndBB: acceptEndIdx,
		rejectEndBB: rejectEndIdx,
		hasReject:   hasReject,
	}

	// Continue emitting into the merge block.
	e.currentBB = mergeBB
	return nil
}

// bbIndexOf returns the slice index of bb within fn.BasicBlocks, or
// -1 if not found. Used by branch-snapshot logic to record the BB
// where a control-flow branch terminates so phi nodes at the merge
// can map PhiPredKey -> BB index for FUNC_CODE_INST_PHI emission.
func bbIndexOf(fn *module.Function, bb *module.BasicBlock) int {
	for i, candidate := range fn.BasicBlocks {
		if candidate == bb {
			return i
		}
	}
	return -1
}

// emitSwitchStatement emits a switch construct as a chain of conditional branches.
//
// DXIL doesn't have a native switch instruction in our module, so we lower it to
// cascading icmp eq + conditional branches. Each non-default case becomes:
//
//	%cmp = icmp eq i32 %selector, <caseValue>
//	br i1 %cmp, label %case_N, label %next_test
//
// The default case (if present) becomes the final else branch target.
// All case bodies branch to a merge block at the end.
//
// FallThrough cases are handled by branching to the next case body instead of merge.
func (e *Emitter) emitSwitchStatement(fn *ir.Function, stmt ir.StmtSwitch) error {
	// Emit the selector expression.
	selectorID, err := e.emitExpression(fn, stmt.Selector)
	if err != nil {
		return fmt.Errorf("switch selector: %w", err)
	}

	if len(stmt.Cases) == 0 {
		return nil
	}

	// Separate default case from valued cases.
	defaultIdx := -1
	for i, c := range stmt.Cases {
		if _, isDefault := c.Value.(ir.SwitchValueDefault); isDefault {
			defaultIdx = i
			break
		}
	}

	// Create merge block.
	mergeBB := e.mainFn.AddBasicBlock("switch.merge")
	mergeBBIndex := len(e.mainFn.BasicBlocks) - 1

	// Create basic blocks for each case body.
	caseBBIndices := make([]int, len(stmt.Cases))
	for i := range stmt.Cases {
		label := fmt.Sprintf("switch.case_%d", i)
		if i == defaultIdx {
			label = "switch.default"
		}
		e.mainFn.AddBasicBlock(label)
		caseBBIndices[i] = len(e.mainFn.BasicBlocks) - 1
	}

	// Emit the comparison chain.
	// For each non-default case, compare selector == caseValue and branch.
	for i, c := range stmt.Cases {
		if i == defaultIdx {
			continue
		}

		var caseConstID int
		switch cv := c.Value.(type) {
		case ir.SwitchValueI32:
			caseConstID = e.getIntConstID(int64(cv))
		case ir.SwitchValueU32:
			caseConstID = e.getIntConstID(int64(cv))
		default:
			continue
		}

		cmpID := e.addCmpInstr(ICmpEQ, selectorID, caseConstID)

		// Determine the false target: either next comparison or default/merge.
		// We need to create a "next test" BB for the cascade.
		e.mainFn.AddBasicBlock(fmt.Sprintf("switch.test_%d", i))
		nextTestBBIndex := len(e.mainFn.BasicBlocks) - 1

		e.currentBB.AddInstruction(module.NewBrCondInstr(caseBBIndices[i], nextTestBBIndex, cmpID))
		e.currentBB = e.mainFn.BasicBlocks[nextTestBBIndex]
	}

	// After all comparisons, branch to default case or merge.
	if defaultIdx >= 0 {
		e.currentBB.AddInstruction(module.NewBrInstr(caseBBIndices[defaultIdx]))
	} else {
		e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
	}

	// Emit each case body.
	caseEndBBs := make([]int, len(stmt.Cases))
	for i := range stmt.Cases {
		c := &stmt.Cases[i]
		e.currentBB = e.mainFn.BasicBlocks[caseBBIndices[i]]
		if err := e.emitBlock(fn, c.Body); err != nil {
			return fmt.Errorf("switch case %d: %w", i, err)
		}
		caseEndBBs[i] = bbIndexOf(e.mainFn, e.currentBB)
		// If fallthrough, branch to next case body; otherwise branch to merge.
		if !e.blockHasTerminator(e.currentBB) {
			if c.FallThrough && i+1 < len(stmt.Cases) {
				e.currentBB.AddInstruction(module.NewBrInstr(caseBBIndices[i+1]))
			} else {
				e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
			}
		}
	}

	// Snapshot per-case end BB indices for any ExprPhi that
	// immediately follows this StmtSwitch (placed by DXIL mem2reg
	// Phase B walker). PhiIncoming.PredKey == PhiPredSwitchCase,
	// CaseIdx selects which entry of caseEndBBs to use.
	e.lastBranchBBs = &branchBBs{
		kind:       branchKindSwitch,
		caseEndBBs: caseEndBBs,
	}

	// Continue emitting into the merge block.
	e.currentBB = mergeBB
	return nil
}

// emitLoopStatement emits a loop construct using LLVM basic blocks.
//
// DXIL loop structure:
//
//	current_bb:
//	  br label %loop_header
//	loop_header:
//	  br label %loop_body
//	loop_body:
//	  <body statements>
//	  br label %loop_continuing  (or break → br %loop_merge)
//	loop_continuing:
//	  <continuing statements>
//	  br label %loop_header      (back edge)
//	loop_merge:
//	  <continues after loop>
//
// Reference: Mesa nir_to_dxil.c emit_cf_list / emit_loop
func (e *Emitter) emitLoopStatement(fn *ir.Function, stmt ir.StmtLoop) error {
	// Create basic blocks for the loop structure.
	headerBB := e.mainFn.AddBasicBlock("loop.header")
	headerBBIndex := len(e.mainFn.BasicBlocks) - 1

	bodyBB := e.mainFn.AddBasicBlock("loop.body")
	bodyBBIndex := len(e.mainFn.BasicBlocks) - 1
	_ = bodyBBIndex // used implicitly via headerBB branch

	continuingBB := e.mainFn.AddBasicBlock("loop.continuing")
	continuingBBIndex := len(e.mainFn.BasicBlocks) - 1

	mergeBB := e.mainFn.AddBasicBlock("loop.merge")
	mergeBBIndex := len(e.mainFn.BasicBlocks) - 1

	// Branch from current BB to loop header.
	e.currentBB.AddInstruction(module.NewBrInstr(headerBBIndex))

	// Header branches unconditionally to body.
	headerBB.AddInstruction(module.NewBrInstr(bodyBBIndex))

	// Push loop context for break/continue.
	e.loopStack = append(e.loopStack, loopContext{
		continuingBBIndex: continuingBBIndex,
		mergeBBIndex:      mergeBBIndex,
	})

	// Emit body.
	e.currentBB = bodyBB
	if err := e.emitBlock(fn, stmt.Body); err != nil {
		return fmt.Errorf("loop body: %w", err)
	}
	// After body, branch to continuing (unless terminated by break/continue/return).
	if !e.blockHasTerminator(e.currentBB) {
		e.currentBB.AddInstruction(module.NewBrInstr(continuingBBIndex))
	}

	// Emit continuing block.
	e.currentBB = continuingBB
	if len(stmt.Continuing) > 0 {
		if err := e.emitBlock(fn, stmt.Continuing); err != nil {
			return fmt.Errorf("loop continuing: %w", err)
		}
	}

	// Handle break_if: conditional back-edge vs break.
	if stmt.BreakIf != nil {
		breakCondID, err := e.emitExpression(fn, *stmt.BreakIf)
		if err != nil {
			return fmt.Errorf("loop break_if: %w", err)
		}
		// If break condition is true, go to merge; otherwise loop back.
		e.currentBB.AddInstruction(module.NewBrCondInstr(mergeBBIndex, headerBBIndex, breakCondID))
	} else if !e.blockHasTerminator(e.currentBB) {
		// Unconditional back edge.
		e.currentBB.AddInstruction(module.NewBrInstr(headerBBIndex))
	}

	// Pop loop context.
	e.loopStack = e.loopStack[:len(e.loopStack)-1]

	// Continue emitting into merge block.
	e.currentBB = mergeBB
	return nil
}

// emitBreak emits a branch to the loop merge block (exits the loop).
func (e *Emitter) emitBreak() error {
	if len(e.loopStack) == 0 {
		return fmt.Errorf("break outside of loop")
	}
	ctx := e.loopStack[len(e.loopStack)-1]
	e.currentBB.AddInstruction(module.NewBrInstr(ctx.mergeBBIndex))
	return nil
}

// emitContinue emits a branch to the loop continuing block.
func (e *Emitter) emitContinue() error {
	if len(e.loopStack) == 0 {
		return fmt.Errorf("continue outside of loop")
	}
	ctx := e.loopStack[len(e.loopStack)-1]
	e.currentBB.AddInstruction(module.NewBrInstr(ctx.continuingBBIndex))
	return nil
}

// blockHasTerminator checks whether the basic block already ends with
// a terminator instruction (return or branch).
func (e *Emitter) blockHasTerminator(bb *module.BasicBlock) bool {
	if len(bb.Instructions) == 0 {
		return false
	}
	last := bb.Instructions[len(bb.Instructions)-1]
	return last.Kind == module.InstrRet || last.Kind == module.InstrBr
}

// --- Mesh Shader Store Handling ---

// meshAccessChain represents a resolved access chain into a mesh output variable.
type meshAccessChain struct {
	// category indicates what part of mesh output we're writing to.
	category meshStoreCategory

	// For vertex/primitive stores: the element index expression handle.
	// e.g., mesh_output.vertices[idx] — idx is this handle.
	elementIndex ir.ExpressionHandle
	hasConstIdx  bool  // true if elementIndex is a constant
	constIdx     int64 // constant index value (if hasConstIdx)

	// For vertex/primitive attribute stores: the struct member binding.
	memberBinding *ir.Binding
	memberType    ir.TypeHandle
}

// meshStoreCategory identifies what part of a mesh output is being stored.
type meshStoreCategory int

const (
	meshStoreUnknown meshStoreCategory = iota
	meshStoreVertexCount
	meshStorePrimitiveCount
	meshStoreVertexAttribute    // vertices[i].position, vertices[i].color
	meshStorePrimitiveAttribute // primitives[i].cull, primitives[i].colorMask
	meshStoreTriangleIndices    // primitives[i].indices
)

// tryEmitMeshOutputStore checks if a store targets the mesh output variable and
// emits the appropriate dx.op mesh shader intrinsics. Returns (true, nil) if handled.
func (e *Emitter) tryEmitMeshOutputStore(fn *ir.Function, store ir.StmtStore) (bool, error) {
	chain, ok := e.resolveMeshAccessChain(fn, store.Pointer)
	if !ok {
		return false, nil
	}

	switch chain.category {
	case meshStoreVertexCount:
		// Buffer the vertex count value; emit SetMeshOutputCounts when both are ready.
		valueID, err := e.emitExpression(fn, store.Value)
		if err != nil {
			return true, fmt.Errorf("mesh vertex count value: %w", err)
		}
		e.meshCtx.pendingVertexCount = valueID
		e.meshCtx.hasVertexCount = true
		if e.meshCtx.hasPrimitiveCount {
			e.emitSetMeshOutputCounts(e.meshCtx.pendingVertexCount, e.meshCtx.pendingPrimitiveCount)
		}
		return true, nil

	case meshStorePrimitiveCount:
		valueID, err := e.emitExpression(fn, store.Value)
		if err != nil {
			return true, fmt.Errorf("mesh primitive count value: %w", err)
		}
		e.meshCtx.pendingPrimitiveCount = valueID
		e.meshCtx.hasPrimitiveCount = true
		if e.meshCtx.hasVertexCount {
			e.emitSetMeshOutputCounts(e.meshCtx.pendingVertexCount, e.meshCtx.pendingPrimitiveCount)
		}
		return true, nil

	case meshStoreTriangleIndices:
		return true, e.emitMeshTriangleIndicesStore(fn, chain, store.Value)

	case meshStoreVertexAttribute:
		return true, e.emitMeshVertexAttributeStore(fn, chain, store.Value)

	case meshStorePrimitiveAttribute:
		return true, e.emitMeshPrimitiveAttributeStore(fn, chain, store.Value)

	default:
		return false, nil
	}
}

// resolveMeshAccessChain walks the access chain from a store pointer to determine
// if it targets the mesh output variable, and if so, what part.
//
// The access patterns in naga IR for mesh output stores:
//
//	mesh_output.vertex_count = N
//	  -> AccessIndex(GlobalVar(mesh_output), field_idx_of_vertex_count)
//
//	mesh_output.vertices[i].position = V
//	  -> AccessIndex(Access(AccessIndex(GlobalVar(mesh_output), field_idx_of_vertices), i), field_idx_of_position)
//
//	mesh_output.primitives[i].indices = V
//	  -> AccessIndex(Access(AccessIndex(GlobalVar(mesh_output), field_idx_of_primitives), i), field_idx_of_indices)
//
// meshAccessStep is a single step in a mesh output access chain.
type meshAccessStep struct {
	isIndex bool // true = AccessIndex, false = Access
	index   uint32
	handle  ir.ExpressionHandle // for Access: dynamic index
}

func (e *Emitter) resolveMeshAccessChain(fn *ir.Function, ptrHandle ir.ExpressionHandle) (meshAccessChain, bool) {
	var steps []meshAccessStep
	cur := ptrHandle

	for {
		expr := fn.Expressions[cur]
		switch ek := expr.Kind.(type) {
		case ir.ExprAccessIndex:
			steps = append(steps, meshAccessStep{isIndex: true, index: ek.Index})
			cur = ek.Base
		case ir.ExprAccess:
			steps = append(steps, meshAccessStep{isIndex: false, handle: ek.Index})
			cur = ek.Base
		case ir.ExprGlobalVariable:
			if ek.Variable != e.meshCtx.outputVar {
				return meshAccessChain{}, false
			}
			// We found the mesh output root. Now interpret the steps (they're in reverse order).
			return e.interpretMeshAccessSteps(fn, steps)
		default:
			return meshAccessChain{}, false
		}
	}
}

// interpretMeshAccessSteps interprets the reversed access steps from a mesh output chain.
//
//nolint:gocognit,gocyclo,cyclop // mesh chain interpretation requires checking many path combinations
func (e *Emitter) interpretMeshAccessSteps(fn *ir.Function, steps []meshAccessStep) (meshAccessChain, bool) {
	// Steps are in reverse order (leaf first). Reverse them.
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}

	if len(steps) == 0 {
		return meshAccessChain{}, false
	}

	// First step: AccessIndex into MeshOutput struct.
	// Get the MeshOutput struct type to identify which field.
	meshOutGV := &e.ir.GlobalVariables[e.meshCtx.outputVar]
	meshOutType := e.ir.Types[meshOutGV.Type]
	meshOutSt, ok := meshOutType.Inner.(ir.StructType)
	if !ok {
		return meshAccessChain{}, false
	}

	if !steps[0].isIndex {
		return meshAccessChain{}, false
	}
	fieldIdx := int(steps[0].index)
	if fieldIdx >= len(meshOutSt.Members) {
		return meshAccessChain{}, false
	}
	member := &meshOutSt.Members[fieldIdx]

	// Identify the field by its builtin binding.
	if member.Binding == nil {
		return meshAccessChain{}, false
	}
	bb, isBB := (*member.Binding).(ir.BuiltinBinding)
	if !isBB {
		return meshAccessChain{}, false
	}

	switch bb.Builtin {
	case ir.BuiltinVertexCount:
		return meshAccessChain{category: meshStoreVertexCount}, true

	case ir.BuiltinPrimitiveCount:
		return meshAccessChain{category: meshStorePrimitiveCount}, true

	case ir.BuiltinVertices:
		// steps[1] = array index (Access or AccessIndex)
		// steps[2] = field in VertexOutput struct (AccessIndex)
		if len(steps) < 3 {
			return meshAccessChain{}, false
		}
		elemIdx, constIdx, hasConst := e.resolveArrayIndex(fn, steps[1])
		if len(steps) < 3 || !steps[2].isIndex {
			return meshAccessChain{}, false
		}
		vtxStepIdx := steps[2].index
		vtxType := e.ir.Types[e.meshCtx.meshInfo.VertexOutputType]
		vtxSt, isVtxSt := vtxType.Inner.(ir.StructType)
		if !isVtxSt || int(vtxStepIdx) >= len(vtxSt.Members) {
			return meshAccessChain{}, false
		}
		vtxMember := &vtxSt.Members[vtxStepIdx]
		return meshAccessChain{
			category:      meshStoreVertexAttribute,
			elementIndex:  elemIdx,
			hasConstIdx:   hasConst,
			constIdx:      constIdx,
			memberBinding: vtxMember.Binding,
			memberType:    vtxMember.Type,
		}, true

	case ir.BuiltinPrimitives:
		// steps[1] = array index (Access or AccessIndex)
		// steps[2] = field in PrimitiveOutput struct (AccessIndex)
		if len(steps) < 3 {
			return meshAccessChain{}, false
		}
		elemIdx, constIdx, hasConst := e.resolveArrayIndex(fn, steps[1])
		if !steps[2].isIndex {
			return meshAccessChain{}, false
		}
		primStepIdx := steps[2].index
		primType := e.ir.Types[e.meshCtx.meshInfo.PrimitiveOutputType]
		primSt, isPrimSt := primType.Inner.(ir.StructType)
		if !isPrimSt || int(primStepIdx) >= len(primSt.Members) {
			return meshAccessChain{}, false
		}
		primMember := &primSt.Members[primStepIdx]

		// Check if this is the triangle_indices field.
		if primMember.Binding != nil {
			if pbb, isPBB := (*primMember.Binding).(ir.BuiltinBinding); isPBB {
				if pbb.Builtin == ir.BuiltinTriangleIndices {
					return meshAccessChain{
						category:     meshStoreTriangleIndices,
						elementIndex: elemIdx,
						hasConstIdx:  hasConst,
						constIdx:     constIdx,
					}, true
				}
			}
		}

		return meshAccessChain{
			category:      meshStorePrimitiveAttribute,
			elementIndex:  elemIdx,
			hasConstIdx:   hasConst,
			constIdx:      constIdx,
			memberBinding: primMember.Binding,
			memberType:    primMember.Type,
		}, true

	default:
		return meshAccessChain{}, false
	}
}

// resolveArrayIndex resolves an access step that indexes into an array.
// Returns the expression handle for the index, and if constant, its value.
func (e *Emitter) resolveArrayIndex(_ *ir.Function, step meshAccessStep) (ir.ExpressionHandle, int64, bool) {
	if step.isIndex {
		// Constant index via AccessIndex.
		return 0, int64(step.index), true
	}
	// Dynamic index via Access — check if the index expression is a known constant.
	return step.handle, 0, false
}

// emitMeshTriangleIndicesStore emits dx.op.emitIndices for a store to primitives[i].indices.
// The value is vec3<u32> which must be decomposed into 3 scalar u32 values.
func (e *Emitter) emitMeshTriangleIndicesStore(fn *ir.Function, chain meshAccessChain, valueHandle ir.ExpressionHandle) error {
	// Emit the vec3<u32> value.
	if _, err := e.emitExpression(fn, valueHandle); err != nil {
		return fmt.Errorf("triangle indices value: %w", err)
	}

	// Get the primitive index.
	primIdx, err := e.getMeshElementIndex(fn, chain)
	if err != nil {
		return err
	}

	// Decompose the vec3<u32> into 3 scalar components.
	v0 := e.getComponentID(valueHandle, 0)
	v1 := e.getComponentID(valueHandle, 1)
	v2 := e.getComponentID(valueHandle, 2)

	e.emitEmitIndices(primIdx, v0, v1, v2)
	return nil
}

// emitMeshVertexAttributeStore emits dx.op.storeVertexOutput for a store to vertices[i].field.
func (e *Emitter) emitMeshVertexAttributeStore(fn *ir.Function, chain meshAccessChain, valueHandle ir.ExpressionHandle) error {
	if chain.memberBinding == nil {
		return fmt.Errorf("mesh vertex attribute: no binding")
	}

	// Emit the value expression.
	valueID, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("mesh vertex attribute value: %w", err)
	}

	// Get the vertex index.
	vtxIdx, err := e.getMeshElementIndex(fn, chain)
	if err != nil {
		return err
	}

	// Resolve signature element ID.
	key := bindingToMeshOutputKey(*chain.memberBinding)
	sigID, ok := e.meshCtx.vertexOutputSigIDs[key]
	if !ok {
		return fmt.Errorf("mesh vertex attribute: no signature ID for binding")
	}

	// Determine component count and overload from the member type.
	memberIRType := e.ir.Types[chain.memberType]
	scalar, numComps := scalarAndComponentCount(memberIRType.Inner)
	ol := overloadForScalar(scalar)

	// Check if the value is a scalar being stored to a vector target.
	// This happens when const evaluation folds array access to a scalar literal.
	// In this case, splat the scalar to all components.
	_, hasComps := e.exprComponents[valueHandle]
	isScalarValue := !hasComps

	for comp := 0; comp < numComps; comp++ {
		var compValueID int
		if isScalarValue {
			compValueID = valueID // splat scalar to all components
		} else {
			compValueID = e.getComponentID(valueHandle, comp)
		}
		e.emitStoreVertexOutput(sigID, comp, compValueID, vtxIdx, ol)
	}
	return nil
}

// emitMeshPrimitiveAttributeStore emits dx.op.storePrimitiveOutput for a store to primitives[i].field.
func (e *Emitter) emitMeshPrimitiveAttributeStore(fn *ir.Function, chain meshAccessChain, valueHandle ir.ExpressionHandle) error {
	if chain.memberBinding == nil {
		return fmt.Errorf("mesh primitive attribute: no binding")
	}

	// Emit the value expression.
	if _, err := e.emitExpression(fn, valueHandle); err != nil {
		return fmt.Errorf("mesh primitive attribute value: %w", err)
	}

	// Get the primitive index.
	primIdx, err := e.getMeshElementIndex(fn, chain)
	if err != nil {
		return err
	}

	// Resolve signature element ID.
	key := bindingToMeshOutputKey(*chain.memberBinding)
	sigID, ok := e.meshCtx.primitiveOutputSigIDs[key]
	if !ok {
		return fmt.Errorf("mesh primitive attribute: no signature ID for binding")
	}

	// Determine component count and overload from the member type.
	memberIRType := e.ir.Types[chain.memberType]
	scalar, numComps := scalarAndComponentCount(memberIRType.Inner)
	ol := overloadForScalar(scalar)

	// Special case: bool (cull_primitive) is stored as i1 in DXIL.
	if scalar.Kind == ir.ScalarBool {
		ol = overloadI1
		compValueID := e.getComponentID(valueHandle, 0)
		e.emitStorePrimitiveOutput(sigID, 0, compValueID, primIdx, ol)
		return nil
	}

	for comp := 0; comp < numComps; comp++ {
		compValueID := e.getComponentID(valueHandle, comp)
		e.emitStorePrimitiveOutput(sigID, comp, compValueID, primIdx, ol)
	}
	return nil
}

// getMeshElementIndex resolves the vertex/primitive index for a mesh store.
func (e *Emitter) getMeshElementIndex(fn *ir.Function, chain meshAccessChain) (int, error) {
	if chain.hasConstIdx {
		return e.getIntConstID(chain.constIdx), nil
	}
	// Dynamic index — emit the expression.
	id, err := e.emitExpression(fn, chain.elementIndex)
	if err != nil {
		return 0, fmt.Errorf("mesh element index: %w", err)
	}
	return id, nil
}

// scalarAndComponentCount returns the scalar type and number of components for a type.
// For scalars, returns (scalar, 1). For vectors, returns (scalar, size).
func scalarAndComponentCount(inner ir.TypeInner) (ir.ScalarType, int) {
	switch ti := inner.(type) {
	case ir.ScalarType:
		return ti, 1
	case ir.VectorType:
		return ti.Scalar, int(ti.Size)
	default:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 1
	}
}

// emitStmtImageAtomic emits a dx.op.atomicBinOp or dx.op.atomicCompareExchange call
// for atomic operations on storage texture (RWTexture) resources.
//
// Uses the same dx.op intrinsics as UAV buffer atomics but with texture coordinates
// instead of buffer index. The coordinate layout depends on the image dimension:
//   - 1D:        c0
//   - 2D:        c0, c1
//   - 2DArray:   c0, c1, c2 (array slice)
//   - 3D:        c0, c1, c2
//
// Reference: DXC DXIL.rst atomicBinOp section (opcode 78)
func (e *Emitter) emitStmtImageAtomic(fn *ir.Function, imgAtomic ir.StmtImageAtomic) error {
	// Resolve image handle.
	imageHandleID, err := e.resolveResourceHandle(fn, imgAtomic.Image)
	if err != nil {
		return fmt.Errorf("StmtImageAtomic: image handle: %w", err)
	}

	// Resolve image type for coordinate count.
	imgInner := e.resolveExprType(fn, imgAtomic.Image)
	imgType, ok := imgInner.(ir.ImageType)
	if !ok {
		return fmt.Errorf("StmtImageAtomic: image is not ImageType, got %T", imgInner)
	}

	// Build coordinates.
	coords, err := e.buildImageAtomicCoords(fn, imgAtomic, imgType)
	if err != nil {
		return err
	}

	ol := imageOverload(imgType)

	// Emit the value expression.
	valueID, err := e.emitExpression(fn, imgAtomic.Value)
	if err != nil {
		return fmt.Errorf("StmtImageAtomic: value: %w", err)
	}

	return e.dispatchImageAtomic(fn, imgAtomic, imageHandleID, coords, valueID, ol, imgType)
}

// buildImageAtomicCoords emits coordinates for an image atomic operation.
func (e *Emitter) buildImageAtomicCoords(fn *ir.Function, imgAtomic ir.StmtImageAtomic, imgType ir.ImageType) ([3]int, error) {
	if _, err := e.emitExpression(fn, imgAtomic.Coordinate); err != nil {
		return [3]int{}, fmt.Errorf("StmtImageAtomic: coordinate: %w", err)
	}
	undefI32 := e.getUndefConstID()
	coords := [3]int{undefI32, undefI32, undefI32}
	spatialComps := imageDimSpatialComponents(imgType.Dim)
	for i := 0; i < spatialComps; i++ {
		coords[i] = e.getComponentID(imgAtomic.Coordinate, i)
	}
	if imgType.Arrayed && imgAtomic.ArrayIndex != nil {
		arrIdx, err := e.emitExpression(fn, *imgAtomic.ArrayIndex)
		if err != nil {
			return [3]int{}, fmt.Errorf("StmtImageAtomic: array index: %w", err)
		}
		if spatialComps < 3 {
			coords[spatialComps] = arrIdx
		}
	}
	return coords, nil
}

// dispatchImageAtomic dispatches the atomic function to the appropriate DXIL intrinsic.
func (e *Emitter) dispatchImageAtomic(fn *ir.Function, imgAtomic ir.StmtImageAtomic, handleID int, coords [3]int, valueID int, ol overloadType, imgType ir.ImageType) error {
	switch af := imgAtomic.Fun.(type) {
	case ir.AtomicAdd:
		return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicAdd, valueID, ol)
	case ir.AtomicSubtract:
		return e.emitImageAtomicSubtract(handleID, coords, valueID, ol)
	case ir.AtomicAnd:
		return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicAnd, valueID, ol)
	case ir.AtomicInclusiveOr:
		return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicOr, valueID, ol)
	case ir.AtomicExclusiveOr:
		return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicXor, valueID, ol)
	case ir.AtomicMin:
		op := e.imageAtomicMinMaxOp(imgType, true)
		return e.emitImageAtomicBinOp(handleID, coords, op, valueID, ol)
	case ir.AtomicMax:
		op := e.imageAtomicMinMaxOp(imgType, false)
		return e.emitImageAtomicBinOp(handleID, coords, op, valueID, ol)
	case ir.AtomicExchange:
		if af.Compare != nil {
			cmpValID, err := e.emitExpression(fn, *af.Compare)
			if err != nil {
				return fmt.Errorf("StmtImageAtomic: compare value: %w", err)
			}
			return e.emitImageAtomicCmpXchg(handleID, coords, cmpValID, valueID, ol)
		}
		return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicExchange, valueID, ol)
	default:
		return fmt.Errorf("StmtImageAtomic: unsupported atomic function: %T", imgAtomic.Fun)
	}
}

// imageAtomicMinMaxOp selects the correct DXIL atomic min/max opcode based on the
// image's storage format signedness.
func (e *Emitter) imageAtomicMinMaxOp(imgType ir.ImageType, isMin bool) DXILAtomicOp {
	scalar := imgType.StorageFormat.Scalar()
	if scalar.Kind == ir.ScalarUint {
		if isMin {
			return DXILAtomicUMin
		}
		return DXILAtomicUMax
	}
	if isMin {
		return DXILAtomicIMin
	}
	return DXILAtomicIMax
}

// emitImageAtomicBinOp emits dx.op.atomicBinOp with texture handle and coordinates.
func (e *Emitter) emitImageAtomicBinOp(handleID int, coords [3]int, atomicOp DXILAtomicOp, valueID int, ol overloadType) error {
	atomicFn := e.getDxOpAtomicBinOpFuncTyped(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpAtomicBinOp))
	atomicOpVal := e.getIntConstID(int64(atomicOp))

	e.addCallInstr(atomicFn, retTy, []int{
		opcodeVal, handleID, atomicOpVal,
		coords[0], coords[1], coords[2],
		valueID,
	})
	return nil
}

// emitImageAtomicSubtract emits an atomic subtract on a texture by negating and using ADD.
func (e *Emitter) emitImageAtomicSubtract(handleID int, coords [3]int, valueID int, ol overloadType) error {
	retTy := e.overloadReturnType(ol)

	// Negate: 0 - value.
	var negatedID int
	switch ol {
	case overloadF32:
		zeroVal := e.getFloatConstID(0.0)
		negatedID = e.addBinOpInstr(retTy, BinOpFSub, zeroVal, valueID)
	case overloadI64:
		c := e.mod.AddIntConst(e.mod.GetIntType(64), 0)
		id := e.allocValue()
		e.constMap[id] = c
		negatedID = e.addBinOpInstr(retTy, BinOpSub, id, valueID)
	default:
		zeroVal := e.getIntConstID(0)
		negatedID = e.addBinOpInstr(retTy, BinOpSub, zeroVal, valueID)
	}

	return e.emitImageAtomicBinOp(handleID, coords, DXILAtomicAdd, negatedID, ol)
}

// emitImageAtomicCmpXchg emits dx.op.atomicCompareExchange with texture handle and coordinates.
func (e *Emitter) emitImageAtomicCmpXchg(handleID int, coords [3]int, cmpValID, newValID int, ol overloadType) error {
	cmpXchgFn := e.getDxOpAtomicCmpXchgFuncTyped(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpAtomicCmpXchg))

	e.addCallInstr(cmpXchgFn, retTy, []int{
		opcodeVal, handleID,
		coords[0], coords[1], coords[2],
		cmpValID, newValID,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Subgroup / wave operations
// ---------------------------------------------------------------------------

// emitStmtSubgroupBallot emits dx.op.waveActiveBallot (opcode 116).
// Signature: %dx.types.fouri32 @dx.op.waveActiveBallot(i32 116, i1 condition) → {i32, i32, i32, i32}
// The result is a vec4<u32>.
func (e *Emitter) emitStmtSubgroupBallot(fn *ir.Function, ballot ir.StmtSubgroupBallot) error {
	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)

	var condID int
	if ballot.Predicate != nil {
		var err error
		condID, err = e.emitExpression(fn, *ballot.Predicate)
		if err != nil {
			return fmt.Errorf("StmtSubgroupBallot: predicate: %w", err)
		}
	} else {
		// No predicate → true (all active lanes).
		condID = e.getBoolConstID(true)
	}

	// Get or create the function declaration.
	ballotFn := e.getWaveBallotFunc()
	// The return type is a struct {i32, i32, i32, i32}.
	retTy := e.getWaveBallotRetType()

	opcodeVal := e.getIntConstID(int64(OpWaveBallot))
	resultID := e.addCallInstr(ballotFn, retTy, []int{opcodeVal, condID})

	// Extract 4 components.
	comps := make([]int, 4)
	for i := 0; i < 4; i++ {
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: i32Ty,
			Operands:   []int{resultID, i},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}

	e.exprValues[ballot.Result] = comps[0]
	e.exprComponents[ballot.Result] = comps
	_ = i1Ty
	return nil
}

// getWaveBallotRetType returns the struct type {i32, i32, i32, i32} for WaveActiveBallot.
func (e *Emitter) getWaveBallotRetType() *module.Type {
	if e.waveBallotRetTy != nil {
		return e.waveBallotRetTy
	}
	i32Ty := e.mod.GetIntType(32)
	e.waveBallotRetTy = e.mod.GetStructType("dx.types.fouri32", []*module.Type{i32Ty, i32Ty, i32Ty, i32Ty})
	return e.waveBallotRetTy
}

// getWaveBallotFunc returns the dx.op.waveActiveBallot function declaration.
func (e *Emitter) getWaveBallotFunc() *module.Function {
	name := "dx.op.waveActiveBallot"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)
	retTy := e.getWaveBallotRetType()
	params := []*module.Type{i32Ty, i1Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStmtSubgroupCollectiveOp emits wave collective operations.
//
// Mapping from naga IR to DXIL:
//   - All/Any (reduce) → dx.op.waveAllTrue(114) / dx.op.waveAnyTrue(113)
//   - Add/Mul/Min/Max (reduce) → dx.op.waveActiveOp(119, value, op, sign)
//   - And/Or/Xor (reduce) → dx.op.waveActiveBit(120, value, bitOp)
//   - Add/Mul/Min/Max (inclusive/exclusive scan) → dx.op.wavePrefixOp(121, value, op, sign)
func (e *Emitter) emitStmtSubgroupCollectiveOp(fn *ir.Function, op ir.StmtSubgroupCollectiveOperation) error {
	argID, err := e.emitExpression(fn, op.Argument)
	if err != nil {
		return fmt.Errorf("StmtSubgroupCollectiveOp: argument: %w", err)
	}

	i32Ty := e.mod.GetIntType(32)

	// Resolve the argument type to determine overload.
	argInner := e.resolveExprType(fn, op.Argument)
	argScalar, _ := scalarAndComponentCount(argInner)
	ol := overloadForScalar(argScalar)

	var resultID int

	switch op.CollectiveOp {
	case ir.CollectiveReduce:
		switch op.Op {
		case ir.SubgroupOperationAll:
			// dx.op.waveAllTrue(i32 114, i1 cond) → i1
			waveFn := e.getWaveBoolFunc("dx.op.waveAllTrue", OpWaveAllTrue)
			opcodeVal := e.getIntConstID(int64(OpWaveAllTrue))
			resultID = e.addCallInstr(waveFn, e.mod.GetIntType(1), []int{opcodeVal, argID})
			// Extend i1 → i32 (zext) since the IR result type is typically u32.
			resultID = e.emitCastInstr(i32Ty, CastZExt, resultID)

		case ir.SubgroupOperationAny:
			waveFn := e.getWaveBoolFunc("dx.op.waveAnyTrue", OpWaveAnyTrue)
			opcodeVal := e.getIntConstID(int64(OpWaveAnyTrue))
			resultID = e.addCallInstr(waveFn, e.mod.GetIntType(1), []int{opcodeVal, argID})
			resultID = e.emitCastInstr(i32Ty, CastZExt, resultID)

		case ir.SubgroupOperationAnd:
			resultID = e.emitWaveActiveBit(fn, argID, DXILWaveBitAnd, ol)
		case ir.SubgroupOperationOr:
			resultID = e.emitWaveActiveBit(fn, argID, DXILWaveBitOr, ol)
		case ir.SubgroupOperationXor:
			resultID = e.emitWaveActiveBit(fn, argID, DXILWaveBitXor, ol)

		default:
			waveOp, sign := subgroupOpToWaveOp(op.Op, argScalar)
			resultID = e.emitWaveActiveOp(fn, argID, waveOp, sign, ol)
		}

	case ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan:
		// And/Or/Xor scans are not available in DXIL wavePrefixOp.
		// Only arithmetic ops (sum, product, min, max) are supported.
		switch op.Op {
		case ir.SubgroupOperationAnd, ir.SubgroupOperationOr, ir.SubgroupOperationXor:
			// Fallback: use reduce result as approximation (best-effort).
			bitOp := DXILWaveBitAnd
			switch op.Op {
			case ir.SubgroupOperationOr:
				bitOp = DXILWaveBitOr
			case ir.SubgroupOperationXor:
				bitOp = DXILWaveBitXor
			}
			resultID = e.emitWaveActiveBit(fn, argID, bitOp, ol)
		default:
			waveOp, sign := subgroupOpToWaveOp(op.Op, argScalar)
			resultID = e.emitWavePrefixOp(fn, argID, waveOp, sign, ol)
			// Inclusive scan = prefix + current value. DXIL wavePrefixOp is exclusive.
			if op.CollectiveOp == ir.CollectiveInclusiveScan {
				resultID = e.combineForInclusive(argID, resultID, op.Op, ol)
			}
		}

	default:
		return fmt.Errorf("unsupported collective operation: %d", op.CollectiveOp)
	}

	e.exprValues[op.Result] = resultID
	return nil
}

// subgroupOpToWaveOp converts a naga SubgroupOperation to DXIL WaveOp + sign.
func subgroupOpToWaveOp(op ir.SubgroupOperation, scalar ir.ScalarType) (DXILWaveOp, DXILWaveOpSign) {
	sign := DXILWaveOpSignSigned
	if scalar.Kind == ir.ScalarUint {
		sign = DXILWaveOpSignUnsigned
	}
	switch op {
	case ir.SubgroupOperationAdd:
		return DXILWaveOpSum, sign
	case ir.SubgroupOperationMul:
		return DXILWaveOpMul, sign
	case ir.SubgroupOperationMin:
		return DXILWaveOpMin, sign
	case ir.SubgroupOperationMax:
		return DXILWaveOpMax, sign
	default:
		return DXILWaveOpSum, sign
	}
}

// emitWaveActiveOp emits dx.op.waveActiveOp.T(i32 119, T value, i8 op, i8 sign).
func (e *Emitter) emitWaveActiveOp(_ *ir.Function, valueID int, waveOp DXILWaveOp, sign DXILWaveOpSign, ol overloadType) int {
	waveFn := e.getWaveActiveOpFunc(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpWaveActiveOp))
	opVal := e.getI8ConstID(int64(waveOp))
	signVal := e.getI8ConstID(int64(sign))
	return e.addCallInstr(waveFn, retTy, []int{opcodeVal, valueID, opVal, signVal})
}

// emitWaveActiveBit emits dx.op.waveActiveBit.T(i32 120, T value, i8 op).
func (e *Emitter) emitWaveActiveBit(_ *ir.Function, valueID int, bitOp DXILWaveBitOp, ol overloadType) int {
	waveFn := e.getWaveActiveBitFunc(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpWaveActiveBit))
	opVal := e.getI8ConstID(int64(bitOp))
	return e.addCallInstr(waveFn, retTy, []int{opcodeVal, valueID, opVal})
}

// emitWavePrefixOp emits dx.op.wavePrefixOp.T(i32 121, T value, i8 op, i8 sign).
// Note: DXIL wavePrefixOp is exclusive prefix (does NOT include current lane).
func (e *Emitter) emitWavePrefixOp(_ *ir.Function, valueID int, waveOp DXILWaveOp, sign DXILWaveOpSign, ol overloadType) int {
	waveFn := e.getWavePrefixOpFunc(ol)
	retTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpWavePrefixOp))
	opVal := e.getI8ConstID(int64(waveOp))
	signVal := e.getI8ConstID(int64(sign))
	return e.addCallInstr(waveFn, retTy, []int{opcodeVal, valueID, opVal, signVal})
}

// combineForInclusive combines prefix result with current value to make an inclusive scan.
// inclusive = exclusive + current (for add/mul/min/max).
func (e *Emitter) combineForInclusive(currentID, prefixID int, op ir.SubgroupOperation, ol overloadType) int {
	retTy := e.overloadReturnType(ol)
	isFloat := ol == overloadF16 || ol == overloadF32 || ol == overloadF64
	switch op {
	case ir.SubgroupOperationAdd:
		if isFloat {
			return e.addBinOpInstr(retTy, BinOpFAdd, prefixID, currentID)
		}
		return e.addBinOpInstr(retTy, BinOpAdd, prefixID, currentID)
	case ir.SubgroupOperationMul:
		if isFloat {
			return e.addBinOpInstr(retTy, BinOpFMul, prefixID, currentID)
		}
		return e.addBinOpInstr(retTy, BinOpMul, prefixID, currentID)
	default:
		// For min/max, the inclusive and exclusive results differ but
		// we can't easily combine them. Return prefix as-is (best-effort).
		return prefixID
	}
}

// emitStmtSubgroupGather emits wave gather/shuffle operations.
//
//nolint:funlen // dispatch for multiple gather modes
func (e *Emitter) emitStmtSubgroupGather(fn *ir.Function, gather ir.StmtSubgroupGather) error {
	argID, err := e.emitExpression(fn, gather.Argument)
	if err != nil {
		return fmt.Errorf("StmtSubgroupGather: argument: %w", err)
	}

	argInner := e.resolveExprType(fn, gather.Argument)
	argScalar, _ := scalarAndComponentCount(argInner)
	ol := overloadForScalar(argScalar)

	var resultID int

	switch mode := gather.Mode.(type) {
	case ir.GatherBroadcastFirst:
		// dx.op.waveReadLaneFirst.T(i32 118, T value)
		waveFn := e.getWaveReadLaneFirstFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneFirst))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID})

	case ir.GatherBroadcast:
		// dx.op.waveReadLaneAt.T(i32 117, T value, i32 lane)
		laneID, err2 := e.emitExpression(fn, mode.Index)
		if err2 != nil {
			return fmt.Errorf("GatherBroadcast: lane: %w", err2)
		}
		waveFn := e.getWaveReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, laneID})

	case ir.GatherShuffle:
		laneID, err2 := e.emitExpression(fn, mode.Index)
		if err2 != nil {
			return fmt.Errorf("GatherShuffle: index: %w", err2)
		}
		waveFn := e.getWaveReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, laneID})

	case ir.GatherShuffleDown:
		// lane = WaveGetLaneIndex() + delta → WaveReadLaneAt
		deltaID, err2 := e.emitExpression(fn, mode.Delta)
		if err2 != nil {
			return fmt.Errorf("GatherShuffleDown: delta: %w", err2)
		}
		i32Ty := e.mod.GetIntType(32)
		laneIdxID := e.emitWaveGetLaneIndex()
		targetLane := e.addBinOpInstr(i32Ty, BinOpAdd, laneIdxID, deltaID)
		waveFn := e.getWaveReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, targetLane})

	case ir.GatherShuffleUp:
		// lane = WaveGetLaneIndex() - delta → WaveReadLaneAt
		deltaID, err2 := e.emitExpression(fn, mode.Delta)
		if err2 != nil {
			return fmt.Errorf("GatherShuffleUp: delta: %w", err2)
		}
		i32Ty := e.mod.GetIntType(32)
		laneIdxID := e.emitWaveGetLaneIndex()
		targetLane := e.addBinOpInstr(i32Ty, BinOpSub, laneIdxID, deltaID)
		waveFn := e.getWaveReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, targetLane})

	case ir.GatherShuffleXor:
		// lane = WaveGetLaneIndex() ^ mask → WaveReadLaneAt
		maskID, err2 := e.emitExpression(fn, mode.Mask)
		if err2 != nil {
			return fmt.Errorf("GatherShuffleXor: mask: %w", err2)
		}
		i32Ty := e.mod.GetIntType(32)
		laneIdxID := e.emitWaveGetLaneIndex()
		targetLane := e.addBinOpInstr(i32Ty, BinOpXor, laneIdxID, maskID)
		waveFn := e.getWaveReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpWaveReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, targetLane})

	case ir.GatherQuadBroadcast:
		// dx.op.quadReadLaneAt.T(i32 122, T value, i32 quadLane)
		laneID, err2 := e.emitExpression(fn, mode.Index)
		if err2 != nil {
			return fmt.Errorf("GatherQuadBroadcast: lane: %w", err2)
		}
		waveFn := e.getQuadReadLaneAtFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpQuadReadLaneAt))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, laneID})

	case ir.GatherQuadSwap:
		// dx.op.quadOp.T(i32 123, T value, i8 opKind)
		var qop DXILQuadOpKind
		switch mode.Direction {
		case ir.QuadDirectionX:
			qop = DXILQuadOpReadAcrossX
		case ir.QuadDirectionY:
			qop = DXILQuadOpReadAcrossY
		case ir.QuadDirectionDiagonal:
			qop = DXILQuadOpReadAcrossDiag
		}
		waveFn := e.getQuadOpFunc(ol)
		retTy := e.overloadReturnType(ol)
		opcodeVal := e.getIntConstID(int64(OpQuadOp))
		opVal := e.getI8ConstID(int64(qop))
		resultID = e.addCallInstr(waveFn, retTy, []int{opcodeVal, argID, opVal})

	default:
		return fmt.Errorf("unsupported gather mode: %T", gather.Mode)
	}

	e.exprValues[gather.Result] = resultID
	return nil
}

// emitWaveGetLaneIndex emits a dx.op.waveGetLaneIndex call and returns the value ID.
func (e *Emitter) emitWaveGetLaneIndex() int {
	i32Ty := e.mod.GetIntType(32)
	key := dxOpKey{name: "dx.op.waveGetLaneIndex", overload: overloadI32}
	fn, ok := e.dxOpFuncs[key]
	if !ok {
		params := []*module.Type{i32Ty}
		funcTy := e.mod.GetFunctionType(i32Ty, params)
		fn = e.mod.AddFunction("dx.op.waveGetLaneIndex", funcTy, true)
		e.dxOpFuncs[key] = fn
	}
	opcodeVal := e.getIntConstID(int64(OpWaveGetLaneIndex))
	return e.addCallInstr(fn, i32Ty, []int{opcodeVal})
}

// emitCastInstr emits an LLVM cast instruction.
func (e *Emitter) emitCastInstr(destTy *module.Type, castOp CastOpKind, srcID int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrCast,
		HasValue:   true,
		ResultType: destTy,
		Operands:   []int{srcID, int(castOp), destTy.ID},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// getBoolConstID returns the value ID for an i1 boolean constant.
func (e *Emitter) getBoolConstID(val bool) int {
	v := int64(0)
	if val {
		v = 1
	}
	i1Ty := e.mod.GetIntType(1)
	c := e.mod.AddIntConst(i1Ty, v)
	id := e.allocValue()
	e.constMap[id] = c
	return id
}

// Wave function declarations.

func (e *Emitter) getWaveBoolFunc(name string, _ DXILOpcode) *module.Function {
	key := dxOpKey{name: name, overload: overloadI1}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)
	params := []*module.Type{i32Ty, i1Ty}
	funcTy := e.mod.GetFunctionType(i1Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getWaveActiveOpFunc(ol overloadType) *module.Function {
	name := "dx.op.waveActiveOp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i8Ty, i8Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getWaveActiveBitFunc(ol overloadType) *module.Function {
	name := "dx.op.waveActiveBit"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i8Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getWavePrefixOpFunc(ol overloadType) *module.Function {
	name := "dx.op.wavePrefixOp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i8Ty, i8Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getWaveReadLaneAtFunc(ol overloadType) *module.Function {
	name := "dx.op.waveReadLaneAt"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i32Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getWaveReadLaneFirstFunc(ol overloadType) *module.Function {
	name := "dx.op.waveReadLaneFirst"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getQuadReadLaneAtFunc(ol overloadType) *module.Function {
	name := "dx.op.quadReadLaneAt"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i32Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getQuadOpFunc(ol overloadType) *module.Function {
	name := "dx.op.quadOp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	retTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, retTy, i8Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// ---------------------------------------------------------------------------
// Ray query operations (SM 6.5)
// ---------------------------------------------------------------------------

// emitStmtRayQuery emits DXIL ray query intrinsics.
//
// Ray query in DXIL uses a RayQuery handle allocated by dx.op.allocateRayQuery (178).
// The handle is stored in a local variable (alloca), and all subsequent ray query
// operations reference it.
//
// Reference: DXC DXIL.rst ray query opcodes 178-215
func (e *Emitter) emitStmtRayQuery(fn *ir.Function, rq ir.StmtRayQuery) error {
	switch rqf := rq.Fun.(type) {
	case ir.RayQueryInitialize:
		return e.emitRayQueryInitialize(fn, rq.Query, rqf)

	case ir.RayQueryProceed:
		return e.emitRayQueryProceed(fn, rq.Query, rqf)

	case ir.RayQueryTerminate:
		return e.emitRayQueryTerminate(fn, rq.Query)

	case ir.RayQueryGenerateIntersection:
		return e.emitRayQueryGenerateIntersection(fn, rq.Query, rqf)

	case ir.RayQueryConfirmIntersection:
		return e.emitRayQueryConfirmIntersection(fn, rq.Query)

	default:
		return fmt.Errorf("unsupported ray query function: %T", rq.Fun)
	}
}

// getRayQueryHandle returns or creates the ray query handle for a given expression.
// On first call, allocates a ray query via dx.op.allocateRayQuery(178).
func (e *Emitter) getRayQueryHandle(fn *ir.Function, queryExpr ir.ExpressionHandle) int {
	if id, ok := e.rayQueryHandles[queryExpr]; ok {
		return id
	}

	i32Ty := e.mod.GetIntType(32)

	// dx.op.allocateRayQuery(i32 178, i32 0) → i32 handle
	allocFn := e.getRayQueryAllocFunc()
	opcodeVal := e.getIntConstID(int64(OpAllocateRayQuery))
	flagsVal := e.getIntConstID(0) // RAY_FLAG_NONE for allocation
	handleID := e.addCallInstr(allocFn, i32Ty, []int{opcodeVal, flagsVal})

	if e.rayQueryHandles == nil {
		e.rayQueryHandles = make(map[ir.ExpressionHandle]int)
	}
	e.rayQueryHandles[queryExpr] = handleID
	_ = fn
	return handleID
}

// emitRayQueryInitialize emits dx.op.rayQuery_TraceRayInline (179).
// Signature: void @dx.op.rayQuery_TraceRayInline(i32 179, i32 rayQueryHandle,
//
//	%dx.types.Handle accelStruct, i32 rayFlags, i32 instanceMask,
//	float originX, float originY, float originZ,
//	float tMin, float dirX, float dirY, float dirZ, float tMax)
func (e *Emitter) emitRayQueryInitialize(fn *ir.Function, queryExpr ir.ExpressionHandle, init ir.RayQueryInitialize) error {
	handleID := e.getRayQueryHandle(fn, queryExpr)

	// Resolve acceleration structure handle.
	asHandleID, err := e.resolveResourceHandle(fn, init.AccelerationStructure)
	if err != nil {
		return fmt.Errorf("RayQueryInitialize: accel struct: %w", err)
	}

	// The descriptor is a RayDesc struct with fields:
	// { flags: u32, cull_mask: u32, t_min: f32, t_max: f32, origin: vec3<f32>, dir: vec3<f32> }
	if _, err := e.emitExpression(fn, init.Descriptor); err != nil {
		return fmt.Errorf("RayQueryInitialize: descriptor: %w", err)
	}

	// Extract struct fields from the descriptor.
	flagsID := e.getComponentID(init.Descriptor, 0)
	cullMaskID := e.getComponentID(init.Descriptor, 1)
	tMinID := e.getComponentID(init.Descriptor, 2)
	tMaxID := e.getComponentID(init.Descriptor, 3)
	originXID := e.getComponentID(init.Descriptor, 4)
	originYID := e.getComponentID(init.Descriptor, 5)
	originZID := e.getComponentID(init.Descriptor, 6)
	dirXID := e.getComponentID(init.Descriptor, 7)
	dirYID := e.getComponentID(init.Descriptor, 8)
	dirZID := e.getComponentID(init.Descriptor, 9)

	traceFn := e.getRayQueryTraceFunc()
	voidTy := e.mod.GetVoidType()
	opcodeVal := e.getIntConstID(int64(OpRayQueryTraceRayInline))

	e.addCallInstr(traceFn, voidTy, []int{
		opcodeVal, handleID, asHandleID,
		flagsID, cullMaskID,
		originXID, originYID, originZID,
		tMinID,
		dirXID, dirYID, dirZID,
		tMaxID,
	})
	return nil
}

// emitRayQueryProceed emits dx.op.rayQuery_Proceed (180).
// Signature: i1 @dx.op.rayQuery_Proceed(i32 180, i32 rayQueryHandle)
func (e *Emitter) emitRayQueryProceed(fn *ir.Function, queryExpr ir.ExpressionHandle, proceed ir.RayQueryProceed) error {
	handleID := e.getRayQueryHandle(fn, queryExpr)

	proceedFn := e.getRayQueryProceedFunc()
	i1Ty := e.mod.GetIntType(1)
	opcodeVal := e.getIntConstID(int64(OpRayQueryProceed))
	resultID := e.addCallInstr(proceedFn, i1Ty, []int{opcodeVal, handleID})

	e.exprValues[proceed.Result] = resultID
	return nil
}

// emitRayQueryTerminate emits dx.op.rayQuery_Abort (181).
func (e *Emitter) emitRayQueryTerminate(fn *ir.Function, queryExpr ir.ExpressionHandle) error {
	handleID := e.getRayQueryHandle(fn, queryExpr)

	abortFn := e.getRayQueryAbortFunc()
	voidTy := e.mod.GetVoidType()
	opcodeVal := e.getIntConstID(int64(OpRayQueryAbort))
	e.addCallInstr(abortFn, voidTy, []int{opcodeVal, handleID})
	return nil
}

// emitRayQueryGenerateIntersection emits dx.op.rayQuery_CommitProceduralPrimitiveHit (183).
func (e *Emitter) emitRayQueryGenerateIntersection(fn *ir.Function, queryExpr ir.ExpressionHandle, gi ir.RayQueryGenerateIntersection) error {
	handleID := e.getRayQueryHandle(fn, queryExpr)

	hitTID, err := e.emitExpression(fn, gi.HitT)
	if err != nil {
		return fmt.Errorf("RayQueryGenerateIntersection: hitT: %w", err)
	}

	commitFn := e.getRayQueryCommitProceduralFunc()
	voidTy := e.mod.GetVoidType()
	opcodeVal := e.getIntConstID(int64(OpRayQueryCommitProceduralPrimitiveHit))
	e.addCallInstr(commitFn, voidTy, []int{opcodeVal, handleID, hitTID})
	return nil
}

// emitRayQueryConfirmIntersection emits dx.op.rayQuery_CommitNonOpaqueTriangleHit (182).
func (e *Emitter) emitRayQueryConfirmIntersection(fn *ir.Function, queryExpr ir.ExpressionHandle) error {
	handleID := e.getRayQueryHandle(fn, queryExpr)

	commitFn := e.getRayQueryCommitTriangleFunc()
	voidTy := e.mod.GetVoidType()
	opcodeVal := e.getIntConstID(int64(OpRayQueryCommitNonOpaqueTriangleHit))
	e.addCallInstr(commitFn, voidTy, []int{opcodeVal, handleID})
	return nil
}

// emitRayQueryGetIntersection emits various dx.op.rayQuery_* intrinsics to build
// the RayIntersection struct from query results.
//
// This is called as an expression (ExprRayQueryGetIntersection), not a statement.
// It must produce a composite struct result with all intersection fields.
//
//nolint:funlen // many intersection struct fields to extract
func (e *Emitter) emitRayQueryGetIntersection(fn *ir.Function, gi ir.ExprRayQueryGetIntersection) (int, error) {
	handleID := e.getRayQueryHandle(fn, gi.Query)

	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	i1Ty := e.mod.GetIntType(1)

	// Choose committed vs candidate opcodes.
	var (
		statusOp, instanceIndexOp, instanceIDOp DXILOpcode
		geometryIndexOp, primitiveIndexOp       DXILOpcode
		objectRayOriginOp, objectRayDirOp       DXILOpcode
		rayTOp                                  DXILOpcode
		frontFaceOp                             DXILOpcode
		baryCentricOp                           DXILOpcode
		obj2worldOp, world2objOp                DXILOpcode
	)
	if gi.Committed {
		statusOp = OpRayQueryCommittedStatus
		instanceIndexOp = OpRayQueryCommittedInstanceIndex
		instanceIDOp = OpRayQueryCommittedInstanceID
		geometryIndexOp = OpRayQueryCommittedGeometryIndex
		primitiveIndexOp = OpRayQueryCommittedPrimitiveIndex
		objectRayOriginOp = OpRayQueryCommittedObjectRayOrigin
		objectRayDirOp = OpRayQueryCommittedObjectRayDirection
		rayTOp = OpRayQueryCommittedRayT
		frontFaceOp = OpRayQueryCommittedTriangleFrontFace
		baryCentricOp = OpRayQueryCommittedTriangleBarycentrics
		obj2worldOp = OpRayQueryCommittedObjectToWorld3x4
		world2objOp = OpRayQueryCommittedWorldToObject3x4
	} else {
		statusOp = OpRayQueryCandidateType
		instanceIndexOp = OpRayQueryCandidateInstanceIndex
		instanceIDOp = OpRayQueryCandidateInstanceID
		geometryIndexOp = OpRayQueryCandidateGeometryIndex
		primitiveIndexOp = OpRayQueryCandidatePrimitiveIndex
		objectRayOriginOp = OpRayQueryCandidateObjectRayOrigin
		objectRayDirOp = OpRayQueryCandidateObjectRayDirection
		rayTOp = OpRayQueryCandidateTriangleRayT
		frontFaceOp = OpRayQueryCandidateTriangleFrontFace
		baryCentricOp = OpRayQueryCandidateTriangleBarycentrics
		obj2worldOp = OpRayQueryCandidateObjectToWorld3x4
		world2objOp = OpRayQueryCandidateWorldToObject3x4
	}

	// Helper: emit a scalar ray query call returning i32.
	emitI32 := func(op DXILOpcode) int {
		rqFn := e.getRayQueryScalarI32Func(op)
		opcodeVal := e.getIntConstID(int64(op))
		return e.addCallInstr(rqFn, i32Ty, []int{opcodeVal, handleID})
	}
	// Helper: emit a scalar ray query call returning f32.
	emitF32 := func(op DXILOpcode) int {
		rqFn := e.getRayQueryScalarF32Func(op)
		opcodeVal := e.getIntConstID(int64(op))
		return e.addCallInstr(rqFn, f32Ty, []int{opcodeVal, handleID})
	}
	// Helper: emit a component ray query call returning f32 for a vec3.
	// Component index is i8 per dxc rayQuery_StateVector signature.
	emitVec3F32 := func(op DXILOpcode) (int, int, int) {
		rqFn := e.getRayQueryCompF32Func(op)
		opcodeVal := e.getIntConstID(int64(op))
		c0 := e.addCallInstr(rqFn, f32Ty, []int{opcodeVal, handleID, e.getI8ConstID(0)})
		c1 := e.addCallInstr(rqFn, f32Ty, []int{opcodeVal, handleID, e.getI8ConstID(1)})
		c2 := e.addCallInstr(rqFn, f32Ty, []int{opcodeVal, handleID, e.getI8ConstID(2)})
		return c0, c1, c2
	}
	// Helper: emit 4x3 matrix (returns 12 f32 components).
	// Row is i32, col is i8 per dxc rayQuery_StateMatrix signature.
	emitMat4x3 := func(op DXILOpcode) [12]int {
		rqFn := e.getRayQueryMatrixFunc(op)
		opcodeVal := e.getIntConstID(int64(op))
		var comps [12]int
		for row := 0; row < 4; row++ {
			for col := 0; col < 3; col++ {
				rowVal := e.getIntConstID(int64(row))
				colVal := e.getI8ConstID(int64(col))
				comps[row*3+col] = e.addCallInstr(rqFn, f32Ty, []int{opcodeVal, handleID, rowVal, colVal})
			}
		}
		return comps
	}

	// RayIntersection struct layout:
	// kind: u32, t: f32, instance_custom_data: u32, instance_index: u32,
	// sbt_record_offset: u32, geometry_index: u32, primitive_index: u32,
	// barycentrics: vec2<f32>, front_face: bool,
	// object_to_world: mat4x3<f32>, world_to_object: mat4x3<f32>
	// Total: 7 scalars + 2 bary + 1 bool + 12 + 12 = 34 components
	comps := make([]int, 0, 34)

	// barycentrics (vec2<f32>)
	baryFn := e.getRayQueryCompF32Func(baryCentricOp)
	baryOp := e.getIntConstID(int64(baryCentricOp))
	baryX := e.addCallInstr(baryFn, f32Ty, []int{baryOp, handleID, e.getI8ConstID(0)})
	baryY := e.addCallInstr(baryFn, f32Ty, []int{baryOp, handleID, e.getI8ConstID(1)})

	// front_face (bool -> i1)
	frontFaceFn := e.getRayQueryBoolFunc(frontFaceOp)
	frontFaceOpVal := e.getIntConstID(int64(frontFaceOp))
	frontFaceID := e.addCallInstr(frontFaceFn, i1Ty, []int{frontFaceOpVal, handleID})

	// object_to_world (mat4x3 = 12 floats)
	o2w := emitMat4x3(obj2worldOp)
	// world_to_object (mat4x3 = 12 floats)
	w2o := emitMat4x3(world2objOp)

	// Build flat component array.
	comps = append(comps,
		emitI32(statusOp),         // kind
		emitF32(rayTOp),           // t
		emitI32(instanceIDOp),     // instance_custom_data
		emitI32(instanceIndexOp),  // instance_index
		e.getIntConstID(0),        // sbt_record_offset (unavailable in inline ray query)
		emitI32(geometryIndexOp),  // geometry_index
		emitI32(primitiveIndexOp), // primitive_index
		baryX, baryY,              // barycentrics
		frontFaceID, // front_face
	)
	for _, c := range o2w {
		comps = append(comps, c)
	}
	for _, c := range w2o {
		comps = append(comps, c)
	}

	_ = emitVec3F32
	_ = objectRayOriginOp
	_ = objectRayDirOp

	e.pendingComponents = comps
	if len(comps) > 0 {
		return comps[0], nil
	}
	return 0, fmt.Errorf("ray query intersection produced no components")
}

// Ray query function declarations.

func (e *Emitter) getRayQueryAllocFunc() *module.Function {
	name := "dx.op.allocateRayQuery"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i32Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryTraceFunc() *module.Function {
	name := "dx.op.rayQuery_TraceRayInline"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	handleTy := e.getDxHandleType()
	voidTy := e.mod.GetVoidType()
	// (i32 opcode, i32 rqHandle, %handle accelStruct, i32 rayFlags, i32 instanceMask,
	//  f32 origX, f32 origY, f32 origZ, f32 tMin, f32 dirX, f32 dirY, f32 dirZ, f32 tMax)
	params := []*module.Type{
		i32Ty, i32Ty, handleTy, i32Ty, i32Ty,
		f32Ty, f32Ty, f32Ty, f32Ty, f32Ty, f32Ty, f32Ty, f32Ty,
	}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryProceedFunc() *module.Function {
	// DXC (DxilOperations.cpp:1614) declares rayQuery_Proceed with OpClass
	// "rayQuery_Proceed" and overload mask 0x8 = i1 (bool). Function symbol
	// is dx.op.rayQuery_Proceed.i1 per OP::ConstructOverloadName:
	// "dx.op." + className + "." + overloadTypeName.
	name := "dx.op.rayQuery_Proceed"
	key := dxOpKey{name: name, overload: overloadI1}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i1Ty, params)
	fullName := name + overloadSuffix(overloadI1)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryAbortFunc() *module.Function {
	name := "dx.op.rayQuery_Abort"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	voidTy := e.mod.GetVoidType()
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryCommitTriangleFunc() *module.Function {
	name := "dx.op.rayQuery_CommitNonOpaqueTriangleHit"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	voidTy := e.mod.GetVoidType()
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryCommitProceduralFunc() *module.Function {
	name := "dx.op.rayQuery_CommitProceduralPrimitiveHit"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	voidTy := e.mod.GetVoidType()
	params := []*module.Type{i32Ty, i32Ty, f32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryScalarI32Func(_ DXILOpcode) *module.Function {
	// All RayQuery state-scalar-int opcodes share the OCC::RayQuery_StateScalar
	// op class. The opcode is encoded in the call's first i32 argument; the
	// function symbol is dx.op.rayQuery_StateScalar.<overload>.
	name := "dx.op.rayQuery_StateScalar.i32"
	key := dxOpKey{name: name, overload: overloadI32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i32Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryScalarF32Func(_ DXILOpcode) *module.Function {
	name := "dx.op.rayQuery_StateScalar.f32"
	key := dxOpKey{name: name, overload: overloadF32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(f32Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryCompF32Func(_ DXILOpcode) *module.Function {
	// Vector-component state queries share OCC::RayQuery_StateVector.
	// Signature per dxc DxilOperations.cpp:5749 (RayQuery_WorldRayOrigin):
	//   ret f32, args (i32 opcode, i32 rqHandle, i8 componentIdx)
	name := "dx.op.rayQuery_StateVector.f32"
	key := dxOpKey{name: name, overload: overloadF32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	f32Ty := e.mod.GetFloatType(32)
	params := []*module.Type{i32Ty, i32Ty, i8Ty}
	funcTy := e.mod.GetFunctionType(f32Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryBoolFunc(_ DXILOpcode) *module.Function {
	name := "dx.op.rayQuery_StateScalar.i1"
	key := dxOpKey{name: name, overload: overloadI1}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)
	params := []*module.Type{i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i1Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

func (e *Emitter) getRayQueryMatrixFunc(_ DXILOpcode) *module.Function {
	// Matrix state queries share OCC::RayQuery_StateMatrix.
	// Signature per dxc DxilOperations.cpp (RayQuery_CandidateObjectToWorld3x4):
	//   ret f32, args (i32 opcode, i32 rqHandle, i32 row, i8 col)
	name := "dx.op.rayQuery_StateMatrix.f32"
	key := dxOpKey{name: name, overload: overloadF32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	f32Ty := e.mod.GetFloatType(32)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty}
	funcTy := e.mod.GetFunctionType(f32Ty, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}
