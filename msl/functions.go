package msl

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/ir"
)

// blockEndsWithReturn checks if a block's control flow always ends with a return.
// This recursively checks the last statement: If/Switch/Block are handled by
// checking their sub-blocks, matching Rust naga's ensure_block_returns logic.
func blockEndsWithReturn(block ir.Block) bool {
	if len(block) == 0 {
		return false
	}
	last := block[len(block)-1]
	switch s := last.Kind.(type) {
	case ir.StmtReturn, ir.StmtKill:
		return true
	case ir.StmtBlock:
		return blockEndsWithReturn(ir.Block(s.Block))
	case ir.StmtIf:
		return blockEndsWithReturn(ir.Block(s.Accept)) && blockEndsWithReturn(ir.Block(s.Reject))
	case ir.StmtSwitch:
		for _, c := range s.Cases {
			if !c.FallThrough && !blockEndsWithReturn(c.Body) {
				return false
			}
		}
		return len(s.Cases) > 0
	default:
		return false
	}
}

// MSL attribute constants
const (
	attrPosition            = "[[position]]"
	spaceConstant           = "constant"
	spaceDevice             = "device"
	interpCenterPerspective = "center_perspective"
)

// collectOobLocalTypes scans Call statements for pointer arguments that need
// RZSW bounds checks and collects the types needed for out-of-bounds locals.
// Matches Rust naga's oob_local_types function in proc/index.rs.
func (w *Writer) collectOobLocalTypes(fn *ir.Function) {
	w.oobLocals = make(map[ir.TypeHandle]string)

	if !w.options.BoundsCheckPolicies.Contains(BoundsCheckReadZeroSkipWrite) {
		return
	}

	// Walk top-level Call statements looking for pointer arguments with bounds checks
	for _, stmt := range fn.Body {
		call, ok := stmt.Kind.(ir.StmtCall)
		if !ok {
			continue
		}
		if int(call.Function) >= len(w.module.Functions) {
			continue
		}
		callee := &w.module.Functions[call.Function]
		for argIdx, argExpr := range call.Arguments {
			if argIdx >= len(callee.Arguments) {
				break
			}
			argInfo := &callee.Arguments[argIdx]
			if int(argInfo.Type) >= len(w.module.Types) {
				continue
			}
			argTypeInner := w.module.Types[argInfo.Type].Inner
			pt, isPtr := argTypeInner.(ir.PointerType)
			if !isPtr {
				continue
			}
			// Check if this argument expression has any bounds checks needed
			if w.exprHasBoundsCheck(argExpr, fn) {
				w.oobLocals[pt.Base] = "" // placeholder, name assigned below
			}
		}
	}

	// Assign unique names in sorted order for deterministic output.
	// Rust naga uses FxHashSet which has hash-dependent iteration order;
	// sorting by type handle gives us consistent results regardless of
	// expression renumbering from const-folding.
	oobHandles := make([]ir.TypeHandle, 0, len(w.oobLocals))
	for tyHandle := range w.oobLocals {
		oobHandles = append(oobHandles, tyHandle)
	}
	sort.Slice(oobHandles, func(i, j int) bool {
		return oobHandles[i] < oobHandles[j]
	})
	for _, tyHandle := range oobHandles {
		w.oobLocals[tyHandle] = w.namer.call("oob")
	}
}

// exprHasBoundsCheck checks if an expression (or any part of its access chain)
// has bounds checks. Matches Rust naga's bounds_check_iter having any items.
func (w *Writer) exprHasBoundsCheck(handle ir.ExpressionHandle, fn *ir.Function) bool {
	if int(handle) >= len(fn.Expressions) {
		return false
	}
	expr := fn.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprAccess:
		return true // dynamic access always needs check
	case ir.ExprAccessIndex:
		// Static index on a runtime-sized array needs check too,
		// but more importantly, check if there's a deeper access
		return w.exprHasBoundsCheck(k.Base, fn)
	default:
		return false
	}
}

// writeLocalVars writes local variable declarations for the current function.
// The lowerer sets LocalVariable.Init for const expressions outside loops,
// and leaves Init=nil for runtime expressions (which use zero-init + Store).
// Local names are looked up from pre-registered names (registerNames), matching
// Rust naga's namer.reset() pre-population.
func (w *Writer) writeLocalVars(fn *ir.Function) error {
	for i, local := range fn.LocalVars {
		// Use pre-registered name from registerNames().
		localName := w.getName(nameKey{kind: nameKeyLocal, handle1: uint32(w.currentFuncHandle), handle2: uint32(i)})
		w.localNames[uint32(i)] = localName

		localType := w.writeTypeName(local.Type, StorageAccess(0))
		w.writeIndent()
		w.write("%s %s", localType, localName)

		if local.Init != nil {
			w.write(" = ")
			if err := w.writeExpression(*local.Init); err != nil {
				return err
			}
		} else {
			w.write(" = {}")
		}
		w.write(";\n")
	}

	// Emit oob locals for RZSW pointer bounds checks.
	// These are zero-initialized locals used as pointer targets when access is out of bounds.
	// Sorted by name for deterministic output.
	if len(w.oobLocals) > 0 {
		type oobEntry struct {
			tyHandle ir.TypeHandle
			name     string
		}
		entries := make([]oobEntry, 0, len(w.oobLocals))
		for tyHandle, name := range w.oobLocals {
			entries = append(entries, oobEntry{tyHandle, name})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].name < entries[j].name
		})
		for _, e := range entries {
			typeName := w.writeTypeName(e.tyHandle, StorageAccess(0))
			w.writeIndent()
			w.write("%s %s = {};\n", typeName, e.name)
		}
	}
	return nil
}

// needsPassThrough returns true if global variables in this address space
// must be passed as function arguments in MSL. MSL doesn't have true globals
// for resources — they must be passed through from entry points.
func needsPassThrough(space ir.AddressSpace) bool {
	switch space {
	case ir.SpaceUniform, ir.SpaceStorage, ir.SpaceHandle, ir.SpacePrivate, ir.SpaceWorkGroup, ir.SpaceImmediate:
		return true
	default:
		return false
	}
}

// analyzeFuncPassThroughGlobals scans each function to determine which global
// variables it references. These globals must be added as extra parameters since
// MSL helper functions cannot access entry point bindings.
// Uses memoization to handle transitive calls (A calls B which uses globals).
func (w *Writer) analyzeFuncPassThroughGlobals() {
	// Analyze regular functions first.
	for handle := range w.module.Functions {
		w.getPassThroughGlobals(ir.FunctionHandle(handle))
	}
	// Analyze entry point functions (stored inline, keyed by synthetic handle).
	for epIdx := range w.module.EntryPoints {
		w.getEntryPointPassThroughGlobals(epIdx)
	}
}

// getEntryPointPassThroughGlobals computes pass-through globals for an entry point's inline function.
func (w *Writer) getEntryPointPassThroughGlobals(epIdx int) []uint32 {
	handle := epFuncHandle(epIdx)
	if cached, ok := w.funcPassThroughGlobals[handle]; ok {
		return cached
	}

	w.funcPassThroughGlobals[handle] = []uint32{}

	fn := &w.module.EntryPoints[epIdx].Function
	used := make(map[uint32]struct{})

	for _, expr := range fn.Expressions {
		if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
			h := uint32(gv.Variable)
			if int(h) < len(w.module.GlobalVariables) {
				if needsPassThrough(w.module.GlobalVariables[h].Space) {
					used[h] = struct{}{}
				}
			}
		}
	}

	// Transitive: globals used by called functions
	w.walkStmts(fn.Body, func(call ir.StmtCall) {
		if int(call.Function) < len(w.module.Functions) {
			for _, h := range w.getPassThroughGlobals(call.Function) {
				used[h] = struct{}{}
			}
		}
	})

	var result []uint32
	for i := range w.module.GlobalVariables {
		h := uint32(i)
		if _, ok := used[h]; ok {
			result = append(result, h)
		}
	}

	w.funcPassThroughGlobals[handle] = result
	return result
}

// getPassThroughGlobals returns (and caches) the list of global variable handles
// that a function needs as pass-through parameters.
func (w *Writer) getPassThroughGlobals(handle ir.FunctionHandle) []uint32 {
	if cached, ok := w.funcPassThroughGlobals[handle]; ok {
		return cached
	}

	// Mark as visited (empty slice) to prevent infinite recursion
	w.funcPassThroughGlobals[handle] = []uint32{}

	fn := &w.module.Functions[handle]
	used := make(map[uint32]struct{})

	// Collect all globals used directly in expressions
	for _, expr := range fn.Expressions {
		if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
			h := uint32(gv.Variable)
			if int(h) < len(w.module.GlobalVariables) {
				if needsPassThrough(w.module.GlobalVariables[h].Space) {
					used[h] = struct{}{}
				}
			}
		}
	}

	// Transitive: globals used by called functions
	w.walkStmts(fn.Body, func(call ir.StmtCall) {
		if int(call.Function) < len(w.module.Functions) {
			for _, h := range w.getPassThroughGlobals(call.Function) {
				used[h] = struct{}{}
			}
		}
	})

	// Collect in global variable declaration order (matching Rust naga which
	// iterates module.global_variables in order and checks fun_info[handle]).
	var result []uint32
	for i := range w.module.GlobalVariables {
		h := uint32(i)
		if _, ok := used[h]; ok {
			result = append(result, h)
		}
	}

	w.funcPassThroughGlobals[handle] = result
	return result
}

// walkStmts walks all statements (including nested blocks) and calls
// the visitor for each StmtCall found.
func (w *Writer) walkStmts(stmts ir.Block, visitCall func(ir.StmtCall)) {
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case ir.StmtCall:
			visitCall(s)
		case ir.StmtBlock:
			w.walkStmts(s.Block, visitCall)
		case ir.StmtIf:
			w.walkStmts(s.Accept, visitCall)
			w.walkStmts(s.Reject, visitCall)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				w.walkStmts(c.Body, visitCall)
			}
		case ir.StmtLoop:
			w.walkStmts(s.Body, visitCall)
			w.walkStmts(s.Continuing, visitCall)
		}
	}
}

// writeFunctions writes all non-entry-point function definitions.
func (w *Writer) writeFunctions() error {
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		// Skip functions that are entry points (handled separately)
		if w.isEntryPointFunction(ir.FunctionHandle(handle)) {
			continue
		}

		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
	}
	return nil
}

// isEntryPointFunction checks if a function handle refers to an entry point.
// Since entry point functions are stored inline in EntryPoint.Function (not in
// Module.Functions[]), this always returns false — all handles in Functions[]
// are regular functions.
func (w *Writer) isEntryPointFunction(_ ir.FunctionHandle) bool {
	return false
}

// writeFunction writes a regular function definition.
func (w *Writer) writeFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	// Blank line before each function (matches Rust naga: writeln!(self.out)?)
	w.writeLine("")

	// Set context
	w.currentFunction = fn
	w.currentFuncHandle = handle
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.guardedIndices = make(map[ir.ExpressionHandle]struct{})
	w.unnamedCount = 0

	// Pre-scan function body to mark expressions that need baking.
	// Matches Rust naga's collect_needs_bake_expressions pass.
	w.collectNeedBakeExpressions(fn)
	// Find RZSW guarded indices that need baking.
	// Matches Rust naga's find_checked_indexes pass.
	w.findGuardedIndices(fn)
	// Collect oob local types for RZSW pointer bounds checks.
	w.collectOobLocalTypes(fn)

	defer func() {
		w.currentFunction = nil
		w.localNames = nil
		w.namedExpressions = nil
		w.needBakeExpression = nil
		w.guardedIndices = nil
		w.oobLocals = nil
	}()

	// Function name
	funcName := w.getName(nameKey{kind: nameKeyFunction, handle1: uint32(handle)})

	// Return type
	returnType := "void"
	if fn.Result != nil {
		returnType = w.writeTypeName(fn.Result.Type, StorageAccess(0))
	}

	// Function signature — Rust naga format: each param on own line, 4-space indent
	w.write("%s %s(", returnType, funcName)

	// Parameters
	paramCount := 0
	for i, arg := range fn.Arguments {
		if paramCount > 0 {
			w.write(",\n")
		} else {
			w.write("\n")
		}
		argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(i)})
		argType := w.writeTypeName(arg.Type, StorageAccess(0))
		w.write("    %s %s", argType, argName)
		paramCount++
	}

	// Pass-through global resources (textures, samplers, buffers)
	if globals, ok := w.funcPassThroughGlobals[handle]; ok {
		for _, gHandle := range globals {
			if paramCount > 0 {
				w.write(",\n")
			} else {
				w.write("\n")
			}
			w.write("    ")
			w.writePassThroughParam(gHandle)
			paramCount++
		}
	}

	// _buffer_sizes pass-through for helper functions that access runtime-sized arrays.
	if w.funcNeedsBufferSizes(handle) {
		if paramCount > 0 {
			w.write(",\n")
		} else {
			w.write("\n")
		}
		w.write("    constant _mslBufferSizes& _buffer_sizes")
	}

	w.write("\n) {\n")
	w.pushIndent()

	// Local variables
	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// writeEntryPoints writes all entry point functions.
func (w *Writer) writeEntryPoints() error {
	for epIdx, ep := range w.module.EntryPoints {
		// Check if we should skip this entry point
		if w.pipeline.EntryPoint != nil {
			if w.pipeline.EntryPoint.Name != ep.Name || w.pipeline.EntryPoint.Stage != ep.Stage {
				continue
			}
		}

		if err := w.writeEntryPoint(epIdx, &ep); err != nil {
			return err
		}
	}
	return nil
}

// writeEntryPoint writes a single entry point function.
func (w *Writer) writeEntryPoint(epIdx int, ep *ir.EntryPoint) error {
	// Blank line before each entry point (matches Rust naga: writeln!(self.out)?)
	w.writeLine("")

	// Entry point function is stored inline in ep.Function (not in Module.Functions[]).
	fn := &ep.Function

	// Set context
	w.currentFunction = fn
	w.currentFuncHandle = epFuncHandle(epIdx)
	w.currentEPIndex = epIdx
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.guardedIndices = make(map[ir.ExpressionHandle]struct{})
	w.unnamedCount = 0
	w.entryPointOutputVar = ""
	w.entryPointOutputTypeActive = false
	w.entryPointInputStructArg = -1
	w.entryPointStage = ep.Stage
	w.flattenedMemberNames = make(map[nameKey]string)
	w.hasVaryings = false

	// Pre-scan function body to mark expressions that need baking.
	w.collectNeedBakeExpressions(fn)
	// Find RZSW guarded indices that need baking.
	w.findGuardedIndices(fn)
	// Collect oob local types for RZSW pointer bounds checks.
	w.collectOobLocalTypes(fn)

	defer func() {
		w.currentFunction = nil
		w.currentEPIndex = -1
		w.localNames = nil
		w.namedExpressions = nil
		w.needBakeExpression = nil
		w.guardedIndices = nil
		w.oobLocals = nil
		w.entryPointOutputVar = ""
		w.entryPointOutputTypeActive = false
		w.entryPointInputStructArg = -1
		w.flattenedMemberNames = nil
		w.hasVaryings = false
	}()

	// Determine if this entry point should do vertex pulling.
	doVPT := w.doVertexPulling(ep)

	// Write input/output structs if needed.
	// Namer calls must match Rust naga's exact order (writer.rs ~6815-6867):
	//   1. namer.call("{fun_name}Input")   — inside writeEntryPointInputStruct
	//   2. namer.call("varyings")          — always, before output struct
	//   3. namer.call("{fun_name}Output")  — inside writeEntryPointOutputStruct
	//   4. namer.call("member")            — always, inside writeEntryPointOutputStruct
	var inputStructName string
	var hasInputStruct bool
	var vptAMResolved map[uint32]vptAttributeResolved

	if doVPT {
		// VPT mode: don't emit input struct, instead build attribute mapping.
		vptAMResolved = w.writeVPTEntryPointInputStruct(epIdx, ep, fn)
		hasInputStruct = true // we still have flattened member names for reconstruction
	} else {
		inputStructName, hasInputStruct = w.writeEntryPointInputStruct(epIdx, ep, fn)
	}

	// Register "varyings" name BEFORE the output struct, matching Rust naga order.
	varyingsName := w.namer.call("varyings")

	outputStructName, hasOutputStruct := w.writeEntryPointOutputStruct(epIdx, ep, fn)

	// If doing VPT, emit buffer type structs after the output struct.
	if doVPT {
		w.writeVPTBufferTypeStructs()
	}

	// Entry point name
	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})

	// Stage keyword
	var stageKeyword string
	switch ep.Stage {
	case ir.StageVertex:
		stageKeyword = "vertex"
	case ir.StageFragment:
		stageKeyword = "fragment"
	case ir.StageCompute:
		stageKeyword = "kernel"
	}

	// Return type
	resolveReturnSignature := func() (string, string) {
		if hasOutputStruct {
			w.entryPointOutputVar = "_output"
			w.entryPointOutputType = fn.Result.Type
			w.entryPointOutputTypeActive = true
			w.entryPointOutputStructName = outputStructName
			return outputStructName, ""
		}
		if fn.Result == nil {
			return "void", ""
		}
		returnType := w.writeTypeName(fn.Result.Type, StorageAccess(0))
		if fn.Result.Binding == nil {
			return returnType, ""
		}
		if _, ok := (*fn.Result.Binding).(ir.BuiltinBinding); !ok {
			return returnType, ""
		}
		return returnType, w.writeBindingAttribute(*fn.Result.Binding)
	}
	returnType, returnAttr := resolveReturnSignature()

	// Function signature — Rust naga format:
	// First param: "\n  param", subsequent: "\n, param"
	w.write("%s %s %s(", stageKeyword, returnType, epName)

	// Collect all parameters, then format them
	paramCount := 0

	// Check if we need workgroup zero-initialization for this entry point.
	// This requires: compute shader + ZeroInitializeWorkgroupMemory + workgroup vars.
	needWorkgroupInit := false
	localInvocationIDName := ""
	if w.options.ZeroInitializeWorkgroupMemory && ep.Stage == ir.StageCompute {
		for _, global := range w.module.GlobalVariables {
			if global.Space == ir.SpaceWorkGroup {
				needWorkgroupInit = true
				break
			}
		}
	}
	// Stage input struct — only emit if there are actual varyings (location-bound members).
	// Rust naga uses has_varyings to decide whether to emit the stage_in parameter.
	if w.hasVaryings {
		w.writeEntryPointParam(paramCount, fmt.Sprintf("%s %s [[stage_in]]", inputStructName, varyingsName))
		paramCount++
	}

	// Built-in inputs — both direct arguments and struct members.
	// Rust naga flattens struct arguments and emits builtin members as separate params.
	// Track if we find a local_invocation_id builtin to reuse for workgroup init.
	// For VPT: also track existing vertex_id and instance_id.
	existingLocalInvocationID := ""
	vptExistingVertexID := ""
	vptExistingInstanceID := ""
	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			// Direct argument with builtin binding
			if builtin, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				attr := builtinInputAttribute(builtin.Builtin, ep.Stage)
				if attr != "" {
					w.requireBuiltinVersion(builtin.Builtin)
					argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)})
					argType := w.writeTypeName(arg.Type, StorageAccess(0))
					w.writeEntryPointParam(paramCount, fmt.Sprintf("%s %s %s", argType, argName, attr))
					paramCount++
					// Track local_invocation_id for workgroup init reuse
					if builtin.Builtin == ir.BuiltinLocalInvocationID {
						existingLocalInvocationID = argName
					}
					if builtin.Builtin == ir.BuiltinVertexIndex {
						vptExistingVertexID = argName
					}
					if builtin.Builtin == ir.BuiltinInstanceIndex {
						vptExistingInstanceID = argName
					}
				}
			}
			continue
		}
		// Struct argument — emit builtin members as separate params
		if int(arg.Type) >= len(w.module.Types) {
			continue
		}
		typeInfo := &w.module.Types[arg.Type]
		st, ok := typeInfo.Inner.(ir.StructType)
		if !ok {
			continue
		}
		for memberIdx, member := range st.Members {
			if member.Binding == nil {
				continue
			}
			builtin, ok := (*member.Binding).(ir.BuiltinBinding)
			if !ok {
				continue
			}
			attr := builtinInputAttribute(builtin.Builtin, ep.Stage)
			if attr == "" {
				continue
			}
			w.requireBuiltinVersion(builtin.Builtin)
			key := nameKey{kind: nameKeyStructMember, handle1: uint32(arg.Type), handle2: uint32(memberIdx)}
			memberName := w.flattenedMemberNames[key]
			memberType := w.writeTypeName(member.Type, StorageAccess(0))
			w.writeEntryPointParam(paramCount, fmt.Sprintf("%s %s %s", memberType, memberName, attr))
			paramCount++
			if builtin.Builtin == ir.BuiltinLocalInvocationID {
				existingLocalInvocationID = memberName
			}
			if builtin.Builtin == ir.BuiltinVertexIndex {
				vptExistingVertexID = memberName
			}
			if builtin.Builtin == ir.BuiltinInstanceIndex {
				vptExistingInstanceID = memberName
			}
		}
	}

	// Workgroup zero-init parameter: __local_invocation_id.
	// Emitted AFTER builtin parameters but BEFORE resource bindings.
	// Matches Rust naga: only emit if no existing local_invocation_id argument.
	if needWorkgroupInit {
		if existingLocalInvocationID != "" {
			// Reuse the existing local_invocation_id argument
			localInvocationIDName = existingLocalInvocationID
		} else {
			// Add a synthetic parameter
			localInvocationIDName = "__local_invocation_id"
			w.writeEntryPointParam(paramCount, fmt.Sprintf("%suint3 %s [[thread_position_in_threadgroup]]",
				Namespace, localInvocationIDName))
			paramCount++
		}
	}

	// Compute Metal binding indices for this entry point.
	// This assigns sequential per-type indices across all bind groups,
	// preventing collisions when multiple groups share binding numbers.
	w.computeResourceMap(ep.Name)

	// Build set of globals actually referenced by this entry point (direct + transitive).
	// Rust naga only emits resources that the entry point actually uses, not ALL globals.
	epUsedGlobals := make(map[uint32]struct{})
	if globals, ok := w.funcPassThroughGlobals[epFuncHandle(epIdx)]; ok {
		for _, h := range globals {
			epUsedGlobals[h] = struct{}{}
		}
	}

	// Global variable parameters — emitted in declaration order (matching Rust naga).
	// This includes both resource bindings (device/constant with [[buffer]]/[[texture]])
	// and workgroup variables (threadgroup without binding attributes).
	// Rust naga iterates module.global_variables in order, emitting all that need
	// pass-through. The order follows the WGSL source declaration order.
	for i, global := range w.module.GlobalVariables {
		if _, used := epUsedGlobals[uint32(i)]; !used {
			continue
		}
		if global.Binding != nil {
			// Check for external texture — needs special multi-parameter emission.
			if w.isExternalTextureGlobal(uint32(i)) {
				w.writeExternalTextureEntryPointParams(uint32(i), &global, ep.Name, &paramCount)
				continue
			}
			// Resource binding (storage, uniform, texture, sampler)
			paramStr := w.formatGlobalResourceParam(uint32(i), &global)
			if paramStr == "" {
				// Inline sampler — skip from parameters, emitted in function body
				continue
			}
			w.writeEntryPointParam(paramCount, paramStr)
			paramCount++
		} else if global.Space == ir.SpaceWorkGroup {
			// Workgroup variable — threadgroup parameter without binding attribute
			name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
			typeName := w.writeTypeName(global.Type, StorageAccess(0))
			w.writeEntryPointParam(paramCount, fmt.Sprintf("threadgroup %s& %s", typeName, name))
			paramCount++
		} else if global.Space == ir.SpaceImmediate {
			// Immediate data variable — constant buffer parameter.
			// Resolve binding slot from per-entry-point ImmediatesBuffer config.
			name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
			typeName := w.writeTypeName(global.Type, StorageAccess(0))
			attr := w.resolveImmediatesBufferBinding(ep.Name)
			w.writeEntryPointParam(paramCount, fmt.Sprintf("constant %s& %s %s", typeName, name, attr))
			paramCount++
		}
	}

	// VPT: emit vertex_id, instance_id, buffer pointers AFTER resource bindings.
	if doVPT {
		w.writeVPTFunctionParams(ep, fn, &paramCount, vptExistingVertexID, vptExistingInstanceID)
	}

	// _mslBufferSizes parameter for runtime-sized arrays (and VPT).
	// Rust naga: needs_buffer_sizes = do_vertex_pulling || any global has runtime-sized array.
	epNeedsSizesBuffer := doVPT
	if !epNeedsSizesBuffer && w.needsSizesBuffer {
		for handle := range epUsedGlobals {
			if int(handle) < len(w.module.GlobalVariables) {
				global := &w.module.GlobalVariables[handle]
				if w.needsArrayLength(global.Type) {
					epNeedsSizesBuffer = true
					break
				}
			}
		}
	}
	if epNeedsSizesBuffer {
		bufferSizeAttr := w.resolveBufferSizesBinding(ep.Name)
		w.writeEntryPointParam(paramCount, fmt.Sprintf("constant _mslBufferSizes& _buffer_sizes %s", bufferSizeAttr))
		paramCount++
	}

	if returnAttr != "" {
		w.write("\n) %s {\n", returnAttr)
	} else {
		w.write("\n) {\n")
	}
	w.pushIndent()

	// VPT body prologue: emit zero-init + bounds check + unpacking BEFORE input aliases.
	// This must happen first because the input alias reconstruction uses the VPT locals.
	if doVPT {
		w.writeVPTBodyPrologue(vptAMResolved, vptExistingVertexID, vptExistingInstanceID)
	}

	// Reconstruct entry point arguments from flattened Metal parameters.
	// Matches Rust naga writer.rs ~line 7528: rebuild structs from stage_in + separate params.
	emitInputAliases := func() {
		if !hasInputStruct {
			return
		}
		for i, arg := range fn.Arguments {
			argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)})

			if arg.Binding != nil {
				// Direct argument with location binding — extract from varyings struct
				if _, ok := (*arg.Binding).(ir.LocationBinding); ok {
					if w.hasVaryings {
						w.writeLine("const auto %s = %s.%s;", argName, varyingsName, argName)
					}
				}
				continue
			}

			// Struct argument — reconstruct via aggregate initialization.
			// Location members come from varyings (or VPT locals), builtin members from separate params.
			if int(arg.Type) >= len(w.module.Types) {
				continue
			}
			typeInfo := &w.module.Types[arg.Type]
			st, ok := typeInfo.Inner.(ir.StructType)
			if !ok {
				continue
			}

			structName := w.getTypeName(arg.Type)
			w.writeIndent()
			w.write("const %s %s = { ", structName, argName)
			for memberIdx, member := range st.Members {
				if memberIdx != 0 {
					w.write(", ")
				}
				// Insert padding initialization if this member has padding before it.
				padKey := nameKey{kind: nameKeyStructMember, handle1: uint32(arg.Type), handle2: uint32(memberIdx)}
				if _, hasPad := w.structPads[padKey]; hasPad {
					w.write("{}, ")
				}
				key := nameKey{kind: nameKeyStructMember, handle1: uint32(arg.Type), handle2: uint32(memberIdx)}
				memberName := w.flattenedMemberNames[key]
				if !doVPT {
					// Normal mode: location members from varyings struct
					if member.Binding != nil {
						if _, isLoc := (*member.Binding).(ir.LocationBinding); isLoc {
							if w.hasVaryings {
								w.write("%s.", varyingsName)
							}
						}
					}
				}
				// In VPT mode: location members are local variables, use directly.
				// Builtin members — use the separate parameter name directly.
				w.write("%s", memberName)
			}
			w.write(" };\n")
		}
	}
	// Emit inline (constexpr) samplers in the function body.
	// Matches Rust naga writer.rs ~line 7483: inline samplers are declared
	// inside the entry point body, not as function parameters.
	w.writeInlineSamplers(ep.Name, epUsedGlobals)

	// Emit external texture wrapper constructions.
	// Matches Rust naga writer.rs ~line 7497: construct NagaExternalTextureWrapper
	// from the individual plane/params arguments for each external texture global.
	for i := range w.module.GlobalVariables {
		if _, used := epUsedGlobals[uint32(i)]; !used {
			continue
		}
		if w.isExternalTextureGlobal(uint32(i)) {
			w.writeExternalTextureWrapperConstruction(uint32(i))
		}
	}

	emitInputAliases()

	// Note: output struct variable is NOT pre-declared here.
	// For struct results, writeEntryPointOutputReturn uses the Rust naga pattern:
	//   const auto _tmp = <expr>;
	//   return OutputStruct { _tmp.field1, _tmp.field2 };
	// For simple results, inline aggregate initialization is used.
	_ = outputStructName

	// Workgroup zero-initialization prologue.
	// Matches Rust naga: zero all workgroup vars if __local_invocation_id == uint3(0).
	// Must come BEFORE local variables and private var locals.
	if needWorkgroupInit {
		if err := w.writeWorkgroupZeroInit(localInvocationIDName); err != nil {
			return err
		}
	}

	// Emit private global variables as local variables BEFORE function locals.
	// Metal doesn't support private mutable variables outside of functions,
	// so we declare them here in the entry point.
	// Matches Rust naga writer.rs order: private globals come before locals.
	if err := w.writePrivateVarLocals(fn); err != nil {
		return err
	}

	// Local variables — after private globals, matching Rust naga order.
	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("") // trailing blank line after entry point (matches Rust naga: writeln!(self.out, "}}")?)
	return nil
}

// writeEntryPointInputStruct writes the input struct for an entry point.
// Matches Rust naga behavior: always emits `struct <name>Input { };` when
// there are any arguments with bindings, even if the struct body is empty
// (e.g., compute shaders with only builtin arguments).
func (w *Writer) writeEntryPointInputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool) {
	// Check what kinds of inputs we have
	hasLocationInputs := false
	hasAnyBindingInputs := false
	for _, arg := range fn.Arguments {
		if arg.Binding == nil {
			continue
		}
		hasAnyBindingInputs = true
		if _, ok := (*arg.Binding).(ir.LocationBinding); ok {
			hasLocationInputs = true
		}
	}

	emitInputStruct := func(structName string, emitFields func()) {
		w.writeLine("struct %s {", structName)
		w.pushIndent()
		emitFields()
		w.popIndent()
		w.writeLine("};")
	}

	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})
	// Use namer.call to generate the input struct name, matching Rust naga:
	// self.namer.call(&format!("{fun_name}Input"))
	structName := w.namer.call(epName + "Input")

	if hasLocationInputs {
		emitInputStruct(structName, func() {
			for i, arg := range fn.Arguments {
				if arg.Binding == nil {
					continue
				}
				loc, ok := (*arg.Binding).(ir.LocationBinding)
				if !ok {
					continue
				}
				argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)})
				argType := w.writeTypeName(arg.Type, StorageAccess(0))

				attr := locationInputAttribute(loc, ep.Stage, w.typeScalarKind(arg.Type))
				w.writeLine("%s %s %s;", argType, argName, attr)
			}
		})

		w.hasVaryings = true
		return structName, true
	}

	// Collect ALL struct args without bindings (flattened struct inputs).
	// Rust naga flattens location members from ALL struct args into a single input struct.
	type structArgInfo struct {
		argIdx int
		st     ir.StructType
		tyH    ir.TypeHandle
	}
	var structArgs []structArgInfo
	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			continue
		}
		if int(arg.Type) >= len(w.module.Types) {
			continue
		}
		if st, ok := w.module.Types[arg.Type].Inner.(ir.StructType); ok {
			structArgs = append(structArgs, structArgInfo{argIdx: i, st: st, tyH: arg.Type})
		}
	}

	// Emit empty input struct for entry points with builtin-only arguments
	// (matching Rust naga behavior), but only if no struct arg will emit it.
	if hasAnyBindingInputs && len(structArgs) == 0 {
		emitInputStruct(structName, func() {})
	}

	if len(structArgs) > 0 {
		// Track first struct arg for reconstruction.
		w.entryPointInputStructArg = structArgs[0].argIdx

		// Register names for ALL struct args' members.
		// Location members get their own namespace (varyings_namer) to avoid
		// collisions with global names. Builtin members use the global namer.
		hasLocations := false
		varyingsNamer := newNamer()
		for _, sa := range structArgs {
			for memberIdx, member := range sa.st.Members {
				key := nameKey{kind: nameKeyStructMember, handle1: uint32(sa.tyH), handle2: uint32(memberIdx)}
				baseName := w.getName(key)

				isLocation := false
				if member.Binding != nil {
					if _, ok := (*member.Binding).(ir.LocationBinding); ok {
						isLocation = true
						hasLocations = true
					}
				}

				var freshName string
				if isLocation {
					freshName = varyingsNamer.call(baseName)
				} else {
					freshName = w.namer.call(baseName)
				}
				w.flattenedMemberNames[key] = freshName
			}
		}

		// Emit input struct with location-bound members from ALL struct args.
		emitInputStruct(structName, func() {
			for _, sa := range structArgs {
				for memberIdx, member := range sa.st.Members {
					if member.Binding == nil {
						continue
					}
					loc, ok := (*member.Binding).(ir.LocationBinding)
					if !ok {
						continue // skip builtins — they become separate params
					}
					key := nameKey{kind: nameKeyStructMember, handle1: uint32(sa.tyH), handle2: uint32(memberIdx)}
					memberName := w.flattenedMemberNames[key]
					memberType := w.writeTypeName(member.Type, StorageAccess(0))
					attr := locationInputAttribute(loc, ep.Stage, w.typeScalarKind(member.Type))
					w.writeLine("%s %s %s;", memberType, memberName, attr)
				}
			}
		})

		w.hasVaryings = hasLocations
		return structName, true
	}
	return "", false
}

// writeEntryPointOutputStruct writes the output struct for an entry point.
func (w *Writer) writeEntryPointOutputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool) {
	if fn.Result == nil {
		// Rust naga always calls namer.call("member") for every entry point,
		// even if the entry point has no result. This ensures the namer counter
		// advances consistently. Without this, subsequent entry points get
		// wrong member name suffixes.
		w.namer.call("member")
		return "", false
	}

	// Check if result type is a struct with bindings
	resultType := fn.Result.Type
	if int(resultType) >= len(w.module.Types) {
		// Still need to advance the "member" counter.
		w.namer.call("member")
		return "", false
	}

	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})
	// Use namer.call to generate the output struct name, matching Rust naga:
	// self.namer.call(&format!("{fun_name}Output"))
	structName := w.namer.call(epName + "Output")

	// Always register "member" name via namer.call, matching Rust naga (writer.rs:6867):
	//   let result_member_name = self.namer.call("member");
	// Rust calls this for EVERY entry point regardless of whether the result is a
	// struct or not. This ensures the namer counter advances consistently, so later
	// entry points that do use "member" get the correct suffix (e.g., member_1).
	resultMemberName := w.namer.call("member")

	typeInfo := &w.module.Types[resultType]
	st, ok := typeInfo.Inner.(ir.StructType)
	if !ok {
		// Simple return type - check if it has a binding that requires output struct
		// In MSL, [[position]] and [[color(N)]] must be on struct members
		if fn.Result.Binding != nil {
			returnType := w.writeTypeName(fn.Result.Type, StorageAccess(0))
			attr := w.writeBindingAttribute(*fn.Result.Binding)

			if attr != "" {
				w.writeLine("struct %s {", structName)
				w.pushIndent()
				w.writeLine("%s %s %s;", returnType, resultMemberName, attr)
				// Add _point_size for vertex shaders when forced
				if ep.Stage == ir.StageVertex && w.options.AllowAndForcePointSize {
					w.writeLine("float _point_size [[point_size]];")
				}
				w.popIndent()
				w.writeLine("};")

				return structName, true
			}
		}
		return "", false
	}

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	for memberIdx, member := range st.Members {
		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(resultType), handle2: uint32(memberIdx)})
		memberType := w.writeTypeName(member.Type, StorageAccess(0))

		var attr string
		if member.Binding != nil {
			attr = w.outputMemberAttribute(*member.Binding, ep.Stage)
		} else {
			// Fallback for members without explicit bindings
			switch ep.Stage {
			case ir.StageVertex:
				if memberIdx == 0 {
					attr = attrPosition
				} else {
					attr = fmt.Sprintf("[[user(loc%d)]]", memberIdx-1)
				}
			case ir.StageFragment:
				attr = fmt.Sprintf("[[color(%d)]]", memberIdx)
			}
		}

		w.writeLine("%s %s %s;", memberType, memberName, attr)
	}

	// Add _point_size member for vertex shaders when AllowAndForcePointSize is enabled.
	// Matches Rust naga: "float _point_size [[point_size]];"
	if ep.Stage == ir.StageVertex && w.options.AllowAndForcePointSize {
		// Check if any member already has PointSize binding
		hasPointSize := false
		for _, member := range st.Members {
			if member.Binding != nil {
				if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinPointSize {
					hasPointSize = true
					break
				}
			}
		}
		if !hasPointSize {
			w.writeLine("float _point_size [[point_size]];")
		}
	}

	w.popIndent()
	w.writeLine("};")

	return structName, true
}

// writeEntryPointParam writes a single entry point parameter in Rust naga format.
// First param (idx 0): "\n  param", subsequent params: "\n, param".
func (w *Writer) writeEntryPointParam(idx int, param string) {
	if idx == 0 {
		w.write("\n  %s", param)
	} else {
		w.write("\n, %s", param)
	}
}

// writePrivateVarLocals emits private global variables as local variables in the entry point.
// Metal doesn't support private mutable variables outside of functions, so they must be
// declared here. Matches Rust naga writer.rs ~line 7450.
// Iterates globals in declaration order (like Rust), only emitting those actually used.
func (w *Writer) writePrivateVarLocals(fn *ir.Function) error {
	// Build set of referenced private globals from this function's expressions.
	usedPrivate := make(map[uint32]struct{})
	for _, expr := range fn.Expressions {
		gv, ok := expr.Kind.(ir.ExprGlobalVariable)
		if !ok {
			continue
		}
		if int(gv.Variable) < len(w.module.GlobalVariables) {
			if w.module.GlobalVariables[gv.Variable].Space == ir.SpacePrivate {
				usedPrivate[uint32(gv.Variable)] = struct{}{}
			}
		}
	}
	// Also check transitively via called functions.
	if globals, ok := w.funcPassThroughGlobals[w.currentFuncHandle]; ok {
		for _, h := range globals {
			if int(h) < len(w.module.GlobalVariables) {
				if w.module.GlobalVariables[h].Space == ir.SpacePrivate {
					usedPrivate[h] = struct{}{}
				}
			}
		}
	}

	// Emit in declaration order.
	for i, global := range w.module.GlobalVariables {
		if global.Space != ir.SpacePrivate {
			continue
		}
		if _, used := usedPrivate[uint32(i)]; !used {
			continue
		}
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		typeName := w.writeTypeName(global.Type, StorageAccess(0))
		w.writeIndent()
		if global.InitExpr != nil {
			// Init from GlobalExpressions (preferred, matches Rust naga).
			w.write("%s %s = ", typeName, name)
			if err := w.writeGlobalExpression(*global.InitExpr); err != nil {
				return err
			}
			w.write(";\n")
		} else if global.Init != nil {
			// Init from Constants (legacy path).
			w.write("%s %s = ", typeName, name)
			if err := w.writeConstExpression(*global.Init); err != nil {
				return err
			}
			w.write(";\n")
		} else {
			w.write("%s %s = {};\n", typeName, name)
		}
	}
	return nil
}

// writeConstExpression writes a constant expression (global_expressions arena).
func (w *Writer) writeConstExpression(handle ir.ConstantHandle) error {
	// For private variable initializers, we need to emit the constant's value.
	if int(handle) < len(w.module.Constants) {
		c := &w.module.Constants[handle]
		// If Value is nil, use Init (GlobalExpressions handle) instead.
		if c.Value == nil {
			return w.writeGlobalExpression(c.Init)
		}
		return w.writeConstantValue(c.Value, c.Type)
	}
	w.write("{}")
	return nil
}

// writeGlobalExpression writes a global expression value (from Module.GlobalExpressions).
func (w *Writer) writeGlobalExpression(handle ir.ExpressionHandle) error {
	if int(handle) >= len(w.module.GlobalExpressions) {
		w.write("{}")
		return nil
	}
	expr := w.module.GlobalExpressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		if err := w.writeLiteral(k); err != nil {
			return err
		}
	case ir.ExprCompose:
		typeName := w.writeTypeName(k.Type, StorageAccess(0))

		// Handle 0-component Compose for vectors/matrices (from zero-arg constructors).
		// Expand to explicit zeros: metal::int2(0, 0). Matches Rust naga.
		if len(k.Components) == 0 && int(k.Type) < len(w.module.Types) {
			switch t := w.module.Types[k.Type].Inner.(type) {
			case ir.VectorType:
				w.write("%s(", typeName)
				for i := ir.VectorSize(0); i < t.Size; i++ {
					if i > 0 {
						w.write(", ")
					}
					w.writeScalarZeroLiteral(t.Scalar)
				}
				w.write(")")
				return nil
			case ir.MatrixType:
				w.write("%s(", typeName)
				for c := ir.VectorSize(0); c < t.Columns; c++ {
					if c > 0 {
						w.write(", ")
					}
					colTypeName := fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(t.Scalar), t.Rows)
					w.write("%s(", colTypeName)
					for r := ir.VectorSize(0); r < t.Rows; r++ {
						if r > 0 {
							w.write(", ")
						}
						w.writeScalarZeroLiteral(t.Scalar)
					}
					w.write(")")
				}
				w.write(")")
				return nil
			}
		}

		// Structs use braces {}, vectors/matrices use parens ()
		isStruct := false
		if int(k.Type) < len(w.module.Types) {
			_, isStruct = w.module.Types[k.Type].Inner.(ir.StructType)
			if !isStruct {
				if _, isArr := w.arrayWrappers[k.Type]; isArr {
					isStruct = true
				}
			}
		}
		if isStruct {
			w.write("%s {", typeName)
		} else {
			w.write("%s(", typeName)
		}
		for i, comp := range k.Components {
			if i > 0 {
				w.write(", ")
			}
			// Insert padding initializer for struct members with padding before them
			if isStruct {
				padKey := nameKey{kind: nameKeyStructMember, handle1: uint32(k.Type), handle2: uint32(i)}
				if _, hasPad := w.structPads[padKey]; hasPad {
					w.write("{}, ")
				}
			}
			if err := w.writeGlobalExpression(comp); err != nil {
				return err
			}
		}
		if isStruct {
			w.write("}")
		} else {
			w.write(")")
		}
	case ir.ExprZeroValue:
		// ZeroValue always renders as "{ty_name} {}" in MSL.
		// Matches Rust naga put_const_expression: write!(self.out, "{ty_name} {{}}")?;
		typeName := w.writeTypeName(k.Type, StorageAccess(0))
		w.write("%s {}", typeName)
	case ir.ExprConstant:
		// Constant reference in global expression: write the constant's name or value.
		if int(k.Constant) < len(w.module.Constants) {
			c := &w.module.Constants[k.Constant]
			if c.Name != "" {
				name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(k.Constant)})
				w.write("%s", name)
			} else {
				// Unnamed constant: inline its init expression.
				return w.writeGlobalExpression(c.Init)
			}
		}
	case ir.ExprOverride:
		// Override reference in global expression: write the override's name.
		name := w.getName(nameKey{kind: nameKeyOverride, handle1: uint32(k.Override)})
		w.write("%s", name)
	case ir.ExprSplat:
		// Splat in global expression: write as type(value)
		typeName := fmt.Sprintf("%s%s%d", Namespace, "float", k.Size)
		if int(k.Value) < len(w.module.GlobalExpressions) {
			valExpr := w.module.GlobalExpressions[k.Value]
			if lit, ok := valExpr.Kind.(ir.Literal); ok {
				scalarName := w.literalScalarTypeName(lit)
				if scalarName != "" {
					typeName = fmt.Sprintf("%s%s%d", Namespace, scalarName, k.Size)
				}
			}
		}
		w.write("%s(", typeName)
		if err := w.writeGlobalExpression(k.Value); err != nil {
			return err
		}
		w.write(")")
	default:
		w.write("{}")
	}
	return nil
}

// writeScalarZeroLiteral writes a zero literal for a scalar type.
func (w *Writer) writeScalarZeroLiteral(scalar ir.ScalarType) {
	switch scalar.Kind {
	case ir.ScalarFloat:
		w.write("0.0")
	case ir.ScalarUint:
		w.write("0u")
	case ir.ScalarBool:
		w.write("false")
	default:
		w.write("0")
	}
}

// literalScalarTypeName returns the MSL scalar type name for a literal type.
func (w *Writer) literalScalarTypeName(lit ir.Literal) string {
	switch lit.Value.(type) {
	case ir.LiteralI32:
		return "int"
	case ir.LiteralU32:
		return "uint"
	case ir.LiteralF32:
		return "float"
	case ir.LiteralBool:
		return "bool"
	default:
		return ""
	}
}

// formatGlobalResourceParam formats a global resource as an entry point parameter string.
// Unlike writeGlobalResourceParam, this returns the formatted string without writing it.
func (w *Writer) formatGlobalResourceParam(handle uint32, global *ir.GlobalVariable) string {
	name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: handle})

	if int(global.Type) >= len(w.module.Types) {
		return fmt.Sprintf("/* invalid type %d */ int %s", global.Type, name)
	}

	typeInfo := &w.module.Types[global.Type]

	// Look up the Metal binding index from the pre-computed resource map.
	var bt BindTarget
	hasMappedBinding := false
	if global.Binding != nil && w.currentResourceMap != nil {
		bt, hasMappedBinding = w.currentResourceMap[*global.Binding]
	}

	// When FakeMissingBindings is enabled and there's no mapping for this resource,
	// use [[user(fakeN)]] attributes to match Rust naga behavior.
	useFake := w.options.FakeMissingBindings && !hasMappedBinding

	switch inner := typeInfo.Inner.(type) {
	case ir.SamplerType:
		if useFake {
			return fmt.Sprintf("%ssampler %s [[user(fake0)]]", Namespace, name)
		}
		if bt.Sampler != nil {
			if bt.Sampler.IsInline {
				// Inline samplers are emitted as constexpr in the function body, not as parameters.
				return ""
			}
			return fmt.Sprintf("%ssampler %s [[sampler(%d)]]", Namespace, name, bt.Sampler.Slot)
		}
		if global.Binding != nil {
			return fmt.Sprintf("%ssampler %s [[sampler(%d)]]", Namespace, name, global.Binding.Binding)
		}
		return fmt.Sprintf("%ssampler %s [[sampler(0)]]", Namespace, name)

	case ir.ImageType:
		typeName := w.imageTypeName(inner, StorageAccess(0))
		if useFake {
			return fmt.Sprintf("%s %s [[user(fake0)]]", typeName, name)
		}
		idx := w.bindTargetIndex(bt.Texture, global.Binding)
		return fmt.Sprintf("%s %s [[texture(%d)]]", typeName, name, idx)

	default:
		// Buffer types
		space := addressSpaceName(global.Space)
		typeName := w.writeTypeName(global.Type, StorageAccess(0))

		// Determine const qualifier for read-only storage buffers
		constQual := ""
		if w.isStorageGlobalReadOnly(handle) {
			constQual = " const"
		}

		if useFake {
			if space == spaceConstant || space == spaceDevice {
				return fmt.Sprintf("%s %s%s& %s [[user(fake0)]]", space, typeName, constQual, name)
			}
			return fmt.Sprintf("%s %s [[user(fake0)]]", typeName, name)
		}

		idx := w.bindTargetIndex(bt.Buffer, global.Binding)
		if space == spaceConstant || space == spaceDevice {
			return fmt.Sprintf("%s %s%s& %s [[buffer(%d)]]", space, typeName, constQual, name, idx)
		}
		return fmt.Sprintf("%s %s [[buffer(%d)]]", typeName, name, idx)
	}
}

// writePassThroughParam writes a global variable as a pass-through parameter
// for a helper function. Unlike entry point params, these have no [[binding]] attributes.
func (w *Writer) writePassThroughParam(handle uint32) {
	global := &w.module.GlobalVariables[handle]
	name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: handle})
	typeInfo := &w.module.Types[global.Type]

	switch inner := typeInfo.Inner.(type) {
	case ir.SamplerType:
		w.write("%ssampler %s", Namespace, name)

	case ir.ImageType:
		typeName := w.imageTypeName(inner, StorageAccess(0))
		w.write("%s %s", typeName, name)

	default:
		// Buffer types (uniform, storage) and other address spaces (private, workgroup)
		space := addressSpaceName(global.Space)
		typeName := w.writeTypeName(global.Type, StorageAccess(0))

		if space == spaceConstant || space == spaceDevice {
			constQual := ""
			if w.isStorageGlobalReadOnly(handle) {
				constQual = " const"
			}
			w.write("%s %s%s& %s", space, typeName, constQual, name)
		} else if space != "" {
			// threadgroup, thread — pass by reference with address space qualifier
			w.write("%s %s& %s", space, typeName, name)
		} else {
			w.write("%s %s", typeName, name)
		}
	}
}

// computeResourceMap builds the Metal binding index map for the current entry point.
// If PerEntryPointMap has an explicit mapping for epName, it is used directly.
// Otherwise, sequential indices per resource type (buffer, texture, sampler) are
// assigned across all globals sorted by (group, binding), matching the approach
// used by Rust wgpu-hal's Metal device.
func (w *Writer) computeResourceMap(epName string) {
	// Check for explicit mapping first.
	if w.options.PerEntryPointMap != nil {
		if epRes, ok := w.options.PerEntryPointMap[epName]; ok {
			w.currentResourceMap = epRes.Resources
			return
		}
	}

	// When FakeMissingBindings is enabled and no explicit map, use empty map.
	// Resources will get [[user(fake0)]] attributes in formatGlobalResourceParam.
	if w.options.FakeMissingBindings {
		w.currentResourceMap = nil
		return
	}

	// Auto-compute: collect all globals with bindings, classify by resource type,
	// sort by (group, binding), assign sequential indices per type.
	type globalEntry struct {
		binding ir.ResourceBinding
		resKind int // 0=buffer, 1=texture, 2=sampler
	}

	var entries []globalEntry
	for _, global := range w.module.GlobalVariables {
		if global.Binding == nil {
			continue
		}
		if int(global.Type) >= len(w.module.Types) {
			continue
		}

		var kind int
		switch w.module.Types[global.Type].Inner.(type) {
		case ir.SamplerType:
			kind = 2
		case ir.ImageType:
			kind = 1
		default:
			kind = 0
		}

		entries = append(entries, globalEntry{
			binding: *global.Binding,
			resKind: kind,
		})
	}

	// Sort by (group, binding) for deterministic assignment.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].binding.Group != entries[j].binding.Group {
			return entries[i].binding.Group < entries[j].binding.Group
		}
		return entries[i].binding.Binding < entries[j].binding.Binding
	})

	// Assign sequential indices per resource type.
	resMap := make(map[ir.ResourceBinding]BindTarget, len(entries))
	var nextBuffer, nextTexture, nextSampler uint8
	for _, e := range entries {
		var bt BindTarget
		switch e.resKind {
		case 0: // buffer
			idx := nextBuffer
			bt.Buffer = &idx
			nextBuffer++
		case 1: // texture
			idx := nextTexture
			bt.Texture = &idx
			nextTexture++
		case 2: // sampler
			idx := nextSampler
			bt.Sampler = &BindSamplerTarget{IsInline: false, Slot: idx}
			nextSampler++
		}
		resMap[e.binding] = bt
	}

	w.currentResourceMap = resMap
}

// bindTargetIndex returns the Metal binding index from a BindTarget slot pointer.
// If the slot is non-nil (explicit or auto-computed mapping), its value is used.
// Otherwise falls back to the raw WGSL binding number (legacy behavior).
func (w *Writer) bindTargetIndex(slot *uint8, binding *ir.ResourceBinding) uint32 {
	if slot != nil {
		return uint32(*slot)
	}
	if binding != nil {
		return binding.Binding
	}
	return 0
}

// writeInlineSamplers emits constexpr sampler declarations for inline samplers
// in the current entry point. Matches Rust naga writer.rs ~line 7483.
func (w *Writer) writeInlineSamplers(epName string, epUsedGlobals map[uint32]struct{}) {
	if len(w.options.InlineSamplers) == 0 {
		return
	}
	for i, global := range w.module.GlobalVariables {
		if _, used := epUsedGlobals[uint32(i)]; !used {
			continue
		}
		if global.Binding == nil {
			continue
		}
		if _, ok := w.module.Types[global.Type].Inner.(ir.SamplerType); !ok {
			continue
		}
		// Check if this sampler has an inline binding
		if w.currentResourceMap == nil {
			continue
		}
		bt, ok := w.currentResourceMap[*global.Binding]
		if !ok || bt.Sampler == nil || !bt.Sampler.IsInline {
			continue
		}
		idx := int(bt.Sampler.Slot)
		if idx >= len(w.options.InlineSamplers) {
			continue
		}
		sampler := &w.options.InlineSamplers[idx]
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		w.writeLine("constexpr %ssampler %s(", Namespace, name)
		w.pushIndent()
		// Address modes
		letters := [3]byte{'s', 't', 'r'}
		addressStrs := [5]string{"repeat", "mirrored_repeat", "clamp_to_edge", "clamp_to_zero", "clamp_to_border"}
		for j, addr := range sampler.Address {
			w.writeLine("%s%c_address::%s,", Namespace, letters[j], addressStrs[addr])
		}
		// Filters
		filterStrs := [2]string{"nearest", "linear"}
		w.writeLine("%smag_filter::%s,", Namespace, filterStrs[sampler.MagFilter])
		w.writeLine("%smin_filter::%s,", Namespace, filterStrs[sampler.MinFilter])
		if sampler.MipFilter != nil {
			w.writeLine("%smip_filter::%s,", Namespace, filterStrs[*sampler.MipFilter])
		}
		// Border color (only if not TransparentBlack)
		if sampler.BorderColor != SamplerBorderColorTransparentBlack {
			borderStrs := [3]string{"transparent_black", "opaque_black", "opaque_white"}
			w.writeLine("%sborder_color::%s,", Namespace, borderStrs[sampler.BorderColor])
		}
		// Compare func (only if not Never)
		if sampler.CompareFunc != SamplerCompareFuncNever {
			compareFuncStrs := [8]string{"never", "less", "less_equal", "greater", "greater_equal", "equal", "not_equal", "always"}
			w.writeLine("%scompare_func::%s,", Namespace, compareFuncStrs[sampler.CompareFunc])
		}
		// Coord
		coordStrs := [2]string{"normalized", "pixel"}
		w.writeLine("%scoord::%s", Namespace, coordStrs[sampler.Coord])
		w.popIndent()
		w.writeLine(");")
	}
}

// writeBindingAttribute writes the MSL attribute for a binding.
func (w *Writer) writeBindingAttribute(binding ir.Binding) string {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return builtinOutputAttribute(b)
	case ir.LocationBinding:
		return fmt.Sprintf("[[color(%d)]]", b.Location)
	}
	return ""
}

// outputMemberAttribute returns the MSL attribute for an output struct member.
// Handles builtins, location bindings with dual-source blending, and vertex outputs.
func (w *Writer) outputMemberAttribute(binding ir.Binding, stage ir.ShaderStage) string {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		if b.Invariant {
			w.requireVersion(Version2_1)
		}
		return builtinOutputAttribute(b)
	case ir.LocationBinding:
		switch stage {
		case ir.StageVertex:
			// Vertex outputs use user() for custom varyings, with interpolation
			interpStr := resolveInterpolationString(b.Interpolation)
			if interpStr != "" {
				return fmt.Sprintf("[[user(loc%d), %s]]", b.Location, interpStr)
			}
			return fmt.Sprintf("[[user(loc%d)]]", b.Location)
		case ir.StageFragment:
			// Fragment outputs use color() with optional dual-source index()
			if b.BlendSrc != nil {
				w.requireVersion(Version1_2) // dual source blending requires Metal 1.2
				return fmt.Sprintf("[[color(%d) index(%d)]]", b.Location, *b.BlendSrc)
			}
			return fmt.Sprintf("[[color(%d)]]", b.Location)
		}
	}
	return ""
}

// requireBuiltinVersion bumps the minimum required Metal version based on
// which builtin is used. Matches Rust naga version requirements:
//   - BuiltinBarycentric requires Metal 2.3+
//   - BuiltinViewIndex requires Metal 2.2+ (we use 2.3 to match Rust naga snapshot output)
func (w *Writer) requireBuiltinVersion(builtin ir.BuiltinValue) {
	switch builtin {
	case ir.BuiltinBarycentric:
		w.requireVersion(Version2_3)
	case ir.BuiltinViewIndex:
		w.requireVersion(Version2_3)
	}
}

// builtinInputAttribute returns the MSL attribute for a built-in input.
func builtinInputAttribute(builtin ir.BuiltinValue, stage ir.ShaderStage) string {
	switch builtin {
	case ir.BuiltinPosition:
		if stage == ir.StageFragment {
			return attrPosition
		}
		return ""
	case ir.BuiltinVertexIndex:
		return "[[vertex_id]]"
	case ir.BuiltinInstanceIndex:
		return "[[instance_id]]"
	case ir.BuiltinFrontFacing:
		return "[[front_facing]]"
	case ir.BuiltinSampleIndex:
		return "[[sample_id]]"
	case ir.BuiltinSampleMask:
		return "[[sample_mask]]"
	case ir.BuiltinLocalInvocationID:
		return "[[thread_position_in_threadgroup]]"
	case ir.BuiltinLocalInvocationIndex:
		return "[[thread_index_in_threadgroup]]"
	case ir.BuiltinGlobalInvocationID:
		return "[[thread_position_in_grid]]"
	case ir.BuiltinWorkGroupID:
		return "[[threadgroup_position_in_grid]]"
	case ir.BuiltinNumWorkGroups:
		return "[[threadgroups_per_grid]]"
	case ir.BuiltinBarycentric:
		return "[[barycentric_coord]]"
	case ir.BuiltinViewIndex:
		return "[[amplification_id]]"
	case ir.BuiltinPrimitiveIndex:
		return "[[primitive_id]]"
	case ir.BuiltinNumSubgroups:
		return "[[simdgroups_per_threadgroup]]"
	case ir.BuiltinSubgroupSize:
		return "[[threads_per_simdgroup]]"
	case ir.BuiltinSubgroupID:
		return "[[simdgroup_index_in_threadgroup]]"
	case ir.BuiltinSubgroupInvocationID:
		return "[[thread_index_in_simdgroup]]"
	}
	return ""
}

// builtinOutputAttribute returns the MSL attribute for a built-in output.
func builtinOutputAttribute(binding ir.BuiltinBinding) string {
	switch binding.Builtin {
	case ir.BuiltinPosition:
		if binding.Invariant {
			return "[[position, invariant]]"
		}
		return attrPosition
	case ir.BuiltinFragDepth:
		return "[[depth(any)]]"
	case ir.BuiltinSampleMask:
		return "[[sample_mask]]"
	case ir.BuiltinPointSize:
		return "[[point_size]]"
	}
	return ""
}

// locationInputAttribute returns the MSL attribute for a location input.
// scalarKind is used to determine default interpolation (integers → flat).
func locationInputAttribute(loc ir.LocationBinding, stage ir.ShaderStage, scalarKind ir.ScalarKind) string {
	switch stage {
	case ir.StageVertex:
		return fmt.Sprintf("[[attribute(%d)]]", loc.Location)
	case ir.StageFragment:
		// Fragment inputs use user() for custom varyings, with interpolation qualifier.
		// Rust naga always emits interpolation for fragment inputs.
		// Integer types default to flat (WGSL spec); floats default to center_perspective.
		interp := loc.Interpolation
		if interp == nil && (scalarKind == ir.ScalarSint || scalarKind == ir.ScalarUint) {
			flat := &ir.Interpolation{Kind: ir.InterpolationFlat}
			interp = flat
		}
		interpStr := resolveInterpolationString(interp)
		if interpStr != "" {
			return fmt.Sprintf("[[user(loc%d), %s]]", loc.Location, interpStr)
		}
		return fmt.Sprintf("[[user(loc%d)]]", loc.Location)
	}
	return ""
}

// typeScalarKind returns the scalar kind for a type handle.
func (w *Writer) typeScalarKind(th ir.TypeHandle) ir.ScalarKind {
	if int(th) >= len(w.module.Types) {
		return ir.ScalarFloat
	}
	inner := w.module.Types[th].Inner
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind
	case ir.VectorType:
		return t.Scalar.Kind
	default:
		return ir.ScalarFloat
	}
}

// resolveInterpolationString returns the MSL interpolation qualifier string
// for a given interpolation setting. Returns empty string if no qualifier needed.
// Matches Rust naga's ResolvedInterpolation.
func resolveInterpolationString(interp *ir.Interpolation) string {
	if interp == nil {
		// Default for float fragment inputs: center_perspective
		return interpCenterPerspective
	}
	switch interp.Kind {
	case ir.InterpolationFlat:
		return "flat"
	case ir.InterpolationLinear:
		switch interp.Sampling {
		case ir.SamplingCenter:
			return "center_no_perspective"
		case ir.SamplingCentroid:
			return "centroid_no_perspective"
		case ir.SamplingSample:
			return "sample_no_perspective"
		}
	case ir.InterpolationPerspective:
		switch interp.Sampling {
		case ir.SamplingCenter:
			return interpCenterPerspective
		case ir.SamplingCentroid:
			return "centroid_perspective"
		case ir.SamplingSample:
			return "sample_perspective"
		}
	}
	return "center_perspective"
}

// writeWorkgroupZeroInit writes the zero-initialization prologue for workgroup variables.
// Matches Rust naga: check __local_invocation_id == uint3(0), then zero-init all workgroup vars.
func (w *Writer) writeWorkgroupZeroInit(localInvIDName string) error {
	w.writeLine("if (%sall(%s == %suint3(0u))) {", Namespace, localInvIDName, Namespace)
	w.pushIndent()

	for i, global := range w.module.GlobalVariables {
		if global.Space != ir.SpaceWorkGroup {
			continue
		}
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		if err := w.writeZeroInitMember(name, global.Type, 0); err != nil {
			return err
		}
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("%sthreadgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)
	return nil
}

// writeZeroInitMember writes zero-initialization code for a single member.
// For atomics: atomic_store_explicit(&member, 0, memory_order_relaxed)
// For arrays of atomics: nested for-loops
// For plain data: member = {};
func (w *Writer) writeZeroInitMember(accessPath string, typeHandle ir.TypeHandle, depth int) error {
	if int(typeHandle) >= len(w.module.Types) {
		// Unknown type — use aggregate zero init
		w.writeLine("%s = {};", accessPath)
		return nil
	}

	typeInfo := &w.module.Types[typeHandle]

	switch inner := typeInfo.Inner.(type) {
	case ir.AtomicType:
		// Atomic: use atomic_store_explicit
		w.writeLine("%satomic_store_explicit(&%s, 0, %smemory_order_relaxed);", Namespace, accessPath, Namespace)

	case ir.ArrayType:
		// Check if the element type contains atomics
		if w.typeContainsAtomics(inner.Base) {
			// Need nested loop to zero-init each element
			iterVar := fmt.Sprintf("__i%d", depth)
			size := uint32(0)
			if inner.Size.Constant != nil {
				size = *inner.Size.Constant
			}
			w.writeLine("for (int %s = 0; %s < %d; %s++) {", iterVar, iterVar, size, iterVar)
			w.pushIndent()
			elemPath := fmt.Sprintf("%s.inner[%s]", accessPath, iterVar)
			if err := w.writeZeroInitMember(elemPath, inner.Base, depth+1); err != nil {
				return err
			}
			w.popIndent()
			w.writeLine("}")
		} else {
			// Plain array — aggregate zero init
			w.writeLine("%s = {};", accessPath)
		}

	case ir.StructType:
		// Check if the struct contains atomics
		if w.typeContainsAtomics(typeHandle) {
			// Zero-init each member individually
			for _, member := range inner.Members {
				memberName := member.Name
				if memberName == "" {
					memberName = "inner" // fallback
				}
				memberPath := fmt.Sprintf("%s.%s", accessPath, memberName)
				if err := w.writeZeroInitMember(memberPath, member.Type, depth); err != nil {
					return err
				}
			}
		} else {
			// Plain struct — aggregate zero init
			w.writeLine("%s = {};", accessPath)
		}

	default:
		// All other types — aggregate zero init
		w.writeLine("%s = {};", accessPath)
	}

	return nil
}

// typeContainsAtomics returns true if the type or any of its nested types contains atomics.
func (w *Writer) typeContainsAtomics(typeHandle ir.TypeHandle) bool {
	if int(typeHandle) >= len(w.module.Types) {
		return false
	}
	typeInfo := &w.module.Types[typeHandle]

	switch inner := typeInfo.Inner.(type) {
	case ir.AtomicType:
		return true
	case ir.ArrayType:
		return w.typeContainsAtomics(inner.Base)
	case ir.StructType:
		for _, member := range inner.Members {
			if w.typeContainsAtomics(member.Type) {
				return true
			}
		}
	}
	return false
}

// isExternalTextureGlobal returns true if the global variable at the given handle
// is an external texture (ImageClass::External).
func (w *Writer) isExternalTextureGlobal(handle uint32) bool {
	if int(handle) >= len(w.module.GlobalVariables) {
		return false
	}
	gvar := &w.module.GlobalVariables[handle]
	if int(gvar.Type) >= len(w.module.Types) {
		return false
	}
	if img, ok := w.module.Types[gvar.Type].Inner.(ir.ImageType); ok {
		return img.Class == ir.ImageClassExternal
	}
	return false
}

// writeExternalTextureEntryPointParams emits the 4 entry point parameters for an external
// texture global variable: 3 texture2d planes + 1 constant NagaExternalTextureParams buffer.
// Matches Rust naga writer.rs ~line 7184.
func (w *Writer) writeExternalTextureEntryPointParams(handle uint32, global *ir.GlobalVariable, epName string, paramCount *int) {
	// Look up the external texture bind target from the resource map.
	var extTarget *BindExternalTextureTarget
	if global.Binding != nil && w.currentResourceMap != nil {
		if bt, ok := w.currentResourceMap[*global.Binding]; ok {
			extTarget = bt.ExternalTexture
		}
	}

	// Emit 3 plane textures.
	planeNameKeys := [3]nameKeyKind{nameKeyExternalTexturePlane0, nameKeyExternalTexturePlane1, nameKeyExternalTexturePlane2}
	for i := 0; i < 3; i++ {
		planeName := w.getName(nameKey{kind: planeNameKeys[i], handle1: handle})
		param := fmt.Sprintf("%stexture2d<float, %saccess::sample> %s", Namespace, Namespace, planeName)
		if extTarget != nil {
			param += fmt.Sprintf(" [[texture(%d)]]", extTarget.Planes[i])
		}
		w.writeEntryPointParam(*paramCount, param)
		*paramCount++
	}

	// Emit params constant buffer.
	paramsName := w.getName(nameKey{kind: nameKeyExternalTextureParams, handle1: handle})
	paramsTypeName := ""
	if w.module.SpecialTypes.ExternalTextureParams != nil {
		paramsTypeName = w.getTypeName(*w.module.SpecialTypes.ExternalTextureParams)
	}
	if paramsTypeName == "" {
		paramsTypeName = "NagaExternalTextureParams"
	}
	param := fmt.Sprintf("constant %s& %s", paramsTypeName, paramsName)
	if extTarget != nil {
		param += fmt.Sprintf(" [[buffer(%d)]]", extTarget.Params)
	}
	w.writeEntryPointParam(*paramCount, param)
	*paramCount++
}

// writeExternalTextureWrapperConstruction emits the const NagaExternalTextureWrapper construction
// at the start of an entry point body for an external texture global variable.
// Matches Rust naga writer.rs ~line 7497.
func (w *Writer) writeExternalTextureWrapperConstruction(handle uint32) {
	wrapperName := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: handle})
	w.writeLine("const NagaExternalTextureWrapper %s {", wrapperName)
	w.pushIndent()
	for i, key := range []nameKeyKind{nameKeyExternalTexturePlane0, nameKeyExternalTexturePlane1, nameKeyExternalTexturePlane2} {
		planeName := w.getName(nameKey{kind: key, handle1: handle})
		w.writeLine(".plane%d = %s,", i, planeName)
	}
	paramsName := w.getName(nameKey{kind: nameKeyExternalTextureParams, handle1: handle})
	w.writeLine(".params = %s,", paramsName)
	w.popIndent()
	w.writeLine("};")
}
