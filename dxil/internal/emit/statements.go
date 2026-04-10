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
	return e.emitBlock(fn, fn.Body)
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
func (e *Emitter) emitStmtReturn(fn *ir.Function, ret ir.StmtReturn) error {
	if ret.Value == nil || fn.Result == nil {
		return nil
	}

	// Evaluate the return value expression.
	valueID, err := e.emitExpression(fn, *ret.Value)
	if err != nil {
		return fmt.Errorf("return value: %w", err)
	}

	// Store the return value as output(s).
	resultType := e.ir.Types[fn.Result.Type]
	scalar, ok := scalarOfType(resultType.Inner)
	if !ok {
		// Non-scalar result type — try to handle vector.
		return e.emitReturnComposite(fn, valueID, resultType.Inner)
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

// emitReturnComposite handles returning composite types (vectors, structs).
func (e *Emitter) emitReturnComposite(fn *ir.Function, valueID int, inner ir.TypeInner) error {
	switch t := inner.(type) {
	case ir.VectorType:
		// This is called when scalarOfType fails on the result type,
		// which shouldn't happen for VectorType. Just handle it.
		ol := overloadForScalar(t.Scalar)
		outputID := e.outputLocationFromBinding(fn.Result.Binding)
		for comp := 0; comp < int(t.Size); comp++ {
			e.emitOutputStore(outputID, comp, valueID+comp, ol)
		}
		return nil

	case ir.StructType:
		// Struct outputs: each member has its own binding.
		for _, member := range t.Members {
			if member.Binding == nil {
				continue
			}
			memberType := e.ir.Types[member.Type]
			scalar, ok := scalarOfType(memberType.Inner)
			if !ok {
				continue
			}
			numComps := componentCount(memberType.Inner)
			ol := overloadForScalar(scalar)
			outputID := e.locationFromBinding(*member.Binding)
			for comp := 0; comp < numComps; comp++ {
				e.emitOutputStore(outputID, comp, valueID+comp, ol)
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported return type: %T", inner)
	}
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
	// Check if this store targets a UAV (storage buffer).
	// UAV stores use dx.op.bufferStore instead of LLVM store instructions.
	if chain, ok := e.resolveUAVPointerChain(fn, store.Pointer); ok {
		return e.emitUAVStore(fn, chain, store.Value)
	}

	// Check if this is a vector store to a local variable with per-component allocas.
	if lv, ok := fn.Expressions[store.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if compPtrs, hasComps := e.localVarComponentPtrs[lv.Variable]; hasComps {
			return e.emitVectorStore(fn, compPtrs, store.Value)
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
func (e *Emitter) emitVectorStore(fn *ir.Function, compPtrs []int, valueHandle ir.ExpressionHandle) error {
	// Emit the value expression to get component tracking.
	_, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("store value: %w", err)
	}

	// Determine the scalar type for alignment.
	scalarTy := e.mod.GetFloatType(32) // default
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
func (e *Emitter) emitStmtCall(fn *ir.Function, call ir.StmtCall) error {
	// Evaluate arguments.
	for _, arg := range call.Arguments {
		if _, err := e.emitExpression(fn, arg); err != nil {
			return fmt.Errorf("call argument: %w", err)
		}
	}
	return nil
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
	// Resolve the pointer to a UAV handle + index.
	chain, ok := e.resolveUAVPointerChain(fn, atomic.Pointer)
	if !ok {
		return fmt.Errorf("atomic: pointer does not resolve to UAV")
	}

	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return fmt.Errorf("atomic: UAV handle not found for global variable %d", chain.varHandle)
	}

	// Emit the index expression.
	indexID, err := e.emitExpression(fn, chain.indexExpr)
	if err != nil {
		return fmt.Errorf("atomic index: %w", err)
	}

	switch af := atomic.Fun.(type) {
	case ir.AtomicLoad:
		return e.emitAtomicLoad(fn, atomic, handleID, indexID, chain)

	case ir.AtomicStore:
		return e.emitAtomicStore(fn, atomic, handleID, indexID, chain)

	case ir.AtomicExchange:
		if af.Compare != nil {
			return e.emitAtomicCmpXchg(fn, atomic, af, handleID, indexID)
		}
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicExchange, handleID, indexID)

	case ir.AtomicAdd:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicAdd, handleID, indexID)

	case ir.AtomicSubtract:
		// DXIL has no subtract atomic — negate the value and use ADD.
		return e.emitAtomicSubtract(fn, atomic, handleID, indexID)

	case ir.AtomicAnd:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicAnd, handleID, indexID)

	case ir.AtomicInclusiveOr:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicOr, handleID, indexID)

	case ir.AtomicExclusiveOr:
		return e.emitAtomicBinOp(fn, atomic, DXILAtomicXor, handleID, indexID)

	case ir.AtomicMin:
		op := e.atomicMinMaxOp(chain.scalar, true)
		return e.emitAtomicBinOp(fn, atomic, op, handleID, indexID)

	case ir.AtomicMax:
		op := e.atomicMinMaxOp(chain.scalar, false)
		return e.emitAtomicBinOp(fn, atomic, op, handleID, indexID)

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

// emitAtomicBinOp emits a dx.op.atomicBinOp call.
//
// Signature: i32 @dx.op.atomicBinOp.i32(i32 78, %handle, i32 atomicOp,
//
//	i32 coord0, i32 coord1, i32 coord2, i32 value) → i32
//
// Reference: Mesa nir_to_dxil.c emit_atomic_binop() ~949
func (e *Emitter) emitAtomicBinOp(fn *ir.Function, atomic ir.StmtAtomic, atomicOp DXILAtomicOp, handleID, indexID int) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic value: %w", err)
	}

	atomicFn := e.getDxOpAtomicBinOpFunc()
	opcodeVal := e.getIntConstID(int64(OpAtomicBinOp))
	atomicOpVal := e.getIntConstID(int64(atomicOp))
	undefVal := e.getUndefConstID()

	// dx.op.atomicBinOp(i32 78, %handle, i32 atomicOp, i32 coord0, i32 coord1, i32 coord2, i32 value)
	resultID := e.addCallInstr(atomicFn, e.mod.GetIntType(32), []int{
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
func (e *Emitter) emitAtomicSubtract(fn *ir.Function, atomic ir.StmtAtomic, handleID, indexID int) error {
	valueID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic subtract value: %w", err)
	}

	// Negate: 0 - value.
	zeroVal := e.getIntConstID(0)
	negatedID := e.addBinOpInstr(e.mod.GetIntType(32), BinOpSub, zeroVal, valueID)

	atomicFn := e.getDxOpAtomicBinOpFunc()
	opcodeVal := e.getIntConstID(int64(OpAtomicBinOp))
	atomicOpVal := e.getIntConstID(int64(DXILAtomicAdd))
	undefVal := e.getUndefConstID()

	resultID := e.addCallInstr(atomicFn, e.mod.GetIntType(32), []int{
		opcodeVal, handleID, atomicOpVal,
		indexID, undefVal, undefVal,
		negatedID,
	})

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
	}

	return nil
}

// emitAtomicCmpXchg emits a dx.op.atomicCompareExchange call.
//
// Signature: i32 @dx.op.atomicCompareExchange.i32(i32 79, %handle,
//
//	i32 coord0, i32 coord1, i32 coord2, i32 cmpVal, i32 newVal) → i32
//
// Reference: Mesa nir_to_dxil.c emit_atomic_cmpxchg() ~973
func (e *Emitter) emitAtomicCmpXchg(fn *ir.Function, atomic ir.StmtAtomic, exchange ir.AtomicExchange, handleID, indexID int) error {
	newValID, err := e.emitExpression(fn, atomic.Value)
	if err != nil {
		return fmt.Errorf("atomic cmpxchg new value: %w", err)
	}

	cmpValID, err := e.emitExpression(fn, *exchange.Compare)
	if err != nil {
		return fmt.Errorf("atomic cmpxchg compare value: %w", err)
	}

	cmpXchgFn := e.getDxOpAtomicCmpXchgFunc()
	opcodeVal := e.getIntConstID(int64(OpAtomicCmpXchg))
	undefVal := e.getUndefConstID()

	// dx.op.atomicCompareExchange(i32 79, %handle, i32 coord0, coord1, coord2, i32 cmpVal, i32 newVal)
	resultID := e.addCallInstr(cmpXchgFn, e.mod.GetIntType(32), []int{
		opcodeVal, handleID,
		indexID, undefVal, undefVal, // coord0, coord1, coord2
		cmpValID, newValID,
	})

	if atomic.Result != nil {
		e.exprValues[*atomic.Result] = resultID
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

// getDxOpAtomicBinOpFunc creates the dx.op.atomicBinOp.i32 function declaration.
// Signature: i32 @dx.op.atomicBinOp.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32)
func (e *Emitter) getDxOpAtomicBinOpFunc() *module.Function {
	name := "dx.op.atomicBinOp"
	key := dxOpKey{name: name, overload: overloadI32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	handleTy := e.getDxHandleType()
	fullName := name + suffixI32

	// (i32 opcode, %handle, i32 atomicOp, i32 coord0, i32 coord1, i32 coord2, i32 value) → i32
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i32Ty, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpAtomicCmpXchgFunc creates the dx.op.atomicCompareExchange.i32 function declaration.
// Signature: i32 @dx.op.atomicCompareExchange.i32(i32, %handle, i32, i32, i32, i32, i32)
func (e *Emitter) getDxOpAtomicCmpXchgFunc() *module.Function {
	name := "dx.op.atomicCompareExchange"
	key := dxOpKey{name: name, overload: overloadI32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	handleTy := e.getDxHandleType()
	fullName := name + suffixI32

	// (i32 opcode, %handle, i32 coord0, i32 coord1, i32 coord2, i32 cmpVal, i32 newVal) → i32
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(i32Ty, params)
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
