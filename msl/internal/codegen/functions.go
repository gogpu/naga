package codegen

import (
	"fmt"
	"math"
	"sort"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/unsupported"
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

	if !policiesContain(w.options.BoundsCheckPolicies, backend.BoundsCheckReadZeroSkipWrite) {
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
		w.WriteIndent()
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
			w.WriteIndent()
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
	w.WriteLine("")

	// Set context
	w.currentFunction = fn
	w.types = w.typifiers.Function(fn)
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
		w.types = nil
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
	w.PushIndent()

	// Local variables
	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}

	w.PopIndent()
	w.WriteLine("}")
	return nil
}

// writeEntryPoints writes all entry point functions.
func (w *Writer) writeEntryPoints() error {
	for epIdx := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[epIdx]
		// Check if we should skip this entry point
		if w.pipeline.EntryPoint != nil {
			if w.pipeline.EntryPoint.Name != ep.Name || w.pipeline.EntryPoint.Stage != ep.Stage {
				continue
			}
		}

		if err := w.writeEntryPoint(epIdx, ep); err != nil {
			return err
		}
	}
	return nil
}

// writeEntryPoint writes a single entry point function.
func (w *Writer) writeEntryPoint(epIdx int, ep *ir.EntryPoint) error {
	if err := w.checkVaryings(ep); err != nil {
		return err
	}
	if err := w.checkInterfaceVersion(ep); err != nil {
		return err
	}
	// Blank line before each entry point (matches Rust naga: writeln!(self.out)?)
	w.WriteLine("")

	// Entry point function is stored inline in ep.Function (not in Module.Functions[]).
	fn := &ep.Function

	// Set context
	w.currentFunction = fn
	w.types = w.typifiers.Function(fn)
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
		w.types = nil
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
		w.fragDepthRange = ""
		w.sampleMaskOutput = ""
		w.voidOutputStruct = ""
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
	default:
		return unsupported.Errorf("unsupported shader stage %d for entry point %q", ep.Stage, ep.Name)
	}

	// Return type
	resolveReturnSignature := func() (string, string) {
		if hasOutputStruct && fn.Result == nil {
			return outputStructName, ""
		}
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

	if ep.Stage == ir.StageCompute && !w.options.LangVersion.Less(Version2_1) {
		w.write("[[max_total_threads_per_threadgroup(%d)]] ", ep.Workgroup[0]*ep.Workgroup[1]*ep.Workgroup[2])
	}

	// Function signature — Rust naga format:
	// First param: "\n  param", subsequent: "\n, param"
	w.write("%s %s %s(", stageKeyword, returnType, epName)

	// Collect all parameters, then format them
	paramCount := 0

	// Build set of globals actually referenced by this entry point (direct + transitive).
	// Rust naga only emits resources that the entry point actually uses, not ALL globals.
	epUsedGlobals := make(map[uint32]struct{})
	if globals, ok := w.funcPassThroughGlobals[epFuncHandle(epIdx)]; ok {
		for _, h := range globals {
			epUsedGlobals[h] = struct{}{}
		}
	}

	// Check if we need workgroup zero-initialization for this entry point.
	// This requires: compute shader + ZeroInitializeWorkgroupMemory + workgroup vars
	// actually used by this entry point (matching Rust naga, which filters by
	// !fun_info[handle].is_empty()).
	needWorkgroupInit := false
	localInvocationIDName := ""
	if w.options.ZeroInitializeWorkgroupMemory == backend.ZeroInitWorkgroupPolyfill && ep.Stage == ir.StageCompute {
		for i, global := range w.module.GlobalVariables {
			if _, used := epUsedGlobals[uint32(i)]; !used {
				continue
			}
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
			if builtin, ok := (*arg.Binding).(ir.BuiltinBinding); ok && !isVarying(arg.Binding) {
				attr := builtinInputAttribute(builtin.Builtin, ep.Stage)
				if attr != "" {
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
			if !ok || isVarying(member.Binding) {
				continue
			}
			attr := builtinInputAttribute(builtin.Builtin, ep.Stage)
			if attr == "" {
				continue
			}
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
	if err := w.resolveEntryPointResources(ep.Name); err != nil {
		return err
	}
	if err := w.checkGlobalsVersion(ep, epUsedGlobals); err != nil {
		return err
	}
	if err := w.checkBindTargets(ep, epUsedGlobals); err != nil {
		return err
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
			// Workgroup variables are NOT emitted as entry-point parameters.
			// Metal requires the host to call setThreadgroupMemoryLength:atIndex:
			// for threadgroup entry-point parameters; instead they are declared
			// at function-body scope inside the kernel (see below), which needs
			// no host-side setup.
			continue
		} else if global.Space == ir.SpaceImmediate {
			// Immediate data variable — constant buffer parameter.
			// Resolve binding slot from per-entry-point ImmediatesBuffer config.
			name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
			typeName := w.writeTypeName(global.Type, StorageAccess(0))
			attr, err := w.resolveImmediatesBufferBinding(ep.Name)
			if err != nil {
				return err
			}
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
		bufferSizeAttr, err := w.resolveBufferSizesBinding(ep.Name)
		if err != nil {
			return err
		}
		w.writeEntryPointParam(paramCount, fmt.Sprintf("constant _mslBufferSizes& _buffer_sizes %s", bufferSizeAttr))
		paramCount++
	}
	if err := w.writeFragDepthRangeParam(ep, epUsedGlobals, &paramCount); err != nil {
		return err
	}

	if returnAttr != "" {
		w.write("\n) %s {\n", returnAttr)
	} else {
		w.write("\n) {\n")
	}
	w.PushIndent()
	w.writeFragDepthRange()

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
				if isVarying(arg.Binding) && w.hasVaryings {
					if isPerVertex(arg.Binding) {
						w.WriteLine("const %s %s = %s;", w.writeTypeName(arg.Type, StorageAccess(0)), argName,
							perVertexValues(varyingsName, argName))
					} else {
						w.WriteLine("const auto %s = %s.%s;", argName, varyingsName, argName)
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
			w.WriteIndent()
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
				// A per-vertex member is its three vertices' values. Rust
				// writes the argument's name and a line break here
				// (writer.rs, struct arguments); the member's is right.
				if isPerVertex(member.Binding) && !doVPT && w.hasVaryings {
					w.write("%s", perVertexValues(varyingsName, memberName))
					continue
				}
				if !doVPT {
					// Normal mode: location members from varyings struct
					if isVarying(member.Binding) && w.hasVaryings {
						w.write("%s.", varyingsName)
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

	// Note: output struct variable is NOT pre-declared here.
	// For struct results, writeEntryPointOutputReturn uses the Rust naga pattern:
	//   const auto _tmp = <expr>;
	//   return OutputStruct { _tmp.field1, _tmp.field2 };
	// For simple results, inline aggregate initialization is used.
	_ = outputStructName

	// Workgroup variable declarations — function-body scope, threadgroup
	// address space. Declared inside the kernel (legal MSL) rather than as
	// entry-point parameters, because threadgroup *parameters* require the
	// host to call setThreadgroupMemoryLength:atIndex: which the pure-Go
	// Metal HAL does not do. Names are identical to the former parameters,
	// so all body references and helper-function call sites resolve unchanged.
	// Must come BEFORE the zero-init prologue, which references these names.
	for i, global := range w.module.GlobalVariables {
		if _, used := epUsedGlobals[uint32(i)]; !used {
			continue
		}
		if global.Space != ir.SpaceWorkGroup {
			continue
		}
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		typeName := w.writeTypeName(global.Type, StorageAccess(0))
		w.WriteLine("threadgroup %s %s;", typeName, name)
	}

	// Workgroup zero-initialization prologue.
	// Matches Rust naga: zero all workgroup vars if __local_invocation_id == uint3(0).
	// Must come BEFORE local variables and private var locals.
	if needWorkgroupInit {
		if err := w.writeWorkgroupZeroInit(localInvocationIDName, epUsedGlobals); err != nil {
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

	// Reconstruct the entry point's flattened arguments from the stage_in
	// struct: Rust naga writes the private globals as locals first, then these
	// aliases, then the function's own local variables (back/msl/writer.rs,
	// the private-global block, the argument reconstruction, put_locals).
	emitInputAliases()

	// Local variables — after the private globals and the input aliases,
	// matching Rust naga order.
	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}
	if w.voidOutputStruct != "" && mayReachEnd(fn.Body) {
		w.writeVoidOutputReturn()
	}

	w.PopIndent()
	w.WriteLine("}")
	w.WriteLine("") // trailing blank line after entry point (matches Rust naga: writeln!(self.out, "}}")?)
	return nil
}

// isVarying reports whether an entry point input travels in the [[stage_in]]
// struct: a location, or a barycentric built-in (Rust naga fc9060f5:
// barycentric_coord is a stage_in attribute, not a separate parameter).
func isVarying(b *ir.Binding) bool {
	if b == nil {
		return false
	}
	switch bb := (*b).(type) {
	case ir.LocationBinding:
		return true
	case ir.BuiltinBinding:
		return bb.Builtin == ir.BuiltinBarycentric || bb.Builtin == ir.BuiltinBarycentricNoPerspective
	}
	return false
}

// varyingAttribute returns the [[stage_in]] struct attribute of a varying.
func (w *Writer) varyingAttribute(b ir.Binding, stage ir.ShaderStage, ty ir.TypeHandle) string {
	if bb, ok := b.(ir.BuiltinBinding); ok {
		return builtinInputAttribute(bb.Builtin, stage)
	}
	return locationInputAttribute(b.(ir.LocationBinding), stage, w.typeScalarKind(ty))
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
		if isVarying(arg.Binding) {
			hasLocationInputs = true
		}
	}

	emitInputStruct := func(structName string, emitFields func()) {
		w.WriteLine("struct %s {", structName)
		w.PushIndent()
		emitFields()
		w.PopIndent()
		w.WriteLine("};")
	}

	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})
	// Use namer.call to generate the input struct name, matching Rust naga:
	// self.namer.call(&format!("{fun_name}Input"))
	structName := w.namer.call(epName + "Input")

	if hasLocationInputs {
		emitInputStruct(structName, func() {
			for i, arg := range fn.Arguments {
				if !isVarying(arg.Binding) {
					continue
				}
				argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)})
				w.WriteLine("%s;", w.varyingMember(*arg.Binding, ep.Stage, arg.Type, argName))
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

				isLocation := isVarying(member.Binding)
				if isLocation {
					hasLocations = true
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
					if !isVarying(member.Binding) {
						continue // skip builtins — they become separate params
					}
					key := nameKey{kind: nameKeyStructMember, handle1: uint32(sa.tyH), handle2: uint32(memberIdx)}
					memberName := w.flattenedMemberNames[key]
					w.WriteLine("%s;", w.varyingMember(*member.Binding, ep.Stage, member.Type, memberName))
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
		if !w.appliesSampleMask(ep) {
			return "", false
		}
		epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})
		w.voidOutputStruct = w.namer.call(epName + "Output")
		w.WriteLine("struct %s {", w.voidOutputStruct)
		w.PushIndent()
		w.writeSampleMaskMember(ep)
		w.PopIndent()
		w.WriteLine("};")
		return w.voidOutputStruct, true
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
				w.WriteLine("struct %s {", structName)
				w.PushIndent()
				w.WriteLine("%s %s %s;", returnType, resultMemberName, attr)
				// Add _point_size for vertex shaders when forced
				if ep.Stage == ir.StageVertex && w.options.AllowAndForcePointSize {
					w.WriteLine("float _point_size [[point_size]];")
				}
				w.writeSampleMaskMember(ep)
				w.PopIndent()
				w.WriteLine("};")

				return structName, true
			}
		}
		return "", false
	}

	w.WriteLine("struct %s {", structName)
	w.PushIndent()

	for memberIdx, member := range st.Members {
		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(resultType), handle2: uint32(memberIdx)})
		memberType := w.writeTypeName(member.Type, StorageAccess(0))

		// Metal reads clip distances from a raw float array with the
		// [[clip_distance]] attribute (Rust: `float name [[clip_distance]]
		// [N];`); the return copies the wrapped WGSL array into it.
		if arr, n, ok := w.clipDistancesArray(member); ok {
			w.WriteLine("%s %s [[clip_distance]] [%d];", w.writeTypeName(arr.Base, StorageAccess(0)), memberName, n)
			continue
		}

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

		w.WriteLine("%s %s %s;", memberType, memberName, attr)
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
			w.WriteLine("float _point_size [[point_size]];")
		}
	}
	w.writeSampleMaskMember(ep)

	w.PopIndent()
	w.WriteLine("};")

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
		w.WriteIndent()
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
		return w.writeGlobalExpression(w.module.Constants[handle].Init)
	}
	w.write("{}")
	return nil
}

// writeGlobalExpression writes a global expression value (from Module.GlobalExpressions).
func (w *Writer) writeGlobalExpression(handle ir.ExpressionHandle) error {
	if int(handle) >= len(w.module.GlobalExpressions) {
		return fmt.Errorf("global expression [%d] out of range (%d expressions)", handle, len(w.module.GlobalExpressions))
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
			if _, hasName := ir.NamePresence(c.Name, c.NamePresent); hasName {
				name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(k.Constant)})
				w.write("%s", name)
			} else {
				// Unnamed constant: inline its init expression.
				return w.writeGlobalExpression(c.Init)
			}
		}
	case ir.ExprOverride:
		// Rust Error::Override: naga.ProcessOverrides replaces them first.
		return ir.ErrUnresolvedOverride
	case ir.ExprSplat:
		// The vector type is the value's scalar type, from the global
		// expression types, whatever expression the value is (Rust naga's
		// put_possibly_const_expression).
		global, err := w.typifiers.Global()
		if err != nil {
			return err
		}
		if int(k.Value) >= global.Len() {
			return fmt.Errorf("splat value [%d] has no type", k.Value)
		}
		scalar, ok := global.Inner(w.module, k.Value).(ir.ScalarType)
		if !ok {
			return fmt.Errorf("splat value [%d] must be a scalar, got %T", k.Value, global.Inner(w.module, k.Value))
		}
		w.write("%s%s%d(", Namespace, scalarTypeName(scalar), k.Size)
		if err := w.writeGlobalExpression(k.Value); err != nil {
			return err
		}
		w.write(")")
	default:
		// Only literals, constants, zero values, composes and splats are
		// constant expressions once naga.ProcessOverrides has run; Rust
		// naga reports anything else as Error::Override.
		return ir.ErrUnresolvedOverride
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
	if global.Binding != nil {
		bt, hasMappedBinding = w.currentEPResources.Resources[*global.Binding]
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
				coherent := ""
				if global.MemoryDecorations.Contains(ir.MemoryCoherent) {
					coherent = "coherent "
				}
				return fmt.Sprintf("%s%s %s%s& %s [[user(fake0)]]", coherent, space, typeName, constQual, name)
			}
			return fmt.Sprintf("%s %s [[user(fake0)]]", typeName, name)
		}

		idx := w.bindTargetIndex(bt.Buffer, global.Binding)
		if space == spaceConstant || space == spaceDevice {
			// A coherent buffer takes the coherent qualifier before the space,
			// as Rust naga writes it (back/msl/writer.rs:463-471).
			coherent := ""
			if global.MemoryDecorations.Contains(ir.MemoryCoherent) {
				coherent = "coherent "
			}
			return fmt.Sprintf("%s%s %s%s& %s [[buffer(%d)]]", coherent, space, typeName, constQual, name, idx)
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
			coherent := ""
			if global.MemoryDecorations.Contains(ir.MemoryCoherent) {
				coherent = "coherent "
			}
			w.write("%s%s %s%s& %s", coherent, space, typeName, constQual, name)
		} else if space != "" {
			// threadgroup, thread — pass by reference with address space qualifier
			w.write("%s %s& %s", space, typeName, name)
		} else {
			w.write("%s %s", typeName, name)
		}
	}
}

// resolveEntryPointResources sets the slots of the entry point being written
// (currentEPResources). A non-nil PerEntryPointMap gives them: the entry
// point's entry, or none, in which case checkBindTargets refuses what the
// entry point uses unless FakeMissingBindings writes placeholders. A nil
// map takes the automatic numbering (autoEntryPointResources).
func (w *Writer) resolveEntryPointResources(epName string) error {
	if w.options.PerEntryPointMap != nil {
		w.currentEPResources = w.options.PerEntryPointMap[epName]
		return nil
	}
	if w.autoResources == nil {
		auto, err := autoEntryPointResources(w.module, w.options.ClampFragDepth != nil)
		if err != nil {
			return err
		}
		w.autoResources = &auto
	}
	w.currentEPResources = *w.autoResources
	return nil
}

// slotCounter hands out the consecutive slots of one Metal argument table.
// Slots are uint8 in EntryPointResources; running past 255 is an error, not
// a wrap that would bind two resources to one slot.
type slotCounter struct {
	what string
	next int
}

func (c *slotCounter) take(n int) (uint8, error) {
	if c.next+n > math.MaxUint8+1 {
		return 0, fmt.Errorf("automatic binding: more than %d %s slots", math.MaxUint8+1, c.what)
	}
	slot := uint8(c.next)
	c.next += n
	return slot, nil
}

// autoEntryPointResources numbers the slots of every entry point of module
// for a nil PerEntryPointMap, as Rust wgpu-hal's Metal device numbers a
// pipeline layout (wgpu-hal/src/metal/device.rs create_pipeline_layout):
// the immediates buffer first, when the module has immediate data or the
// frag_depth clamp needs that buffer; then every bound global by (group,
// binding), a buffer, texture or sampler slot by its kind and an external
// texture three texture slots for its planes and a buffer slot for its
// parameters; the sizes buffer after the last buffer. One numbering serves
// every entry point, as one layout does in wgpu-hal.
func autoEntryPointResources(module *ir.Module, clampFragDepth bool) (EntryPointResources, error) {
	buffers := slotCounter{what: "buffer"}
	textures := slotCounter{what: "texture"}
	samplers := slotCounter{what: "sampler"}
	var res EntryPointResources

	needsImmediates := clampFragDepth
	for i := range module.GlobalVariables {
		needsImmediates = needsImmediates || module.GlobalVariables[i].Space == ir.SpaceImmediate
	}
	if needsImmediates {
		slot, err := buffers.take(1)
		if err != nil {
			return EntryPointResources{}, err
		}
		res.ImmediatesBuffer = &slot
	}

	bound := make([]*ir.GlobalVariable, 0, len(module.GlobalVariables))
	for i := range module.GlobalVariables {
		if module.GlobalVariables[i].Binding != nil {
			bound = append(bound, &module.GlobalVariables[i])
		}
	}
	sort.Slice(bound, func(i, j int) bool {
		a, b := *bound[i].Binding, *bound[j].Binding
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		return a.Binding < b.Binding
	})

	res.Resources = make(map[ir.ResourceBinding]BindTarget, len(bound))
	for _, global := range bound {
		var target BindTarget
		var err error
		switch t := module.Types[global.Type].Inner.(type) {
		case ir.SamplerType:
			var slot uint8
			slot, err = samplers.take(1)
			target.Sampler = &BindSamplerTarget{Slot: slot}
		case ir.ImageType:
			if t.Class == ir.ImageClassExternal {
				target.ExternalTexture, err = takeExternalTextureSlots(&textures, &buffers)
				break
			}
			var slot uint8
			slot, err = textures.take(1)
			target.Texture = &slot
		default:
			var slot uint8
			slot, err = buffers.take(1)
			target.Buffer = &slot
		}
		if err != nil {
			return EntryPointResources{}, err
		}
		res.Resources[*global.Binding] = target
	}

	sizes, err := buffers.take(1)
	if err != nil {
		return EntryPointResources{}, err
	}
	res.SizesBuffer = &sizes
	return res, nil
}

// takeExternalTextureSlots takes the three plane textures and the params
// buffer of an external texture, in wgpu-hal's order.
func takeExternalTextureSlots(textures, buffers *slotCounter) (*BindExternalTextureTarget, error) {
	plane, err := textures.take(3)
	if err != nil {
		return nil, err
	}
	params, err := buffers.take(1)
	if err != nil {
		return nil, err
	}
	return &BindExternalTextureTarget{Planes: [3]uint8{plane, plane + 1, plane + 2}, Params: params}, nil
}

// checkBindTargets reports, as Rust naga does, a resource the entry point
// uses that its PerEntryPointMap entry lacks or holds as another kind of
// resource, and a sizes or immediates buffer the entry point needs that the
// entry doesn't place; an entry point a non-nil map has no entry for has
// none of them. The errors wrap backend.ErrMissingBinding. A nil
// PerEntryPointMap numbers every slot automatically (autoEntryPointResources),
// and FakeMissingBindings writes placeholders for the missing ones.
func (w *Writer) checkBindTargets(ep *ir.EntryPoint, used map[uint32]struct{}) error {
	if w.options.PerEntryPointMap == nil || w.options.FakeMissingBindings {
		return nil
	}
	epRes := w.currentEPResources
	needsSizes := w.doVertexPulling(ep)
	for h := range w.module.GlobalVariables {
		if _, ok := used[uint32(h)]; !ok {
			continue
		}
		global := &w.module.GlobalVariables[h]
		needsSizes = needsSizes || w.needsArrayLength(global.Type)
		switch global.Space {
		case ir.SpaceUniform, ir.SpaceStorage, ir.SpaceHandle:
		case ir.SpaceImmediate:
			if epRes.ImmediatesBuffer == nil {
				return fmt.Errorf("%w: entry point %q has no immediates buffer", backend.ErrMissingBinding, ep.Name)
			}
			continue
		default:
			continue
		}
		if global.Binding == nil {
			return fmt.Errorf("entry point %q uses %s, which has no binding", ep.Name, global.Name)
		}
		target, ok := epRes.Resources[*global.Binding]
		switch t := w.module.Types[global.Type].Inner.(type) {
		case ir.ImageType:
			if t.Class == ir.ImageClassExternal {
				ok = ok && target.ExternalTexture != nil
			} else {
				ok = ok && target.Texture != nil
			}
		case ir.SamplerType:
			ok = ok && target.Sampler != nil
		default:
			ok = ok && target.Buffer != nil
		}
		if !ok {
			return fmt.Errorf("%w: entry point %q has no bind target for %s at @group(%d) @binding(%d)", backend.ErrMissingBinding,
				ep.Name, global.Name, global.Binding.Group, global.Binding.Binding)
		}
	}
	if needsSizes && epRes.SizesBuffer == nil {
		return fmt.Errorf("%w: entry point %q has no sizes buffer", backend.ErrMissingBinding, ep.Name)
	}
	return nil
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
		bt, ok := w.currentEPResources.Resources[*global.Binding]
		if !ok || bt.Sampler == nil || !bt.Sampler.IsInline {
			continue
		}
		idx := int(bt.Sampler.Slot)
		if idx >= len(w.options.InlineSamplers) {
			continue
		}
		sampler := &w.options.InlineSamplers[idx]
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		w.WriteLine("constexpr %ssampler %s(", Namespace, name)
		w.PushIndent()
		// Address modes
		letters := [3]byte{'s', 't', 'r'}
		addressStrs := [5]string{"repeat", "mirrored_repeat", "clamp_to_edge", "clamp_to_zero", "clamp_to_border"}
		for j, addr := range sampler.Address {
			w.WriteLine("%s%c_address::%s,", Namespace, letters[j], addressStrs[addr])
		}
		// Filters
		filterStrs := [2]string{"nearest", "linear"}
		w.WriteLine("%smag_filter::%s,", Namespace, filterStrs[sampler.MagFilter])
		w.WriteLine("%smin_filter::%s,", Namespace, filterStrs[sampler.MinFilter])
		if sampler.MipFilter != nil {
			w.WriteLine("%smip_filter::%s,", Namespace, filterStrs[*sampler.MipFilter])
		}
		// Border color (only if not TransparentBlack)
		if sampler.BorderColor != SamplerBorderColorTransparentBlack {
			borderStrs := [3]string{"transparent_black", "opaque_black", "opaque_white"}
			w.WriteLine("%sborder_color::%s,", Namespace, borderStrs[sampler.BorderColor])
		}
		// Compare func (only if not Never)
		if sampler.CompareFunc != SamplerCompareFuncNever {
			compareFuncStrs := [8]string{"never", "less", "less_equal", "greater", "greater_equal", "equal", "not_equal", "always"}
			w.WriteLine("%scompare_func::%s,", Namespace, compareFuncStrs[sampler.CompareFunc])
		}
		// Coord
		coordStrs := [2]string{"normalized", "pixel"}
		w.WriteLine("%scoord::%s", Namespace, coordStrs[sampler.Coord])
		w.PopIndent()
		w.WriteLine(");")
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
		return builtinOutputAttribute(b)
	case ir.LocationBinding:
		switch stage {
		case ir.StageVertex:
			// Vertex outputs use user() for custom varyings, with interpolation
			interpStr := resolveInterpolationString(b)
			if interpStr != "" {
				return fmt.Sprintf("[[user(loc%d), %s]]", b.Location, interpStr)
			}
			return fmt.Sprintf("[[user(loc%d)]]", b.Location)
		case ir.StageFragment:
			// Fragment outputs use color() with optional dual-source index()
			if b.BlendSrc != nil {
				return fmt.Sprintf("[[color(%d) index(%d)]]", b.Location, *b.BlendSrc)
			}
			return fmt.Sprintf("[[color(%d)]]", b.Location)
		}
	}
	return ""
}

// mslBuiltinUnmapped is builtinInputAttribute's and builtinOutputAttribute's
// result for a built-in MSL cannot express. checkBuiltinAttribute refuses
// such an entry point before the attribute is written (Rust naga returns
// Error::UnsupportedBuiltIn at the same point, back/msl/mod.rs), so the value
// never reaches the generated MSL.
const mslBuiltinUnmapped = "[[unmapped]]"

// builtinInputAttribute returns the MSL attribute for a built-in input.
func builtinInputAttribute(builtin ir.BuiltinValue, stage ir.ShaderStage) string {
	switch builtin {
	case ir.BuiltinPosition:
		if stage == ir.StageFragment {
			return attrPosition
		}
		return ""
	case ir.BuiltinBaseInstance:
		return "[[base_instance]]"
	case ir.BuiltinBaseVertex:
		return "[[base_vertex]]"
	case ir.BuiltinPointCoord:
		return "[[point_coord]]"
	case ir.BuiltinWorkGroupSize:
		return "[[dispatch_threads_per_threadgroup]]"
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
	case ir.BuiltinBarycentricNoPerspective:
		return "[[barycentric_coord, center_no_perspective]]"
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
	return mslBuiltinUnmapped
}

// builtinOutputAttribute returns the MSL attribute for a built-in output.
// clipDistancesArray returns the array type and length of an output member
// bound to @builtin(clip_distances), which WGSL requires to be a fixed-size
// array of f32.
func (w *Writer) clipDistancesArray(member ir.StructMember) (ir.ArrayType, uint32, bool) {
	if member.Binding == nil {
		return ir.ArrayType{}, 0, false
	}
	bb, ok := (*member.Binding).(ir.BuiltinBinding)
	if !ok || bb.Builtin != ir.BuiltinClipDistances || int(member.Type) >= len(w.module.Types) {
		return ir.ArrayType{}, 0, false
	}
	arr, ok := w.module.Types[member.Type].Inner.(ir.ArrayType)
	if !ok || arr.Size.Constant == nil {
		return ir.ArrayType{}, 0, false
	}
	return arr, *arr.Size.Constant, true
}

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
	return mslBuiltinUnmapped
}

// checkBuiltinAttribute refuses a built-in MSL cannot express, as Rust naga's
// Error::UnsupportedBuiltIn does (back/msl/mod.rs, try_fmt). Before this
// check a built-in without a mapping produced an empty attribute, which Metal
// rejects with a message naming neither the built-in nor the entry point.
func (w *Writer) checkBuiltinAttribute(ep *ir.EntryPoint, b ir.BuiltinBinding, ty ir.TypeHandle, output, member bool) error {
	// A task or mesh entry point is refused by writeEntryPoint itself
	// ("unsupported shader stage"), and its built-ins are the virtual ones a
	// mesh wrapper would extract (Rust back/msl/mod.rs returns no attribute
	// for them), so no MSL attribute is involved.
	if ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask {
		return nil
	}
	var attr string
	if output {
		// clip_distances is written as a raw [[clip_distance]] array only for
		// an output struct member with a constant array type (the output
		// struct path); a direct result has no such path.
		if b.Builtin == ir.BuiltinClipDistances {
			if member && int(ty) < len(w.module.Types) {
				if arr, ok := w.module.Types[ty].Inner.(ir.ArrayType); ok && arr.Size.Constant != nil {
					return nil
				}
			}
			return unsupported.Errorf("msl: entry point %q: built-in %s is not supported as a direct result", ep.Name, b.Builtin)
		}
		attr = builtinOutputAttribute(b)
	} else {
		attr = builtinInputAttribute(b.Builtin, ep.Stage)
	}
	if attr == mslBuiltinUnmapped {
		return unsupported.Errorf("msl: entry point %q: built-in %s is not supported", ep.Name, b.Builtin)
	}
	return nil
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
		if loc.Interpolation == nil && (scalarKind == ir.ScalarSint || scalarKind == ir.ScalarUint) {
			flat := ir.InterpolationFlat
			loc.Interpolation = &flat
		}
		interpStr := resolveInterpolationString(loc)
		if interpStr != "" {
			return fmt.Sprintf("[[user(loc%d), %s]]", loc.Location, interpStr)
		}
		return fmt.Sprintf("[[user(loc%d)]]", loc.Location)
	}
	return ""
}

// isPerVertex reports whether b is a per-vertex fragment input location.
func isPerVertex(b *ir.Binding) bool {
	if b == nil {
		return false
	}
	loc, ok := (*b).(ir.LocationBinding)
	return ok && loc.Interpolation != nil && *loc.Interpolation == ir.InterpolationPerVertex
}

// varyingMember returns a [[stage_in]] struct member declaration without
// its semicolon. A per-vertex input is Rust naga's
// `metal::vertex_value<T> name [[user(locN)]]` over the element type of its
// array<T, 3> (back/msl/writer.rs); any other is `type name attribute`.
func (w *Writer) varyingMember(b ir.Binding, stage ir.ShaderStage, ty ir.TypeHandle, name string) string {
	if isPerVertex(&b) {
		elem := ty
		if arr, ok := w.module.Types[ty].Inner.(ir.ArrayType); ok {
			elem = arr.Base
		}
		return fmt.Sprintf("metal::vertex_value<%s> %s [[user(loc%d)]]",
			w.writeTypeName(elem, StorageAccess(0)), name, b.(ir.LocationBinding).Location)
	}
	return fmt.Sprintf("%s %s %s", w.writeTypeName(ty, StorageAccess(0)), name, w.varyingAttribute(b, stage, ty))
}

// perVertexValues is Rust naga's initializer of a per-vertex input: the
// values of the three vertices read from the vertex_value member.
func perVertexValues(varyings, member string) string {
	return fmt.Sprintf("{ %[1]s.%[2]s.get(metal::vertex_index::first), %[1]s.%[2]s.get(metal::vertex_index::second), %[1]s.%[2]s.get(metal::vertex_index::third) }",
		varyings, member)
}

// checkVaryings refuses a per-primitive location, which only mesh shading
// has and this backend does not write. A per-vertex input
// (metal::vertex_value) needs MSL 4.0 and is refused below it, as Rust's
// PerVertexNotSupported (writer.rs).
func (w *Writer) checkVaryings(ep *ir.EntryPoint) error {
	check := func(b *ir.Binding) error {
		if b == nil {
			return nil
		}
		loc, ok := (*b).(ir.LocationBinding)
		if !ok {
			return nil
		}
		if loc.PerPrimitive {
			return unsupported.Errorf("entry point %q: location %d: per_primitive is not supported", ep.Name, loc.Location)
		}
		if isPerVertex(b) {
			return w.requireVersion(Version4_0, "@interpolate(per_vertex)")
		}
		return nil
	}
	for _, arg := range ep.Function.Arguments {
		if err := check(arg.Binding); err != nil {
			return err
		}
		if st, ok := w.module.Types[arg.Type].Inner.(ir.StructType); ok && arg.Binding == nil {
			for _, m := range st.Members {
				if err := check(m.Binding); err != nil {
					return err
				}
			}
		}
	}
	if res := ep.Function.Result; res != nil {
		if err := check(res.Binding); err != nil {
			return err
		}
		if st, ok := w.module.Types[res.Type].Inner.(ir.StructType); ok && res.Binding == nil {
			for _, m := range st.Members {
				if err := check(m.Binding); err != nil {
					return err
				}
			}
		}
	}
	return nil
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

// resolveInterpolationString returns the MSL interpolation qualifier of a
// vertex output or fragment input: Rust naga's
// ResolvedInterpolation::from_binding (back/msl/mod.rs), sampling None being
// Center. Rust unwraps the interpolation, which the validator requires; a
// binding without one is center_perspective. Per-vertex inputs are written
// by varyingMember, without a qualifier.
func resolveInterpolationString(loc ir.LocationBinding) string {
	if loc.PerPrimitive {
		return "flat"
	}
	if loc.Interpolation == nil {
		return interpCenterPerspective
	}
	sampling := ir.SamplingCenter
	if loc.Sampling != nil {
		sampling = *loc.Sampling
	}
	switch *loc.Interpolation {
	case ir.InterpolationFlat:
		return "flat"
	case ir.InterpolationLinear:
		switch sampling {
		case ir.SamplingCenter:
			return "center_no_perspective"
		case ir.SamplingCentroid:
			return "centroid_no_perspective"
		case ir.SamplingSample:
			return "sample_no_perspective"
		}
	case ir.InterpolationPerspective:
		switch sampling {
		case ir.SamplingCenter:
			return interpCenterPerspective
		case ir.SamplingCentroid:
			return "centroid_perspective"
		case ir.SamplingSample:
			return "sample_perspective"
		}
	}
	return interpCenterPerspective
}

// writeWorkgroupZeroInit writes the zero-initialization prologue for workgroup variables.
// Matches Rust naga: check __local_invocation_id == uint3(0), then zero-init the workgroup
// vars used by this entry point (Rust filters by !fun_info[handle].is_empty()).
func (w *Writer) writeWorkgroupZeroInit(localInvIDName string, usedGlobals map[uint32]struct{}) error {
	w.WriteLine("if (%sall(%s == %suint3(0u))) {", Namespace, localInvIDName, Namespace)
	w.PushIndent()

	for i, global := range w.module.GlobalVariables {
		if _, used := usedGlobals[uint32(i)]; !used {
			continue
		}
		if global.Space != ir.SpaceWorkGroup {
			continue
		}
		name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)})
		if err := w.writeZeroInitMember(name, global.Type, 0); err != nil {
			return err
		}
	}

	w.PopIndent()
	w.WriteLine("}")
	w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)
	return nil
}

// writeZeroInitMember writes zero-initialization code for a single member.
// For atomics: atomic_store_explicit(&member, 0, memory_order_relaxed)
// For arrays of atomics: nested for-loops
// For plain data: member = {};
func (w *Writer) writeZeroInitMember(accessPath string, typeHandle ir.TypeHandle, depth int) error {
	if int(typeHandle) >= len(w.module.Types) {
		// Unknown type — use aggregate zero init
		w.WriteLine("%s = {};", accessPath)
		return nil
	}

	typeInfo := &w.module.Types[typeHandle]

	switch inner := typeInfo.Inner.(type) {
	case ir.AtomicType:
		// Atomic: use atomic_store_explicit
		w.WriteLine("%satomic_store_explicit(&%s, 0, %smemory_order_relaxed);", Namespace, accessPath, Namespace)

	case ir.ArrayType:
		// Check if the element type contains atomics
		if w.typeContainsAtomics(inner.Base) {
			// Need nested loop to zero-init each element
			iterVar := fmt.Sprintf("__i%d", depth)
			size := uint32(0)
			if inner.Size.Constant != nil {
				size = *inner.Size.Constant
			}
			w.WriteLine("for (int %s = 0; %s < %d; %s++) {", iterVar, iterVar, size, iterVar)
			w.PushIndent()
			elemPath := fmt.Sprintf("%s.inner[%s]", accessPath, iterVar)
			if err := w.writeZeroInitMember(elemPath, inner.Base, depth+1); err != nil {
				return err
			}
			w.PopIndent()
			w.WriteLine("}")
		} else {
			// Plain array — aggregate zero init
			w.WriteLine("%s = {};", accessPath)
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
			w.WriteLine("%s = {};", accessPath)
		}

	default:
		// All other types — aggregate zero init
		w.WriteLine("%s = {};", accessPath)
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
	if global.Binding != nil {
		if bt, ok := w.currentEPResources.Resources[*global.Binding]; ok {
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
	w.WriteLine("const NagaExternalTextureWrapper %s {", wrapperName)
	w.PushIndent()
	for i, key := range []nameKeyKind{nameKeyExternalTexturePlane0, nameKeyExternalTexturePlane1, nameKeyExternalTexturePlane2} {
		planeName := w.getName(nameKey{kind: key, handle1: handle})
		w.WriteLine(".plane%d = %s,", i, planeName)
	}
	paramsName := w.getName(nameKey{kind: nameKeyExternalTextureParams, handle1: handle})
	w.WriteLine(".params = %s,", paramsName)
	w.PopIndent()
	w.WriteLine("};")
}
