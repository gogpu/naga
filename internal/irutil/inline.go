package irutil

import (
	"fmt"
	"slices"

	"github.com/gogpu/naga/ir"
)

// InlineUserFunctions rewrites the module so that every user-defined helper
// function called from an entry point (directly or transitively) is expanded
// inline at its call site. After the pass, entry-point function bodies
// contain no StmtCall to user helpers; the Functions[] array is preserved
// (the handles it holds remain valid for any lingering references) but no
// StmtCall statement targets it.
//
// Enterprise rationale:
//
// DXIL's bitcode validator and several backend code paths do not support
// user helper functions in full generality — specifically, functions that
// access module globals, return aggregate types, or contain complex local
// variable shapes. DXC resolves this by running LLVM's AlwaysInliner as a
// post-emit pass (DxilLinker.cpp:1248, createAlwaysInlinerPass); Mesa runs
// nir_inline_functions as a NIR pre-pass before nir_to_dxil. We mirror the
// Mesa approach at the naga IR level: transform the module once, then let
// each backend emit from the simplified IR.
//
// Phase 1 covers:
//   - Helpers with single tail return (StmtReturn at the very end of the
//     top-level block, or void return with no explicit StmtReturn at all)
//   - Any argument / return type, including aggregates
//   - Any expression kind (handles are remapped into the caller's expression
//     array)
//   - Any statement kind except nested StmtCall inside a helper body; those
//     are handled by topological processing — callees are inlined first so
//     by the time a caller reaches them, they contain no nested StmtCall
//   - Globals and constants: no remap needed, they are module-scoped and
//     stay referenced from the inlined expressions verbatim
//
// Phase 2 (future) will add:
//   - Early returns via loop-break wrap (same transform DXC's AlwaysInliner
//     applies when a callee has multiple return sites)
//   - Mutual recursion detection (WGSL forbids recursion, but defense in
//     depth keeps the pass hardened against malformed IR)
//
// The pass is idempotent: running it twice is a no-op on the second run
// because after the first pass no StmtCall targets a user helper.
func InlineUserFunctions(module *ir.Module, shouldInline func(callee *ir.Function) bool) error {
	if module == nil {
		return nil
	}
	if shouldInline == nil {
		shouldInline = func(*ir.Function) bool { return true }
	}

	// Build call graph so we can process callees before callers. A helper
	// that itself calls another helper must have its own body flattened
	// first, otherwise the outer caller would inherit the nested StmtCall.
	order, err := topologicalCallOrder(module)
	if err != nil {
		return err
	}

	// Phase 1: inline inside each non-entry-point function, bottom-up.
	// After this loop, every Functions[i] that is reachable has no
	// StmtCall to a callee that passes the shouldInline policy.
	for _, fh := range order {
		if int(fh) >= len(module.Functions) {
			continue
		}
		if err := inlineCallsInFunction(module, &module.Functions[fh], shouldInline); err != nil {
			return fmt.Errorf("inline in %q: %w", module.Functions[fh].Name, err)
		}
	}

	// Phase 2: inline inside each entry point.
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		if err := inlineCallsInFunction(module, &ep.Function, shouldInline); err != nil {
			return fmt.Errorf("inline in entry point %q: %w", ep.Name, err)
		}
	}

	return nil
}

// topologicalCallOrder returns Functions[] handles in bottom-up order
// (leaves first). Errors if the call graph contains a cycle.
func topologicalCallOrder(module *ir.Module) ([]ir.FunctionHandle, error) {
	visited := make(map[ir.FunctionHandle]int) // 0 = unseen, 1 = in progress, 2 = done
	var order []ir.FunctionHandle

	var visit func(h ir.FunctionHandle) error
	visit = func(h ir.FunctionHandle) error {
		if int(h) >= len(module.Functions) {
			return nil
		}
		switch visited[h] {
		case 2:
			return nil
		case 1:
			return fmt.Errorf("function call cycle involving %q", module.Functions[h].Name)
		}
		visited[h] = 1
		for _, callee := range collectCalleeHandles(module.Functions[h].Body) {
			if err := visit(callee); err != nil {
				return err
			}
		}
		visited[h] = 2
		order = append(order, h)
		return nil
	}

	for i := range module.Functions {
		if err := visit(ir.FunctionHandle(i)); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// collectCalleeHandles walks a statement block and returns the set of
// FunctionHandles that StmtCall statements target.
func collectCalleeHandles(stmts ir.Block) []ir.FunctionHandle {
	seen := make(map[ir.FunctionHandle]struct{})
	var walk func(b ir.Block)
	walk = func(b ir.Block) {
		for i := range b {
			switch sk := b[i].Kind.(type) {
			case ir.StmtCall:
				seen[sk.Function] = struct{}{}
			case ir.StmtBlock:
				walk(sk.Block)
			case ir.StmtIf:
				walk(sk.Accept)
				walk(sk.Reject)
			case ir.StmtLoop:
				walk(sk.Body)
				walk(sk.Continuing)
			case ir.StmtSwitch:
				for j := range sk.Cases {
					walk(sk.Cases[j].Body)
				}
			}
		}
	}
	walk(stmts)
	out := make([]ir.FunctionHandle, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out
}

// inlineCallsInFunction walks the function body, replacing each StmtCall
// to a user helper with the expanded body of the callee. The caller's
// Expressions, LocalVars, and NamedExpressions are grown to hold the
// inlined content with properly remapped handles.
func inlineCallsInFunction(module *ir.Module, caller *ir.Function, shouldInline func(*ir.Function) bool) error {
	newBody, changed, err := inlineBlock(module, caller, caller.Body, shouldInline)
	if err != nil {
		return err
	}
	if changed {
		caller.Body = newBody
	}
	return nil
}

// inlineBlock walks a statement block and returns a new block with any
// StmtCall expanded. The `changed` flag is true iff at least one
// substitution happened.
func inlineBlock(module *ir.Module, caller *ir.Function, block ir.Block, shouldInline func(*ir.Function) bool) (ir.Block, bool, error) {
	out := make(ir.Block, 0, len(block))
	changed := false
	for _, stmt := range block {
		var stmtChanged bool
		var err error
		switch sk := stmt.Kind.(type) {
		case ir.StmtCall:
			out, stmtChanged, err = inlineCall(module, caller, out, stmt, sk, shouldInline)
		case ir.StmtBlock, ir.StmtIf, ir.StmtLoop, ir.StmtSwitch:
			stmt, stmtChanged, err = inlineNested(module, caller, stmt, shouldInline)
			out = append(out, stmt)
		default:
			out = append(out, stmt)
		}
		if err != nil {
			return nil, false, err
		}
		changed = changed || stmtChanged
	}
	return out, changed, nil
}

// inlineCall appends to out the expansion of call, or stmt itself when its
// callee is not a module function or the policy keeps the call.
func inlineCall(module *ir.Module, caller *ir.Function, out ir.Block, stmt ir.Statement, call ir.StmtCall, shouldInline func(*ir.Function) bool) (ir.Block, bool, error) {
	if int(call.Function) >= len(module.Functions) {
		return append(out, stmt), false, nil
	}
	callee := &module.Functions[call.Function]
	if !shouldInline(callee) {
		return append(out, stmt), false, nil
	}
	inlined, err := inlineOneCall(module, caller, call, callee)
	if err != nil {
		return nil, false, err
	}
	return append(out, inlined...), true, nil
}

// inlineNested inlines the calls in the blocks of a block, if, loop or
// switch statement; it returns stmt itself when none changed.
func inlineNested(module *ir.Module, caller *ir.Function, stmt ir.Statement, shouldInline func(*ir.Function) bool) (ir.Statement, bool, error) {
	block := func(b ir.Block) (ir.Block, bool, error) { return inlineBlock(module, caller, b, shouldInline) }
	switch sk := stmt.Kind.(type) {
	case ir.StmtBlock:
		newInner, changed, err := block(sk.Block)
		if err != nil || !changed {
			return stmt, false, err
		}
		return ir.Statement{Kind: ir.StmtBlock{Block: newInner}}, true, nil
	case ir.StmtIf:
		newAccept, a, err := block(sk.Accept)
		if err != nil {
			return stmt, false, err
		}
		newReject, r, err := block(sk.Reject)
		if err != nil || !a && !r {
			return stmt, false, err
		}
		return ir.Statement{Kind: ir.StmtIf{Condition: sk.Condition, Accept: newAccept, Reject: newReject}}, true, nil
	case ir.StmtLoop:
		newBody, b, err := block(sk.Body)
		if err != nil {
			return stmt, false, err
		}
		newCont, c, err := block(sk.Continuing)
		if err != nil || !b && !c {
			return stmt, false, err
		}
		return ir.Statement{Kind: ir.StmtLoop{Body: newBody, Continuing: newCont, BreakIf: sk.BreakIf}}, true, nil
	case ir.StmtSwitch:
		newCases := make([]ir.SwitchCase, len(sk.Cases))
		caseChanged := false
		for j := range sk.Cases {
			newCaseBody, bc, err := block(sk.Cases[j].Body)
			if err != nil {
				return stmt, false, err
			}
			newCases[j] = ir.SwitchCase{Value: sk.Cases[j].Value, Body: newCaseBody, FallThrough: sk.Cases[j].FallThrough}
			caseChanged = caseChanged || bc
		}
		if !caseChanged {
			return stmt, false, nil
		}
		return ir.Statement{Kind: ir.StmtSwitch{Selector: sk.Selector, Cases: newCases}}, true, nil
	default:
		return stmt, false, nil
	}
}

// inlineOneCall expands a single StmtCall into a sequence of statements
// that the caller can execute in place of the call. Responsible for:
//   - Appending callee's LocalVars to caller (with index remap).
//   - Appending callee's Expressions to caller (with handle remap that
//     rewrites ExprLocalVariable, ExprFunctionArgument, and nested
//     handles).
//   - Creating a return-slot local if the callee has a non-void return,
//     and rewriting StmtReturn in the copy to store into the slot.
//   - Rewriting the caller's ExprCallResult expression (call.Result) to
//     load from the return slot.
//
// Phase 1 restriction: assumes the callee's top-level body ends with a
// single StmtReturn (or is void) and contains no StmtCall (guaranteed by
// topological processing — we inline callees before callers).
func inlineOneCall(module *ir.Module, caller *ir.Function, call ir.StmtCall, callee *ir.Function) ([]ir.Statement, error) {
	// The statements put in place of the call start with one StmtStore per
	// spilled argument, materializing the caller-side value into a
	// freshly-allocated local so the inlined body reads the arg via ExprLoad
	// of a local variable. Mirrors LLVM AlwaysInliner / Mesa
	// nir_inline_functions which both spill args into temporaries.
	//
	// Layout of caller.Expressions after this call returns:
	//
	//   [pre-existing caller exprs ......................]
	//   [arg-spill-ptr expressions, one per call arg ....]  <- destinations for arg-spill stores
	//   [arg-load expressions, two per arg (ptr + Load) .]  <- callee's ExprFunctionArgument(i) → arg-load expr
	//   [calleeExprMap range — one slot per callee expr .]  <- contiguous; StmtEmit ranges remap linearly
	//   [return-slot expressions (ptr + Load), if any ...]
	//
	// Keeping the calleeExprMap range a single contiguous block is what
	// makes StmtEmit Range remapping correct: an emit range [start, end)
	// in the callee maps to [calleeExprMap[start], calleeExprMap[end-1]+1)
	// in the caller. If we interleaved helper expressions inside the
	// calleeExprMap range, the Range.End boundary would no longer line up.
	if len(call.Arguments) != len(callee.Arguments) {
		return nil, fmt.Errorf("callee %q expects %d arguments but call provides %d",
			callee.Name, len(callee.Arguments), len(call.Arguments))
	}

	// 1. Remap callee's locals into caller's LocalVars.
	localOffset := appendCalleeLocals(caller, callee)

	// 2 and 3. Spill or alias each argument.
	args, err := materializeArguments(module, caller, call, callee)
	if err != nil {
		return nil, err
	}

	// 4. Reserve a contiguous slot per callee expression. StmtEmit ranges
	// will remap linearly through calleeExprMap, with End handled as an
	// exclusive boundary (see step 8).
	calleeExprMap := reserveCalleeExpressions(caller, callee)
	exprRefs := args.expressionRefs(callee, calleeExprMap)

	// 5. Fill in the reserved slots with remapped content.
	if err := fillCalleeExpressions(caller, callee, calleeExprMap, exprRefs, args.load, localOffset); err != nil {
		return nil, err
	}
	remapCalleeLocalInits(caller, callee, localOffset, exprRefs)
	localInits := initCalleeLocals(module, caller, callee, localOffset)

	// 6. Copy callee's NamedExpressions into caller with remapped handles.
	copyNamedExpressions(caller, callee, calleeExprMap)

	// 7. Handle the return slot.
	retPtr, retLoadExpr := addReturnSlot(caller, callee)

	// 8. Remap a copy of the callee body into the caller's handle space; the
	// callee stays in the module for its other callers. An Emit range
	// [start, end) maps to [exprRefs[start], exprRefs[end-1]+1), which is
	// right because the callee's expressions map to one contiguous run of
	// caller slots (step 4); the aliased arguments that exprRefs points
	// outside that run are function arguments, which no Emit covers.
	rm := remapThrough(exprRefs)
	bodyMapper := blockMapper{handle: rm, emit: adjustEmitRange(rm), fresh: true}
	inlinedBody := bodyMapper.block(callee.Body)

	// 8b. Emit coverage for scalar-spilled arguments, then the callee's
	// locals start afresh (initCalleeLocals).
	prefixStmts := args.spills
	prefixStmts = append(prefixStmts, args.spilledArgumentEmits(callee, calleeExprMap)...)
	prefixStmts = append(prefixStmts, localInits...)

	// 9. Rewrite StmtReturn in the inlined body so control reaches the code
	// after the call (see lowerReturns).
	lowering := returnLowering{module: module, caller: caller, flagName: "_inline_returned_" + callee.Name, retPtr: retPtr}
	inlinedBody, err = lowering.lower(inlinedBody)
	if err != nil {
		return nil, fmt.Errorf("callee %q: %w", callee.Name, err)
	}

	// 10. Wire the caller's ExprCallResult to load from the return slot.
	inlinedBody = wireCallResult(caller, call, retLoadExpr, inlinedBody)

	return append(prefixStmts, inlinedBody...), nil
}

// appendCalleeLocals appends callee's local variables to caller's and
// returns the handle the first one gets. An initializer still names a callee
// expression; remapCalleeLocalInits fixes it once the callee's expressions
// have their caller handles.
func appendCalleeLocals(caller, callee *ir.Function) ir.LocalVariableHandle {
	offset := ir.LocalVariableHandle(ArenaIndex(len(caller.LocalVars)))
	for i := range callee.LocalVars {
		lv := callee.LocalVars[i]
		if lv.Init != nil {
			init := *lv.Init
			lv.Init = &init
		}
		caller.LocalVars = append(caller.LocalVars, lv)
	}
	return offset
}

// initCalleeLocals returns, for the n locals the callee brought into caller
// from localOffset, a store of each one's initializer, or of its zero value
// without one, and drops the initializer from the caller's copy. WGSL creates
// a function's variables afresh at every call; a caller-level local with an
// initializer is set once, at the caller's entry, so a call inside a loop saw
// the value the previous iteration left (P0-DXIL-046 follow-up). This is what
// the WGSL lowerer does for a variable declared inside a loop, and what Mesa's
// nir_inline_functions gets from lowering temporaries' initializers to stores
// (nir_lower_variable_initializers) before inlining.
//
// Every scalar and vector local is stored: they are what the DXIL mem2reg
// used to promote from the initial value in every call. A struct, array or
// matrix local is stored when the callee writes it (in whole or in part);
// one it never writes holds its initializer, or zero, at every call anyway,
// and a store would only keep it in memory. Opaque locals such as a ray
// query have no value to store. A local without an initializer that the
// callee's straight-line start stores before it reads it (the lowerer's
// store of a run-time initializer at the declaration) needs no zero store:
// it would only cost an alloca.
func initCalleeLocals(module *ir.Module, caller, callee *ir.Function, localOffset ir.LocalVariableHandle) []ir.Statement {
	storedFirst := storedBeforeRead(callee)
	written := writtenLocals(callee)
	stmts := make([]ir.Statement, 0, len(callee.LocalVars))
	for i := range callee.LocalVars {
		h := localOffset + ir.LocalVariableHandle(ArenaIndex(i))
		lv := &caller.LocalVars[h]
		switch module.Types[lv.Type].Inner.(type) {
		case ir.ScalarType, ir.VectorType:
		default:
			if !written[ir.LocalVariableHandle(ArenaIndex(i))] || !constructible(module, lv.Type) {
				continue
			}
		}
		if lv.Init == nil && storedFirst[ir.LocalVariableHandle(ArenaIndex(i))] {
			continue
		}
		var value ir.ExpressionHandle
		if lv.Init != nil {
			value = *lv.Init
			lv.Init = nil
		} else {
			value = appendExpression(caller, ir.ExprZeroValue{Type: lv.Type})
		}
		ptr := appendExpression(caller, ir.ExprLocalVariable{Variable: h})
		stmts = append(stmts, ir.Statement{Kind: ir.StmtStore{Pointer: ptr, Value: value}})
	}
	return stmts
}

// storedBeforeRead returns the locals of fn that a top-level Store of its
// body writes, whole, before anything mentions them. Any other statement (an
// if, a loop, a call) that mentions a local, at any depth, settles it as not
// stored first, since the order inside is not tracked; a statement that does
// not mention it is skipped. An unlisted local gets its zero store.
func storedBeforeRead(fn *ir.Function) map[ir.LocalVariableHandle]bool {
	stored := make(map[ir.LocalVariableHandle]bool)
	seen := make(map[ir.LocalVariableHandle]bool)
	localOf := func(h ir.ExpressionHandle) (ir.LocalVariableHandle, bool) {
		if int(h) >= len(fn.Expressions) {
			return 0, false
		}
		lv, ok := fn.Expressions[h].Kind.(ir.ExprLocalVariable)
		return lv.Variable, ok
	}
	for _, stmt := range fn.Body {
		switch sk := stmt.Kind.(type) {
		case ir.StmtEmit:
			for h := sk.Range.Start; h < sk.Range.End; h++ {
				// Any use of a local's pointer in an emitted expression (a
				// load, or an access into it) counts as a read.
				ExprHandles(fn.Expressions[h].Kind, func(op ir.ExpressionHandle) {
					if v, ok := localOf(op); ok {
						seen[v] = true
					}
				})
			}
		case ir.StmtStore:
			if v, ok := localOf(sk.Pointer); ok {
				if !seen[v] {
					stored[v] = true
				}
				seen[v] = true
			}
		default:
			mentionedLocals(fn, ir.Block{stmt}, localOf, func(v ir.LocalVariableHandle) { seen[v] = true })
		}
	}
	return stored
}

// writtenLocals returns the locals of fn that a statement at any depth may
// write: the target of a Store, whole or through an access chain, and a local
// passed by pointer to a call the inliner kept.
func writtenLocals(fn *ir.Function) map[ir.LocalVariableHandle]bool {
	written := make(map[ir.LocalVariableHandle]bool)
	mark := func(h ir.ExpressionHandle) {
		if v, ok := rootLocal(fn, h); ok {
			written[v] = true
		}
	}
	var walk func(b ir.Block)
	walk = func(b ir.Block) {
		for i := range b {
			switch sk := b[i].Kind.(type) {
			case ir.StmtStore:
				mark(sk.Pointer)
			case ir.StmtCall:
				for _, a := range sk.Arguments {
					mark(a)
				}
			}
			NestedBlocks(b[i].Kind, walk)
		}
	}
	walk(fn.Body)
	return written
}

// rootLocal returns the local variable a pointer expression points into,
// through any chain of Access and AccessIndex.
func rootLocal(fn *ir.Function, h ir.ExpressionHandle) (ir.LocalVariableHandle, bool) {
	for int(h) < len(fn.Expressions) {
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprLocalVariable:
			return k.Variable, true
		case ir.ExprAccess:
			h = k.Base
		case ir.ExprAccessIndex:
			h = k.Base
		default:
			return 0, false
		}
	}
	return 0, false
}

// constructible reports whether ty is a type with a value a store can write:
// scalars, vectors, matrices, and fixed-size arrays and structs of them. A ray
// query or another opaque type is not.
func constructible(module *ir.Module, ty ir.TypeHandle) bool {
	switch t := module.Types[ty].Inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		return true
	case ir.ArrayType:
		return t.Size.Constant != nil && constructible(module, t.Base)
	case ir.StructType:
		for _, m := range t.Members {
			if !constructible(module, m.Type) {
				return false
			}
		}
		return true
	}
	return false
}

// mentionedLocals calls mark for every local whose pointer an expression or a
// statement of block, at any depth, uses.
func mentionedLocals(fn *ir.Function, block ir.Block, localOf func(ir.ExpressionHandle) (ir.LocalVariableHandle, bool), mark func(ir.LocalVariableHandle)) {
	use := func(h ir.ExpressionHandle) {
		if v, ok := localOf(h); ok {
			mark(v)
		}
	}
	for _, stmt := range block {
		if em, ok := stmt.Kind.(ir.StmtEmit); ok {
			for h := em.Range.Start; h < em.Range.End; h++ {
				ExprHandles(fn.Expressions[h].Kind, use)
			}
			continue
		}
		StmtExprHandles(stmt.Kind, use)
		NestedBlocks(stmt.Kind, func(b ir.Block) { mentionedLocals(fn, b, localOf, mark) })
	}
}

// callArguments is how an inlined call passes its arguments.
type callArguments struct {
	// alias[i] reports that argument i is the caller's expression itself.
	alias []bool
	// load[i] is what the callee's ExprFunctionArgument(i) becomes: the
	// caller's expression for an aliased argument, the load of its local for
	// a spilled one.
	load []ir.ExpressionHandle
	// spills are the stores of the spilled arguments into their locals.
	spills []ir.Statement
}

// materializeArguments classifies and materializes the arguments of call.
// Each call argument is classified:
//
//	(a) Aliased argument — the inlined body substitutes every reference
//	    to ExprFunctionArgument(i) with the caller's argument expression
//	    handle directly. No local alloca, no spill store, no load.
//	    This path is used for:
//
//	    - Pointer-typed args (ptr<function, T>): spilling would create a
//	      pointer-to-pointer alloca which DXIL rejects.
//
//	    - Aggregate value args (struct, vector, matrix, array): spilling
//	      requires decomposing into per-scalar-leaf stores and then
//	      loading back. When the source is in a different address space
//	      (e.g., workgroup addrspace 3 → function addrspace 0), the
//	      emitter's GEP chains misattribute the address space, causing
//	      "Explicit load/store type does not match pointee type" errors.
//	      Aliasing avoids this entirely: the callee accesses the loaded
//	      SSA value via extractvalue, which is address-space-agnostic.
//
//	    - Image/sampler/binding-array args: opaque resource handles
//	      cannot be stored to memory.
//
//	(b) Scalar spill — for plain scalar args (u32, f32, i32, bool),
//	    allocate a caller-side local, store the value, and replace
//	    ExprFunctionArgument(i) with a Load from that local. This
//	    mirrors LLVM AlwaysInliner / Mesa nir_inline_functions.
//
// Aliasing is safe for by-value args because the caller's expression
// is an SSA value (typically ExprLoad or ExprCompose). Multiple
// references to the same expression handle in the emitter deduplicate
// to a single SSA value, preserving snapshot semantics.
func materializeArguments(module *ir.Module, caller *ir.Function, call ir.StmtCall, callee *ir.Function) (callArguments, error) {
	args := callArguments{
		alias: make([]bool, len(call.Arguments)),
		load:  make([]ir.ExpressionHandle, len(call.Arguments)),
	}
	for i := range call.Arguments {
		argType := callee.Arguments[i].Type
		if int(argType) >= len(module.Types) {
			return callArguments{}, fmt.Errorf("callee %q argument %d has out-of-range type handle", callee.Name, i)
		}
		args.alias[i] = shouldAliasArgType(module.Types[argType].Inner)
	}

	slots := make([]ir.LocalVariableHandle, len(call.Arguments))
	for i, argH := range call.Arguments {
		if args.alias[i] {
			continue
		}
		argType := callee.Arguments[i].Type
		slots[i] = appendLocal(caller, ir.LocalVariable{
			Name: "_inline_arg_" + callee.Name,
			Type: argType,
		})

		// ExprLocalVariable handle for the slot — the destination pointer
		// for the argument-spill store.
		slotPtrHandle := appendExpression(caller, ir.ExprLocalVariable{Variable: slots[i]})

		args.spills = append(args.spills, ir.Statement{Kind: ir.StmtStore{
			Pointer: slotPtrHandle,
			Value:   argH,
		}})
	}

	// 3. Pre-allocate the per-argument load expressions BEFORE reserving
	// the calleeExprMap range. Each ExprFunctionArgument(i) in the callee
	// will be replaced with the load expression at args.load[i]. We
	// allocate them here (outside calleeExprMap) so the calleeExprMap
	// region remains a single contiguous run of slots.
	//
	// For aliased arguments we set args.load[i] to the caller's argH
	// directly — no slot expressions are allocated and no ExprLoad is
	// inserted; step 5 points every callee reference at argH, a pure alias.
	for i, argH := range call.Arguments {
		if args.alias[i] {
			args.load[i] = argH
			continue
		}
		// The slot pointer and its load, the value the caller passed in.
		slotPtrHandle := appendExpression(caller, ir.ExprLocalVariable{Variable: slots[i]})
		args.load[i] = appendExpression(caller, ir.ExprLoad{Pointer: slotPtrHandle})
	}
	return args, nil
}

// reserveCalleeExpressions appends one empty caller expression per callee
// expression, a single contiguous run, and returns the callee-to-caller
// handle map.
func reserveCalleeExpressions(caller, callee *ir.Function) []ir.ExpressionHandle {
	calleeExprMap := make([]ir.ExpressionHandle, len(callee.Expressions))
	for i := range callee.Expressions {
		calleeExprMap[i] = appendExpression(caller, nil)
	}
	return calleeExprMap
}

// expressionRefs returns what each callee expression handle stands for in
// the caller: its reserved slot, or for an aliased argument the caller's
// expression itself, so the body reads the value the caller evaluated, in
// argument order, and not a copy evaluated after the arguments to its right.
// The aliased argument's slot keeps a copy that nothing reads.
func (a callArguments) expressionRefs(callee *ir.Function, calleeExprMap []ir.ExpressionHandle) []ir.ExpressionHandle {
	exprRefs := slices.Clone(calleeExprMap)
	for i := range callee.Expressions {
		if k, ok := callee.Expressions[i].Kind.(ir.ExprFunctionArgument); ok && int(k.Index) < len(a.alias) && a.alias[k.Index] {
			exprRefs[i] = a.load[k.Index]
		}
	}
	return exprRefs
}

// fillCalleeExpressions fills the reserved slots with the callee's
// expressions in the caller's handle space.
func fillCalleeExpressions(caller, callee *ir.Function, calleeExprMap, exprRefs, argLoad []ir.ExpressionHandle, localOffset ir.LocalVariableHandle) error {
	for i := range callee.Expressions {
		origKind := callee.Expressions[i].Kind
		var newKind ir.ExpressionKind
		switch k := origKind.(type) {
		case ir.ExprFunctionArgument:
			if int(k.Index) >= len(argLoad) {
				return fmt.Errorf("callee %q references argument %d but call provides only %d arguments",
					callee.Name, k.Index, len(argLoad))
			}
			// Replace ExprFunctionArgument(i) with a Load from the
			// pre-spilled argument slot. We embed the same Kind that
			// argLoad[i] holds so downstream code sees an equivalent
			// expression at this location AND at the pre-allocated
			// argLoad[i] slot.
			newKind = caller.Expressions[argLoad[k.Index]].Kind
		case ir.ExprLocalVariable:
			newKind = ir.ExprLocalVariable{Variable: k.Variable + localOffset}
		default:
			newKind = RemapExpressionHandles(origKind, exprRefs)
		}
		caller.Expressions[calleeExprMap[i]].Kind = newKind
	}
	return nil
}

// remapCalleeLocalInits points the initializers of the callee locals copied
// by appendCalleeLocals at the caller's expressions.
func remapCalleeLocalInits(caller, callee *ir.Function, localOffset ir.LocalVariableHandle, exprRefs []ir.ExpressionHandle) {
	for i := range callee.LocalVars {
		if callee.LocalVars[i].Init == nil {
			continue
		}
		orig := *callee.LocalVars[i].Init
		if int(orig) < len(exprRefs) {
			v := exprRefs[orig]
			caller.LocalVars[int(localOffset)+i].Init = &v
		}
	}
}

// copyNamedExpressions copies callee's NamedExpressions into caller with
// remapped handles. Names are prefixed with the callee function name so two
// helpers that both define `let foo = ...` don't collide in the caller's
// namespace after both are inlined.
func copyNamedExpressions(caller, callee *ir.Function, calleeExprMap []ir.ExpressionHandle) {
	if caller.NamedExpressions == nil && len(callee.NamedExpressions) > 0 {
		caller.NamedExpressions = make(map[ir.ExpressionHandle]string)
	}
	for oldH, name := range callee.NamedExpressions {
		if int(oldH) < len(calleeExprMap) {
			caller.NamedExpressions[calleeExprMap[oldH]] = "_" + callee.Name + "_" + name
		}
	}
}

// addReturnSlot allocates, for a callee that returns a value, a caller-side
// local of the return type and returns the expressions of its pointer and of
// its load; returnLowering rewrites StmtReturn in the inlined body to a store
// through the pointer. Both synthesized expressions get their types
// populated so downstream emitters see a fully typed arena (otherwise
// resolveExprType falls through to a scalar-float default, which the DXIL
// backend then propagates as an invalid pointer kind). For a void callee
// both are nil.
func addReturnSlot(caller, callee *ir.Function) (retPtr, retLoad *ir.ExpressionHandle) {
	if callee.Result == nil {
		return nil, nil
	}
	retType := callee.Result.Type
	slot := appendLocal(caller, ir.LocalVariable{
		Name: "_inline_ret_" + callee.Name,
		Type: retType,
	})

	// The return-slot pointer and its load, a T.
	ptr := appendExpression(caller, ir.ExprLocalVariable{Variable: slot})
	load := appendExpression(caller, ir.ExprLoad{Pointer: ptr})
	return &ptr, &load
}

// spilledArgumentEmits returns an Emit for the load of each scalar-spilled
// argument. The callee's ExprFunctionArgument expressions were implicitly
// available (like constants) and never needed StmtEmit in the callee body.
// After replacement with ExprLoad (step 5), they DO need evaluation via
// StmtEmit so the mem2reg pass can detect them as live loads in the same
// block scope. Without this, mem2reg's Phase A rejects the variable because
// its loads don't appear in any Emit range.
func (a callArguments) spilledArgumentEmits(callee *ir.Function, calleeExprMap []ir.ExpressionHandle) []ir.Statement {
	var emits []ir.Statement
	for i := range callee.Expressions {
		fa, ok := callee.Expressions[i].Kind.(ir.ExprFunctionArgument)
		if !ok || int(fa.Index) >= len(a.alias) || a.alias[fa.Index] {
			continue
		}
		mappedH := calleeExprMap[i]
		emits = append(emits, ir.Statement{Kind: ir.StmtEmit{
			Range: ir.Range{Start: mappedH, End: mappedH + 1},
		}})
	}
	return emits
}

// wireCallResult rewrites the caller's ExprCallResult, the StmtCall's
// Result in the caller's own Expressions array, into the load of the return
// slot, and appends its Emit to body.
func wireCallResult(caller *ir.Function, call ir.StmtCall, retLoad *ir.ExpressionHandle, body ir.Block) ir.Block {
	if call.Result == nil || retLoad == nil {
		return body
	}
	crIdx := int(*call.Result)
	if crIdx >= len(caller.Expressions) {
		return body
	}
	// Copy the Kind of the load expression so downstream remappers
	// see a valid ExprLoad at the original ExprCallResult handle.
	caller.Expressions[crIdx].Kind = caller.Expressions[int(*retLoad)].Kind
	// The call result was produced by the StmtCall; as a load it
	// needs an emit after the body, or a backend evaluates it at its
	// first use, which can be inside a branch its other uses
	// don't follow.
	return append(body, ir.Statement{Kind: ir.StmtEmit{
		Range: ir.Range{Start: *call.Result, End: *call.Result + 1},
	}})
}

// returnLowering rewrites the returns of an inlined callee body so that
// control reaches the statement after the call. It is Mesa's
// nir_lower_returns for our structured IR:
//
//   - a return stores its value into the return slot, sets the "returned"
//     flag and, inside a loop or a switch, leaves the innermost one with a
//     break;
//   - after a loop or switch that a return left, an enclosing loop or switch
//     is left too: `if returned { break }`;
//   - outside any loop or switch, the statements after a construct a return
//     left run only when it did not: `if returned {} else { rest }`.
//
// The LLVM inliner's shape (DXC's AlwaysInliner) wraps the body in a loop
// and turns each return into a branch to its exit. That is right for an
// unstructured CFG but not here: in the IR, as in WGSL and Rust naga, a
// break inside a switch only leaves the switch, and one inside a nested loop
// only leaves that loop.
type returnLowering struct {
	module   *ir.Module
	caller   *ir.Function
	flagName string               // name of the local that holds the "returned" flag
	retPtr   *ir.ExpressionHandle // pointer to the return slot; nil for a void callee

	// dry counts the guards without building anything; useFlag is set for
	// the real run when there is a guard to read the flag.
	dry     bool
	guards  int
	useFlag bool

	hasFlag bool
	flagPtr ir.ExpressionHandle
	boolTy  ir.TypeHandle
	err     error
}

// lower returns the rewritten body. A first, dry run decides whether any
// guard reads the flag; without one (every return is followed by nothing but
// the end of the body) the flag would be dead, so it is not made. With one,
// the body starts by clearing it: the caller may run the inlined body many
// times (a call inside a loop), and a flag left set by a previous run would
// skip this one.
func (l *returnLowering) lower(body ir.Block) (ir.Block, error) {
	l.dry = true
	l.block(body, false, true)
	if l.err != nil {
		return nil, l.err
	}
	l.dry, l.useFlag = false, l.guards > 0
	out, _ := l.block(body, false, true)
	if l.useFlag {
		out = append(ir.Block{l.setFlag(false)}, out...)
	}
	return out, nil
}

// block lowers the returns in b. breakable reports whether b lies inside a
// loop or switch of the callee (a break there leaves it); top reports
// whether b is the callee's body itself, where nothing follows a return.
// The result reports whether a return may leave b.
func (l *returnLowering) block(b ir.Block, breakable, top bool) (ir.Block, bool) {
	out := make(ir.Block, 0, len(b))
	for i := range b {
		var left, ownsBreak bool
		switch sk := b[i].Kind.(type) {
		case ir.StmtReturn:
			// What follows a return in the same block never runs.
			return append(out, l.returnSite(sk, breakable, top)...), true
		case ir.StmtBlock:
			var inner ir.Block
			inner, left = l.block(sk.Block, breakable, false)
			out = append(out, ir.Statement{Kind: ir.StmtBlock{Block: inner}})
		case ir.StmtIf:
			acc, la := l.block(sk.Accept, breakable, false)
			rej, lr := l.block(sk.Reject, breakable, false)
			left = la || lr
			out = append(out, ir.Statement{Kind: ir.StmtIf{Condition: sk.Condition, Accept: acc, Reject: rej}})
		case ir.StmtSwitch:
			cases := make([]ir.SwitchCase, len(sk.Cases))
			for j := range sk.Cases {
				body, lc := l.block(sk.Cases[j].Body, true, false)
				left = left || lc
				cases[j] = ir.SwitchCase{Value: sk.Cases[j].Value, Body: body, FallThrough: sk.Cases[j].FallThrough}
			}
			ownsBreak = true
			out = append(out, ir.Statement{Kind: ir.StmtSwitch{Selector: sk.Selector, Cases: cases}})
		case ir.StmtLoop:
			if blockContainsReturn(sk.Continuing) {
				l.err = fmt.Errorf("return inside a loop's continuing block")
			}
			var body ir.Block
			body, left = l.block(sk.Body, true, false)
			ownsBreak = true
			out = append(out, ir.Statement{Kind: ir.StmtLoop{Body: body, Continuing: sk.Continuing, BreakIf: sk.BreakIf}})
		default:
			out = append(out, b[i])
		}
		if left {
			return l.afterExit(out, b[i+1:], breakable, top, ownsBreak), true
		}
	}
	return out, false
}

// afterExit appends the statements that follow a construct a return may
// have left. ownsBreak reports whether that construct is a loop or switch,
// whose break the return used up.
func (l *returnLowering) afterExit(out, rest ir.Block, breakable, top, ownsBreak bool) ir.Block {
	switch {
	case breakable && ownsBreak:
		out = append(out, l.guard(ir.Block{ir.Statement{Kind: ir.StmtBreak{}}}, nil)...)
	case breakable:
		// The return already left the enclosing loop or switch.
	default:
		if len(rest) == 0 {
			return out
		}
		restLowered, _ := l.block(rest, false, top)
		return append(out, l.guard(nil, restLowered)...)
	}
	restLowered, _ := l.block(rest, breakable, false)
	return append(out, restLowered...)
}

func (l *returnLowering) returnSite(ret ir.StmtReturn, breakable, top bool) []ir.Statement {
	var out []ir.Statement
	if ret.Value != nil && l.retPtr != nil {
		out = append(out, ir.Statement{Kind: ir.StmtStore{Pointer: *l.retPtr, Value: *ret.Value}})
	}
	if !top && l.useFlag {
		out = append(out, l.setFlag(true))
	}
	if breakable {
		out = append(out, ir.Statement{Kind: ir.StmtBreak{}})
	}
	return out
}

// guard returns `if returned { accept } else { reject }`.
func (l *returnLowering) guard(accept, reject ir.Block) []ir.Statement {
	l.guards++
	if l.dry {
		return []ir.Statement{{Kind: ir.StmtIf{Accept: accept, Reject: reject}}}
	}
	l.ensureFlag()
	load := l.addExpression(ir.ExprLoad{Pointer: l.flagPtr})
	return []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: load, End: load + 1}}},
		{Kind: ir.StmtIf{Condition: load, Accept: accept, Reject: reject}},
	}
}

func (l *returnLowering) setFlag(v bool) ir.Statement {
	l.ensureFlag()
	lit := l.addExpression(ir.Literal{Value: ir.LiteralBool(v)})
	return ir.Statement{Kind: ir.StmtStore{Pointer: l.flagPtr, Value: lit}}
}

func (l *returnLowering) ensureFlag() {
	if l.hasFlag {
		return
	}
	l.hasFlag = true
	l.boolTy = boolTypeHandle(l.module)
	idx := appendLocal(l.caller, ir.LocalVariable{
		Name: l.flagName,
		Type: l.boolTy,
	})
	l.flagPtr = l.addExpression(ir.ExprLocalVariable{Variable: idx})
}

func (l *returnLowering) addExpression(kind ir.ExpressionKind) ir.ExpressionHandle {
	return appendExpression(l.caller, kind)
}

// boolTypeHandle returns the module's bool type, adding it if the module has
// none (types are unique, as in Rust naga's UniqueArena).
func boolTypeHandle(m *ir.Module) ir.TypeHandle {
	want := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	for i := range m.Types {
		if st, ok := m.Types[i].Inner.(ir.ScalarType); ok && st == want {
			return ir.TypeHandle(i)
		}
	}
	h := ir.TypeHandle(ArenaIndex(len(m.Types)))
	m.Types = append(m.Types, ir.Type{Inner: want})
	return h
}

// blockContainsReturn reports whether any StmtReturn exists anywhere in the
// block or its sub-blocks.
func blockContainsReturn(block ir.Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtReturn:
			return true
		case ir.StmtIf:
			if blockContainsReturn(sk.Accept) || blockContainsReturn(sk.Reject) {
				return true
			}
		case ir.StmtSwitch:
			for j := range sk.Cases {
				if blockContainsReturn(sk.Cases[j].Body) {
					return true
				}
			}
		case ir.StmtLoop:
			if blockContainsReturn(sk.Body) || blockContainsReturn(sk.Continuing) {
				return true
			}
		case ir.StmtBlock:
			if blockContainsReturn(sk.Block) {
				return true
			}
		}
	}
	return false
}

// shouldAliasArgType reports whether a function argument of this type should
// be aliased (direct expression substitution) rather than spilled to a
// function-local alloca. Aliased types include:
//
//   - Pointer types: spilling creates pointer-to-pointer, which DXIL rejects.
//   - Image/Sampler/BindingArray types: opaque resource handles cannot be stored.
//   - Aggregate value types (struct, vector, matrix, array): spilling across
//     address spaces (e.g., workgroup → function) causes type mismatches in
//     the emitter's GEP chains. Aliasing lets the callee access fields via
//     extractvalue on the loaded SSA value, which is address-space-agnostic.
//
// Only plain scalar types (u32, f32, i32, bool) are spilled — they have no
// address-space or aggregate decomposition issues.
func shouldAliasArgType(inner ir.TypeInner) bool {
	switch inner.(type) {
	case ir.PointerType:
		return true
	case ir.ImageType, ir.SamplerType, ir.BindingArrayType:
		return true
	case ir.VectorType, ir.MatrixType, ir.ArrayType, ir.StructType:
		return true
	}
	return false
}
