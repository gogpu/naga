package msl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// writeBlock writes a block of statements.
func (w *Writer) writeBlock(block ir.Block) error {
	for _, stmt := range block {
		if err := w.writeStatement(stmt); err != nil {
			return err
		}
	}
	return nil
}

// writeStatement writes a single statement.
func (w *Writer) writeStatement(stmt ir.Statement) error {
	return w.writeStatementKind(stmt.Kind)
}

// writeStatementKind writes a statement based on its kind.
func (w *Writer) writeStatementKind(kind ir.StatementKind) error {
	switch k := kind.(type) {
	case ir.StmtEmit:
		return w.writeEmit(k)

	case ir.StmtBlock:
		w.writeLine("{")
		w.pushIndent()
		if err := w.writeBlock(k.Block); err != nil {
			return err
		}
		w.popIndent()
		w.writeLine("}")
		return nil

	case ir.StmtIf:
		return w.writeIf(k)

	case ir.StmtSwitch:
		return w.writeSwitch(k)

	case ir.StmtLoop:
		return w.writeLoop(k)

	case ir.StmtBreak:
		w.writeLine("break;")
		return nil

	case ir.StmtContinue:
		w.writeLine("continue;")
		return nil

	case ir.StmtReturn:
		return w.writeReturn(k)

	case ir.StmtKill:
		w.writeLine("discard_fragment();")
		return nil

	case ir.StmtBarrier:
		return w.writeBarrier(k)

	case ir.StmtStore:
		return w.writeStore(k)

	case ir.StmtImageStore:
		return w.writeImageStore(k)

	case ir.StmtAtomic:
		return w.writeAtomic(k)

	case ir.StmtCall:
		return w.writeCall(k)

	case ir.StmtWorkGroupUniformLoad:
		return w.writeWorkGroupUniformLoad(k)

	case ir.StmtRayQuery:
		return w.writeRayQuery(k)

	default:
		return fmt.Errorf("unsupported statement kind: %T", kind)
	}
}

// writeEmit writes an emit statement (materializes expressions).
func (w *Writer) writeEmit(emit ir.StmtEmit) error {
	// Emit statements mark when expressions should be evaluated.
	// We may need to assign some expressions to temporary variables.
	for handle := emit.Range.Start; handle < emit.Range.End; handle++ {
		if _, needsBake := w.needBakeExpression[handle]; needsBake {
			// Create a temporary variable for this expression
			if err := w.bakeExpression(handle); err != nil {
				return err
			}
		}
	}
	return nil
}

// bakeExpression creates a temporary variable for an expression.
func (w *Writer) bakeExpression(handle ir.ExpressionHandle) error {
	// Determine expression type
	typeName := "auto"
	if typeHandle := w.getExpressionTypeHandle(handle); typeHandle != nil {
		typeName = w.writeTypeName(*typeHandle, StorageAccess(0))
	}

	// Generate temporary name
	tempName := fmt.Sprintf("_e%d", handle)
	w.namedExpressions[handle] = tempName

	// Write the declaration
	w.writeIndent()
	w.write("%s %s = ", typeName, tempName)
	if err := w.writeExpressionInline(handle); err != nil {
		return err
	}
	w.write(";\n")
	return nil
}

// writeIf writes an if statement.
func (w *Writer) writeIf(ifStmt ir.StmtIf) error {
	w.writeIndent()
	w.write("if (")
	if err := w.writeExpression(ifStmt.Condition); err != nil {
		return err
	}
	w.write(") {\n")
	w.pushIndent()
	if err := w.writeBlock(ifStmt.Accept); err != nil {
		return err
	}
	w.popIndent()

	if len(ifStmt.Reject) > 0 {
		w.writeLine("} else {")
		w.pushIndent()
		if err := w.writeBlock(ifStmt.Reject); err != nil {
			return err
		}
		w.popIndent()
	}

	w.writeLine("}")
	return nil
}

// writeSwitch writes a switch statement.
func (w *Writer) writeSwitch(switchStmt ir.StmtSwitch) error {
	w.writeIndent()
	w.write("switch (")
	if err := w.writeExpression(switchStmt.Selector); err != nil {
		return err
	}
	w.write(") {\n")
	w.pushIndent()

	for _, switchCase := range switchStmt.Cases {
		// Write case label
		switch v := switchCase.Value.(type) {
		case ir.SwitchValueI32:
			w.writeLine("case %d:", int32(v))
		case ir.SwitchValueU32:
			w.writeLine("case %du:", uint32(v))
		case ir.SwitchValueDefault:
			w.writeLine("default:")
		}

		w.pushIndent()
		if err := w.writeBlock(switchCase.Body); err != nil {
			return err
		}

		// Add break unless fallthrough
		if !switchCase.FallThrough {
			w.writeLine("break;")
		}
		w.popIndent()
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// writeLoop writes a loop statement.
func (w *Writer) writeLoop(loop ir.StmtLoop) error {
	// In MSL/C++, we use while(true) with manual control
	w.writeLine("while (true) {")
	w.pushIndent()

	// Write body
	if err := w.writeBlock(loop.Body); err != nil {
		return err
	}

	// Write continuing block if present
	if len(loop.Continuing) > 0 {
		if err := w.writeBlock(loop.Continuing); err != nil {
			return err
		}
	}

	// Write break-if condition if present
	if loop.BreakIf != nil {
		w.writeIndent()
		w.write("if (")
		if err := w.writeExpression(*loop.BreakIf); err != nil {
			return err
		}
		w.write(") { break; }\n")
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

func (w *Writer) writeEntryPointOutputReturn(value ir.ExpressionHandle) (bool, error) {
	if !w.entryPointOutputTypeActive {
		return false, nil
	}
	typeHandle := w.entryPointOutputType
	if int(typeHandle) >= len(w.module.Types) {
		return false, nil
	}

	st, ok := w.module.Types[typeHandle].Inner.(ir.StructType)
	if !ok {
		// Simple type with output struct (e.g., float4 with [[position]])
		// The output struct has a single "member" field
		w.writeIndent()
		w.write("%s.member = ", w.entryPointOutputVar)
		if err := w.writeExpression(value); err != nil {
			return false, err
		}
		w.write(";\n")
		w.writeLine("return %s;", w.entryPointOutputVar)
		return true, nil
	}

	tempName := fmt.Sprintf("_ret_%d", value)
	w.writeIndent()
	w.write("auto %s = ", tempName)
	if err := w.writeExpression(value); err != nil {
		return false, err
	}
	w.write(";\n")

	for memberIdx := range st.Members {
		memberName := st.Members[memberIdx].Name
		if memberName == "" {
			memberName = fmt.Sprintf("member_%d", memberIdx)
		}
		memberName = escapeName(memberName)
		w.writeLine("%s.%s = %s.%s;", w.entryPointOutputVar, memberName, tempName, memberName)
	}
	w.writeLine("return %s;", w.entryPointOutputVar)
	return true, nil
}

// writeReturn writes a return statement.
func (w *Writer) writeReturn(ret ir.StmtReturn) error {
	if ret.Value == nil {
		w.writeLine("return;")
		return nil
	}

	handled, err := w.writeEntryPointOutputReturn(*ret.Value)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	w.writeIndent()
	w.write("return ")
	if err := w.writeExpression(*ret.Value); err != nil {
		return err
	}
	w.write(";\n")
	return nil
}

// writeBarrier writes a barrier statement.
func (w *Writer) writeBarrier(barrier ir.StmtBarrier) error {
	// Metal uses different barrier functions based on the memory being synchronized
	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		w.writeLine("threadgroup_barrier(mem_flags::mem_threadgroup);")
	}
	if barrier.Flags&ir.BarrierStorage != 0 {
		w.writeLine("threadgroup_barrier(mem_flags::mem_device);")
	}
	if barrier.Flags&ir.BarrierTexture != 0 {
		w.writeLine("threadgroup_barrier(mem_flags::mem_texture);")
	}
	if barrier.Flags == 0 {
		// Pure execution barrier
		w.writeLine("threadgroup_barrier(mem_flags::mem_none);")
	}
	return nil
}

// writeStore writes a store statement.
func (w *Writer) writeStore(store ir.StmtStore) error {
	w.writeIndent()
	if w.shouldDerefPointer(store.Pointer) {
		w.write("*")
	}
	if err := w.writeExpression(store.Pointer); err != nil {
		return err
	}
	w.write(" = ")
	if err := w.writeExpression(store.Value); err != nil {
		return err
	}
	w.write(";\n")
	return nil
}

// writeImageStore writes an image store statement.
func (w *Writer) writeImageStore(imgStore ir.StmtImageStore) error {
	w.writeIndent()
	if err := w.writeExpression(imgStore.Image); err != nil {
		return err
	}
	w.write(".write(")

	// Value
	if err := w.writeExpression(imgStore.Value); err != nil {
		return err
	}

	// Coordinate
	w.write(", uint2(")
	if err := w.writeExpression(imgStore.Coordinate); err != nil {
		return err
	}
	w.write(")")

	// Array index
	if imgStore.ArrayIndex != nil {
		w.write(", ")
		if err := w.writeExpression(*imgStore.ArrayIndex); err != nil {
			return err
		}
	}

	w.write(");\n")
	return nil
}

// writeAtomic writes an atomic operation statement.
func (w *Writer) writeAtomic(atomic ir.StmtAtomic) error {
	// Determine the function based on atomic operation type
	var funcName string
	switch f := atomic.Fun.(type) {
	case ir.AtomicAdd:
		funcName = "atomic_fetch_add_explicit"
	case ir.AtomicSubtract:
		funcName = "atomic_fetch_sub_explicit"
	case ir.AtomicAnd:
		funcName = "atomic_fetch_and_explicit"
	case ir.AtomicExclusiveOr:
		funcName = "atomic_fetch_xor_explicit"
	case ir.AtomicInclusiveOr:
		funcName = "atomic_fetch_or_explicit"
	case ir.AtomicMin:
		funcName = "atomic_fetch_min_explicit"
	case ir.AtomicMax:
		funcName = "atomic_fetch_max_explicit"
	case ir.AtomicExchange:
		if f.Compare != nil {
			// Compare-and-exchange
			return w.writeAtomicCompareExchange(atomic, f)
		}
		funcName = "atomic_exchange_explicit"
	default:
		return fmt.Errorf("unsupported atomic function: %T", atomic.Fun)
	}

	w.writeIndent()

	// If there's a result, assign it
	if atomic.Result != nil {
		tempName := fmt.Sprintf("_ae%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.write("auto %s = ", tempName)
	}

	w.write("%s(", funcName)
	if err := w.writeExpression(atomic.Pointer); err != nil {
		return err
	}
	w.write(", ")
	if err := w.writeExpression(atomic.Value); err != nil {
		return err
	}
	w.write(", memory_order_relaxed);\n")
	return nil
}

// writeAtomicCompareExchange writes an atomic compare-exchange operation.
func (w *Writer) writeAtomicCompareExchange(atomic ir.StmtAtomic, exchange ir.AtomicExchange) error {
	w.writeIndent()

	// Create expected variable
	w.write("{\n")
	w.pushIndent()
	w.writeIndent()
	w.write("auto _expected = ")
	if err := w.writeExpression(*exchange.Compare); err != nil {
		return err
	}
	w.write(";\n")

	w.writeIndent()
	w.write("auto _result = atomic_compare_exchange_weak_explicit(")
	if err := w.writeExpression(atomic.Pointer); err != nil {
		return err
	}
	w.write(", &_expected, ")
	if err := w.writeExpression(atomic.Value); err != nil {
		return err
	}
	w.write(", memory_order_relaxed, memory_order_relaxed);\n")

	// Assign result if needed
	if atomic.Result != nil {
		tempName := fmt.Sprintf("_ae%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.writeLine("auto %s = _expected;", tempName)
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// writeCall writes a function call statement.
func (w *Writer) writeCall(call ir.StmtCall) error {
	w.writeIndent()

	// Assign result if needed
	if call.Result != nil {
		typeName := "auto"
		if typeHandle := w.getExpressionTypeHandle(*call.Result); typeHandle != nil {
			typeName = w.writeTypeName(*typeHandle, StorageAccess(0))
		}
		tempName := fmt.Sprintf("_fc%d", *call.Result)
		w.namedExpressions[*call.Result] = tempName
		w.write("%s %s = ", typeName, tempName)
	}

	// Function name
	funcName := w.getName(nameKey{kind: nameKeyFunction, handle1: uint32(call.Function)})
	w.write("%s(", funcName)

	// Arguments
	for i, arg := range call.Arguments {
		if i > 0 {
			w.write(", ")
		}
		if err := w.writeExpression(arg); err != nil {
			return err
		}
	}

	w.write(");\n")
	return nil
}

// writeWorkGroupUniformLoad writes a workgroup uniform load.
func (w *Writer) writeWorkGroupUniformLoad(load ir.StmtWorkGroupUniformLoad) error {
	// First barrier
	w.writeLine("threadgroup_barrier(mem_flags::mem_threadgroup);")

	// Create result variable
	tempName := fmt.Sprintf("_wul%d", load.Result)
	w.namedExpressions[load.Result] = tempName

	w.writeIndent()
	w.write("auto %s = *", tempName)
	if err := w.writeExpression(load.Pointer); err != nil {
		return err
	}
	w.write(";\n")

	// Second barrier
	w.writeLine("threadgroup_barrier(mem_flags::mem_threadgroup);")
	return nil
}

// writeRayQuery writes a ray query statement.
func (w *Writer) writeRayQuery(rayQuery ir.StmtRayQuery) error {
	switch f := rayQuery.Fun.(type) {
	case ir.RayQueryInitialize:
		w.writeIndent()
		if err := w.writeExpression(rayQuery.Query); err != nil {
			return err
		}
		w.write(".reset(")
		if err := w.writeExpression(f.AccelerationStructure); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(f.Descriptor); err != nil {
			return err
		}
		w.write(");\n")

	case ir.RayQueryProceed:
		tempName := fmt.Sprintf("_rqp%d", f.Result)
		w.namedExpressions[f.Result] = tempName
		w.writeIndent()
		w.write("bool %s = ", tempName)
		if err := w.writeExpression(rayQuery.Query); err != nil {
			return err
		}
		w.write(".next();\n")

	case ir.RayQueryTerminate:
		w.writeIndent()
		if err := w.writeExpression(rayQuery.Query); err != nil {
			return err
		}
		w.write(".abort();\n")

	default:
		return fmt.Errorf("unsupported ray query function: %T", rayQuery.Fun)
	}
	return nil
}
