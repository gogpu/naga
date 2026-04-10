package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// emitInputLoads emits dx.op.loadInput calls for each entry point argument.
// Each vector component becomes a separate loadInput call.
// For compute shaders, builtin arguments use dx.op.threadId/groupId/etc.
//
// Reference: Mesa nir_to_dxil.c emit_load_input_via_intrinsic(),
// emit_load_global_invocation_id(), emit_load_local_invocation_id()
func (e *Emitter) emitInputLoads(fn *ir.Function, stage ir.ShaderStage) error {
	// inputRow tracks the ISG1 row index. Each loadInput call uses this as
	// the inputID operand. Must match the order in buildSignatures/buildSignaturesEx.
	inputRow := 0

	for argIdx, arg := range fn.Arguments {
		if arg.Binding == nil {
			// Struct-typed arguments have per-member bindings instead of a top-level binding.
			argType := e.ir.Types[arg.Type]
			if st, isSt := argType.Inner.(ir.StructType); isSt {
				n, err := e.emitStructInputLoads(fn, argIdx, &st, stage, inputRow)
				if err != nil {
					return err
				}
				inputRow += n
			}
			continue
		}

		// For compute and mesh shaders, builtins are loaded via dx.op thread ID intrinsics.
		if stage == ir.StageCompute || stage == ir.StageMesh || stage == ir.StageTask {
			if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				if err := e.emitComputeBuiltinLoad(fn, argIdx, bb.Builtin); err != nil {
					return err
				}
				continue
			}
		}

		if err := e.emitSingleInputLoad(fn, argIdx, &arg, stage, inputRow); err != nil {
			return err
		}
		inputRow++
	}
	return nil
}

// emitSingleInputLoad emits dx.op.loadInput calls for a single argument.
// inputRow is the ISG1 row index for this argument's signature element.
func (e *Emitter) emitSingleInputLoad(fn *ir.Function, argIdx int, arg *ir.FunctionArgument, stage ir.ShaderStage, inputRow int) error {
	argType := e.ir.Types[arg.Type]
	scalar, ok := scalarOfType(argType.Inner)
	if !ok {
		if st, isSt := argType.Inner.(ir.StructType); isSt {
			_, err := e.emitStructInputLoads(fn, argIdx, &st, stage, inputRow)
			return err
		}
		return fmt.Errorf("unsupported input type for argument %d", argIdx)
	}

	numComps := componentCount(argType.Inner)
	ol := overloadForScalar(scalar)
	loadFn := e.getDxOpLoadFunc(ol)
	inputID := inputRow
	exprHandle := e.findArgExprHandle(fn, argIdx)

	for comp := 0; comp < numComps; comp++ {
		opcodeVal := e.getIntConstID(int64(OpLoadInput))
		inputIDVal := e.getIntConstID(int64(inputID))
		rowVal := e.getIntConstID(0)
		colVal := e.getI8ConstID(int64(comp))
		vertexIDVal := e.getUndefConstID()

		valueID := e.addCallInstr(loadFn, loadFn.FuncType.RetType,
			[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})

		if comp == 0 {
			e.exprValues[exprHandle] = valueID
			if numComps > 1 {
				comps := make([]int, numComps)
				comps[0] = valueID
				e.exprComponents[exprHandle] = comps
			}
		} else if comps, ok := e.exprComponents[exprHandle]; ok && comp < len(comps) {
			comps[comp] = valueID
		}
	}
	return nil
}

// emitComputeBuiltinLoad emits dx.op thread ID intrinsic calls for compute
// shader builtins (GlobalInvocationId, LocalInvocationId, WorkGroupId, etc.).
//
// Each builtin maps to a specific dx.op intrinsic:
//   - GlobalInvocationId  → dx.op.threadId(i32 93, i32 component)
//   - LocalInvocationId   → dx.op.threadIdInGroup(i32 95, i32 component)
//   - WorkGroupId         → dx.op.groupId(i32 94, i32 component)
//   - LocalInvocationIndex → dx.op.flattenedThreadIdInGroup(i32 96)
//   - NumWorkGroups       → dx.op.groupId(i32 94, i32 component) [placeholder]
//
// Reference: Mesa nir_to_dxil.c emit_threadid_call() ~727,
// emit_threadidingroup_call() ~747, emit_groupid_call() ~789,
// emit_flattenedthreadidingroup_call() ~769
func (e *Emitter) emitComputeBuiltinLoad(fn *ir.Function, argIdx int, builtin ir.BuiltinValue) error {
	exprHandle := e.findArgExprHandle(fn, argIdx)

	switch builtin {
	case ir.BuiltinGlobalInvocationID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.threadId", OpThreadID)

	case ir.BuiltinLocalInvocationID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.threadIdInGroup", OpThreadIDInGroup)

	case ir.BuiltinWorkGroupID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.groupId", OpGroupID)

	case ir.BuiltinLocalInvocationIndex:
		return e.emitScalarThreadIDLoad(fn, exprHandle, "dx.op.flattenedThreadIdInGroup", OpFlattenedTIDInGroup)

	case ir.BuiltinNumWorkGroups:
		// NumWorkGroups is not directly available as a DXIL intrinsic.
		// It would require a CBV or root constant. For now, emit a placeholder.
		return fmt.Errorf("BuiltinNumWorkGroups not yet supported in DXIL compute shaders")

	default:
		return fmt.Errorf("unsupported compute builtin: %d", builtin)
	}
}

// emitVec3ThreadIDLoad emits 3 dx.op calls for a vec3<u32> thread ID builtin.
// Each component is loaded separately via: i32 @dx.op.NAME(i32 opcode, i32 component)
//
// Reference: Mesa nir_to_dxil.c emit_load_global_invocation_id() ~3131
func (e *Emitter) emitVec3ThreadIDLoad(_ *ir.Function, exprHandle ir.ExpressionHandle, funcName string, opcode DXILOpcode) error {
	i32Ty := e.mod.GetIntType(32)
	threadFn := e.getDxOpComputeBuiltinFunc(funcName, true)

	comps := make([]int, 3)
	for comp := 0; comp < 3; comp++ {
		opcodeVal := e.getIntConstID(int64(opcode))
		compVal := e.getIntConstID(int64(comp))
		valueID := e.addCallInstr(threadFn, i32Ty, []int{opcodeVal, compVal})
		comps[comp] = valueID
	}

	e.exprValues[exprHandle] = comps[0]
	e.exprComponents[exprHandle] = comps
	return nil
}

// emitScalarThreadIDLoad emits a single dx.op call for a scalar thread ID builtin.
// Signature: i32 @dx.op.NAME(i32 opcode)
//
// Reference: Mesa nir_to_dxil.c emit_flattenedthreadidingroup_call() ~769
func (e *Emitter) emitScalarThreadIDLoad(_ *ir.Function, exprHandle ir.ExpressionHandle, funcName string, opcode DXILOpcode) error {
	i32Ty := e.mod.GetIntType(32)
	threadFn := e.getDxOpComputeBuiltinFunc(funcName, false)

	opcodeVal := e.getIntConstID(int64(opcode))
	valueID := e.addCallInstr(threadFn, i32Ty, []int{opcodeVal})

	e.exprValues[exprHandle] = valueID
	return nil
}

// getDxOpComputeBuiltinFunc creates a dx.op function declaration for compute
// builtins. For vec3 builtins (hasComponent=true): i32 @name(i32, i32).
// For scalar builtins (hasComponent=false): i32 @name(i32).
func (e *Emitter) getDxOpComputeBuiltinFunc(name string, hasComponent bool) *module.Function {
	key := dxOpKey{name: name, overload: overloadI32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	fullName := name + suffixI32

	var params []*module.Type
	if hasComponent {
		params = []*module.Type{i32Ty, i32Ty} // opcode, component
	} else {
		params = []*module.Type{i32Ty} // opcode only
	}

	funcTy := e.mod.GetFunctionType(i32Ty, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStructInputLoads handles struct-typed inputs (e.g., fragment shader struct arguments).
// Each struct member with a binding becomes a separate loadInput call sequence.
// The inputRow parameter is the starting ISG1 row index; returns the number of
// signature elements consumed so the caller can advance the row counter.
//
// The loaded values are pre-registered on AccessIndex expressions that reference
// this struct's members, so downstream expression evaluation resolves them directly.
func (e *Emitter) emitStructInputLoads(fn *ir.Function, argIdx int, st *ir.StructType, _ ir.ShaderStage, inputRow int) (int, error) {
	// Find the expression handle for this FunctionArgument.
	argExprHandle := e.findArgExprHandle(fn, argIdx)

	type memberInfo struct {
		firstID int
		comps   []int
	}
	memberValues := make(map[int]*memberInfo, len(st.Members))
	rowCount := 0

	for memberIdx := range st.Members {
		member := &st.Members[memberIdx]
		if member.Binding == nil {
			continue
		}

		memberType := e.ir.Types[member.Type]
		scalar, ok := scalarOfType(memberType.Inner)
		if !ok {
			return 0, fmt.Errorf("unsupported struct member type for input %q", member.Name)
		}

		numComps := componentCount(memberType.Inner)
		ol := overloadForScalar(scalar)
		loadFn := e.getDxOpLoadFunc(ol)
		inputID := inputRow + rowCount

		info := &memberInfo{comps: make([]int, numComps)}
		for comp := 0; comp < numComps; comp++ {
			opcodeVal := e.getIntConstID(int64(OpLoadInput))
			inputIDVal := e.getIntConstID(int64(inputID))
			rowVal := e.getIntConstID(0)
			colVal := e.getI8ConstID(int64(comp))
			vertexIDVal := e.getUndefConstID()

			valueID := e.addCallInstr(loadFn, loadFn.FuncType.RetType,
				[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})
			info.comps[comp] = valueID
			if comp == 0 {
				info.firstID = valueID
			}
		}
		memberValues[memberIdx] = info
		rowCount++
	}

	// Build a flat component list for the whole struct argument so that
	// downstream struct stores (e.g., "var input = input_original;") can
	// access all scalar components via getComponentID.
	var allComps []int
	for memberIdx := range st.Members {
		info, hasInfo := memberValues[memberIdx]
		if !hasInfo {
			// No binding — insert a placeholder zero per scalar component.
			memberType := e.ir.Types[st.Members[memberIdx].Type]
			numScalars := totalScalarCount(e.ir, memberType.Inner)
			for j := 0; j < numScalars; j++ {
				allComps = append(allComps, 0)
			}
			continue
		}
		allComps = append(allComps, info.comps...)
	}

	// Register the first loaded value on the FunctionArgument expression.
	// Also register all flat components so struct stores work correctly.
	if len(allComps) > 0 {
		e.exprValues[argExprHandle] = allComps[0]
		e.exprComponents[argExprHandle] = allComps
	} else {
		e.exprValues[argExprHandle] = 0
	}

	// Pre-register AccessIndex expressions that reference members of this struct.
	// This allows downstream expression evaluation to find loaded values directly.
	for exprIdx := range fn.Expressions {
		ai, ok := fn.Expressions[exprIdx].Kind.(ir.ExprAccessIndex)
		if !ok || ai.Base != argExprHandle {
			continue
		}
		info, hasInfo := memberValues[int(ai.Index)]
		if !hasInfo {
			continue
		}
		handle := ir.ExpressionHandle(exprIdx)
		e.exprValues[handle] = info.firstID
		if len(info.comps) > 1 {
			e.exprComponents[handle] = info.comps
		}
	}
	return rowCount, nil
}

// emitOutputStore emits a dx.op.storeOutput call for a single output.
func (e *Emitter) emitOutputStore(outputID int, comp int, valueID int, ol overloadType) {
	storeFn := e.getDxOpStoreFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStoreOutput))
	outputIDVal := e.getIntConstID(int64(outputID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp)) // col is i8

	e.addCallInstr(storeFn, e.mod.GetVoidType(),
		[]int{opcodeVal, outputIDVal, rowVal, colVal, valueID})
}

// findArgExprHandle finds the ExpressionHandle for a FunctionArgument with the given index.
func (e *Emitter) findArgExprHandle(fn *ir.Function, argIdx int) ir.ExpressionHandle {
	for exprIdx := range fn.Expressions {
		if fa, ok := fn.Expressions[exprIdx].Kind.(ir.ExprFunctionArgument); ok {
			if int(fa.Index) == argIdx {
				return ir.ExpressionHandle(exprIdx)
			}
		}
	}
	return 0
}

// locationFromBinding extracts the location index from a Binding.
func (e *Emitter) locationFromBinding(b ir.Binding) int {
	switch bb := b.(type) {
	case ir.LocationBinding:
		return int(bb.Location)
	case ir.BuiltinBinding:
		return builtinSemanticIndex(bb.Builtin)
	default:
		return 0
	}
}

// builtinSemanticIndex maps naga builtins to DXIL semantic indices.
// For vertex shaders, SV_Position is output index 0.
func builtinSemanticIndex(b ir.BuiltinValue) int {
	switch b {
	case ir.BuiltinPosition:
		return 0
	case ir.BuiltinVertexIndex:
		return 1
	case ir.BuiltinInstanceIndex:
		return 2
	case ir.BuiltinFrontFacing:
		return 0
	case ir.BuiltinFragDepth:
		return 0
	default:
		return 0
	}
}

// --- Mesh Shader Intrinsic Emission ---

// emitSetMeshOutputCounts emits dx.op.setMeshOutputCounts(i32 168, i32 numVertices, i32 numPrimitives).
func (e *Emitter) emitSetMeshOutputCounts(vertexCountID, primitiveCountID int) {
	fn := e.getDxOpSetMeshOutputCountsFunc()
	opcodeVal := e.getIntConstID(int64(OpSetMeshOutputCounts))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, vertexCountID, primitiveCountID})
}

// getDxOpSetMeshOutputCountsFunc creates the dx.op.setMeshOutputCounts function declaration.
// void @dx.op.setMeshOutputCounts(i32 opcode, i32 numVertices, i32 numPrimitives)
func (e *Emitter) getDxOpSetMeshOutputCountsFunc() *module.Function {
	name := "dx.op.setMeshOutputCounts"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitEmitIndices emits dx.op.emitIndices(i32 169, i32 primIdx, i32 v0, i32 v1, i32 v2).
func (e *Emitter) emitEmitIndices(primitiveIdx, v0, v1, v2 int) {
	fn := e.getDxOpEmitIndicesFunc()
	opcodeVal := e.getIntConstID(int64(OpEmitIndices))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, primitiveIdx, v0, v1, v2})
}

// getDxOpEmitIndicesFunc creates the dx.op.emitIndices function declaration.
// void @dx.op.emitIndices(i32 opcode, i32 primIdx, i32 v0, i32 v1, i32 v2)
func (e *Emitter) getDxOpEmitIndicesFunc() *module.Function {
	name := "dx.op.emitIndices"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStoreVertexOutput emits dx.op.storeVertexOutput.TYPE(i32 171, i32 sigId, i32 row, i8 col, TYPE value, i32 vertexIdx).
func (e *Emitter) emitStoreVertexOutput(sigID, comp, valueID, vertexIdx int, ol overloadType) {
	fn := e.getDxOpStoreVertexOutputFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStoreVertexOutput))
	sigIDVal := e.getIntConstID(int64(sigID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, sigIDVal, rowVal, colVal, valueID, vertexIdx})
}

// getDxOpStoreVertexOutputFunc creates the dx.op.storeVertexOutput function declaration.
// void @dx.op.storeVertexOutput.TYPE(i32 opcode, i32 sigId, i32 row, i8 col, TYPE value, i32 vertexIdx)
func (e *Emitter) getDxOpStoreVertexOutputFunc(ol overloadType) *module.Function {
	name := "dx.op.storeVertexOutput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valueTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStorePrimitiveOutput emits dx.op.storePrimitiveOutput.TYPE(i32 172, i32 sigId, i32 row, i8 col, TYPE value, i32 primIdx).
func (e *Emitter) emitStorePrimitiveOutput(sigID, comp, valueID, primIdx int, ol overloadType) {
	fn := e.getDxOpStorePrimitiveOutputFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStorePrimitiveOutput))
	sigIDVal := e.getIntConstID(int64(sigID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, sigIDVal, rowVal, colVal, valueID, primIdx})
}

// getDxOpStorePrimitiveOutputFunc creates the dx.op.storePrimitiveOutput function declaration.
func (e *Emitter) getDxOpStorePrimitiveOutputFunc(ol overloadType) *module.Function {
	name := "dx.op.storePrimitiveOutput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valueTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// TODO: emitGetMeshPayload — implement when task payload access is needed.
// Signature: TYPE* @dx.op.getMeshPayload.TYPE(i32 170)
