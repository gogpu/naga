package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
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
func (e *Emitter) preAllocateLocalVars(fn *ir.Function) error {
	for i := range fn.LocalVars {
		lv := ir.ExprLocalVariable{Variable: uint32(i)}
		if _, err := e.emitLocalVariable(fn, lv); err != nil {
			return fmt.Errorf("local var %d (%s): %w", i, fn.LocalVars[i].Name, err)
		}
	}
	return nil
}

// emitBlock emits all statements in a block.
func (e *Emitter) emitBlock(fn *ir.Function, block ir.Block) error {
	for i := range block {
		if err := e.emitStatement(fn, &block[i]); err != nil {
			return err
		}
	}
	return nil
}

// emitStatement emits a single statement.
func (e *Emitter) emitStatement(fn *ir.Function, stmt *ir.Statement) error {
	switch sk := stmt.Kind.(type) {
	case ir.StmtEmit:
		return e.emitStmtEmit(fn, sk)

	case ir.StmtReturn:
		return e.emitStmtReturn(fn, sk)

	case ir.StmtStore:
		return e.emitStmtStore(fn, sk)

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

	case ir.StmtSwitch:
		return e.emitSwitchStatement(fn, sk)

	case ir.StmtKill:
		// Fragment shader discard — not yet implemented.
		return nil

	case ir.StmtCall:
		// Function calls.
		return e.emitStmtCall(fn, sk)

	default:
		return fmt.Errorf("unsupported statement kind: %T", sk)
	}
}

// emitStmtEmit evaluates expressions in the emit range.
// In naga IR, StmtEmit marks when expressions should be materialized.
func (e *Emitter) emitStmtEmit(fn *ir.Function, emit ir.StmtEmit) error {
	for h := emit.Range.Start; h < emit.Range.End; h++ {
		if _, err := e.emitExpression(fn, h); err != nil {
			return fmt.Errorf("emit range [%d]: %w", h, err)
		}
	}
	return nil
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

	// Store the return value as output(s).
	scalar, ok := scalarOfType(resultType.Inner)
	if !ok {
		// Vector type — handle via component access.
		if vt, isVec := resultType.Inner.(ir.VectorType); isVec {
			ol := overloadForScalar(vt.Scalar)
			outputID := e.outputLocationFromBinding(fn.Result.Binding)
			for comp := 0; comp < int(vt.Size); comp++ {
				compValueID := e.getComponentID(*ret.Value, comp)
				e.emitOutputStore(outputID, comp, compValueID, ol)
			}
			return nil
		}
		return fmt.Errorf("unsupported return type: %T", resultType.Inner)
	}

	numComps := componentCount(resultType.Inner)
	ol := overloadForScalar(scalar)
	outputID := e.outputLocationFromBinding(fn.Result.Binding)

	for comp := 0; comp < numComps; comp++ {
		compValueID := e.getComponentID(*ret.Value, comp)
		e.emitOutputStore(outputID, comp, compValueID, ol)
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

	// Now emit storeOutput for each struct member using the flattened component IDs.
	compIdx := 0
	for _, member := range st.Members {
		memberType := e.ir.Types[member.Type]

		// Handle array members by unwrapping the array.
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
			compIdx += numComps
			continue
		}
		ol := overloadForScalar(scalar)
		outputID := e.locationFromBinding(*member.Binding)
		for comp := 0; comp < numComps; comp++ {
			valueID := e.getComponentID(*ret.Value, compIdx)
			e.emitOutputStore(outputID, comp, valueID, ol)
			compIdx++
		}
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

	// Compute the flattened scalar type for GEP element access.
	// Since the struct is flattened to all-scalar fields, each GEP
	// produces a pointer to a scalar element.
	flatIdx := 0
	for _, member := range st.Members {
		delta, err := e.emitStructMemberOutput(fn, structTy, ptr, member, flatIdx)
		if err != nil {
			return err
		}
		flatIdx += delta
	}

	return nil
}

// emitStructMemberOutput emits output stores for a single struct member during return.
// Returns the flat index delta for the member.
func (e *Emitter) emitStructMemberOutput(_ *ir.Function, structTy *module.Type, ptr int, member ir.StructMember, flatIdx int) (int, error) {
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
	outputID := e.locationFromBinding(*member.Binding)
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

// emitStmtStore handles store statements.
//
// In naga IR, stores write a value through a pointer expression.
// For UAV (storage buffers), this emits dx.op.bufferStore.
// For local variables, this emits an LLVM store instruction.
// For vector local variables, each component is stored separately.
//
// Reference: Mesa nir_to_dxil.c dxil_emit_store(), emit_bufferstore_call()
//
//nolint:nestif // store dispatch for different pointer targets requires nesting
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

	// Check if this is a vector store to a local variable with per-component allocas.
	if lv, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if compPtrs, hasComps := e.localVarComponentPtrs[lv.Variable]; hasComps {
			// Resolve the actual scalar element type from the local variable's IR type.
			localVar := &fn.LocalVars[lv.Variable]
			irType := e.ir.Types[localVar.Type]
			elemDXILTy, resolveErr := typeToDXIL(e.mod, e.ir, irType.Inner)
			if resolveErr != nil {
				return fmt.Errorf("vector store type resolution: %w", resolveErr)
			}
			return e.emitVectorStore(fn, compPtrs, store.Value, elemDXILTy)
		}

		// Check if this is a struct store to a local variable.
		// Struct stores must be decomposed into per-field GEP + store operations
		// because the value is represented as per-component scalars.
		if int(lv.Variable) < len(fn.LocalVars) {
			localVar := &fn.LocalVars[lv.Variable]
			irType := e.ir.Types[localVar.Type]
			if st, isSt := irType.Inner.(ir.StructType); isSt {
				return e.emitStructStore(fn, lv.Variable, irType, st, store.Value)
			}
		}
	}

	// Handle AccessIndex chain to struct member's vector component:
	// Pattern: AccessIndex(AccessIndex(LocalVariable, fieldIdx), compIdx) → store to alloca
	if handled, hErr := e.tryStructMemberComponentStore(fn, store); hErr != nil {
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
		// Helper function not emitted (unsupported features). If the call has
		// a result, provide a zero-valued fallback so the shader compiles
		// (output may be incorrect for this call, but other paths still work).
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
func (e *Emitter) getZeroValueForResult(_ *ir.Function, call ir.StmtCall) int {
	// Look up the called function's return type.
	if int(call.Function) < len(e.ir.Functions) {
		calledFn := &e.ir.Functions[call.Function]
		if calledFn.Result != nil {
			resultType := e.ir.Types[calledFn.Result.Type].Inner
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

// getZeroForType returns a zero constant ID for the given type's scalar.
// Recurses into arrays and structs to find the base scalar type, ensuring
// the zero constant has the correct type (e.g., f32(0) for float arrays).
func (e *Emitter) getZeroForType(inner ir.TypeInner) int {
	scalar, ok := scalarOfType(inner)
	if ok {
		if scalar.Kind == ir.ScalarFloat {
			return e.getFloatConstID(0.0)
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
	if ol == overloadF32 || ol == overloadF64 {
		// For float: use fsub(0.0, value).
		zeroVal := e.getFloatConstID(0.0)
		negatedID = e.addBinOpInstr(retTy, BinOpFSub, zeroVal, valueID)
	} else {
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
	align := 2 // log2(4) for i32
	if bitWidth == 64 {
		align = 3 // log2(8) for i64
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
//   - BarrierSubGroup  → SYNC_THREAD_GROUP (1) (best approximation)
//
// Reference: Mesa nir_to_dxil.c emit_barrier_impl() ~3082
func (e *Emitter) emitStmtBarrier(barrier ir.StmtBarrier) error {
	var flags DXILBarrierMode

	if barrier.Flags&ir.BarrierStorage != 0 {
		flags |= BarrierModeUAVFenceGlobal
	}

	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		flags |= BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence
	}

	if barrier.Flags&ir.BarrierSubGroup != 0 {
		// DXIL does not have a direct subgroup barrier; use thread group sync.
		flags |= BarrierModeSyncThreadGroup
	}

	// If no flags are set, use a default thread group sync.
	if flags == 0 {
		flags = BarrierModeSyncThreadGroup
	}

	barrierFn := e.getDxOpBarrierFunc()
	opcodeVal := e.getIntConstID(int64(OpBarrier))
	flagsVal := e.getIntConstID(int64(flags))

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
	// Branch from then to merge (unless block already ends with a terminator).
	if !e.blockHasTerminator(e.currentBB) {
		e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
	}

	// Emit reject (else) block if present.
	if hasReject {
		e.currentBB = e.mainFn.BasicBlocks[elseBBIndex]
		if err := e.emitBlock(fn, stmt.Reject); err != nil {
			return fmt.Errorf("if reject: %w", err)
		}
		if !e.blockHasTerminator(e.currentBB) {
			e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
		}
	}

	// Continue emitting into the merge block.
	e.currentBB = mergeBB
	return nil
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
	for i := range stmt.Cases {
		c := &stmt.Cases[i]
		e.currentBB = e.mainFn.BasicBlocks[caseBBIndices[i]]
		if err := e.emitBlock(fn, c.Body); err != nil {
			return fmt.Errorf("switch case %d: %w", i, err)
		}
		// If fallthrough, branch to next case body; otherwise branch to merge.
		if !e.blockHasTerminator(e.currentBB) {
			if c.FallThrough && i+1 < len(stmt.Cases) {
				e.currentBB.AddInstruction(module.NewBrInstr(caseBBIndices[i+1]))
			} else {
				e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
			}
		}
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

// outputLocationFromBinding extracts the output location from a binding.
func (e *Emitter) outputLocationFromBinding(b *ir.Binding) int {
	if b == nil {
		return 0
	}
	return e.locationFromBinding(*b)
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
