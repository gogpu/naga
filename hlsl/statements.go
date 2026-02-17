// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl implements HLSL statement generation for all IR statement types.
// Statement functions are called via writeBlock from function body and entry point writers.
package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Block and Statement Dispatch
// =============================================================================

// writeBlock writes a block of statements.
func (w *Writer) writeBlock(block ir.Block) error {
	for i := range block {
		stmt := &block[i]
		if err := w.writeStatement(stmt.Kind); err != nil {
			return err
		}
	}
	return nil
}

// writeStatement dispatches to the appropriate statement writer.
func (w *Writer) writeStatement(kind ir.StatementKind) error {
	switch s := kind.(type) {
	case ir.StmtEmit:
		return w.writeEmitStatement(s)
	case ir.StmtBlock:
		return w.writeBlockStatement(s)
	case ir.StmtIf:
		return w.writeIfStatement(s)
	case ir.StmtSwitch:
		return w.writeSwitchStatement(s)
	case ir.StmtLoop:
		return w.writeLoopStatement(s)
	case ir.StmtBreak:
		return w.writeBreakStatement()
	case ir.StmtContinue:
		return w.writeContinueStatement()
	case ir.StmtReturn:
		return w.writeReturnStatement(s)
	case ir.StmtKill:
		return w.writeKillStatement()
	case ir.StmtBarrier:
		return w.writeBarrierStatement(s)
	case ir.StmtStore:
		return w.writeStoreStatement(s)
	case ir.StmtImageStore:
		return w.writeImageStoreStatement(s)
	case ir.StmtAtomic:
		return w.writeAtomicStatement(s)
	case ir.StmtWorkGroupUniformLoad:
		return w.writeWorkGroupUniformLoadStatement(s)
	case ir.StmtCall:
		return w.writeCallStatement(s)
	case ir.StmtRayQuery:
		return w.writeRayQueryStatement(s)
	default:
		return fmt.Errorf("unsupported statement type: %T", kind)
	}
}

// =============================================================================
// Emit and Block Statements
// =============================================================================

// writeEmitStatement writes emit statements.
// Emit statements mark which expressions should be evaluated and their results
// stored in temporaries. In HLSL, we write these as variable declarations.
func (w *Writer) writeEmitStatement(s ir.StmtEmit) error {
	for handle := s.Range.Start; handle < s.Range.End; handle++ {
		if err := w.writeEmittedExpression(handle); err != nil {
			return err
		}
	}
	return nil
}

// writeEmittedExpression writes an emitted expression as a variable declaration.
func (w *Writer) writeEmittedExpression(handle ir.ExpressionHandle) error {
	// Check if this expression has a name already
	if name, ok := w.namedExpressions[handle]; ok {
		// Already named, just reference it
		_ = name
		return nil
	}

	// Get expression type
	exprType := w.getExpressionType(handle)
	if exprType == nil {
		return nil // Skip if type is not available
	}

	// Generate a unique name for this expression.
	// The name is added to namedExpressions AFTER writing the initializer,
	// so that writeExpression expands the actual expression rather than
	// outputting the name itself (which would produce "float _e0 = _e0;").
	name := fmt.Sprintf("_e%d", handle)

	// Write variable declaration with initialization
	// HLSL arrays: type name[size], not type[size] name
	typeName, arraySuffix := w.typeToHLSLWithArraySuffix(exprType)
	w.writeIndent()
	fmt.Fprintf(&w.out, "%s %s%s = ", typeName, name, arraySuffix)
	if err := w.writeExpression(handle); err != nil {
		return err
	}
	w.out.WriteString(";\n")

	// Cache the name for subsequent references to this expression
	w.namedExpressions[handle] = name

	return nil
}

// writeBlockStatement writes a nested block.
func (w *Writer) writeBlockStatement(s ir.StmtBlock) error {
	w.writeLine("{")
	w.pushIndent()
	if err := w.writeBlock(s.Block); err != nil {
		w.popIndent()
		return err
	}
	w.popIndent()
	w.writeLine("}")
	return nil
}

// =============================================================================
// Control Flow Statements
// =============================================================================

// writeIfStatement writes an if/else statement.
func (w *Writer) writeIfStatement(s ir.StmtIf) error {
	w.writeIndent()
	w.out.WriteString("if (")
	if err := w.writeExpression(s.Condition); err != nil {
		return fmt.Errorf("if condition: %w", err)
	}
	w.out.WriteString(") {\n")

	w.pushIndent()
	if err := w.writeBlock(s.Accept); err != nil {
		w.popIndent()
		return fmt.Errorf("if accept block: %w", err)
	}
	w.popIndent()

	if len(s.Reject) > 0 {
		w.writeLine("} else {")
		w.pushIndent()
		if err := w.writeBlock(s.Reject); err != nil {
			w.popIndent()
			return fmt.Errorf("if reject block: %w", err)
		}
		w.popIndent()
	}

	w.writeLine("}")
	return nil
}

// writeSwitchStatement writes a switch statement.
func (w *Writer) writeSwitchStatement(s ir.StmtSwitch) error {
	w.writeIndent()
	w.out.WriteString("switch (")
	if err := w.writeExpression(s.Selector); err != nil {
		return fmt.Errorf("switch selector: %w", err)
	}
	w.out.WriteString(") {\n")

	for i := range s.Cases {
		c := &s.Cases[i]
		if err := w.writeSwitchCase(c); err != nil {
			return fmt.Errorf("switch case %d: %w", i, err)
		}
	}

	w.writeLine("}")
	return nil
}

// writeSwitchCase writes a single switch case.
func (w *Writer) writeSwitchCase(c *ir.SwitchCase) error {
	// Write case label
	switch v := c.Value.(type) {
	case ir.SwitchValueI32:
		w.writeLine("case %d:", int32(v))
	case ir.SwitchValueU32:
		w.writeLine("case %du:", uint32(v))
	case ir.SwitchValueDefault:
		w.writeLine("default:")
	default:
		return fmt.Errorf("unsupported switch value type: %T", c.Value)
	}

	// Write case body
	w.pushIndent()
	if err := w.writeBlock(c.Body); err != nil {
		w.popIndent()
		return err
	}

	// Write break unless fallthrough
	if !c.FallThrough {
		w.writeLine("break;")
	}
	w.popIndent()

	return nil
}

// writeLoopStatement writes a loop statement.
// Naga loops are while(true) with explicit break conditions.
func (w *Writer) writeLoopStatement(s ir.StmtLoop) error {
	// Add [loop] attribute to prevent unrolling
	w.writeLine("[loop]")
	w.writeLine("while (true) {")
	w.pushIndent()

	// Write main body
	if err := w.writeBlock(s.Body); err != nil {
		w.popIndent()
		return fmt.Errorf("loop body: %w", err)
	}

	// Write continuing block if present
	if len(s.Continuing) > 0 {
		w.writeLine("// continuing")
		if err := w.writeBlock(s.Continuing); err != nil {
			w.popIndent()
			return fmt.Errorf("loop continuing: %w", err)
		}
	}

	// Write break-if condition after continuing block
	if s.BreakIf != nil {
		w.writeIndent()
		w.out.WriteString("if (")
		if err := w.writeExpression(*s.BreakIf); err != nil {
			w.popIndent()
			return fmt.Errorf("loop break-if: %w", err)
		}
		w.out.WriteString(") break;\n")
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// writeBreakStatement writes a break statement.
func (w *Writer) writeBreakStatement() error {
	w.writeLine("break;")
	return nil
}

// writeContinueStatement writes a continue statement.
func (w *Writer) writeContinueStatement() error {
	w.writeLine("continue;")
	return nil
}

// =============================================================================
// Return and Kill Statements
// =============================================================================

// writeReturnStatement writes a return statement.
func (w *Writer) writeReturnStatement(s ir.StmtReturn) error {
	if s.Value != nil {
		w.writeIndent()
		w.out.WriteString("return ")
		if err := w.writeExpression(*s.Value); err != nil {
			return fmt.Errorf("return value: %w", err)
		}
		w.out.WriteString(";\n")
	} else {
		w.writeLine("return;")
	}
	return nil
}

// writeKillStatement writes a discard/kill statement.
func (w *Writer) writeKillStatement() error {
	w.writeLine("discard;")
	return nil
}

// =============================================================================
// Memory Barrier Statements
// =============================================================================

// writeBarrierStatement writes a memory barrier statement.
func (w *Writer) writeBarrierStatement(s ir.StmtBarrier) error {
	// Determine which barriers are needed based on flags
	hasWorkgroup := s.Flags&ir.BarrierWorkGroup != 0
	hasStorage := s.Flags&ir.BarrierStorage != 0
	hasTexture := s.Flags&ir.BarrierTexture != 0
	hasSubgroup := s.Flags&ir.BarrierSubGroup != 0

	// Handle subgroup barriers separately (SM 6.0+)
	if hasSubgroup {
		w.writeLine("// Subgroup barrier not directly supported in HLSL")
		w.writeLine("GroupMemoryBarrierWithGroupSync();")
		return nil
	}

	// Select the appropriate barrier function
	switch {
	case hasWorkgroup && hasStorage:
		// Both workgroup and storage memory
		w.writeLine("AllMemoryBarrierWithGroupSync();")
	case hasWorkgroup:
		// Only workgroup (shared) memory
		w.writeLine("GroupMemoryBarrierWithGroupSync();")
	case hasStorage || hasTexture:
		// Only device (global) memory or textures
		w.writeLine("DeviceMemoryBarrierWithGroupSync();")
	default:
		// Execution barrier only
		w.writeLine("GroupMemoryBarrierWithGroupSync();")
	}

	return nil
}

// =============================================================================
// Store Statements
// =============================================================================

// writeStoreStatement writes a store statement.
func (w *Writer) writeStoreStatement(s ir.StmtStore) error {
	w.writeIndent()
	if err := w.writeExpression(s.Pointer); err != nil {
		return fmt.Errorf("store pointer: %w", err)
	}
	w.out.WriteString(" = ")
	if err := w.writeExpression(s.Value); err != nil {
		return fmt.Errorf("store value: %w", err)
	}
	w.out.WriteString(";\n")
	return nil
}

// writeImageStoreStatement writes an image store statement.
func (w *Writer) writeImageStoreStatement(s ir.StmtImageStore) error {
	w.writeIndent()

	// Write image reference with bracket access
	if err := w.writeExpression(s.Image); err != nil {
		return fmt.Errorf("image store: image: %w", err)
	}

	w.out.WriteByte('[')

	// Write coordinate
	if err := w.writeExpression(s.Coordinate); err != nil {
		return fmt.Errorf("image store: coordinate: %w", err)
	}

	// Handle array index if present
	if s.ArrayIndex != nil {
		// For arrayed textures, coordinate includes array index
		// This is simplified - full implementation would compose int3/int4
		w.out.WriteString(", ")
		if err := w.writeExpression(*s.ArrayIndex); err != nil {
			return fmt.Errorf("image store: array index: %w", err)
		}
	}

	w.out.WriteString("] = ")

	// Write value
	if err := w.writeExpression(s.Value); err != nil {
		return fmt.Errorf("image store: value: %w", err)
	}
	w.out.WriteString(";\n")

	return nil
}

// =============================================================================
// Atomic Statements
// =============================================================================

// writeAtomicStatement writes an atomic operation statement.
func (w *Writer) writeAtomicStatement(s ir.StmtAtomic) error {
	w.writeIndent()

	// Get atomic function name
	funcName := getAtomicHLSLFunction(s.Fun)

	// Write function call
	w.out.WriteString(funcName)
	w.out.WriteByte('(')

	// Pointer argument
	if err := w.writeExpression(s.Pointer); err != nil {
		return fmt.Errorf("atomic pointer: %w", err)
	}

	// Handle compare-exchange specially
	if cmpXchg, ok := s.Fun.(ir.AtomicExchange); ok && cmpXchg.Compare != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*cmpXchg.Compare); err != nil {
			return fmt.Errorf("atomic compare: %w", err)
		}
	}

	// Value argument
	w.out.WriteString(", ")
	if err := w.writeExpression(s.Value); err != nil {
		return fmt.Errorf("atomic value: %w", err)
	}

	// Result argument (for operations that return the original value)
	// All Interlocked functions support optional result output parameter
	if s.Result != nil {
		w.out.WriteString(", _atomic_result")
	}

	w.out.WriteString(");\n")
	return nil
}

// getAtomicHLSLFunction returns the HLSL Interlocked function name.
// All Interlocked functions support optional result output parameter.
func getAtomicHLSLFunction(fun ir.AtomicFunction) string {
	switch f := fun.(type) {
	case ir.AtomicAdd:
		return "InterlockedAdd"
	case ir.AtomicSubtract:
		// HLSL doesn't have InterlockedSubtract, use Add with negation
		return "InterlockedAdd" // Value should be negated by caller
	case ir.AtomicAnd:
		return "InterlockedAnd"
	case ir.AtomicExclusiveOr:
		return "InterlockedXor"
	case ir.AtomicInclusiveOr:
		return "InterlockedOr"
	case ir.AtomicMin:
		return "InterlockedMin"
	case ir.AtomicMax:
		return "InterlockedMax"
	case ir.AtomicExchange:
		if f.Compare != nil {
			return "InterlockedCompareExchange"
		}
		return "InterlockedExchange"
	default:
		return "InterlockedExchange"
	}
}

// =============================================================================
// Function Call Statements
// =============================================================================

// writeCallStatement writes a function call statement.
func (w *Writer) writeCallStatement(s ir.StmtCall) error {
	w.writeIndent()

	// Get function name
	funcName := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(s.Function)}]
	if funcName == "" {
		funcName = fmt.Sprintf("function_%d", s.Function)
	}

	// If there's a result, assign it
	if s.Result != nil {
		// Store result in a temporary variable
		fmt.Fprintf(&w.out, "_%s_result = ", funcName)
	}

	// Write function call
	w.out.WriteString(funcName)
	w.out.WriteByte('(')

	for i, arg := range s.Arguments {
		if i > 0 {
			w.out.WriteString(", ")
		}
		if err := w.writeExpression(arg); err != nil {
			return fmt.Errorf("call arg %d: %w", i, err)
		}
	}

	w.out.WriteString(");\n")
	return nil
}

// writeWorkGroupUniformLoadStatement writes a workgroup uniform load statement.
func (w *Writer) writeWorkGroupUniformLoadStatement(s ir.StmtWorkGroupUniformLoad) error {
	// WGSL workgroupUniformLoad is a load with implicit barrier semantics
	// In HLSL, we emit a barrier followed by a load
	w.writeLine("GroupMemoryBarrierWithGroupSync();")

	w.writeIndent()
	w.out.WriteString("_workgroup_uniform_result = ")
	if err := w.writeExpression(s.Pointer); err != nil {
		return fmt.Errorf("workgroup uniform load: %w", err)
	}
	w.out.WriteString(";\n")

	return nil
}

// =============================================================================
// Ray Tracing Statements (Placeholder)
// =============================================================================

// writeRayQueryStatement writes a ray query statement.
func (w *Writer) writeRayQueryStatement(s ir.StmtRayQuery) error {
	switch f := s.Fun.(type) {
	case ir.RayQueryInitialize:
		return w.writeRayQueryInitialize(s.Query, f)
	case ir.RayQueryProceed:
		return w.writeRayQueryProceed(s.Query, f)
	case ir.RayQueryTerminate:
		return w.writeRayQueryTerminate(s.Query)
	default:
		return fmt.Errorf("unsupported ray query function: %T", s.Fun)
	}
}

// writeRayQueryInitialize writes ray query initialization.
func (w *Writer) writeRayQueryInitialize(query ir.ExpressionHandle, f ir.RayQueryInitialize) error {
	w.writeIndent()
	if err := w.writeExpression(query); err != nil {
		return fmt.Errorf("ray query: %w", err)
	}
	w.out.WriteString(".TraceRayInline(")

	// Acceleration structure
	if err := w.writeExpression(f.AccelerationStructure); err != nil {
		return fmt.Errorf("ray query acceleration structure: %w", err)
	}

	w.out.WriteString(", RAY_FLAG_NONE, 0xFF, ")

	// Ray descriptor
	if err := w.writeExpression(f.Descriptor); err != nil {
		return fmt.Errorf("ray query descriptor: %w", err)
	}

	w.out.WriteString(");\n")
	return nil
}

// writeRayQueryProceed writes ray query proceed.
func (w *Writer) writeRayQueryProceed(query ir.ExpressionHandle, f ir.RayQueryProceed) error {
	w.writeIndent()

	// Store result in the expression referenced by f.Result
	// For now, use a placeholder variable name
	_ = f.Result // Result expression handle for the proceed result
	w.out.WriteString("_ray_query_proceed_result = ")

	if err := w.writeExpression(query); err != nil {
		return fmt.Errorf("ray query: %w", err)
	}
	w.out.WriteString(".Proceed();\n")
	return nil
}

// writeRayQueryTerminate writes ray query termination.
func (w *Writer) writeRayQueryTerminate(query ir.ExpressionHandle) error {
	w.writeIndent()
	if err := w.writeExpression(query); err != nil {
		return fmt.Errorf("ray query: %w", err)
	}
	w.out.WriteString(".Abort();\n")
	return nil
}

// =============================================================================
// Function Body Integration
// =============================================================================

// writeFunctionBody writes the complete body of a function.
func (w *Writer) writeFunctionBody(fn *ir.Function) error {
	// Write local variables
	for localIdx, local := range fn.LocalVars {
		localName := w.namer.call(local.Name)
		w.localNames[uint32(localIdx)] = localName
		// HLSL arrays: type name[size], not type[size] name
		localType, arraySuffix := w.getTypeNameWithArraySuffix(local.Type)

		// Initialize with zero if requested or required
		if local.Init != nil {
			w.writeIndent()
			fmt.Fprintf(&w.out, "%s %s%s = ", localType, localName, arraySuffix)
			if err := w.writeExpression(*local.Init); err != nil {
				return fmt.Errorf("local var init: %w", err)
			}
			w.out.WriteString(";\n")
		} else {
			w.writeLine("%s %s%s;", localType, localName, arraySuffix)
		}
	}

	if len(fn.LocalVars) > 0 {
		w.writeLine("")
	}

	// Write function body statements
	return w.writeBlock(fn.Body)
}
