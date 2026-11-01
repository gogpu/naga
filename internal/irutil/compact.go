package irutil

import (
	"slices"

	"github.com/gogpu/naga/ir"
)

// Compact removes the objects module does not use and renumbers the handles
// of what is left, as Rust naga's compact::compact does. module must be
// valid.
//
// With either keep value it removes the function expressions nothing refers
// to, the constants, overrides and global expressions nothing uses (named
// constants of a concrete type always stay), the abstract types and the
// anonymous types nothing refers to any more. With KeepUnusedNo it first removes the global variables
// and functions no entry point reaches, and the named overrides nothing uses.
// Entry points, special types and named types always stay; Rust also removes
// an unused named type under KeepUnused::No.
func Compact(module *ir.Module, keep KeepUnused) {
	CompactWithTypeOrder(module, keep, nil)
}

// CompactWithTypeOrder is Compact for the WGSL lowerer, which keeps the order
// it registered the module's types in for ReorderTypes: it returns typeOrder
// with the handles of the types that survive, renumbered.
func CompactWithTypeOrder(module *ir.Module, keep KeepUnused, typeOrder []ir.TypeHandle) []ir.TypeHandle {
	if keep == KeepUnusedNo {
		CompactUnused(module)
	}
	// Dead function expressions first, so that they do not keep a constant
	// alive; then constants with the global expressions only they use; then
	// the types nothing refers to any more.
	compactExpressions(module)
	compactConstants(module, keep)
	return remapTypeOrder(typeOrder, compactTypes(module))
}

// CompactUnused removes globals and functions not reachable from any entry point.
// Matches Rust naga's compact pass which traces from entry points and removes
// unreachable global variables, functions, and their associated types.
func CompactUnused(module *ir.Module) {
	if len(module.EntryPoints) == 0 {
		return
	}
	usedGlobals, usedFunctions := traceEntryPoints(module)
	if slices.Contains(usedGlobals, false) {
		removeUnusedGlobals(module, usedGlobals)
	}
	if slices.Contains(usedFunctions, false) {
		removeUnusedFunctions(module, usedFunctions)
	}
}

// traceEntryPoints returns which global variables and functions the entry
// points use, directly or through the functions they call.
func traceEntryPoints(module *ir.Module) (usedGlobals, usedFunctions []bool) {
	usedGlobals = make([]bool, len(module.GlobalVariables))
	usedFunctions = make([]bool, len(module.Functions))

	// Trace a function's expressions and statements for global/function references.
	var traceFunction func(f *ir.Function)
	traceFunction = func(f *ir.Function) {
		for _, expr := range f.Expressions {
			if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
				usedGlobals[gv.Variable] = true
			}
		}
		traceStatementsForRefs(f.Body, usedFunctions, module, traceFunction)
	}

	markGlobal := func(h ir.GlobalVariableHandle) {
		if int(h) < len(usedGlobals) {
			usedGlobals[h] = true
		}
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		traceFunction(&ep.Function)
		// Mesh shader references
		if ep.TaskPayload != nil {
			markGlobal(*ep.TaskPayload)
		}
		if ep.MeshInfo != nil {
			markGlobal(ep.MeshInfo.OutputVariable)
		}
	}
	return usedGlobals, usedFunctions
}

// removeUnusedGlobals removes the global variables used does not mark and
// renumbers the handles of the others.
func removeUnusedGlobals(module *ir.Module, used []bool) {
	globalRemap := make([]ir.GlobalVariableHandle, len(module.GlobalVariables))
	var newGlobals []ir.GlobalVariable
	for i, g := range module.GlobalVariables {
		if used[i] {
			globalRemap[i] = ir.GlobalVariableHandle(ArenaIndex(len(newGlobals)))
			newGlobals = append(newGlobals, g)
		}
	}
	module.GlobalVariables = newGlobals

	// Remap global variable handles in all expressions.
	remapGlobalVarHandles := func(f *ir.Function) {
		for j := range f.Expressions {
			if gv, ok := f.Expressions[j].Kind.(ir.ExprGlobalVariable); ok {
				f.Expressions[j].Kind = ir.ExprGlobalVariable{Variable: globalRemap[gv.Variable]}
			}
		}
	}
	for i := range module.Functions {
		remapGlobalVarHandles(&module.Functions[i])
	}
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		remapGlobalVarHandles(&ep.Function)
		if ep.TaskPayload != nil {
			v := globalRemap[*ep.TaskPayload]
			ep.TaskPayload = &v
		}
		if ep.MeshInfo != nil {
			ep.MeshInfo.OutputVariable = globalRemap[ep.MeshInfo.OutputVariable]
		}
	}
}

// removeUnusedFunctions removes the functions used does not mark and
// renumbers the handles of the others in call statements and call results.
func removeUnusedFunctions(module *ir.Module, used []bool) {
	funcRemap := make([]ir.FunctionHandle, len(module.Functions))
	var newFunctions []ir.Function
	for i, f := range module.Functions {
		if used[i] {
			funcRemap[i] = ir.FunctionHandle(ArenaIndex(len(newFunctions)))
			newFunctions = append(newFunctions, f)
		}
	}
	module.Functions = newFunctions

	remapFuncHandles := func(f *ir.Function) {
		for j := range f.Expressions {
			if cr, ok := f.Expressions[j].Kind.(ir.ExprCallResult); ok {
				f.Expressions[j].Kind = ir.ExprCallResult{Function: funcRemap[cr.Function]}
			}
		}
		remapStmtFuncHandles(f.Body, funcRemap)
	}
	for i := range module.Functions {
		remapFuncHandles(&module.Functions[i])
	}
	for i := range module.EntryPoints {
		remapFuncHandles(&module.EntryPoints[i].Function)
	}
}

// traceStatementsForRefs marks in usedFunctions the functions stmts call
// and traces each newly reached one with traceFunc.
func traceStatementsForRefs(stmts []ir.Statement, usedFunctions []bool, module *ir.Module, traceFunc func(*ir.Function)) {
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case ir.StmtCall:
			if int(s.Function) < len(usedFunctions) && !usedFunctions[s.Function] {
				usedFunctions[s.Function] = true
				if int(s.Function) < len(module.Functions) {
					traceFunc(&module.Functions[s.Function])
				}
			}
		case ir.StmtBlock:
			traceStatementsForRefs(s.Block, usedFunctions, module, traceFunc)
		case ir.StmtIf:
			traceStatementsForRefs(s.Accept, usedFunctions, module, traceFunc)
			traceStatementsForRefs(s.Reject, usedFunctions, module, traceFunc)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				traceStatementsForRefs(c.Body, usedFunctions, module, traceFunc)
			}
		case ir.StmtLoop:
			traceStatementsForRefs(s.Body, usedFunctions, module, traceFunc)
			traceStatementsForRefs(s.Continuing, usedFunctions, module, traceFunc)
		}
	}
}

// remapStmtFuncHandles remaps FunctionHandle in StmtCall within statement trees.
func remapStmtFuncHandles(stmts []ir.Statement, remap []ir.FunctionHandle) {
	for i := range stmts {
		switch s := stmts[i].Kind.(type) {
		case ir.StmtCall:
			stmts[i].Kind = ir.StmtCall{
				Function:  remap[s.Function],
				Arguments: s.Arguments,
				Result:    s.Result,
			}
		case ir.StmtBlock:
			remapStmtFuncHandles(s.Block, remap)
		case ir.StmtIf:
			remapStmtFuncHandles(s.Accept, remap)
			remapStmtFuncHandles(s.Reject, remap)
		case ir.StmtSwitch:
			for j := range s.Cases {
				remapStmtFuncHandles(s.Cases[j].Body, remap)
			}
		case ir.StmtLoop:
			remapStmtFuncHandles(s.Body, remap)
			remapStmtFuncHandles(s.Continuing, remap)
		}
	}
}

// compactTypes removes anonymous types that are not referenced by any handle
// in the module, and renumbers all type handles to be contiguous.
//
// This replicates Rust naga's compact::compact() with KeepUnused::Yes,
// which the WGSL frontend calls at the end of lowering. The key effect is
// removing scalar types that were registered during vec/mat type resolution
// but are only embedded by value (not referenced by handle) in Vector/Matrix
// types. Named types are always kept.
//
// Verified: produces identical type arenas to Rust naga on 18/18 reference shaders.
// See docs/dev/research/IR-DEEP-ANALYSIS.md for analysis.
func compactTypes(module *ir.Module) (remap []ir.TypeHandle) {
	if len(module.Types) == 0 {
		return nil
	}

	// Step 1: Find all type handles referenced directly by handles
	// (not embedded by value in Vector/Matrix/Atomic).
	referenced := referencedTypes(module)

	// Step 2: Determine which types to keep.
	// Keep referenced types AND all named types.
	// Rust naga's compact keeps all named types (structs, aliases, etc.) regardless
	// of reference count. Only abstract types are force-removed.
	keep := make([]bool, len(module.Types))
	for i, t := range module.Types {
		// Abstract types must not reach backends.
		keep[i] = !IsAbstractType(t.Inner, module.Types) && (referenced[i] || t.Name != "")
	}

	// Check if anything would be removed
	if !slices.Contains(keep, false) {
		return nil // Nothing to compact
	}

	// Step 3: Build remap table (old handle -> new handle)
	remap = make([]ir.TypeHandle, len(module.Types))
	newTypes := make([]ir.Type, 0, len(module.Types))
	for i, t := range module.Types {
		if keep[i] {
			remap[i] = ir.TypeHandle(ArenaIndex(len(newTypes)))
			newTypes = append(newTypes, t)
		} else {
			remap[i] = ^ir.TypeHandle(0) // sentinel for removed
		}
	}

	// Step 4: Remap all type handles throughout the module
	module.Types = newTypes

	// Remap handles within types
	for i := range module.Types {
		module.Types[i].Inner = remapTypeInner(module.Types[i].Inner, remap)
	}

	// Remap constants
	for i := range module.Constants {
		module.Constants[i].Type = remap[module.Constants[i].Type]
	}

	// Remap overrides
	for i := range module.Overrides {
		module.Overrides[i].Ty = remap[module.Overrides[i].Ty]
	}

	// Remap global variables
	for i := range module.GlobalVariables {
		module.GlobalVariables[i].Type = remap[module.GlobalVariables[i].Type]
	}

	// Remap functions (regular)
	for fi := range module.Functions {
		remapFunctionTypes(&module.Functions[fi], remap)
	}

	// Remap entry point functions (inline in EntryPoints)
	for ei := range module.EntryPoints {
		remapFunctionTypes(&module.EntryPoints[ei].Function, remap)
		// Remap MeshStageInfo type handles.
		if mi := module.EntryPoints[ei].MeshInfo; mi != nil {
			mi.VertexOutputType = remap[mi.VertexOutputType]
			mi.PrimitiveOutputType = remap[mi.PrimitiveOutputType]
		}
	}

	// Remap type handles in GlobalExpressions (Compose.Type, ZeroValue.Type, etc.)
	// GlobalExpressions are built inline during constant lowering (before compact),
	// so their type handles need remapping.
	for i := range module.GlobalExpressions {
		module.GlobalExpressions[i].Kind = remapExprTypeHandles(module.GlobalExpressions[i].Kind, remap)
	}

	return remap
}

// remapTypeOrder returns order with the handles of the types remap keeps,
// renumbered; order itself when remap is nil (no type was removed).
func remapTypeOrder(order, remap []ir.TypeHandle) []ir.TypeHandle {
	if remap == nil || len(order) == 0 {
		return order
	}
	filtered := make([]ir.TypeHandle, 0, len(order))
	for _, h := range order {
		if int(h) < len(remap) && remap[h] != ^ir.TypeHandle(0) {
			filtered = append(filtered, remap[h])
		}
	}
	return filtered
}

// referencedTypes returns which types something in module refers to by
// handle, as opposed to embedding them by value as Vector, Matrix and Atomic
// embed their scalar.
func referencedTypes(module *ir.Module) []bool {
	referenced := make([]bool, len(module.Types))

	// Types referencing other types by handle
	for _, t := range module.Types {
		markTypeInnerRefs(t.Inner, referenced)
	}

	// Constants
	for _, c := range module.Constants {
		referenced[c.Type] = true
	}

	// Overrides
	for _, o := range module.Overrides {
		referenced[o.Ty] = true
	}

	// Global variables
	for _, g := range module.GlobalVariables {
		referenced[g.Type] = true
	}

	// Module-scope initializers (GlobalExpressions reachable from constants,
	// overrides and global variables).
	markGlobalExpressionTypeRefs(module, referenced)

	// Functions (regular)
	for i := range module.Functions {
		markFunctionTypeRefs(&module.Functions[i], referenced)
	}

	// Entry point functions (inline in EntryPoints, not in Functions[])
	for i := range module.EntryPoints {
		markFunctionTypeRefs(&module.EntryPoints[i].Function, referenced)
	}
	return referenced
}

// remapFunctionTypes remaps all type handles within a function.
func remapFunctionTypes(f *ir.Function, remap []ir.TypeHandle) {
	for ai := range f.Arguments {
		f.Arguments[ai].Type = remap[f.Arguments[ai].Type]
	}
	if f.Result != nil {
		f.Result.Type = remap[f.Result.Type]
	}
	for li := range f.LocalVars {
		f.LocalVars[li].Type = remap[f.LocalVars[li].Type]
	}
	for ei := range f.Expressions {
		f.Expressions[ei].Kind = remapExprTypeHandles(f.Expressions[ei].Kind, remap)
	}
	for ti := range f.ExpressionTypes {
		tr := &f.ExpressionTypes[ti]
		if tr.Handle != nil {
			newH := remap[*tr.Handle]
			if newH == ^ir.TypeHandle(0) {
				// Abstract type was removed. Drop handle — backend will use
				// Value or infer type from expression context.
				tr.Handle = nil
			} else {
				tr.Handle = &newH
			}
		}
		if tr.Value != nil {
			tr.Value = remapTypeInner(tr.Value, remap)
		}
	}
}

// IsAbstractType reports whether inner is an abstract type: AbstractInt or
// AbstractFloat, or a vector, matrix or array of them (Rust naga's
// TypeInner::is_abstract). Abstract types, and the constants of such a type,
// are removed by compact and must never reach backends.
func IsAbstractType(inner ir.TypeInner, types []ir.Type) bool {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarAbstractInt || t.Kind == ir.ScalarAbstractFloat
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarAbstractInt || t.Scalar.Kind == ir.ScalarAbstractFloat
	case ir.MatrixType:
		return t.Scalar.Kind == ir.ScalarAbstractInt || t.Scalar.Kind == ir.ScalarAbstractFloat
	case ir.ArrayType:
		if int(t.Base) < len(types) {
			return IsAbstractType(types[t.Base].Inner, types)
		}
	}
	return false
}

// markTypeInnerRefs marks type handles referenced by a TypeInner.
// Only types that use handles (not embedded values) are marked.
func markTypeInnerRefs(inner ir.TypeInner, referenced []bool) {
	switch t := inner.(type) {
	case ir.ArrayType:
		referenced[t.Base] = true
	case ir.StructType:
		for _, m := range t.Members {
			referenced[m.Type] = true
		}
	case ir.PointerType:
		referenced[t.Base] = true
	case ir.BindingArrayType:
		referenced[t.Base] = true
		// VectorType, MatrixType, AtomicType, ScalarType, SamplerType,
		// ImageType, AccelerationStructureType, RayQueryType:
		// These either have no type handle refs or embed scalars by value.
	}
}

// markFunctionTypeRefs marks type handles referenced by a function's args, result, locals, and expressions.
func markFunctionTypeRefs(f *ir.Function, referenced []bool) {
	for _, arg := range f.Arguments {
		referenced[arg.Type] = true
	}
	if f.Result != nil {
		referenced[f.Result.Type] = true
	}
	for _, lv := range f.LocalVars {
		referenced[lv.Type] = true
	}
	// Expressions that reference type handles
	for _, expr := range f.Expressions {
		markExprTypeRefs(expr.Kind, referenced)
	}
	// NOTE: ExpressionTypes handles are NOT traced. They contain internal type
	// resolution data that may reference types not needed by backends. Tracing
	// them would preserve extra types and break type numbering. Instead, backends
	// handle nil/sentinel handles gracefully via concretization.
}

// markExprTypeRefs marks type handles referenced by expression kinds.
// Out-of-range handles (e.g. the removed-type sentinel) are ignored.
func markExprTypeRefs(kind ir.ExpressionKind, referenced []bool) {
	mark := func(h ir.TypeHandle) {
		if int(h) < len(referenced) {
			referenced[h] = true
		}
	}
	switch k := kind.(type) {
	case ir.ExprZeroValue:
		mark(k.Type)
	case ir.ExprCompose:
		mark(k.Type)
	case ir.ExprSubgroupOperationResult:
		mark(k.Type)
	case ir.ExprAtomicResult:
		mark(k.Ty)
	case ir.ExprWorkGroupUniformLoadResult:
		mark(k.Ty)
	}
}

// markGlobalExpressionTypeRefs marks the types carried by global expressions
// that a module-scope initializer reaches: Constant.Init, Override.Init and
// GlobalVariable.InitExpr are the roots, and Compose/Splat/... components are
// followed from there.
//
// Matches Rust naga's compact (compact/mod.rs + compact/expressions.rs): only
// used global expressions are traced, and Compose/ZeroValue mark their type.
// Without this, a type referenced only by an initializer — the vec2<f32> column
// type of `const M = mat2x2<f32>(vec2<f32>(..), vec2<f32>(..))` — was removed,
// leaving the column Compose with a dangling handle.
//
// Children normally precede their parents in the arena (the invariant Rust's
// single back-to-front pass relies on), so the loop below finishes in one
// productive pass; it repeats until no new expression is reached so that an
// arena violating the order is still traced completely.
func markGlobalExpressionTypeRefs(module *ir.Module, referenced []bool) {
	n := len(module.GlobalExpressions)
	if n == 0 {
		return
	}
	used := make([]bool, n)
	root := func(h ir.ExpressionHandle) {
		if int(h) < n {
			used[h] = true
		}
	}
	for i := range module.Constants {
		root(module.Constants[i].Init)
	}
	for i := range module.Overrides {
		if init := module.Overrides[i].Init; init != nil {
			root(*init)
		}
	}
	for i := range module.GlobalVariables {
		if init := module.GlobalVariables[i].InitExpr; init != nil {
			root(*init)
		}
	}

	visited := make([]bool, n)
	for progress := true; progress; {
		progress = false
		for h := n - 1; h >= 0; h-- {
			if !used[h] || visited[h] {
				continue
			}
			visited[h] = true
			progress = true
			kind := module.GlobalExpressions[h].Kind
			markExprHandleRefs(kind, used)
			markExprTypeRefs(kind, referenced)
		}
	}
}

// ReorderTypes reorders the type arena so that types appear in first-use order
// when scanning: constants → overrides → globals → functions → entry points.
// This matches Rust naga where types are registered during dependency-ordered
// lowering and dead intermediate types are never re-registered.
//
// useOrder is the order the lowerer registered the types in, with the
// handles of the module as it is now (CompactWithTypeOrder); types it
// leaves out follow in arena order.
//
// Must be called AFTER compactTypes (which removes unreferenced types).
func ReorderTypes(module *ir.Module, useOrder []ir.TypeHandle) {
	if len(module.Types) == 0 {
		return
	}
	order := firstUseTypeOrder(module, useOrder)
	if isIdentityOrder(order, len(module.Types)) {
		return
	}
	applyTypeOrder(module, order)
}

// firstUseTypeOrder returns the type handles of module in first-encounter
// order, each after the types it depends on.
func firstUseTypeOrder(module *ir.Module, useOrder []ir.TypeHandle) []ir.TypeHandle {
	seen := make([]bool, len(module.Types))
	var order []ir.TypeHandle

	visit := func(h ir.TypeHandle) {
		if h == ^ir.TypeHandle(0) || int(h) >= len(module.Types) || seen[h] {
			return
		}
		// Visit type's own dependencies first (array base, struct members, etc.)
		visitTypeInnerDeps(module.Types[h].Inner, module.Types, seen, &order)
		if !seen[h] {
			seen[h] = true
			order = append(order, h)
		}
	}

	// The lowerer's registration order from dependency-ordered lowering,
	// remapped to the surviving handles: it keeps the interleaved
	// constants/functions order that matches Rust naga's type arena.
	for _, h := range useOrder {
		visit(h)
	}
	// Any remaining types not yet visited
	for i := range module.Types {
		if !seen[i] {
			visit(ir.TypeHandle(i))
		}
	}
	return order
}

// isIdentityOrder reports whether order lists the handles 0 to n-1 in order.
func isIdentityOrder(order []ir.TypeHandle, n int) bool {
	if len(order) != n {
		return false
	}
	for i, h := range order {
		if int(h) != i {
			return false
		}
	}
	return true
}

// applyTypeOrder rearranges the type arena of module into order and
// renumbers every type handle in the module.
func applyTypeOrder(module *ir.Module, order []ir.TypeHandle) {
	// Build remap table
	remap := make([]ir.TypeHandle, len(module.Types))
	newTypes := make([]ir.Type, len(order))
	for newIdx, oldIdx := range order {
		if int(oldIdx) < len(remap) {
			remap[oldIdx] = ir.TypeHandle(newIdx)
		}
		if int(oldIdx) < len(module.Types) {
			newTypes[newIdx] = module.Types[oldIdx]
		}
	}

	// Apply remap — use safe wrapper that guards sentinel handles
	module.Types = newTypes
	safeRemap := func(h ir.TypeHandle) ir.TypeHandle {
		if h == ^ir.TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	for i := range module.Types {
		module.Types[i].Inner = remapTypeInner(module.Types[i].Inner, remap)
	}
	for i := range module.Constants {
		module.Constants[i].Type = safeRemap(module.Constants[i].Type)
	}
	for i := range module.Overrides {
		module.Overrides[i].Ty = safeRemap(module.Overrides[i].Ty)
	}
	for i := range module.GlobalVariables {
		module.GlobalVariables[i].Type = safeRemap(module.GlobalVariables[i].Type)
	}
	for fi := range module.Functions {
		safeRemapFunctionTypes(&module.Functions[fi], remap)
	}
	for ei := range module.EntryPoints {
		safeRemapFunctionTypes(&module.EntryPoints[ei].Function, remap)
		// Remap MeshStageInfo type handles.
		if mi := module.EntryPoints[ei].MeshInfo; mi != nil {
			mi.VertexOutputType = safeRemap(mi.VertexOutputType)
			mi.PrimitiveOutputType = safeRemap(mi.PrimitiveOutputType)
		}
	}
	for i := range module.GlobalExpressions {
		module.GlobalExpressions[i].Kind = remapExprTypeHandles(module.GlobalExpressions[i].Kind, remap)
	}
	// Remap special types
	if module.SpecialTypes.RayIntersection != nil {
		h := remap[*module.SpecialTypes.RayIntersection]
		module.SpecialTypes.RayIntersection = &h
	}
	if module.SpecialTypes.ExternalTextureParams != nil {
		h := remap[*module.SpecialTypes.ExternalTextureParams]
		module.SpecialTypes.ExternalTextureParams = &h
	}
	if module.SpecialTypes.ExternalTextureTransferFunction != nil {
		h := remap[*module.SpecialTypes.ExternalTextureTransferFunction]
		module.SpecialTypes.ExternalTextureTransferFunction = &h
	}
}

// visitTypeInnerDeps visits type handle dependencies of a TypeInner recursively.
func visitTypeInnerDeps(inner ir.TypeInner, types []ir.Type, seen []bool, order *[]ir.TypeHandle) {
	guard := func(h ir.TypeHandle) bool {
		return h != ^ir.TypeHandle(0) && int(h) < len(types) && !seen[h]
	}
	switch t := inner.(type) {
	case ir.ArrayType:
		if guard(t.Base) {
			visitTypeInnerDeps(types[t.Base].Inner, types, seen, order)
			if !seen[t.Base] {
				seen[t.Base] = true
				*order = append(*order, t.Base)
			}
		}
	case ir.StructType:
		for _, m := range t.Members {
			if guard(m.Type) {
				visitTypeInnerDeps(types[m.Type].Inner, types, seen, order)
				if guard(m.Type) {
					seen[m.Type] = true
					*order = append(*order, m.Type)
				}
			}
		}
	case ir.PointerType:
		if guard(t.Base) {
			visitTypeInnerDeps(types[t.Base].Inner, types, seen, order)
			if guard(t.Base) {
				seen[t.Base] = true
				*order = append(*order, t.Base)
			}
		}
	case ir.BindingArrayType:
		if guard(t.Base) {
			visitTypeInnerDeps(types[t.Base].Inner, types, seen, order)
			if guard(t.Base) {
				seen[t.Base] = true
				*order = append(*order, t.Base)
			}
		}
	}
}

// visitFunctionTypes visits all type handles in a function.
func visitFunctionTypes(f *ir.Function, visit func(ir.TypeHandle)) {
	for _, arg := range f.Arguments {
		visit(arg.Type)
	}
	if f.Result != nil {
		visit(f.Result.Type)
	}
	for _, lv := range f.LocalVars {
		visit(lv.Type)
	}
	for _, expr := range f.Expressions {
		visitExprTypeHandles(expr.Kind, visit)
	}
}

// visitExprTypeHandles visits type handles referenced by an expression kind.
func visitExprTypeHandles(kind ir.ExpressionKind, visit func(ir.TypeHandle)) {
	switch k := kind.(type) {
	case ir.ExprZeroValue:
		visit(k.Type)
	case ir.ExprCompose:
		visit(k.Type)
	case ir.ExprSubgroupOperationResult:
		visit(k.Type)
	case ir.ExprAtomicResult:
		visit(k.Ty)
	case ir.ExprWorkGroupUniformLoadResult:
		visit(k.Ty)
	}
}

// safeRemapFunctionTypes remaps type handles within a function, guarding against sentinel values.
// Used by ReorderTypes where ExpressionTypes may contain nil/sentinel handles from compactTypes.
func safeRemapFunctionTypes(f *ir.Function, remap []ir.TypeHandle) {
	safe := func(h ir.TypeHandle) ir.TypeHandle {
		if h == ^ir.TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	for ai := range f.Arguments {
		f.Arguments[ai].Type = safe(f.Arguments[ai].Type)
	}
	if f.Result != nil {
		f.Result.Type = safe(f.Result.Type)
	}
	for li := range f.LocalVars {
		f.LocalVars[li].Type = safe(f.LocalVars[li].Type)
	}
	for ei := range f.Expressions {
		f.Expressions[ei].Kind = remapExprTypeHandles(f.Expressions[ei].Kind, remap)
	}
	for ti := range f.ExpressionTypes {
		tr := &f.ExpressionTypes[ti]
		if tr.Handle != nil {
			h := *tr.Handle
			if h != ^ir.TypeHandle(0) && int(h) < len(remap) {
				newH := remap[h]
				tr.Handle = &newH
			}
		}
		if tr.Value != nil {
			tr.Value = remapTypeInner(tr.Value, remap)
		}
	}
}

// remapTypeInner updates type handles within a TypeInner.
func remapTypeInner(inner ir.TypeInner, remap []ir.TypeHandle) ir.TypeInner {
	safe := func(h ir.TypeHandle) ir.TypeHandle {
		if h == ^ir.TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	switch t := inner.(type) {
	case ir.ArrayType:
		t.Base = safe(t.Base)
		return t
	case ir.StructType:
		members := make([]ir.StructMember, len(t.Members))
		copy(members, t.Members)
		for i := range members {
			members[i].Type = safe(members[i].Type)
		}
		t.Members = members
		return t
	case ir.PointerType:
		t.Base = safe(t.Base)
		return t
	case ir.BindingArrayType:
		t.Base = safe(t.Base)
		return t
	default:
		return inner
	}
}

// compactExpressions removes unreferenced expressions from each function
// in the module and renumbers all expression handles. This matches Rust naga's
// compact pass which removes dead expressions (e.g., original abstract literals
// replaced by concretized versions).
//
// The algorithm matches Rust naga's compact:
// 1. Mark expressions directly used by statements (NOT Emit ranges - those are no-ops)
// 2. Mark named expressions and local variable initializers as used
// 3. Propagate usage back-to-front through expressions (transitive closure)
// 4. Remove unused expressions, remap handles, adjust Emit ranges
func compactExpressions(module *ir.Module) {
	for fi := range module.Functions {
		compactFunctionExpressions(&module.Functions[fi])
	}
	for ei := range module.EntryPoints {
		compactFunctionExpressions(&module.EntryPoints[ei].Function)
	}
}

func compactFunctionExpressions(f *ir.Function) {
	n := len(f.Expressions)
	if n == 0 {
		return
	}

	// Phases 1 and 2: mark the expressions in use.
	used := usedExpressions(f)

	// Phase 3: Check if anything would be removed.
	if !slices.Contains(used, false) {
		return
	}

	// Phase 4: Build remap table.
	remap := make([]ir.ExpressionHandle, n)
	newExprs := make([]ir.Expression, 0, n)
	for i, expr := range f.Expressions {
		if used[i] {
			remap[i] = ir.ExpressionHandle(ArenaIndex(len(newExprs)))
			newExprs = append(newExprs, expr)
		} else {
			remap[i] = ^ir.ExpressionHandle(0) // sentinel
		}
	}
	f.Expressions = newExprs

	// Phase 5: Remap all expression handles within expressions.
	for i := range f.Expressions {
		f.Expressions[i].Kind = RemapExpressionHandles(f.Expressions[i].Kind, remap)
	}

	// Remap named expressions.
	newNamed := make(map[ir.ExpressionHandle]string, len(f.NamedExpressions))
	for h, name := range f.NamedExpressions {
		if used[h] {
			newNamed[remap[h]] = name
		}
	}
	f.NamedExpressions = newNamed

	// Remap local variable initializers.
	for i := range f.LocalVars {
		if f.LocalVars[i].Init != nil {
			newH := remap[*f.LocalVars[i].Init]
			f.LocalVars[i].Init = &newH
		}
	}

	// Remap expression types cache.
	if len(f.ExpressionTypes) > 0 {
		newTypes := make([]ir.TypeResolution, len(newExprs))
		for oldIdx := range n {
			if used[oldIdx] && int(remap[oldIdx]) < len(newTypes) && oldIdx < len(f.ExpressionTypes) {
				newTypes[remap[oldIdx]] = f.ExpressionTypes[oldIdx]
			}
		}
		f.ExpressionTypes = newTypes
	}

	// Remap statements, adjusting each Emit range to the expressions left in
	// it and dropping an Emit with none left.
	rm := remapThrough(remap)
	body := blockMapper{handle: rm, emit: compactEmitRange(remap, used)}
	f.Body = body.block(f.Body)
}

// usedExpressions returns which expressions of f are in use.
//
// Phase 1: Mark expressions directly referenced by statements, named
// expressions, and local variable initializers. Emit ranges do NOT mark
// expressions as used - matching Rust naga's trace_block which says:
// "since evaluating expressions has no effect, we don't need to assume
// that everything emitted is live."
func usedExpressions(f *ir.Function) []bool {
	n := len(f.Expressions)
	used := make([]bool, n)

	// Mark expressions referenced by named expressions (Rust: treat named as alive)
	for h := range f.NamedExpressions {
		if int(h) < n {
			used[h] = true
		}
	}

	// Mark expressions referenced by local variable initializers
	for _, lv := range f.LocalVars {
		if lv.Init != nil && int(*lv.Init) < n {
			used[*lv.Init] = true
		}
	}

	// Mark expressions referenced by statements (Emit is skipped inside)
	markStmtExprRefsForCompact(f.Body, used)

	// Phase 2: Propagate usage from back to front through expressions.
	// Since expressions can only refer to earlier expressions, a single
	// back-to-front pass computes the full transitive closure.
	for i := n - 1; i >= 0; i-- {
		if !used[i] {
			continue
		}
		markExprHandleRefs(f.Expressions[i].Kind, used)
	}
	return used
}

// markExprHandleRefs marks in referenced the operand expressions of kind.
func markExprHandleRefs(kind ir.ExpressionKind, referenced []bool) {
	mapExprHandles(kind, markInto(referenced))
}

// markStmtExprRefsForCompact marks in referenced the expressions stmts and
// their nested blocks use, as Rust naga's trace_block does: an Emit range
// marks nothing ("since evaluating expressions has no effect, we don't need
// to assume that everything emitted is live"). stmts is not modified.
func markStmtExprRefsForCompact(stmts []ir.Statement, referenced []bool) {
	tracer := blockMapper{handle: markInto(referenced), emit: keepEmitRange, fresh: true}
	tracer.block(stmts)
}

// compactEmitRange returns the Emit adjustment of expression compaction, as
// Rust naga's adjust_range: the range becomes the one from the first to the
// last of its expressions that stay, and an Emit with none left is dropped.
func compactEmitRange(remap []ir.ExpressionHandle, used []bool) func(ir.Range) (ir.Range, bool) {
	return func(r ir.Range) (ir.Range, bool) {
		var first, last ir.ExpressionHandle
		found := false
		for h := r.Start; h < r.End; h++ {
			if int(h) < len(used) && used[h] {
				if !found {
					first = remap[h]
					found = true
				}
				last = remap[h]
			}
		}
		if !found {
			return r, false
		}
		return ir.Range{Start: first, End: last + 1}, true
	}
}

// remapExprTypeHandles updates type handles within expression kinds.
func remapExprTypeHandles(kind ir.ExpressionKind, remap []ir.TypeHandle) ir.ExpressionKind {
	safe := func(h ir.TypeHandle) ir.TypeHandle {
		if h == ^ir.TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	switch k := kind.(type) {
	case ir.ExprZeroValue:
		k.Type = safe(k.Type)
		return k
	case ir.ExprCompose:
		k.Type = safe(k.Type)
		return k
	case ir.ExprSubgroupOperationResult:
		k.Type = safe(k.Type)
		return k
	case ir.ExprAtomicResult:
		k.Ty = safe(k.Ty)
		return k
	case ir.ExprWorkGroupUniformLoadResult:
		k.Ty = safe(k.Ty)
		return k
	default:
		return kind
	}
}
