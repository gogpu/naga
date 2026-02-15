// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

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
		w.writeLine("discard;")
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
	// For expressions that need to be baked to temporaries.
	for handle := emit.Range.Start; handle < emit.Range.End; handle++ {
		if _, needsBake := w.needBakeExpression[handle]; needsBake {
			if err := w.bakeExpression(handle); err != nil {
				return err
			}
		}
	}
	return nil
}

// bakeExpression creates a temporary variable for an expression.
func (w *Writer) bakeExpression(handle ir.ExpressionHandle) error {
	// Determine expression type using ExpressionTypes
	typeName := "auto"
	if w.currentFunction != nil && int(handle) < len(w.currentFunction.ExpressionTypes) {
		resolution := &w.currentFunction.ExpressionTypes[handle]
		if resolution.Handle != nil {
			typeName = w.getTypeName(*resolution.Handle)
		}
	}

	// Generate temporary name
	tempName := fmt.Sprintf("_e%d", handle)
	w.namedExpressions[handle] = tempName

	// Write the declaration
	exprStr, err := w.writeExpression(handle)
	if err != nil {
		return err
	}
	w.writeLine("%s %s = %s;", typeName, tempName, exprStr)
	return nil
}

// writeIf writes an if statement.
func (w *Writer) writeIf(ifStmt ir.StmtIf) error {
	condition, err := w.writeExpression(ifStmt.Condition)
	if err != nil {
		return err
	}

	w.writeLine("if (%s) {", condition)
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
	selector, err := w.writeExpression(switchStmt.Selector)
	if err != nil {
		return err
	}

	w.writeLine("switch (%s) {", selector)
	w.pushIndent()

	for _, switchCase := range switchStmt.Cases {
		// Write case label based on value type
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
	// GLSL uses for(;;) for infinite loop with manual control
	w.writeLine("for (;;) {")
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
		condition, err := w.writeExpression(*loop.BreakIf)
		if err != nil {
			return err
		}
		w.writeLine("if (%s) { break; }", condition)
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// writeReturn writes a return statement.
// In entry points, return values are assigned to output variables instead.
func (w *Writer) writeReturn(ret ir.StmtReturn) error {
	if ret.Value == nil {
		w.writeLine("return;")
		return nil
	}

	// In entry points, assign to output variables instead of returning.
	if w.inEntryPoint && w.entryPointResult != nil {
		// Case 1: Direct binding on result (scalar/vector output)
		if w.entryPointResult.Binding != nil {
			return w.writeDirectReturn(ret)
		}
		// Case 2: Struct output — expand into individual assignments
		if w.epStructOutput != nil {
			return w.writeStructReturn(ret, w.epStructOutput)
		}
	}

	value, err := w.writeExpression(*ret.Value)
	if err != nil {
		return err
	}
	w.writeLine("return %s;", value)
	return nil
}

// writeDirectReturn handles return statements where the result has a direct binding.
func (w *Writer) writeDirectReturn(ret ir.StmtReturn) error {
	value, err := w.writeExpression(*ret.Value)
	if err != nil {
		return err
	}
	switch b := (*w.entryPointResult.Binding).(type) {
	case ir.BuiltinBinding:
		outputName := glslBuiltIn(b.Builtin, true)
		w.writeLine("%s = %s;", outputName, value)
	case ir.LocationBinding:
		w.writeLine("fragColor = %s;", value)
	default:
		w.writeLine("return %s;", value)
	}
	return nil
}

// writeStructReturn expands a struct return value into individual output assignments.
// The return value can be:
// - An ExprCompose (constructing the struct from individual values)
// - A local variable or other expression referencing a struct
//
//nolint:nestif // Struct return expansion requires nested expression checks
func (w *Writer) writeStructReturn(ret ir.StmtReturn, info *epStructInfo) error {
	// Check if the return value is a Compose expression — we can extract components directly
	if w.currentFunction != nil && int(*ret.Value) < len(w.currentFunction.Expressions) {
		expr := &w.currentFunction.Expressions[*ret.Value]
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			// Each component of the compose maps to a struct member
			for memberIdx, memberInfo := range info.members {
				if memberIdx >= len(compose.Components) {
					break
				}
				compStr, err := w.writeExpression(compose.Components[memberIdx])
				if err != nil {
					return err
				}
				w.writeLine("%s = %s;", memberInfo.glslName, compStr)
			}
			return nil
		}
	}

	// General case: the return value is an expression that evaluates to the struct.
	// We need to evaluate it once, then assign each member.
	value, err := w.writeExpression(*ret.Value)
	if err != nil {
		return err
	}

	// Resolve the struct type to get member names
	if int(info.structType) < len(w.module.Types) {
		if st, ok := w.module.Types[info.structType].Inner.(ir.StructType); ok {
			for memberIdx, memberInfo := range info.members {
				if memberIdx >= len(st.Members) {
					break
				}
				memberName := escapeKeyword(st.Members[memberIdx].Name)
				w.writeLine("%s = %s.%s;", memberInfo.glslName, value, memberName)
			}
			return nil
		}
	}

	// Fallback: cannot expand, write as-is
	w.writeLine("return %s;", value)
	return nil
}

// writeBarrier writes a barrier statement.
func (w *Writer) writeBarrier(barrier ir.StmtBarrier) error {
	if !w.options.LangVersion.SupportsCompute() {
		return fmt.Errorf("barriers require GLSL 4.30+ or ES 3.10+")
	}

	// GLSL barrier functions based on the memory being synchronized
	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		w.writeLine("barrier();")
	}
	if barrier.Flags&ir.BarrierStorage != 0 {
		w.writeLine("memoryBarrierBuffer();")
	}
	if barrier.Flags&ir.BarrierTexture != 0 {
		w.writeLine("memoryBarrierImage();")
	}
	if barrier.Flags == 0 {
		// Pure execution barrier
		w.writeLine("barrier();")
	}
	return nil
}

// writeStore writes a store statement.
func (w *Writer) writeStore(store ir.StmtStore) error {
	pointer, err := w.writeExpression(store.Pointer)
	if err != nil {
		return err
	}
	value, err := w.writeExpression(store.Value)
	if err != nil {
		return err
	}
	// In GLSL, no explicit dereference needed for most cases
	w.writeLine("%s = %s;", pointer, value)
	return nil
}

// writeImageStore writes an image store statement.
func (w *Writer) writeImageStore(imgStore ir.StmtImageStore) error {
	image, err := w.writeExpression(imgStore.Image)
	if err != nil {
		return err
	}
	coordinate, err := w.writeExpression(imgStore.Coordinate)
	if err != nil {
		return err
	}
	value, err := w.writeExpression(imgStore.Value)
	if err != nil {
		return err
	}

	if imgStore.ArrayIndex != nil {
		arrayIdx, err := w.writeExpression(*imgStore.ArrayIndex)
		if err != nil {
			return err
		}
		w.writeLine("imageStore(%s, ivec3(%s, %s), %s);", image, coordinate, arrayIdx, value)
	} else {
		w.writeLine("imageStore(%s, %s, %s);", image, coordinate, value)
	}
	return nil
}

// writeAtomic writes an atomic operation statement.
func (w *Writer) writeAtomic(atomic ir.StmtAtomic) error {
	if !w.options.LangVersion.SupportsCompute() {
		return fmt.Errorf("atomic operations require GLSL 4.30+ or ES 3.10+")
	}

	pointer, err := w.writeExpression(atomic.Pointer)
	if err != nil {
		return err
	}
	value, err := w.writeExpression(atomic.Value)
	if err != nil {
		return err
	}

	// Determine the function based on atomic operation type
	var funcName string
	switch f := atomic.Fun.(type) {
	case ir.AtomicAdd:
		funcName = "atomicAdd"
	case ir.AtomicSubtract:
		// GLSL doesn't have atomicSub, use atomicAdd with negated value
		funcName = "atomicAdd"
		value = fmt.Sprintf("-(%s)", value)
	case ir.AtomicAnd:
		funcName = "atomicAnd"
	case ir.AtomicExclusiveOr:
		funcName = "atomicXor"
	case ir.AtomicInclusiveOr:
		funcName = "atomicOr"
	case ir.AtomicMin:
		funcName = "atomicMin"
	case ir.AtomicMax:
		funcName = "atomicMax"
	case ir.AtomicExchange:
		if f.Compare != nil {
			// Compare-and-exchange
			return w.writeAtomicCompareExchange(atomic, f)
		}
		funcName = "atomicExchange"
	default:
		return fmt.Errorf("unsupported atomic function: %T", atomic.Fun)
	}

	// If there's a result, assign it
	if atomic.Result != nil {
		tempName := fmt.Sprintf("_ae%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.writeLine("int %s = %s(%s, %s);", tempName, funcName, pointer, value)
	} else {
		w.writeLine("%s(%s, %s);", funcName, pointer, value)
	}
	return nil
}

// writeAtomicCompareExchange writes an atomic compare-exchange operation.
func (w *Writer) writeAtomicCompareExchange(atomic ir.StmtAtomic, exchange ir.AtomicExchange) error {
	pointer, err := w.writeExpression(atomic.Pointer)
	if err != nil {
		return err
	}
	compareVal, err := w.writeExpression(*exchange.Compare)
	if err != nil {
		return err
	}
	exchangeVal, err := w.writeExpression(atomic.Value)
	if err != nil {
		return err
	}

	if atomic.Result != nil {
		tempName := fmt.Sprintf("_ae%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.writeLine("int %s = atomicCompSwap(%s, %s, %s);", tempName, pointer, compareVal, exchangeVal)
	} else {
		w.writeLine("atomicCompSwap(%s, %s, %s);", pointer, compareVal, exchangeVal)
	}
	return nil
}

// writeCall writes a function call statement.
//
//nolint:nestif // Result type lookup requires nested checks
func (w *Writer) writeCall(call ir.StmtCall) error {
	// Get function name
	funcName := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(call.Function)}]

	// Write arguments
	argStrs := make([]string, 0, len(call.Arguments))
	for _, arg := range call.Arguments {
		argStr, err := w.writeExpression(arg)
		if err != nil {
			return err
		}
		argStrs = append(argStrs, argStr)
	}

	// Build call expression
	callExpr := fmt.Sprintf("%s(%s)", funcName, joinStrings(argStrs, ", "))

	// Assign result if needed
	if call.Result != nil {
		tempName := fmt.Sprintf("_fc%d", *call.Result)
		w.namedExpressions[*call.Result] = tempName
		// Determine type from ExpressionTypes
		typeName := "/* type */ "
		if w.currentFunction != nil && int(*call.Result) < len(w.currentFunction.ExpressionTypes) {
			resolution := &w.currentFunction.ExpressionTypes[*call.Result]
			if resolution.Handle != nil {
				typeName = w.getTypeName(*resolution.Handle) + " "
			}
		}
		w.writeLine("%s%s = %s;", typeName, tempName, callExpr)
	} else {
		w.writeLine("%s;", callExpr)
	}
	return nil
}

// writeWorkGroupUniformLoad writes a workgroup uniform load.
func (w *Writer) writeWorkGroupUniformLoad(load ir.StmtWorkGroupUniformLoad) error {
	// First barrier to ensure all writes are visible
	w.writeLine("barrier();")
	w.writeLine("memoryBarrierShared();")

	// Create result variable
	tempName := fmt.Sprintf("_wul%d", load.Result)
	w.namedExpressions[load.Result] = tempName

	pointer, err := w.writeExpression(load.Pointer)
	if err != nil {
		return err
	}
	// In GLSL, shared variables don't need dereferencing
	w.writeLine("/* workgroup uniform load */ auto %s = %s;", tempName, pointer)

	// Second barrier
	w.writeLine("barrier();")
	w.writeLine("memoryBarrierShared();")
	return nil
}

// writeRayQuery writes a ray query statement.
func (w *Writer) writeRayQuery(_ ir.StmtRayQuery) error {
	// Ray query requires extensions not commonly available in base GLSL
	// Would need GL_EXT_ray_query extension
	return fmt.Errorf("ray query statements not supported in GLSL (requires GL_EXT_ray_query)")
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
