package emit

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// emitInputLoads emits dx.op.loadInput calls for each entry point argument.
// Each vector component becomes a separate loadInput call.
//
// Reference: Mesa nir_to_dxil.c emit_load_input_via_intrinsic()
func (e *Emitter) emitInputLoads(fn *ir.Function, stage ir.ShaderStage) error {
	for argIdx, arg := range fn.Arguments {
		if arg.Binding == nil {
			continue
		}

		// Resolve the type of this argument.
		argType := e.ir.Types[arg.Type]
		scalar, ok := scalarOfType(argType.Inner)
		if !ok {
			// For struct inputs, we need to iterate over members.
			if st, isSt := argType.Inner.(ir.StructType); isSt {
				if err := e.emitStructInputLoads(fn, argIdx, &st, stage); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("unsupported input type for argument %d", argIdx)
		}

		numComps := componentCount(argType.Inner)
		ol := overloadForScalar(scalar)
		loadFn := e.getDxOpLoadFunc(ol)

		inputID := e.locationFromBinding(*arg.Binding)

		// Emit one loadInput per component.
		// dx.op.loadInput signature: TYPE(i32 opcode, i32 inputID, i32 row, i8 col, i32 vertexID)
		for comp := 0; comp < numComps; comp++ {
			opcodeVal := e.getIntConstID(int64(OpLoadInput))
			inputIDVal := e.getIntConstID(int64(inputID))
			rowVal := e.getIntConstID(0)
			colVal := e.getI8ConstID(int64(comp)) // col is i8

			vertexIDVal := e.getUndefConstID()

			valueID := e.addCallInstr(loadFn, loadFn.FuncType.RetType,
				[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})

			// Track the expression handle for this function argument.
			exprHandle := e.findArgExprHandle(fn, argIdx)
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
	}
	return nil
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
