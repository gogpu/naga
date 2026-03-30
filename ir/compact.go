package ir

// CompactUnused removes globals and functions not reachable from any entry point.
// Matches Rust naga's compact pass which traces from entry points and removes
// unreachable global variables, functions, and their associated types.
func CompactUnused(module *Module) {
	if len(module.EntryPoints) == 0 {
		return
	}

	// Mark used globals and functions by tracing from entry points.
	usedGlobals := make([]bool, len(module.GlobalVariables))
	usedFunctions := make([]bool, len(module.Functions))

	// Trace a function's expressions and statements for global/function references.
	var traceFunction func(f *Function)
	traceFunction = func(f *Function) {
		for _, expr := range f.Expressions {
			if gv, ok := expr.Kind.(ExprGlobalVariable); ok {
				usedGlobals[gv.Variable] = true
			}
		}
		traceStatementsForRefs(f.Body, usedGlobals, usedFunctions, module, traceFunction)
	}

	// Trace from all entry points.
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		traceFunction(&ep.Function)
		// Mesh shader references
		if ep.TaskPayload != nil {
			if int(*ep.TaskPayload) < len(usedGlobals) {
				usedGlobals[*ep.TaskPayload] = true
			}
		}
		if ep.MeshInfo != nil {
			if int(ep.MeshInfo.OutputVariable) < len(usedGlobals) {
				usedGlobals[ep.MeshInfo.OutputVariable] = true
			}
		}
	}

	// Count removals. If nothing to remove, skip.
	removeGlobals := false
	for _, used := range usedGlobals {
		if !used {
			removeGlobals = true
			break
		}
	}
	removeFunctions := false
	for _, used := range usedFunctions {
		if !used {
			removeFunctions = true
			break
		}
	}
	if !removeGlobals && !removeFunctions {
		return
	}

	// Build remap tables and compact.
	if removeGlobals {
		globalRemap := make([]GlobalVariableHandle, len(module.GlobalVariables))
		var newGlobals []GlobalVariable
		for i, g := range module.GlobalVariables {
			if usedGlobals[i] {
				globalRemap[i] = GlobalVariableHandle(len(newGlobals))
				newGlobals = append(newGlobals, g)
			}
		}
		module.GlobalVariables = newGlobals

		// Remap global variable handles in all expressions.
		remapGlobalVarHandles := func(f *Function) {
			for j := range f.Expressions {
				if gv, ok := f.Expressions[j].Kind.(ExprGlobalVariable); ok {
					f.Expressions[j].Kind = ExprGlobalVariable{Variable: globalRemap[gv.Variable]}
				}
			}
		}
		for i := range module.Functions {
			remapGlobalVarHandles(&module.Functions[i])
		}
		for i := range module.EntryPoints {
			remapGlobalVarHandles(&module.EntryPoints[i].Function)
			if module.EntryPoints[i].TaskPayload != nil {
				v := globalRemap[*module.EntryPoints[i].TaskPayload]
				module.EntryPoints[i].TaskPayload = &v
			}
			if module.EntryPoints[i].MeshInfo != nil {
				module.EntryPoints[i].MeshInfo.OutputVariable = globalRemap[module.EntryPoints[i].MeshInfo.OutputVariable]
			}
		}
	}

	if removeFunctions {
		funcRemap := make([]FunctionHandle, len(module.Functions))
		var newFunctions []Function
		for i, f := range module.Functions {
			if usedFunctions[i] {
				funcRemap[i] = FunctionHandle(len(newFunctions))
				newFunctions = append(newFunctions, f)
			}
		}
		module.Functions = newFunctions

		// Remap function handles in call statements and call result expressions.
		remapFuncHandles := func(f *Function) {
			for j := range f.Expressions {
				if cr, ok := f.Expressions[j].Kind.(ExprCallResult); ok {
					f.Expressions[j].Kind = ExprCallResult{Function: funcRemap[cr.Function]}
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
}

// traceStatementsForRefs traces statements for global variable and function call references.
func traceStatementsForRefs(stmts []Statement, usedGlobals []bool, usedFunctions []bool, module *Module, traceFunc func(*Function)) {
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case StmtCall:
			if int(s.Function) < len(usedFunctions) && !usedFunctions[s.Function] {
				usedFunctions[s.Function] = true
				if int(s.Function) < len(module.Functions) {
					traceFunc(&module.Functions[s.Function])
				}
			}
		case StmtBlock:
			traceStatementsForRefs(s.Block, usedGlobals, usedFunctions, module, traceFunc)
		case StmtIf:
			traceStatementsForRefs(s.Accept, usedGlobals, usedFunctions, module, traceFunc)
			traceStatementsForRefs(s.Reject, usedGlobals, usedFunctions, module, traceFunc)
		case StmtSwitch:
			for _, c := range s.Cases {
				traceStatementsForRefs(c.Body, usedGlobals, usedFunctions, module, traceFunc)
			}
		case StmtLoop:
			traceStatementsForRefs(s.Body, usedGlobals, usedFunctions, module, traceFunc)
			traceStatementsForRefs(s.Continuing, usedGlobals, usedFunctions, module, traceFunc)
		}
	}
}

// remapStmtFuncHandles remaps FunctionHandle in StmtCall within statement trees.
func remapStmtFuncHandles(stmts []Statement, remap []FunctionHandle) {
	for i := range stmts {
		switch s := stmts[i].Kind.(type) {
		case StmtCall:
			stmts[i].Kind = StmtCall{
				Function:  remap[s.Function],
				Arguments: s.Arguments,
				Result:    s.Result,
			}
		case StmtBlock:
			remapStmtFuncHandles(s.Block, remap)
		case StmtIf:
			remapStmtFuncHandles(s.Accept, remap)
			remapStmtFuncHandles(s.Reject, remap)
		case StmtSwitch:
			for j := range s.Cases {
				remapStmtFuncHandles(s.Cases[j].Body, remap)
			}
		case StmtLoop:
			remapStmtFuncHandles(s.Body, remap)
			remapStmtFuncHandles(s.Continuing, remap)
		}
	}
}

// CompactTypes removes anonymous types that are not referenced by any handle
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
func CompactTypes(module *Module) {
	if len(module.Types) == 0 {
		return
	}

	// Step 1: Find all type handles referenced directly by handles
	// (not embedded by value in Vector/Matrix/Atomic).
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

	// Functions (regular)
	for i := range module.Functions {
		markFunctionTypeRefs(&module.Functions[i], referenced)
	}

	// Entry point functions (inline in EntryPoints, not in Functions[])
	for i := range module.EntryPoints {
		markFunctionTypeRefs(&module.EntryPoints[i].Function, referenced)
	}

	// Step 2: Determine which types to keep.
	// Keep referenced types AND all named types.
	// Rust naga's compact keeps all named types (structs, aliases, etc.) regardless
	// of reference count. Only abstract types are force-removed.
	keep := make([]bool, len(module.Types))
	for i, t := range module.Types {
		if IsAbstractType(t.Inner, module.Types) {
			continue // Abstract types must not reach backends
		}
		if referenced[i] || t.Name != "" {
			keep[i] = true
		}
	}

	// Check if anything would be removed
	allKept := true
	for _, k := range keep {
		if !k {
			allKept = false
			break
		}
	}
	if allKept {
		return // Nothing to compact
	}

	// Step 3: Build remap table (old handle -> new handle)
	remap := make([]TypeHandle, len(module.Types))
	newTypes := make([]Type, 0, len(module.Types))
	for i, t := range module.Types {
		if keep[i] {
			remap[i] = TypeHandle(len(newTypes))
			newTypes = append(newTypes, t)
		} else {
			remap[i] = ^TypeHandle(0) // sentinel for removed
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
	}

	// Remap type handles in GlobalExpressions (Compose.Type, ZeroValue.Type, etc.)
	// GlobalExpressions are built inline during constant lowering (before compact),
	// so their type handles need remapping.
	for i := range module.GlobalExpressions {
		module.GlobalExpressions[i].Kind = remapExprTypeHandles(module.GlobalExpressions[i].Kind, remap)
	}

	// Remap and filter TypeUseOrder — remove handles for deleted types,
	// remap surviving handles to new indices.
	if len(module.TypeUseOrder) > 0 {
		filtered := make([]TypeHandle, 0, len(module.TypeUseOrder))
		for _, h := range module.TypeUseOrder {
			if int(h) < len(remap) && remap[h] != ^TypeHandle(0) {
				filtered = append(filtered, remap[h])
			}
		}
		module.TypeUseOrder = filtered
	}
}

// remapFunctionTypes remaps all type handles within a function.
func remapFunctionTypes(f *Function, remap []TypeHandle) {
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
			if newH == ^TypeHandle(0) {
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

// concretizeTypeInner converts abstract scalar kinds to concrete equivalents.
// AbstractInt → Sint(4), AbstractFloat → Float(4).
func concretizeTypeInner(inner TypeInner) TypeInner {
	concretize := func(s ScalarType) ScalarType {
		switch s.Kind {
		case ScalarAbstractInt:
			return ScalarType{Kind: ScalarSint, Width: 4}
		case ScalarAbstractFloat:
			return ScalarType{Kind: ScalarFloat, Width: 4}
		default:
			return s
		}
	}
	switch t := inner.(type) {
	case ScalarType:
		return concretize(t)
	case VectorType:
		t.Scalar = concretize(t.Scalar)
		return t
	case MatrixType:
		t.Scalar = concretize(t.Scalar)
		return t
	default:
		return inner
	}
}

// markTypeInnerRefs marks type handles referenced by a TypeInner.
// Only types that use handles (not embedded values) are marked.
// Abstract types are removed by compact and must never reach backends.
func IsAbstractType(inner TypeInner, types []Type) bool {
	switch t := inner.(type) {
	case ScalarType:
		return t.Kind == ScalarAbstractInt || t.Kind == ScalarAbstractFloat
	case VectorType:
		return t.Scalar.Kind == ScalarAbstractInt || t.Scalar.Kind == ScalarAbstractFloat
	case MatrixType:
		return t.Scalar.Kind == ScalarAbstractInt || t.Scalar.Kind == ScalarAbstractFloat
	case ArrayType:
		if int(t.Base) < len(types) {
			return IsAbstractType(types[t.Base].Inner, types)
		}
	}
	return false
}

func markTypeInnerRefs(inner TypeInner, referenced []bool) {
	switch t := inner.(type) {
	case ArrayType:
		referenced[t.Base] = true
	case StructType:
		for _, m := range t.Members {
			referenced[m.Type] = true
		}
	case PointerType:
		referenced[t.Base] = true
	case BindingArrayType:
		referenced[t.Base] = true
		// VectorType, MatrixType, AtomicType, ScalarType, SamplerType,
		// ImageType, AccelerationStructureType, RayQueryType:
		// These either have no type handle refs or embed scalars by value.
	}
}

// markFunctionTypeRefs marks type handles referenced by a function's args, result, locals, and expressions.
func markFunctionTypeRefs(f *Function, referenced []bool) {
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
func markExprTypeRefs(kind ExpressionKind, referenced []bool) {
	switch k := kind.(type) {
	case ExprZeroValue:
		referenced[k.Type] = true
	case ExprCompose:
		referenced[k.Type] = true
	case ExprSubgroupOperationResult:
		referenced[k.Type] = true
	case ExprAtomicResult:
		referenced[k.Ty] = true
	}
}

// ReorderTypes reorders the type arena so that types appear in first-use order
// when scanning: constants → overrides → globals → functions → entry points.
// This matches Rust naga where types are registered during dependency-ordered
// lowering and dead intermediate types are never re-registered.
//
// Must be called AFTER CompactTypes (which removes unreferenced types).
func ReorderTypes(module *Module) {
	if len(module.Types) == 0 {
		return
	}

	// Collect types in first-encounter order
	seen := make([]bool, len(module.Types))
	var order []TypeHandle

	visit := func(h TypeHandle) {
		if h == ^TypeHandle(0) || int(h) >= len(module.Types) || seen[h] {
			return
		}
		// Visit type's own dependencies first (array base, struct members, etc.)
		visitTypeInnerDeps(module.Types[h].Inner, module.Types, seen, &order)
		if !seen[h] {
			seen[h] = true
			order = append(order, h)
		}
	}

	// Use TypeUseOrder — records the exact registration order from
	// dependency-ordered lowering, remapped by CompactTypes to surviving
	// handles only. This preserves the interleaved constants/functions
	// order that matches Rust naga's type arena.
	for _, h := range module.TypeUseOrder {
		visit(h)
	}
	// Any remaining types not yet visited
	for i := range module.Types {
		if !seen[i] {
			visit(TypeHandle(i))
		}
	}

	// Check if order is already correct
	alreadyOrdered := len(order) == len(module.Types)
	if alreadyOrdered {
		for i, h := range order {
			if int(h) != i {
				alreadyOrdered = false
				break
			}
		}
	}
	if alreadyOrdered {
		return
	}

	// Build remap table
	remap := make([]TypeHandle, len(module.Types))
	newTypes := make([]Type, len(order))
	for newIdx, oldIdx := range order {
		if int(oldIdx) < len(remap) {
			remap[oldIdx] = TypeHandle(newIdx)
		}
		if int(oldIdx) < len(module.Types) {
			newTypes[newIdx] = module.Types[oldIdx]
		}
	}

	// Apply remap — use safe wrapper that guards sentinel handles
	module.Types = newTypes
	safeRemap := func(h TypeHandle) TypeHandle {
		if h == ^TypeHandle(0) || int(h) >= len(remap) {
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
func visitTypeInnerDeps(inner TypeInner, types []Type, seen []bool, order *[]TypeHandle) {
	guard := func(h TypeHandle) bool {
		return h != ^TypeHandle(0) && int(h) < len(types) && !seen[h]
	}
	switch t := inner.(type) {
	case ArrayType:
		if guard(t.Base) {
			visitTypeInnerDeps(types[t.Base].Inner, types, seen, order)
			if !seen[t.Base] {
				seen[t.Base] = true
				*order = append(*order, t.Base)
			}
		}
	case StructType:
		for _, m := range t.Members {
			if guard(m.Type) {
				visitTypeInnerDeps(types[m.Type].Inner, types, seen, order)
				if guard(m.Type) {
					seen[m.Type] = true
					*order = append(*order, m.Type)
				}
			}
		}
	case PointerType:
		if guard(t.Base) {
			visitTypeInnerDeps(types[t.Base].Inner, types, seen, order)
			if guard(t.Base) {
				seen[t.Base] = true
				*order = append(*order, t.Base)
			}
		}
	case BindingArrayType:
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
func visitFunctionTypes(f *Function, visit func(TypeHandle)) {
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
func visitExprTypeHandles(kind ExpressionKind, visit func(TypeHandle)) {
	switch k := kind.(type) {
	case ExprZeroValue:
		visit(k.Type)
	case ExprCompose:
		visit(k.Type)
	case ExprSubgroupOperationResult:
		visit(k.Type)
	case ExprAtomicResult:
		visit(k.Ty)
	}
}

// safeRemapFunctionTypes remaps type handles within a function, guarding against sentinel values.
// Used by ReorderTypes where ExpressionTypes may contain nil/sentinel handles from CompactTypes.
func safeRemapFunctionTypes(f *Function, remap []TypeHandle) {
	safe := func(h TypeHandle) TypeHandle {
		if h == ^TypeHandle(0) || int(h) >= len(remap) {
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
			if h != ^TypeHandle(0) && int(h) < len(remap) {
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
func remapTypeInner(inner TypeInner, remap []TypeHandle) TypeInner {
	safe := func(h TypeHandle) TypeHandle {
		if h == ^TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	switch t := inner.(type) {
	case ArrayType:
		t.Base = safe(t.Base)
		return t
	case StructType:
		members := make([]StructMember, len(t.Members))
		copy(members, t.Members)
		for i := range members {
			members[i].Type = safe(members[i].Type)
		}
		t.Members = members
		return t
	case PointerType:
		t.Base = safe(t.Base)
		return t
	case BindingArrayType:
		t.Base = safe(t.Base)
		return t
	default:
		return inner
	}
}

// CompactConstants removes abstract-typed constants from the module and remaps
// all ConstantHandle references. This matches Rust naga's compact pass which
// removes constants whose type is abstract (is_abstract returns true) and
// unnamed constants. With KeepUnused::Yes (which the WGSL frontend uses),
// named non-abstract constants are always kept.
//
// Since our lowerer concretizes abstract types during lowering, we use the
// IsAbstract flag (set during lowering for constants that originated from
// abstract-typed WGSL const declarations) to identify removable constants.
func CompactConstants(module *Module) {
	if len(module.Constants) == 0 {
		return
	}

	// Determine which constants to keep.
	// Matches Rust: constant.name.is_none() || type.is_abstract() → skip
	keep := make([]bool, len(module.Constants))
	for i, c := range module.Constants {
		isNamed := c.Name != "" && c.Name != "_"
		isAbstract := c.IsAbstract
		if !isAbstract && int(c.Type) < len(module.Types) {
			isAbstract = IsAbstractType(module.Types[c.Type].Inner, module.Types)
		}
		if isNamed && !isAbstract {
			keep[i] = true
		}
	}

	// Also keep any constant referenced by ExprConstant in any function.
	markConstantRefs := func(f *Function) {
		for _, expr := range f.Expressions {
			if ec, ok := expr.Kind.(ExprConstant); ok {
				if int(ec.Constant) < len(keep) {
					keep[ec.Constant] = true
				}
			}
		}
	}
	for i := range module.Functions {
		markConstantRefs(&module.Functions[i])
	}
	for i := range module.EntryPoints {
		markConstantRefs(&module.EntryPoints[i].Function)
	}

	// Also keep constants referenced by global variable initializers.
	for _, g := range module.GlobalVariables {
		if g.Init != nil {
			if int(*g.Init) < len(keep) {
				keep[*g.Init] = true
			}
		}
	}

	// Transitively keep constants referenced by kept CompositeValue components.
	changed := true
	for changed {
		changed = false
		for i, c := range module.Constants {
			if !keep[i] {
				continue
			}
			if cv, ok := c.Value.(CompositeValue); ok {
				for _, comp := range cv.Components {
					if int(comp) < len(keep) && !keep[comp] {
						keep[comp] = true
						changed = true
					}
				}
			}
		}
	}

	// Check if anything would be removed.
	allKept := true
	for _, k := range keep {
		if !k {
			allKept = false
			break
		}
	}
	if allKept {
		return
	}

	// Build remap table.
	remap := make([]ConstantHandle, len(module.Constants))
	newConstants := make([]Constant, 0, len(module.Constants))
	for i, c := range module.Constants {
		if keep[i] {
			remap[i] = ConstantHandle(len(newConstants))
			newConstants = append(newConstants, c)
		} else {
			remap[i] = ^ConstantHandle(0) // sentinel
		}
	}

	// Apply new constants.
	module.Constants = newConstants

	// Remap CompositeValue.Components within surviving constants.
	for i := range module.Constants {
		if cv, ok := module.Constants[i].Value.(CompositeValue); ok {
			newComps := make([]ConstantHandle, len(cv.Components))
			for j, c := range cv.Components {
				if int(c) < len(remap) {
					newComps[j] = remap[c]
				} else {
					newComps[j] = c
				}
			}
			cv.Components = newComps
			module.Constants[i].Value = cv
		}
	}

	// Remap ExprConstant handles in all functions.
	remapConstantExprs := func(f *Function) {
		for ei := range f.Expressions {
			if ec, ok := f.Expressions[ei].Kind.(ExprConstant); ok {
				if int(ec.Constant) < len(remap) {
					ec.Constant = remap[ec.Constant]
					f.Expressions[ei].Kind = ec
				}
			}
		}
	}
	for fi := range module.Functions {
		remapConstantExprs(&module.Functions[fi])
	}
	for ei := range module.EntryPoints {
		remapConstantExprs(&module.EntryPoints[ei].Function)
	}

	// Remap GlobalVariable.Init constant handles.
	for i := range module.GlobalVariables {
		if module.GlobalVariables[i].Init != nil {
			old := *module.GlobalVariables[i].Init
			if int(old) < len(remap) {
				newH := remap[old]
				module.GlobalVariables[i].Init = &newH
			}
		}
	}

	// Note: Overrides are a separate arena now (not keyed by ConstantHandle),
	// so no remapping needed here. Override handles are stable.
}

// CompactExpressions removes unreferenced expressions from each function
// in the module and renumbers all expression handles. This matches Rust naga's
// compact pass which removes dead expressions (e.g., original abstract literals
// replaced by concretized versions).
//
// The algorithm matches Rust naga's compact:
// 1. Mark expressions directly used by statements (NOT Emit ranges - those are no-ops)
// 2. Mark named expressions and local variable initializers as used
// 3. Propagate usage back-to-front through expressions (transitive closure)
// 4. Remove unused expressions, remap handles, adjust Emit ranges
func CompactExpressions(module *Module) {
	for fi := range module.Functions {
		compactFunctionExpressions(&module.Functions[fi])
	}
	for ei := range module.EntryPoints {
		compactFunctionExpressions(&module.EntryPoints[ei].Function)
	}
}

func compactFunctionExpressions(f *Function) {
	n := len(f.Expressions)
	if n == 0 {
		return
	}

	// Phase 1: Mark expressions directly referenced by statements, named
	// expressions, and local variable initializers. Emit ranges do NOT mark
	// expressions as used - matching Rust naga's trace_block which says:
	// "since evaluating expressions has no effect, we don't need to assume
	// that everything emitted is live."
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

	// Phase 3: Check if anything would be removed.
	allUsed := true
	for _, u := range used {
		if !u {
			allUsed = false
			break
		}
	}
	if allUsed {
		return
	}

	// Phase 4: Build remap table.
	remap := make([]ExpressionHandle, n)
	newExprs := make([]Expression, 0, n)
	for i, expr := range f.Expressions {
		if used[i] {
			remap[i] = ExpressionHandle(len(newExprs))
			newExprs = append(newExprs, expr)
		} else {
			remap[i] = ^ExpressionHandle(0) // sentinel
		}
	}
	f.Expressions = newExprs

	// Phase 5: Remap all expression handles within expressions.
	for i := range f.Expressions {
		f.Expressions[i].Kind = remapExprHandles(f.Expressions[i].Kind, remap)
	}

	// Remap named expressions.
	newNamed := make(map[ExpressionHandle]string, len(f.NamedExpressions))
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
		newTypes := make([]TypeResolution, len(newExprs))
		for oldIdx := range n {
			if used[oldIdx] && int(remap[oldIdx]) < len(newTypes) && oldIdx < len(f.ExpressionTypes) {
				newTypes[remap[oldIdx]] = f.ExpressionTypes[oldIdx]
			}
		}
		f.ExpressionTypes = newTypes
	}

	// Remap statements (including proper Emit range adjustment).
	f.Body = remapStmtExprHandlesCompact(f.Body, remap, used)
}

// markExprHandleRefs marks expression handles referenced by an expression kind.
func markExprHandleRefs(kind ExpressionKind, referenced []bool) {
	mark := func(h ExpressionHandle) {
		if int(h) < len(referenced) {
			referenced[h] = true
		}
	}
	markOpt := func(h *ExpressionHandle) {
		if h != nil && int(*h) < len(referenced) {
			referenced[*h] = true
		}
	}
	switch k := kind.(type) {
	case ExprCompose:
		for _, c := range k.Components {
			mark(c)
		}
	case ExprSplat:
		mark(k.Value)
	case ExprSwizzle:
		mark(k.Vector)
	case ExprAccess:
		mark(k.Base)
		mark(k.Index)
	case ExprAccessIndex:
		mark(k.Base)
	case ExprLoad:
		mark(k.Pointer)
	case ExprImageSample:
		mark(k.Image)
		mark(k.Sampler)
		mark(k.Coordinate)
		markOpt(k.ArrayIndex)
		markOpt(k.DepthRef)
		markOpt(k.Offset)
		// Level is SampleLevel interface — check concrete types
		markSampleLevelRefs(k.Level, mark)
	case ExprImageLoad:
		mark(k.Image)
		mark(k.Coordinate)
		markOpt(k.ArrayIndex)
		markOpt(k.Sample)
		markOpt(k.Level)
	case ExprImageQuery:
		mark(k.Image)
		if q, ok := k.Query.(ImageQuerySize); ok {
			markOpt(q.Level)
		}
	case ExprUnary:
		mark(k.Expr)
	case ExprBinary:
		mark(k.Left)
		mark(k.Right)
	case ExprSelect:
		mark(k.Condition)
		mark(k.Accept)
		mark(k.Reject)
	case ExprRelational:
		mark(k.Argument)
	case ExprMath:
		mark(k.Arg)
		markOpt(k.Arg1)
		markOpt(k.Arg2)
		markOpt(k.Arg3)
	case ExprAs:
		mark(k.Expr)
	case ExprArrayLength:
		mark(k.Array)
	case ExprRayQueryGetIntersection:
		mark(k.Query)
	case ExprDerivative:
		mark(k.Expr)
		// These have no expression refs:
		// Literal, ExprConstant, ExprZeroValue, ExprGlobalVariable,
		// ExprLocalVariable, ExprFunctionArgument, ExprCallResult,
		// ExprAtomicResult, ExprSubgroupBallotResult,
		// ExprSubgroupOperationResult, ExprRayQueryProceedResult,
		// ExprWorkGroupUniformLoadResult
	}
}

// markSampleLevelRefs marks expression handles in SampleLevel variants.
func markSampleLevelRefs(level SampleLevel, mark func(ExpressionHandle)) {
	if level == nil {
		return
	}
	switch l := level.(type) {
	case SampleLevelExact:
		mark(l.Level)
	case SampleLevelBias:
		mark(l.Bias)
	case SampleLevelGradient:
		mark(l.X)
		mark(l.Y)
	}
}

// markStmtExprRefs marks expression handles referenced by statements.
func markStmtExprRefs(stmts []Statement, referenced []bool) {
	mark := func(h ExpressionHandle) {
		if int(h) < len(referenced) {
			referenced[h] = true
		}
	}
	markOpt := func(h *ExpressionHandle) {
		if h != nil && int(*h) < len(referenced) {
			referenced[*h] = true
		}
	}
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case StmtBlock:
			markStmtExprRefs(s.Block, referenced)
		case StmtIf:
			mark(s.Condition)
			markStmtExprRefs(s.Accept, referenced)
			markStmtExprRefs(s.Reject, referenced)
		case StmtSwitch:
			mark(s.Selector)
			for _, c := range s.Cases {
				markStmtExprRefs(c.Body, referenced)
			}
		case StmtLoop:
			markStmtExprRefs(s.Body, referenced)
			markStmtExprRefs(s.Continuing, referenced)
			markOpt(s.BreakIf)
		case StmtReturn:
			markOpt(s.Value)
		case StmtStore:
			mark(s.Pointer)
			mark(s.Value)
		case StmtImageStore:
			mark(s.Image)
			mark(s.Coordinate)
			markOpt(s.ArrayIndex)
			mark(s.Value)
		case StmtCall:
			for _, a := range s.Arguments {
				mark(a)
			}
			markOpt(s.Result)
		case StmtAtomic:
			mark(s.Pointer)
			markAtomicFunctionRefs(s.Fun, markOpt)
			mark(s.Value)
			markOpt(s.Result)
		case StmtWorkGroupUniformLoad:
			mark(s.Pointer)
			mark(s.Result)
		case StmtRayQuery:
			mark(s.Query)
			markRayQueryFunctionRefs(s.Fun, mark)
		case StmtSubgroupBallot:
			markOpt(s.Predicate)
			mark(s.Result)
		case StmtSubgroupGather:
			markGatherModeRefs(s.Mode, mark)
			mark(s.Argument)
			mark(s.Result)
		case StmtEmit:
			// All expressions in the emit range are referenced.
			for h := s.Range.Start; h < s.Range.End; h++ {
				mark(h)
			}
		case StmtImageAtomic:
			mark(s.Image)
			mark(s.Coordinate)
			markOpt(s.ArrayIndex)
			mark(s.Value)
		case StmtSubgroupCollectiveOperation:
			mark(s.Argument)
			mark(s.Result)
		}
	}
}

// markStmtExprRefsForCompact marks expression handles referenced by statements,
// matching Rust naga's trace_block. Crucially, Emit ranges do NOT mark their
// expressions as used — "since evaluating expressions has no effect, we don't
// need to assume that everything emitted is live."
func markStmtExprRefsForCompact(stmts []Statement, referenced []bool) {
	mark := func(h ExpressionHandle) {
		if int(h) < len(referenced) {
			referenced[h] = true
		}
	}
	markOpt := func(h *ExpressionHandle) {
		if h != nil && int(*h) < len(referenced) {
			referenced[*h] = true
		}
	}
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case StmtEmit:
			// Intentionally empty — Emit does not mark expressions as used.
			// Expressions within the range will be marked if actually used by
			// other statements or expressions.
			_ = s
		case StmtBlock:
			markStmtExprRefsForCompact(s.Block, referenced)
		case StmtIf:
			mark(s.Condition)
			markStmtExprRefsForCompact(s.Accept, referenced)
			markStmtExprRefsForCompact(s.Reject, referenced)
		case StmtSwitch:
			mark(s.Selector)
			for _, c := range s.Cases {
				markStmtExprRefsForCompact(c.Body, referenced)
			}
		case StmtLoop:
			markStmtExprRefsForCompact(s.Body, referenced)
			markStmtExprRefsForCompact(s.Continuing, referenced)
			markOpt(s.BreakIf)
		case StmtReturn:
			markOpt(s.Value)
		case StmtStore:
			mark(s.Pointer)
			mark(s.Value)
		case StmtImageStore:
			mark(s.Image)
			mark(s.Coordinate)
			markOpt(s.ArrayIndex)
			mark(s.Value)
		case StmtCall:
			for _, a := range s.Arguments {
				mark(a)
			}
			markOpt(s.Result)
		case StmtAtomic:
			mark(s.Pointer)
			markAtomicFunctionRefs(s.Fun, markOpt)
			mark(s.Value)
			markOpt(s.Result)
		case StmtWorkGroupUniformLoad:
			mark(s.Pointer)
			mark(s.Result)
		case StmtRayQuery:
			mark(s.Query)
			markRayQueryFunctionRefs(s.Fun, mark)
		case StmtSubgroupBallot:
			markOpt(s.Predicate)
			mark(s.Result)
		case StmtSubgroupGather:
			markGatherModeRefs(s.Mode, mark)
			mark(s.Argument)
			mark(s.Result)
		case StmtImageAtomic:
			mark(s.Image)
			mark(s.Coordinate)
			markOpt(s.ArrayIndex)
			mark(s.Value)
		case StmtSubgroupCollectiveOperation:
			mark(s.Argument)
			mark(s.Result)
		}
	}
}

// markRayQueryFunctionRefs marks expression handles in RayQueryFunction variants.
func markRayQueryFunctionRefs(fun RayQueryFunction, mark func(ExpressionHandle)) {
	switch f := fun.(type) {
	case RayQueryInitialize:
		mark(f.AccelerationStructure)
		mark(f.Descriptor)
	case RayQueryProceed:
		mark(f.Result)
	case RayQueryGenerateIntersection:
		mark(f.HitT)
	}
}

// markAtomicFunctionRefs marks expression handles in AtomicFunction variants.
func markAtomicFunctionRefs(fun AtomicFunction, markOpt func(*ExpressionHandle)) {
	if ex, ok := fun.(AtomicExchange); ok {
		markOpt(ex.Compare)
	}
}

// markGatherModeRefs marks expression handles in GatherMode variants.
func markGatherModeRefs(mode GatherMode, mark func(ExpressionHandle)) {
	switch m := mode.(type) {
	case GatherBroadcast:
		mark(m.Index)
	case GatherShuffle:
		mark(m.Index)
	case GatherShuffleDown:
		mark(m.Delta)
	case GatherShuffleUp:
		mark(m.Delta)
	case GatherShuffleXor:
		mark(m.Mask)
	case GatherQuadBroadcast:
		mark(m.Index)
	}
}

// remapStmtExprHandlesCompact remaps expression handles in all statements,
// with proper Emit range adjustment matching Rust naga's adjust_range.
// For Emit ranges, we find the first and last used handles within the original
// range and create a new contiguous range. If no handles are used, the Emit
// statement is removed from the block.
//
// Since Emit statements may be removed, this function works on block pointers
// for sub-blocks. The top-level call should use remapBodyCompact.
func remapStmtExprHandlesCompact(stmts []Statement, remap []ExpressionHandle, used []bool) []Statement {
	rm := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(remap) {
			return remap[h]
		}
		return h
	}
	rmOpt := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		v := rm(*h)
		return &v
	}

	// Process in-place, filtering out empty Emit ranges.
	w := 0
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case StmtEmit:
			// Adjust Emit range: find first and last used handles within
			// the original range, matching Rust naga's adjust_range.
			var firstNew, lastNew ExpressionHandle
			foundFirst := false
			for h := s.Range.Start; h < s.Range.End; h++ {
				if int(h) < len(used) && used[h] {
					mapped := remap[h]
					if !foundFirst {
						firstNew = mapped
						foundFirst = true
					}
					lastNew = mapped
				}
			}
			if !foundFirst {
				// No used expressions in range — remove this Emit statement
				continue
			}
			s.Range.Start = firstNew
			s.Range.End = lastNew + 1 // end-exclusive
			stmts[w] = Statement{Kind: s}
			w++
		case StmtBlock:
			s.Block = Block(remapStmtExprHandlesCompact([]Statement(s.Block), remap, used))
			stmts[w] = Statement{Kind: s}
			w++
		case StmtIf:
			s.Condition = rm(s.Condition)
			s.Accept = Block(remapStmtExprHandlesCompact([]Statement(s.Accept), remap, used))
			s.Reject = Block(remapStmtExprHandlesCompact([]Statement(s.Reject), remap, used))
			stmts[w] = Statement{Kind: s}
			w++
		case StmtSwitch:
			s.Selector = rm(s.Selector)
			for ci := range s.Cases {
				s.Cases[ci].Body = Block(remapStmtExprHandlesCompact([]Statement(s.Cases[ci].Body), remap, used))
			}
			stmts[w] = Statement{Kind: s}
			w++
		case StmtLoop:
			s.Body = Block(remapStmtExprHandlesCompact([]Statement(s.Body), remap, used))
			s.Continuing = Block(remapStmtExprHandlesCompact([]Statement(s.Continuing), remap, used))
			s.BreakIf = rmOpt(s.BreakIf)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtReturn:
			s.Value = rmOpt(s.Value)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtStore:
			s.Pointer = rm(s.Pointer)
			s.Value = rm(s.Value)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtImageStore:
			s.Image = rm(s.Image)
			s.Coordinate = rm(s.Coordinate)
			s.ArrayIndex = rmOpt(s.ArrayIndex)
			s.Value = rm(s.Value)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtCall:
			for ai := range s.Arguments {
				s.Arguments[ai] = rm(s.Arguments[ai])
			}
			s.Result = rmOpt(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtAtomic:
			s.Pointer = rm(s.Pointer)
			s.Fun = remapAtomicFunction(s.Fun, rmOpt)
			s.Value = rm(s.Value)
			s.Result = rmOpt(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtWorkGroupUniformLoad:
			s.Pointer = rm(s.Pointer)
			s.Result = rm(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtRayQuery:
			s.Query = rm(s.Query)
			s.Fun = remapRayQueryFunction(s.Fun, rm)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtSubgroupBallot:
			s.Predicate = rmOpt(s.Predicate)
			s.Result = rm(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtSubgroupGather:
			s.Mode = remapGatherMode(s.Mode, rm)
			s.Argument = rm(s.Argument)
			s.Result = rm(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtImageAtomic:
			s.Image = rm(s.Image)
			s.Coordinate = rm(s.Coordinate)
			s.ArrayIndex = rmOpt(s.ArrayIndex)
			s.Value = rm(s.Value)
			stmts[w] = Statement{Kind: s}
			w++
		case StmtSubgroupCollectiveOperation:
			s.Argument = rm(s.Argument)
			s.Result = rm(s.Result)
			stmts[w] = Statement{Kind: s}
			w++
		default:
			// Pass through unchanged (Break, Continue, Kill, barriers, etc.)
			stmts[w] = stmt
			w++
		}
	}
	return stmts[:w]
}

// remapRayQueryFunction remaps expression handles in RayQueryFunction variants.
func remapRayQueryFunction(fun RayQueryFunction, rm func(ExpressionHandle) ExpressionHandle) RayQueryFunction {
	switch f := fun.(type) {
	case RayQueryInitialize:
		f.AccelerationStructure = rm(f.AccelerationStructure)
		f.Descriptor = rm(f.Descriptor)
		return f
	case RayQueryProceed:
		f.Result = rm(f.Result)
		return f
	case RayQueryGenerateIntersection:
		f.HitT = rm(f.HitT)
		return f
	default:
		return fun
	}
}

// remapAtomicFunction remaps expression handles in AtomicFunction variants.
func remapAtomicFunction(fun AtomicFunction, rmOpt func(*ExpressionHandle) *ExpressionHandle) AtomicFunction {
	if ex, ok := fun.(AtomicExchange); ok {
		ex.Compare = rmOpt(ex.Compare)
		return ex
	}
	return fun
}

// remapGatherMode remaps expression handles in GatherMode variants.
func remapGatherMode(mode GatherMode, rm func(ExpressionHandle) ExpressionHandle) GatherMode {
	switch m := mode.(type) {
	case GatherBroadcast:
		m.Index = rm(m.Index)
		return m
	case GatherShuffle:
		m.Index = rm(m.Index)
		return m
	case GatherShuffleDown:
		m.Delta = rm(m.Delta)
		return m
	case GatherShuffleUp:
		m.Delta = rm(m.Delta)
		return m
	case GatherShuffleXor:
		m.Mask = rm(m.Mask)
		return m
	case GatherQuadBroadcast:
		m.Index = rm(m.Index)
		return m
	default:
		return mode
	}
}

// remapExprHandles remaps expression handles within an expression kind.
func remapExprHandles(kind ExpressionKind, remap []ExpressionHandle) ExpressionKind {
	rm := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(remap) {
			return remap[h]
		}
		return h
	}
	rmOpt := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		v := rm(*h)
		return &v
	}
	switch k := kind.(type) {
	case ExprCompose:
		comps := make([]ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			comps[i] = rm(c)
		}
		k.Components = comps
		return k
	case ExprSplat:
		k.Value = rm(k.Value)
		return k
	case ExprSwizzle:
		k.Vector = rm(k.Vector)
		return k
	case ExprAccess:
		k.Base = rm(k.Base)
		k.Index = rm(k.Index)
		return k
	case ExprAccessIndex:
		k.Base = rm(k.Base)
		return k
	case ExprLoad:
		k.Pointer = rm(k.Pointer)
		return k
	case ExprImageSample:
		k.Image = rm(k.Image)
		k.Sampler = rm(k.Sampler)
		k.Coordinate = rm(k.Coordinate)
		k.ArrayIndex = rmOpt(k.ArrayIndex)
		k.Level = remapSampleLevel(k.Level, rm)
		k.DepthRef = rmOpt(k.DepthRef)
		k.Offset = rmOpt(k.Offset)
		return k
	case ExprImageLoad:
		k.Image = rm(k.Image)
		k.Coordinate = rm(k.Coordinate)
		k.ArrayIndex = rmOpt(k.ArrayIndex)
		k.Sample = rmOpt(k.Sample)
		k.Level = rmOpt(k.Level)
		return k
	case ExprImageQuery:
		k.Image = rm(k.Image)
		switch q := k.Query.(type) {
		case ImageQuerySize:
			q.Level = rmOpt(q.Level)
			k.Query = q
		}
		return k
	case ExprUnary:
		k.Expr = rm(k.Expr)
		return k
	case ExprBinary:
		k.Left = rm(k.Left)
		k.Right = rm(k.Right)
		return k
	case ExprSelect:
		k.Condition = rm(k.Condition)
		k.Accept = rm(k.Accept)
		k.Reject = rm(k.Reject)
		return k
	case ExprRelational:
		k.Argument = rm(k.Argument)
		return k
	case ExprMath:
		k.Arg = rm(k.Arg)
		k.Arg1 = rmOpt(k.Arg1)
		k.Arg2 = rmOpt(k.Arg2)
		k.Arg3 = rmOpt(k.Arg3)
		return k
	case ExprAs:
		k.Expr = rm(k.Expr)
		return k
	case ExprArrayLength:
		k.Array = rm(k.Array)
		return k
	case ExprRayQueryGetIntersection:
		k.Query = rm(k.Query)
		return k
	case ExprDerivative:
		k.Expr = rm(k.Expr)
		return k
	default:
		return kind
	}
}

// remapSampleLevel remaps expression handles in SampleLevel variants.
func remapSampleLevel(level SampleLevel, rm func(ExpressionHandle) ExpressionHandle) SampleLevel {
	if level == nil {
		return nil
	}
	switch l := level.(type) {
	case SampleLevelExact:
		l.Level = rm(l.Level)
		return l
	case SampleLevelBias:
		l.Bias = rm(l.Bias)
		return l
	case SampleLevelGradient:
		l.X = rm(l.X)
		l.Y = rm(l.Y)
		return l
	default:
		return level
	}
}

// remapStmtExprHandles remaps expression handles in all statements.
func remapStmtExprHandles(stmts []Statement, remap []ExpressionHandle) {
	rm := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(remap) {
			return remap[h]
		}
		return h
	}
	rmOpt := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		v := rm(*h)
		return &v
	}
	for i, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case StmtBlock:
			remapStmtExprHandles(s.Block, remap)
			stmts[i].Kind = s
		case StmtIf:
			s.Condition = rm(s.Condition)
			remapStmtExprHandles(s.Accept, remap)
			remapStmtExprHandles(s.Reject, remap)
			stmts[i].Kind = s
		case StmtSwitch:
			s.Selector = rm(s.Selector)
			for ci := range s.Cases {
				remapStmtExprHandles(s.Cases[ci].Body, remap)
			}
			stmts[i].Kind = s
		case StmtLoop:
			remapStmtExprHandles(s.Body, remap)
			remapStmtExprHandles(s.Continuing, remap)
			s.BreakIf = rmOpt(s.BreakIf)
			stmts[i].Kind = s
		case StmtReturn:
			s.Value = rmOpt(s.Value)
			stmts[i].Kind = s
		case StmtStore:
			s.Pointer = rm(s.Pointer)
			s.Value = rm(s.Value)
			stmts[i].Kind = s
		case StmtImageStore:
			s.Image = rm(s.Image)
			s.Coordinate = rm(s.Coordinate)
			s.ArrayIndex = rmOpt(s.ArrayIndex)
			s.Value = rm(s.Value)
			stmts[i].Kind = s
		case StmtCall:
			for ai := range s.Arguments {
				s.Arguments[ai] = rm(s.Arguments[ai])
			}
			s.Result = rmOpt(s.Result)
			stmts[i].Kind = s
		case StmtAtomic:
			s.Pointer = rm(s.Pointer)
			s.Fun = remapAtomicFunction(s.Fun, rmOpt)
			s.Value = rm(s.Value)
			s.Result = rmOpt(s.Result)
			stmts[i].Kind = s
		case StmtWorkGroupUniformLoad:
			s.Pointer = rm(s.Pointer)
			s.Result = rm(s.Result)
			stmts[i].Kind = s
		case StmtRayQuery:
			s.Query = rm(s.Query)
			s.Fun = remapRayQueryFunction(s.Fun, rm)
			stmts[i].Kind = s
		case StmtSubgroupBallot:
			s.Predicate = rmOpt(s.Predicate)
			s.Result = rm(s.Result)
			stmts[i].Kind = s
		case StmtSubgroupGather:
			s.Mode = remapGatherMode(s.Mode, rm)
			s.Argument = rm(s.Argument)
			s.Result = rm(s.Result)
			stmts[i].Kind = s
		case StmtEmit:
			s.Range.Start = rm(s.Range.Start)
			s.Range.End = rm(s.Range.End)
			stmts[i].Kind = s
		case StmtImageAtomic:
			s.Image = rm(s.Image)
			s.Coordinate = rm(s.Coordinate)
			s.ArrayIndex = rmOpt(s.ArrayIndex)
			s.Value = rm(s.Value)
			stmts[i].Kind = s
		case StmtSubgroupCollectiveOperation:
			s.Argument = rm(s.Argument)
			s.Result = rm(s.Result)
			stmts[i].Kind = s
		}
	}
}

// remapExprTypeHandles updates type handles within expression kinds.
func remapExprTypeHandles(kind ExpressionKind, remap []TypeHandle) ExpressionKind {
	safe := func(h TypeHandle) TypeHandle {
		if h == ^TypeHandle(0) || int(h) >= len(remap) {
			return h
		}
		return remap[h]
	}
	switch k := kind.(type) {
	case ExprZeroValue:
		k.Type = safe(k.Type)
		return k
	case ExprCompose:
		k.Type = safe(k.Type)
		return k
	case ExprSubgroupOperationResult:
		k.Type = safe(k.Type)
		return k
	case ExprAtomicResult:
		k.Ty = safe(k.Ty)
		return k
	default:
		return kind
	}
}

// DeduplicateEmits removes duplicate and redundant Emit statements from all functions.
// An Emit is redundant if its range is already covered by a previous Emit in the same block.
// This handles cases where the emitter flush in function calls generates duplicate ranges.
func DeduplicateEmits(module *Module) {
	for i := range module.Functions {
		module.Functions[i].Body = deduplicateBlockEmits(module.Functions[i].Body, module.Functions[i].Expressions)
	}
	for i := range module.EntryPoints {
		module.EntryPoints[i].Function.Body = deduplicateBlockEmits(module.EntryPoints[i].Function.Body, module.EntryPoints[i].Function.Expressions)
	}
}

// deduplicateBlockEmits removes duplicate, empty, covered, and pre-emit-only Emit statements.
func deduplicateBlockEmits(stmts []Statement, exprs []Expression) []Statement {
	if len(stmts) == 0 {
		return stmts
	}
	// First pass: remove empty, covered, and pre-emit-only Emits.
	emitted := make(map[ExpressionHandle]bool)
	filtered := make([]Statement, 0, len(stmts))
	for _, s := range stmts {
		if emit, ok := s.Kind.(StmtEmit); ok {
			if emit.Range.Start == emit.Range.End {
				continue
			}
			// Check if ALL handles in this range are already emitted
			allCovered := true
			for h := emit.Range.Start; h < emit.Range.End; h++ {
				if !emitted[h] {
					allCovered = false
					break
				}
			}
			if allCovered {
				continue
			}
			// Check if ALL expressions in range are pre-emit (don't need Emit).
			// Matches Rust naga's needs_pre_emit(): Literal, Constant, ZeroValue,
			// GlobalVariable, FunctionArgument, LocalVariable, Override.
			allPreEmit := true
			for h := emit.Range.Start; h < emit.Range.End; h++ {
				if int(h) < len(exprs) && !isPreEmitExpression(exprs[h].Kind) {
					allPreEmit = false
					break
				}
			}
			if allPreEmit {
				continue // Skip: all expressions are pre-emit, no Emit needed
			}
			for h := emit.Range.Start; h < emit.Range.End; h++ {
				emitted[h] = true
			}
		}
		filtered = append(filtered, s)
	}

	// Second pass: recurse into nested blocks
	result := make([]Statement, 0, len(filtered))
	for _, s := range filtered {
		switch k := s.Kind.(type) {
		case StmtBlock:
			k.Block = Block(deduplicateBlockEmits([]Statement(k.Block), exprs))
			s.Kind = k
		case StmtIf:
			k.Accept = Block(deduplicateBlockEmits([]Statement(k.Accept), exprs))
			k.Reject = Block(deduplicateBlockEmits([]Statement(k.Reject), exprs))
			s.Kind = k
		case StmtSwitch:
			for ci := range k.Cases {
				k.Cases[ci].Body = Block(deduplicateBlockEmits([]Statement(k.Cases[ci].Body), exprs))
			}
			s.Kind = k
		case StmtLoop:
			k.Body = Block(deduplicateBlockEmits([]Statement(k.Body), exprs))
			k.Continuing = Block(deduplicateBlockEmits([]Statement(k.Continuing), exprs))
			s.Kind = k
		}
		result = append(result, s)
	}
	return result
}

// isPreEmitExpression returns true for expression types that don't need Emit statements.
// Matches Rust naga's Expression::needs_pre_emit().
func isPreEmitExpression(kind ExpressionKind) bool {
	switch kind.(type) {
	case Literal, ExprConstant, ExprOverride, ExprZeroValue,
		ExprGlobalVariable, ExprFunctionArgument, ExprLocalVariable:
		return true
	default:
		return false
	}
}
