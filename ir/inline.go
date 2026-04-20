package ir

import (
	"fmt"
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
func InlineUserFunctions(module *Module, shouldInline func(callee *Function) bool) error {
	if module == nil {
		return nil
	}
	if shouldInline == nil {
		shouldInline = func(*Function) bool { return true }
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
func topologicalCallOrder(module *Module) ([]FunctionHandle, error) {
	visited := make(map[FunctionHandle]int) // 0 = unseen, 1 = in progress, 2 = done
	var order []FunctionHandle

	var visit func(h FunctionHandle) error
	visit = func(h FunctionHandle) error {
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
		if err := visit(FunctionHandle(i)); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// collectCalleeHandles walks a statement block and returns the set of
// FunctionHandles that StmtCall statements target.
func collectCalleeHandles(stmts Block) []FunctionHandle {
	seen := make(map[FunctionHandle]struct{})
	var walk func(b Block)
	walk = func(b Block) {
		for i := range b {
			switch sk := b[i].Kind.(type) {
			case StmtCall:
				seen[sk.Function] = struct{}{}
			case StmtBlock:
				walk(sk.Block)
			case StmtIf:
				walk(sk.Accept)
				walk(sk.Reject)
			case StmtLoop:
				walk(sk.Body)
				walk(sk.Continuing)
			case StmtSwitch:
				for j := range sk.Cases {
					walk(sk.Cases[j].Body)
				}
			}
		}
	}
	walk(stmts)
	out := make([]FunctionHandle, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out
}

// inlineCallsInFunction walks the function body, replacing each StmtCall
// to a user helper with the expanded body of the callee. The caller's
// Expressions, LocalVars, and NamedExpressions are grown to hold the
// inlined content with properly remapped handles.
func inlineCallsInFunction(module *Module, caller *Function, shouldInline func(*Function) bool) error {
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
func inlineBlock(module *Module, caller *Function, block Block, shouldInline func(*Function) bool) (Block, bool, error) {
	out := make(Block, 0, len(block))
	changed := false
	for i := range block {
		stmt := block[i]
		switch sk := stmt.Kind.(type) {
		case StmtCall:
			if int(sk.Function) >= len(module.Functions) {
				out = append(out, stmt)
				continue
			}
			callee := &module.Functions[sk.Function]
			if !shouldInline(callee) {
				out = append(out, stmt)
				continue
			}
			inlined, inlineErr := inlineOneCall(module, caller, sk, callee)
			if inlineErr != nil {
				return nil, false, inlineErr
			}
			out = append(out, inlined...)
			changed = true
		case StmtBlock:
			newInner, innerChanged, innerErr := inlineBlock(module, caller, sk.Block, shouldInline)
			if innerErr != nil {
				return nil, false, innerErr
			}
			if innerChanged {
				out = append(out, Statement{Kind: StmtBlock{Block: newInner}})
				changed = true
			} else {
				out = append(out, stmt)
			}
		case StmtIf:
			newAccept, a, err := inlineBlock(module, caller, sk.Accept, shouldInline)
			if err != nil {
				return nil, false, err
			}
			newReject, r, err := inlineBlock(module, caller, sk.Reject, shouldInline)
			if err != nil {
				return nil, false, err
			}
			if a || r {
				out = append(out, Statement{Kind: StmtIf{Condition: sk.Condition, Accept: newAccept, Reject: newReject}})
				changed = true
			} else {
				out = append(out, stmt)
			}
		case StmtLoop:
			newBody, b, err := inlineBlock(module, caller, sk.Body, shouldInline)
			if err != nil {
				return nil, false, err
			}
			newCont, c, err := inlineBlock(module, caller, sk.Continuing, shouldInline)
			if err != nil {
				return nil, false, err
			}
			if b || c {
				out = append(out, Statement{Kind: StmtLoop{Body: newBody, Continuing: newCont, BreakIf: sk.BreakIf}})
				changed = true
			} else {
				out = append(out, stmt)
			}
		case StmtSwitch:
			newCases := make([]SwitchCase, len(sk.Cases))
			caseChanged := false
			for j := range sk.Cases {
				newCaseBody, bc, err := inlineBlock(module, caller, sk.Cases[j].Body, shouldInline)
				if err != nil {
					return nil, false, err
				}
				newCases[j] = SwitchCase{Value: sk.Cases[j].Value, Body: newCaseBody, FallThrough: sk.Cases[j].FallThrough}
				if bc {
					caseChanged = true
				}
			}
			if caseChanged {
				out = append(out, Statement{Kind: StmtSwitch{Selector: sk.Selector, Cases: newCases}})
				changed = true
			} else {
				out = append(out, stmt)
			}
		default:
			out = append(out, stmt)
		}
	}
	return out, changed, nil
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
func inlineOneCall(module *Module, caller *Function, call StmtCall, callee *Function) ([]Statement, error) {
	// Statements we prepend to the caller's position: one StmtStore per
	// argument, materializing the caller-side value into a freshly-allocated
	// local so the inlined body reads the arg via ExprLoad of a local
	// variable. Mirrors LLVM AlwaysInliner / Mesa nir_inline_functions which
	// both spill args into temporaries.
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
	var prefixStmts []Statement

	if len(call.Arguments) != len(callee.Arguments) {
		return nil, fmt.Errorf("callee %q expects %d arguments but call provides %d",
			callee.Name, len(callee.Arguments), len(call.Arguments))
	}

	// 1. Remap callee's locals into caller's LocalVars.
	localOffset := uint32(len(caller.LocalVars))
	for i := range callee.LocalVars {
		lv := callee.LocalVars[i]
		if lv.Init != nil {
			// Init references a callee expression handle. We remap after
			// exprOffset is known (pass 2b below).
			lv.Init = &[]ExpressionHandle{*lv.Init}[0]
		}
		caller.LocalVars = append(caller.LocalVars, lv)
	}

	// 2. Argument materialization. Each call argument is classified:
	//
	//   (a) Aliased argument — the inlined body substitutes every reference
	//       to ExprFunctionArgument(i) with the caller's argument expression
	//       handle directly. No local alloca, no spill store, no load.
	//       This path is used for:
	//
	//       - Pointer-typed args (ptr<function, T>): spilling would create a
	//         pointer-to-pointer alloca which DXIL rejects.
	//
	//       - Aggregate value args (struct, vector, matrix, array): spilling
	//         requires decomposing into per-scalar-leaf stores and then
	//         loading back. When the source is in a different address space
	//         (e.g., workgroup addrspace 3 → function addrspace 0), the
	//         emitter's GEP chains misattribute the address space, causing
	//         "Explicit load/store type does not match pointee type" errors.
	//         Aliasing avoids this entirely: the callee accesses the loaded
	//         SSA value via extractvalue, which is address-space-agnostic.
	//
	//       - Image/sampler/binding-array args: opaque resource handles
	//         cannot be stored to memory.
	//
	//   (b) Scalar spill — for plain scalar args (u32, f32, i32, bool),
	//       allocate a caller-side local, store the value, and replace
	//       ExprFunctionArgument(i) with a Load from that local. This
	//       mirrors LLVM AlwaysInliner / Mesa nir_inline_functions.
	//
	// Aliasing is safe for by-value args because the caller's expression
	// is an SSA value (typically ExprLoad or ExprCompose). Multiple
	// references to the same expression handle in the emitter deduplicate
	// to a single SSA value, preserving snapshot semantics.
	argShouldAlias := make([]bool, len(call.Arguments))
	argLocalSlots := make([]uint32, len(call.Arguments))
	for i := range call.Arguments {
		argType := callee.Arguments[i].Type
		if int(argType) >= len(module.Types) {
			return nil, fmt.Errorf("callee %q argument %d has out-of-range type handle", callee.Name, i)
		}
		if shouldAliasArgType(module.Types[argType].Inner) {
			argShouldAlias[i] = true
			continue
		}
	}

	for i, argH := range call.Arguments {
		if argShouldAlias[i] {
			continue
		}
		slot := uint32(len(caller.LocalVars))
		argType := callee.Arguments[i].Type
		caller.LocalVars = append(caller.LocalVars, LocalVariable{
			Name: "_inline_arg_" + callee.Name,
			Type: argType,
		})
		argLocalSlots[i] = slot

		// ExprLocalVariable handle for the slot — the destination pointer
		// for the argument-spill store. Its expression type is "pointer to
		// the local-var's type in function space", matching what the
		// typifier (ir/resolve.go ExprLocalVariable case) computes.
		slotPtrHandle := ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{Kind: ExprLocalVariable{Variable: slot}})
		caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{
			Value: PointerType{Base: argType, Space: SpaceFunction},
		})

		prefixStmts = append(prefixStmts, Statement{Kind: StmtStore{
			Pointer: slotPtrHandle,
			Value:   argH,
		}})
	}

	// 3. Pre-allocate the per-argument load expressions BEFORE reserving
	// the calleeExprMap range. Each ExprFunctionArgument(i) in the callee
	// will be replaced with the load expression at argLoadExprs[i]. We
	// allocate them here (outside calleeExprMap) so the calleeExprMap
	// region remains a single contiguous run of slots.
	//
	// For pointer-aliased arguments (argShouldAlias[i] == true) we set
	// argLoadExprs[i] to the caller's argH directly — no slot expressions
	// are allocated and no ExprLoad is inserted; the substitution in step 5
	// copies the Kind of caller.Expressions[argH] into every callee
	// reference site, behaving as a pure alias.
	//
	// Both the slot-pointer ExprLocalVariable and the ExprLoad need their
	// ExpressionTypes populated — the DXIL backend's resolveExprType
	// reads them when classifying ExprAccess bases (e.g., to detect that
	// the loaded value is itself a pointer that should be GEP'd directly,
	// rather than treated as a value to copy into a temp alloca).
	argLoadExprs := make([]ExpressionHandle, len(call.Arguments))
	for i, argH := range call.Arguments {
		if argShouldAlias[i] {
			argLoadExprs[i] = argH
			continue
		}
		argType := callee.Arguments[i].Type

		// ExprLocalVariable for the slot pointer: type is pointer-to-T.
		slotPtrHandle := ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{Kind: ExprLocalVariable{Variable: argLocalSlots[i]}})
		caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{
			Value: PointerType{Base: argType, Space: SpaceFunction},
		})

		// ExprLoad of the slot returns the original argument value. Its
		// type is the same as callee.Arguments[i].Type — the value the
		// caller passed in.
		loadHandle := ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{Kind: ExprLoad{Pointer: slotPtrHandle}})
		argTypeCopy := argType
		caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{Handle: &argTypeCopy})
		argLoadExprs[i] = loadHandle
	}

	// 4. Reserve a contiguous slot per callee expression. StmtEmit ranges
	// will remap linearly through calleeExprMap, with End handled as an
	// exclusive boundary (see remapInlineStatementHandles).
	//
	// ExpressionTypes is copied verbatim — Module.Types is shared across
	// all functions in the module, so the TypeHandle inside
	// callee.ExpressionTypes[i] (when .Handle is non-nil) is already
	// valid in the caller's view. No remapping is needed for the
	// embedded TypeHandle. For ExprFunctionArgument expressions, we
	// substitute the type when filling the slot (see step 5) since the
	// argument-load expression has the same type as the argument
	// itself.
	calleeExprMap := make([]ExpressionHandle, len(callee.Expressions))
	for i := range callee.Expressions {
		calleeExprMap[i] = ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{})
		if i < len(callee.ExpressionTypes) {
			caller.ExpressionTypes = append(caller.ExpressionTypes, callee.ExpressionTypes[i])
		} else {
			caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{})
		}
	}

	// 5. Fill in the reserved slots with remapped content.
	for i := range callee.Expressions {
		origKind := callee.Expressions[i].Kind
		var newKind ExpressionKind
		switch k := origKind.(type) {
		case ExprFunctionArgument:
			if int(k.Index) >= len(argLoadExprs) {
				return nil, fmt.Errorf("callee %q references argument %d but call provides only %d arguments",
					callee.Name, k.Index, len(argLoadExprs))
			}
			// Replace ExprFunctionArgument(i) with a Load from the
			// pre-spilled argument slot. We embed the same Kind that
			// argLoadExprs[i] holds so downstream code sees an
			// equivalent expression at this location AND at the
			// pre-allocated argLoadExprs[i] slot.
			newKind = caller.Expressions[argLoadExprs[k.Index]].Kind
		case ExprLocalVariable:
			newKind = ExprLocalVariable{Variable: k.Variable + localOffset}
		default:
			newKind = remapExprHandles(origKind, calleeExprMap)
		}
		caller.Expressions[calleeExprMap[i]].Kind = newKind
	}

	// Fix up LocalVar.Init handles we copied verbatim.
	for i := range callee.LocalVars {
		if callee.LocalVars[i].Init == nil {
			continue
		}
		idx := int(localOffset) + i
		orig := *callee.LocalVars[i].Init
		if int(orig) < len(calleeExprMap) {
			v := calleeExprMap[orig]
			caller.LocalVars[idx].Init = &v
		}
	}

	// 6. Copy callee's NamedExpressions into caller with remapped
	// handles. Names are prefixed with the callee function name so two
	// helpers that both define `let foo = ...` don't collide in the
	// caller's namespace after both are inlined.
	if caller.NamedExpressions == nil && len(callee.NamedExpressions) > 0 {
		caller.NamedExpressions = make(map[ExpressionHandle]string)
	}
	for oldH, name := range callee.NamedExpressions {
		if int(oldH) < len(calleeExprMap) {
			caller.NamedExpressions[calleeExprMap[oldH]] = "_" + callee.Name + "_" + name
		}
	}

	// 7. Handle the return slot. If the callee returns a value, allocate a
	// caller-side local of the return type and rewrite StmtReturn in the
	// inlined body to StmtStore into that local. Both synthesized
	// expressions get their types populated so downstream emitters see a
	// fully typed arena (otherwise resolveExprType falls through to a
	// scalar-float default, which the DXIL backend then propagates as an
	// invalid pointer kind).
	var retSlot *uint32
	var retLoadExpr *ExpressionHandle
	if callee.Result != nil {
		slotIdx := uint32(len(caller.LocalVars))
		retType := callee.Result.Type
		caller.LocalVars = append(caller.LocalVars, LocalVariable{
			Name: "_inline_ret_" + callee.Name,
			Type: retType,
		})
		retSlot = &slotIdx

		// Expression for the return-slot pointer: type is pointer-to-T.
		slotPtrHandle := ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{Kind: ExprLocalVariable{Variable: slotIdx}})
		caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{
			Value: PointerType{Base: retType, Space: SpaceFunction},
		})

		// Expression for the load of the return slot — type is T.
		loadHandle := ExpressionHandle(len(caller.Expressions))
		caller.Expressions = append(caller.Expressions, Expression{Kind: ExprLoad{Pointer: slotPtrHandle}})
		retTypeCopy := retType
		caller.ExpressionTypes = append(caller.ExpressionTypes, TypeResolution{Handle: &retTypeCopy})
		retLoadExpr = &loadHandle
	}

	// 8. Remap the callee body statements into caller's handle space.
	inlinedBody := remapInlineBlockHandles(callee.Body, calleeExprMap, localOffset)

	// 8b. Emit coverage for scalar-spilled arguments. The callee's
	// ExprFunctionArgument expressions were implicitly available (like
	// constants) and never needed StmtEmit in the callee body. After
	// replacement with ExprLoad (step 5), they DO need evaluation via
	// StmtEmit so the mem2reg pass can detect them as live loads in the
	// same block scope. Without this, mem2reg's Phase A rejects the
	// variable because its loads don't appear in any Emit range.
	for i := range callee.Expressions {
		if _, isFuncArg := callee.Expressions[i].Kind.(ExprFunctionArgument); !isFuncArg {
			continue
		}
		fa := callee.Expressions[i].Kind.(ExprFunctionArgument)
		if int(fa.Index) >= len(argShouldAlias) || argShouldAlias[fa.Index] {
			continue
		}
		mappedH := calleeExprMap[i]
		prefixStmts = append(prefixStmts, Statement{Kind: StmtEmit{
			Range: Range{Start: mappedH, End: mappedH + 1},
		}})
	}

	// 9. Rewrite StmtReturn in the inlined body. If the callee has early
	// returns (returns inside nested if/switch/loop), wrap the body in a
	// synthetic loop and replace each return with store+break. This mirrors
	// LLVM AlwaysInliner's wrap pattern (DxilLinker.cpp:1248).
	hasEarly := blockHasEarlyReturn(inlinedBody)
	inlinedBody = rewriteReturnsForInline(inlinedBody, retSlot, caller.Expressions, hasEarly)
	if hasEarly {
		// Wrap in loop { <body> } so break statements exit to after the call.
		inlinedBody = Block{Statement{Kind: StmtLoop{
			Body:       inlinedBody,
			Continuing: Block{},
		}}}
	}

	// 10. Wire the caller's ExprCallResult to load from the return slot.
	// The StmtCall's Result handle is an ExprCallResult expression in the
	// caller's own Expressions array — rewrite its Kind in place.
	if call.Result != nil && retLoadExpr != nil {
		crIdx := int(*call.Result)
		if crIdx < len(caller.Expressions) {
			// Copy the Kind of the load expression so downstream remappers
			// see a valid ExprLoad at the original ExprCallResult handle.
			caller.Expressions[crIdx].Kind = caller.Expressions[int(*retLoadExpr)].Kind
		}
	}

	return append(prefixStmts, inlinedBody...), nil
}

// remapInlineBlockHandles walks a statement block and rewrites ExpressionHandles
// (via exprMap) and ExprLocalVariable indices (via localOffset) in place
// on a copied tree. Returns the new block.
func remapInlineBlockHandles(block Block, exprMap []ExpressionHandle, localOffset uint32) Block {
	out := make(Block, 0, len(block))
	for i := range block {
		out = append(out, remapInlineStatementHandles(block[i], exprMap, localOffset))
	}
	return out
}

func remapInlineStatementHandles(stmt Statement, exprMap []ExpressionHandle, localOffset uint32) Statement {
	mapH := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(exprMap) {
			return exprMap[h]
		}
		return h
	}
	mapOpt := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		v := mapH(*h)
		return &v
	}
	// remapEmitRange remaps a half-open expression-handle range
	// [start, end) through exprMap. End is exclusive: in the callee it
	// is one past the last emitted handle. Mapping End through exprMap
	// directly would either go out of bounds or land on a handle that is
	// not the new "one past" boundary, depending on how exprMap was
	// constructed. The correct caller-side End is exprMap[end-1]+1, which
	// works as long as the callee's emit range was non-empty and the
	// inline pass kept calleeExprMap a single contiguous block.
	remapEmitRange := func(start, end ExpressionHandle) Range {
		if start >= end {
			// Empty range — nothing to remap.
			return Range{Start: mapH(start), End: mapH(start)}
		}
		newStart := mapH(start)
		lastIdx := int(end) - 1
		var newEnd ExpressionHandle
		if lastIdx < len(exprMap) {
			newEnd = exprMap[lastIdx] + 1
		} else {
			// Last handle is outside the callee's expression arena —
			// fall back to identity. Should not happen for
			// well-formed IR (validation enforces handle bounds).
			newEnd = end
		}
		return Range{Start: newStart, End: newEnd}
	}
	switch sk := stmt.Kind.(type) {
	case StmtEmit:
		return Statement{Kind: StmtEmit{Range: remapEmitRange(sk.Range.Start, sk.Range.End)}}
	case StmtBlock:
		return Statement{Kind: StmtBlock{Block: remapInlineBlockHandles(sk.Block, exprMap, localOffset)}}
	case StmtIf:
		return Statement{Kind: StmtIf{
			Condition: mapH(sk.Condition),
			Accept:    remapInlineBlockHandles(sk.Accept, exprMap, localOffset),
			Reject:    remapInlineBlockHandles(sk.Reject, exprMap, localOffset),
		}}
	case StmtLoop:
		return Statement{Kind: StmtLoop{
			Body:       remapInlineBlockHandles(sk.Body, exprMap, localOffset),
			Continuing: remapInlineBlockHandles(sk.Continuing, exprMap, localOffset),
			BreakIf:    mapOpt(sk.BreakIf),
		}}
	case StmtSwitch:
		cases := make([]SwitchCase, len(sk.Cases))
		for i := range sk.Cases {
			cases[i] = SwitchCase{
				Value:       sk.Cases[i].Value,
				Body:        remapInlineBlockHandles(sk.Cases[i].Body, exprMap, localOffset),
				FallThrough: sk.Cases[i].FallThrough,
			}
		}
		return Statement{Kind: StmtSwitch{Selector: mapH(sk.Selector), Cases: cases}}
	case StmtReturn:
		return Statement{Kind: StmtReturn{Value: mapOpt(sk.Value)}}
	case StmtStore:
		return Statement{Kind: StmtStore{Pointer: mapH(sk.Pointer), Value: mapH(sk.Value)}}
	case StmtImageStore:
		out := sk
		out.Image = mapH(sk.Image)
		out.Coordinate = mapH(sk.Coordinate)
		out.ArrayIndex = mapOpt(sk.ArrayIndex)
		out.Value = mapH(sk.Value)
		return Statement{Kind: out}
	case StmtAtomic:
		out := sk
		out.Pointer = mapH(sk.Pointer)
		out.Value = mapH(sk.Value)
		out.Result = mapOpt(sk.Result)
		return Statement{Kind: out}
	case StmtCall:
		// Should not occur in Phase 1 — callees are processed bottom-up
		// and their bodies have no StmtCall left by the time we inline
		// them into a caller. Carry through verbatim anyway.
		args := make([]ExpressionHandle, len(sk.Arguments))
		for i, a := range sk.Arguments {
			args[i] = mapH(a)
		}
		return Statement{Kind: StmtCall{Function: sk.Function, Arguments: args, Result: mapOpt(sk.Result)}}
	default:
		return stmt
	}
}

// blockHasEarlyReturn reports whether a statement block contains any
// StmtReturn inside nested control flow (if/switch/loop). A return at the
// very end of the top-level block is NOT considered early — it's a tail
// return that Phase 1 can handle without loop wrapping.
func blockHasEarlyReturn(block Block) bool {
	// Walk the block: any return that appears inside a sub-block of
	// if/switch/loop is early.
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case StmtIf:
			if blockContainsReturn(sk.Accept) || blockContainsReturn(sk.Reject) {
				return true
			}
		case StmtSwitch:
			for j := range sk.Cases {
				if blockContainsReturn(sk.Cases[j].Body) {
					return true
				}
			}
		case StmtLoop:
			if blockContainsReturn(sk.Body) || blockContainsReturn(sk.Continuing) {
				return true
			}
		case StmtBlock:
			if blockHasEarlyReturn(sk.Block) {
				return true
			}
		}
	}
	return false
}

// blockContainsReturn reports whether any StmtReturn exists anywhere in the
// block or its sub-blocks.
func blockContainsReturn(block Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case StmtReturn:
			return true
		case StmtIf:
			if blockContainsReturn(sk.Accept) || blockContainsReturn(sk.Reject) {
				return true
			}
		case StmtSwitch:
			for j := range sk.Cases {
				if blockContainsReturn(sk.Cases[j].Body) {
					return true
				}
			}
		case StmtLoop:
			if blockContainsReturn(sk.Body) || blockContainsReturn(sk.Continuing) {
				return true
			}
		case StmtBlock:
			if blockContainsReturn(sk.Block) {
				return true
			}
		}
	}
	return false
}

// rewriteReturnsForInline walks a freshly inlined body and replaces StmtReturn
// statements. When wrapInLoop is true (early returns detected), each return is
// replaced with store-to-slot + break (the caller wraps in a loop). When
// wrapInLoop is false, returns are simply replaced with store-to-slot (tail
// return only, Phase 1 behavior).
func rewriteReturnsForInline(block Block, retSlot *uint32, exprArena []Expression, wrapInLoop bool) Block {
	// We need to know the index of an ExprLocalVariable pointing to the
	// return slot; allocate one on first use and cache the handle.
	var slotPtrHandle ExpressionHandle
	slotInit := false
	getSlotPtr := func() ExpressionHandle {
		if slotInit {
			return slotPtrHandle
		}
		// Search exprArena for an existing ExprLocalVariable matching the
		// return slot. inlineOneCall already appended one; reuse it.
		if retSlot != nil {
			for i, e := range exprArena {
				if lv, ok := e.Kind.(ExprLocalVariable); ok && lv.Variable == *retSlot {
					slotPtrHandle = ExpressionHandle(i)
					slotInit = true
					return slotPtrHandle
				}
			}
		}
		slotInit = true
		return slotPtrHandle
	}

	var walk func(b Block) Block
	walk = func(b Block) Block {
		out := make(Block, 0, len(b))
		for i := range b {
			switch sk := b[i].Kind.(type) {
			case StmtReturn:
				if retSlot == nil || sk.Value == nil {
					// Void return or no-value return.
					if wrapInLoop {
						out = append(out, Statement{Kind: StmtBreak{}})
					}
					continue
				}
				ptr := getSlotPtr()
				out = append(out, Statement{Kind: StmtStore{Pointer: ptr, Value: *sk.Value}})
				if wrapInLoop {
					out = append(out, Statement{Kind: StmtBreak{}})
				}
			case StmtBlock:
				out = append(out, Statement{Kind: StmtBlock{Block: walk(sk.Block)}})
			case StmtIf:
				out = append(out, Statement{Kind: StmtIf{Condition: sk.Condition, Accept: walk(sk.Accept), Reject: walk(sk.Reject)}})
			case StmtLoop:
				out = append(out, Statement{Kind: StmtLoop{Body: walk(sk.Body), Continuing: walk(sk.Continuing), BreakIf: sk.BreakIf}})
			case StmtSwitch:
				cases := make([]SwitchCase, len(sk.Cases))
				for j := range sk.Cases {
					cases[j] = SwitchCase{Value: sk.Cases[j].Value, Body: walk(sk.Cases[j].Body), FallThrough: sk.Cases[j].FallThrough}
				}
				out = append(out, Statement{Kind: StmtSwitch{Selector: sk.Selector, Cases: cases}})
			default:
				out = append(out, b[i])
			}
		}
		return out
	}
	return walk(block)
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
func shouldAliasArgType(inner TypeInner) bool {
	switch inner.(type) {
	case PointerType:
		return true
	case ImageType, SamplerType, BindingArrayType:
		return true
	case VectorType, MatrixType, ArrayType, StructType:
		return true
	}
	return false
}
