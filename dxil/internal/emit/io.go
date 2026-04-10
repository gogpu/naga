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
	for argIdx, arg := range fn.Arguments {
		if arg.Binding == nil {
			continue
		}

		// For compute shaders, builtins are loaded via dx.op thread ID intrinsics.
		if stage == ir.StageCompute {
			if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				if err := e.emitComputeBuiltinLoad(fn, argIdx, bb.Builtin); err != nil {
					return err
				}
				continue
			}
		}

		if err := e.emitSingleInputLoad(fn, argIdx, &arg, stage); err != nil {
			return err
		}
	}
	return nil
}

// emitSingleInputLoad emits dx.op.loadInput calls for a single argument.
func (e *Emitter) emitSingleInputLoad(fn *ir.Function, argIdx int, arg *ir.FunctionArgument, stage ir.ShaderStage) error {
	argType := e.ir.Types[arg.Type]
	scalar, ok := scalarOfType(argType.Inner)
	if !ok {
		if st, isSt := argType.Inner.(ir.StructType); isSt {
			return e.emitStructInputLoads(fn, argIdx, &st, stage)
		}
		return fmt.Errorf("unsupported input type for argument %d", argIdx)
	}

	numComps := componentCount(argType.Inner)
	ol := overloadForScalar(scalar)
	loadFn := e.getDxOpLoadFunc(ol)
	inputID := e.locationFromBinding(*arg.Binding)
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

// emitStructInputLoads handles struct-typed inputs (e.g., vertex input structs).
func (e *Emitter) emitStructInputLoads(fn *ir.Function, argIdx int, st *ir.StructType, _ ir.ShaderStage) error {
	for _, member := range st.Members {
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
		loadFn := e.getDxOpLoadFunc(ol)
		inputID := e.locationFromBinding(*member.Binding)

		for comp := 0; comp < numComps; comp++ {
			opcodeVal := e.getIntConstID(int64(OpLoadInput))
			inputIDVal := e.getIntConstID(int64(inputID))
			rowVal := e.getIntConstID(0)
			colVal := e.getI8ConstID(int64(comp)) // col is i8
			vertexIDVal := e.getUndefConstID()

			_ = e.addCallInstr(loadFn, loadFn.FuncType.RetType,
				[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})
		}
	}

	// Also record the argument expression.
	for exprIdx := range fn.Expressions {
		if fa, ok := fn.Expressions[exprIdx].Kind.(ir.ExprFunctionArgument); ok {
			if int(fa.Index) == argIdx {
				// For struct args, the value ID is approximate.
				// Real struct handling would track per-member.
				e.exprValues[ir.ExpressionHandle(exprIdx)] = e.nextValue - 1
				break
			}
		}
	}
	return nil
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
