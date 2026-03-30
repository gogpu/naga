// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl implements HLSL statement generation for all IR statement types.
// Statement functions are called via writeBlock from function body and entry point writers.
package hlsl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Expression Baking Analysis (matches Rust naga's update_expressions_to_bake)
// =============================================================================

// updateExpressionsToBake analyzes a function and determines which expressions
// need to be baked (assigned to temporary variables) vs inlined.
// Matches Rust naga's logic based on bake_ref_count and actual reference counts.
func (w *Writer) updateExpressionsToBake(fn *ir.Function) {
	w.needBakeExpressions = make(map[ir.ExpressionHandle]struct{})

	// Count references to each expression
	refCounts := make(map[ir.ExpressionHandle]int)
	countExprRef := func(h ir.ExpressionHandle) {
		refCounts[h]++
	}

	// Count references from expressions
	for i := range fn.Expressions {
		switch e := fn.Expressions[i].Kind.(type) {
		case ir.ExprAccess:
			countExprRef(e.Base)
			countExprRef(e.Index)
		case ir.ExprAccessIndex:
			countExprRef(e.Base)
		case ir.ExprSplat:
			countExprRef(e.Value)
		case ir.ExprSwizzle:
			countExprRef(e.Vector)
		case ir.ExprCompose:
			for _, c := range e.Components {
				countExprRef(c)
			}
		case ir.ExprLoad:
			countExprRef(e.Pointer)
		case ir.ExprUnary:
			countExprRef(e.Expr)
		case ir.ExprBinary:
			countExprRef(e.Left)
			countExprRef(e.Right)
		case ir.ExprSelect:
			countExprRef(e.Condition)
			countExprRef(e.Accept)
			countExprRef(e.Reject)
		case ir.ExprRelational:
			countExprRef(e.Argument)
		case ir.ExprMath:
			countExprRef(e.Arg)
			if e.Arg1 != nil {
				countExprRef(*e.Arg1)
			}
			if e.Arg2 != nil {
				countExprRef(*e.Arg2)
			}
			if e.Arg3 != nil {
				countExprRef(*e.Arg3)
			}
		case ir.ExprAs:
			countExprRef(e.Expr)
		case ir.ExprDerivative:
			countExprRef(e.Expr)
		case ir.ExprImageSample:
			countExprRef(e.Image)
			countExprRef(e.Sampler)
			countExprRef(e.Coordinate)
			if e.ArrayIndex != nil {
				countExprRef(*e.ArrayIndex)
			}
			if e.Offset != nil {
				countExprRef(*e.Offset)
			}
			if e.DepthRef != nil {
				countExprRef(*e.DepthRef)
			}
			// Count level references based on SampleLevel type
			switch lv := e.Level.(type) {
			case ir.SampleLevelExact:
				countExprRef(lv.Level)
			case ir.SampleLevelBias:
				countExprRef(lv.Bias)
			case ir.SampleLevelGradient:
				countExprRef(lv.X)
				countExprRef(lv.Y)
			}
		case ir.ExprImageLoad:
			countExprRef(e.Image)
			countExprRef(e.Coordinate)
			if e.ArrayIndex != nil {
				countExprRef(*e.ArrayIndex)
			}
			if e.Sample != nil {
				countExprRef(*e.Sample)
			}
			if e.Level != nil {
				countExprRef(*e.Level)
			}
		case ir.ExprImageQuery:
			countExprRef(e.Image)
			// Query level is inside the Query interface
			if q, ok := e.Query.(ir.ImageQuerySize); ok {
				if q.Level != nil {
					countExprRef(*q.Level)
				}
			}
		}
	}

	// Count references from statements
	w.countStmtRefs(fn.Body, countExprRef)

	// Determine which expressions need baking based on bake_ref_count
	for i := range fn.Expressions {
		h := ir.ExpressionHandle(i)
		threshold := bakeRefCount(fn.Expressions[i].Kind)
		if threshold <= refCounts[h] {
			w.needBakeExpressions[h] = struct{}{}
		}
	}

	// Additional force-bake rules (matches Rust's update_expressions_to_bake):
	// - Derivative Width+Coarse/Fine: force bake the inner expr
	for i := range fn.Expressions {
		e := fn.Expressions[i].Kind
		if d, ok := e.(ir.ExprDerivative); ok {
			if d.Axis == ir.DerivativeWidth &&
				(d.Control == ir.DerivativeCoarse || d.Control == ir.DerivativeFine) {
				w.needBakeExpressions[d.Expr] = struct{}{}
			}
		}
	}
}

// countStmtRefs counts expression references within statements.
func (w *Writer) countStmtRefs(block ir.Block, count func(ir.ExpressionHandle)) {
	for i := range block {
		switch s := block[i].Kind.(type) {
		case ir.StmtEmit:
			// Emit ranges don't count as references
		case ir.StmtIf:
			count(s.Condition)
			w.countStmtRefs(s.Accept, count)
			w.countStmtRefs(s.Reject, count)
		case ir.StmtSwitch:
			count(s.Selector)
			for _, c := range s.Cases {
				w.countStmtRefs(c.Body, count)
			}
		case ir.StmtLoop:
			w.countStmtRefs(s.Body, count)
			w.countStmtRefs(s.Continuing, count)
			if s.BreakIf != nil {
				count(*s.BreakIf)
			}
		case ir.StmtReturn:
			if s.Value != nil {
				count(*s.Value)
			}
		case ir.StmtStore:
			count(s.Pointer)
			count(s.Value)
		case ir.StmtImageStore:
			count(s.Image)
			count(s.Coordinate)
			count(s.Value)
			if s.ArrayIndex != nil {
				count(*s.ArrayIndex)
			}
		case ir.StmtCall:
			for _, arg := range s.Arguments {
				count(arg)
			}
		case ir.StmtAtomic:
			count(s.Pointer)
			count(s.Value)
			// Count compare expression for CompareExchange (used twice in output:
			// once in the InterlockedCompareExchange call, once in the exchanged check)
			if xchg, ok := s.Fun.(ir.AtomicExchange); ok && xchg.Compare != nil {
				count(*xchg.Compare)
				count(*xchg.Compare) // referenced twice: in the call and in exchanged check
			}
		case ir.StmtBlock:
			w.countStmtRefs(s.Block, count)
		case ir.StmtWorkGroupUniformLoad:
			count(s.Pointer)
		case ir.StmtRayQuery:
			count(s.Query)
		case ir.StmtBarrier:
			// no expression refs
		case ir.StmtBreak, ir.StmtContinue, ir.StmtKill:
			// no expression refs
		}
	}
}

// bakeRefCount returns the minimum reference count needed for baking.
// Matches Rust naga's Expression::bake_ref_count.
func bakeRefCount(kind ir.ExpressionKind) int {
	switch kind.(type) {
	case ir.ExprAccess, ir.ExprAccessIndex:
		return int(^uint(0) >> 1) // MAX: never bake
	case ir.ExprImageSample, ir.ExprImageLoad:
		return 1 // always bake
	case ir.ExprDerivative:
		return 1 // always bake (uses control flow)
	case ir.ExprLoad:
		return 1 // always bake
	default:
		return 2 // bake if used 2+ times
	}
}

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
	case ir.StmtImageAtomic:
		return fmt.Errorf("HLSL backend does not yet support StmtImageAtomic")
	case ir.StmtAtomic:
		return w.writeAtomicStatement(s)
	case ir.StmtWorkGroupUniformLoad:
		return w.writeWorkGroupUniformLoadStatement(s)
	case ir.StmtCall:
		return w.writeCallStatement(s)
	case ir.StmtRayQuery:
		return w.writeRayQueryStatement(s)
	case ir.StmtSubgroupBallot:
		return w.writeSubgroupBallotStatement(s)
	case ir.StmtSubgroupCollectiveOperation:
		return w.writeSubgroupCollectiveOperationStatement(s)
	case ir.StmtSubgroupGather:
		return w.writeSubgroupGatherStatement(s)
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
// Expressions in NamedExpressions (from let bindings and phony assignments)
// are always emitted with their user-given names.
func (w *Writer) writeEmitStatement(s ir.StmtEmit) error {
	for handle := s.Range.Start; handle < s.Range.End; handle++ {
		if err := w.writeEmittedExpression(handle); err != nil {
			return err
		}
	}
	return nil
}

// writeEmittedExpression writes an emitted expression as a variable declaration.
// Matches Rust naga: only bake if it's a named expression or in needBakeExpressions.
// Pointer and variable reference expressions are never baked.
func (w *Writer) writeEmittedExpression(handle ir.ExpressionHandle) error {
	// Check if this expression has a name already (from a previous bake)
	if _, ok := w.namedExpressions[handle]; ok {
		return nil
	}

	// Don't bake pointer expressions (matches Rust naga)
	if w.isPointerExpression(handle) {
		return nil
	}

	// Don't bake variable reference expressions
	if w.isVariableReference(handle) {
		return nil
	}

	// Determine if this expression should be baked:
	// 1. Named expressions (from let bindings / phony) are always baked
	// 2. Expressions in needBakeExpressions are baked
	// 3. Everything else is inlined
	irName := ""
	if w.currentFunction != nil && w.currentFunction.NamedExpressions != nil {
		if name, ok := w.currentFunction.NamedExpressions[handle]; ok {
			irName = name
		}
	}

	_, needsBake := w.needBakeExpressions[handle]
	if irName == "" && !needsBake {
		return nil // inline this expression
	}

	// Get expression type. If not resolved in ExpressionTypes, try resolving
	// from the expression shape (Splat, Compose, etc.) — matches Rust naga which
	// always has types resolved for emitted expressions.
	exprType := w.getExpressionType(handle)
	if exprType == nil {
		exprType = w.resolveExpressionTypeFallback(handle)
	}
	if exprType == nil {
		return nil
	}

	// Determine the variable name
	name := fmt.Sprintf("_e%d", handle)
	if irName != "" {
		name = w.namer.call(irName)
	}

	// Write variable declaration with initialization.
	// Use type handle when available (for proper struct name resolution).
	var typeName, arraySuffix string
	typeHandle := w.getExpressionTypeHandle(handle)
	if typeHandle != nil {
		typeName, arraySuffix = w.getTypeNameWithArraySuffix(*typeHandle)
	} else {
		typeName, arraySuffix = w.typeToHLSLWithArraySuffix(exprType)
	}
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

// isVariableReference returns true if the expression is a variable reference
// (GlobalVariable or LocalVariable) that acts as a pointer in the WGSL Load Rule
// model. These are not baked directly — the ExprLoad wrapping them is baked instead.
func (w *Writer) isVariableReference(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	switch w.currentFunction.Expressions[handle].Kind.(type) {
	case ir.ExprGlobalVariable, ir.ExprLocalVariable:
		return true
	default:
		return false
	}
}

// resolveExpressionTypeFallback resolves the type of an expression from its shape
// when ExpressionTypes is not populated (e.g., Splat type lost during concretization).
// Returns nil if the type cannot be determined.
func (w *Writer) resolveExpressionTypeFallback(handle ir.ExpressionHandle) *ir.Type {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := w.currentFunction.Expressions[handle].Kind
	switch e := expr.(type) {
	case ir.ExprSplat:
		// Splat: derive VectorType from the value's scalar type
		valueType := w.getExpressionType(e.Value)
		if valueType == nil {
			return nil
		}
		var scalar ir.ScalarType
		switch t := valueType.Inner.(type) {
		case ir.ScalarType:
			scalar = t
		default:
			return nil
		}
		return &ir.Type{Inner: ir.VectorType{Size: e.Size, Scalar: scalar}}
	case ir.ExprCompose:
		// Compose: use the declared type
		if int(e.Type) < len(w.module.Types) {
			return &w.module.Types[e.Type]
		}
	case ir.Literal:
		res, err := ir.ResolveLiteralType(e)
		if err == nil && res.Value != nil {
			return &ir.Type{Inner: res.Value}
		}
	}
	return nil
}

// isPointerExpression returns true if the expression has a pointer type.
// Pointer expressions are never baked — they are address-space references.
// Matches Rust naga's ptr_class check in the Emit handler.
func (w *Writer) isPointerExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}

	// Check ExpressionTypes first (covers most cases)
	if int(handle) < len(w.currentFunction.ExpressionTypes) {
		resolution := &w.currentFunction.ExpressionTypes[handle]
		if resolution.Handle != nil && w.module != nil {
			if int(*resolution.Handle) < len(w.module.Types) {
				if _, isPtr := w.module.Types[*resolution.Handle].Inner.(ir.PointerType); isPtr {
					return true
				}
			}
		}
		if resolution.Value != nil {
			if _, isPtr := resolution.Value.(ir.PointerType); isPtr {
				return true
			}
		}
	}

	// Also check if this is an Access/AccessIndex on a pointer base.
	// The ExpressionTypes resolver returns element type (not pointer) for
	// AccessIndex through pointers, but the expression is still pointer-valued
	// in WGSL semantics (address-of an array element produces a pointer).
	if int(handle) < len(w.currentFunction.Expressions) {
		expr := w.currentFunction.Expressions[handle].Kind
		switch e := expr.(type) {
		case ir.ExprAccess:
			return w.isPointerExpression(e.Base)
		case ir.ExprAccessIndex:
			return w.isPointerExpression(e.Base)
		}
	}

	return false
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
// Matches Rust naga: `switch(expr) { case N: { body } }`
// - No space between `switch` and `(`
// - Each non-empty case body is wrapped in `{ }`
// - Uses continueCtx to forward continue statements inside switches within loops.
func (w *Writer) writeSwitchStatement(s ir.StmtSwitch) error {
	// Check if there is only one body: all cases except the last are empty fall-throughs.
	// FXC doesn't handle these correctly, so emit do {} while(false) instead.
	// Matches Rust naga's one_body optimization.
	oneBody := true
	if len(s.Cases) > 1 {
		for i := 0; i < len(s.Cases)-1; i++ {
			c := &s.Cases[i]
			if len(c.Body) > 0 || !c.FallThrough {
				oneBody = false
				break
			}
		}
	}

	// Enter switch in continue context (for both one_body and regular switches)
	variable := w.continueCtx.enterSwitch(w.namer)
	if variable != "" {
		w.writeLine("bool %s = false;", variable)
	}

	if oneBody {
		w.writeLine("do {")
		w.pushIndent()
		if len(s.Cases) > 0 {
			lastCase := &s.Cases[len(s.Cases)-1]
			if err := w.writeBlock(lastCase.Body); err != nil {
				w.popIndent()
				return err
			}
		}
		w.popIndent()
		w.writeLine("} while(false);")
	} else {
		w.writeIndent()
		w.out.WriteString("switch(")
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
	}

	// Exit switch and emit forwarding code if needed
	ecf := w.continueCtx.exitSwitch()
	switch ecf.kind {
	case exitContinue:
		w.writeLine("if (%s) {", ecf.variable)
		w.pushIndent()
		w.writeLine("continue;")
		w.popIndent()
		w.writeLine("}")
	case exitBreak:
		w.writeLine("if (%s) {", ecf.variable)
		w.pushIndent()
		w.writeLine("break;")
		w.popIndent()
		w.writeLine("}")
	}

	return nil
}

// blockEndsWithTerminator returns true if the block's last statement is a
// terminator (Break, Continue, Return, Kill). Matches Rust naga's is_terminator().
func blockEndsWithTerminator(block ir.Block) bool {
	if len(block) == 0 {
		return false
	}
	switch block[len(block)-1].Kind.(type) {
	case ir.StmtBreak, ir.StmtContinue, ir.StmtReturn, ir.StmtKill:
		return true
	}
	return false
}

// writeSwitchCase writes a single switch case.
// Matches Rust naga: each non-empty case body is wrapped in `{ }` braces.
// Fall-through cases with empty bodies don't get braces.
func (w *Writer) writeSwitchCase(c *ir.SwitchCase) error {
	// Write case label at current indent + 1
	w.pushIndent()
	switch v := c.Value.(type) {
	case ir.SwitchValueI32:
		w.writeIndent()
		fmt.Fprintf(&w.out, "case %d:", int32(v))
	case ir.SwitchValueU32:
		w.writeIndent()
		fmt.Fprintf(&w.out, "case %du:", uint32(v))
	case ir.SwitchValueDefault:
		w.writeIndent()
		w.out.WriteString("default:")
	default:
		w.popIndent()
		return fmt.Errorf("unsupported switch value type: %T", c.Value)
	}

	// Determine if we need block braces (Rust naga: non-empty OR non-fallthrough gets braces)
	writeBlockBraces := !(c.FallThrough && len(c.Body) == 0)
	if writeBlockBraces {
		w.out.WriteString(" {\n")
	} else {
		w.out.WriteByte('\n')
	}

	// Write case body
	w.pushIndent()
	if err := w.writeBlock(c.Body); err != nil {
		w.popIndent()
		w.popIndent()
		return err
	}

	// Write break unless fallthrough or body already ends with a terminator.
	if !c.FallThrough && !blockEndsWithTerminator(c.Body) {
		w.writeLine("break;")
	}
	w.popIndent()

	// Close block braces
	if writeBlockBraces {
		w.writeLine("}")
	}

	w.popIndent()
	return nil
}

// writeLoopStatement writes a loop statement.
// Naga loops are while(true) with explicit break conditions.
// When ForceLoopBounding is enabled, adds loop_bound counter to prevent infinite loops.
func (w *Writer) writeLoopStatement(s ir.StmtLoop) error {
	hasContinuing := len(s.Continuing) > 0 || s.BreakIf != nil
	const maxIter = uint32(4294967295) // u32::MAX

	// Loop bounding: declare counter before loop
	var loopBoundName, loopInitName string
	if w.options.ForceLoopBounding {
		loopBoundName = w.namer.call("loop_bound")
		w.writeLine("uint2 %s = uint2(%du, %du);", loopBoundName, maxIter, maxIter)
	}
	if hasContinuing && w.options.ForceLoopBounding {
		loopInitName = w.namer.call("loop_init")
		w.writeLine("bool %s = true;", loopInitName)
	}

	w.continueCtx.enterLoop()

	w.writeLine("while(true) {")
	w.pushIndent()

	// Loop bounding: add break check and decrement at loop start
	if w.options.ForceLoopBounding {
		w.writeLine("if (all(%s == uint2(0u, 0u))) { break; }", loopBoundName)
		w.writeLine("%s -= uint2(%s.y == 0u, 1u);", loopBoundName, loopBoundName)
	}

	// If there's a continuing block, wrap it in a gate
	if hasContinuing && w.options.ForceLoopBounding {
		w.writeLine("if (!%s) {", loopInitName)
		w.pushIndent()
		// Write continuing block
		if len(s.Continuing) > 0 {
			if err := w.writeBlock(s.Continuing); err != nil {
				w.popIndent()
				w.popIndent()
				return fmt.Errorf("loop continuing: %w", err)
			}
		}
		// Write break-if condition
		if s.BreakIf != nil {
			w.writeIndent()
			w.out.WriteString("if (")
			if err := w.writeExpression(*s.BreakIf); err != nil {
				w.popIndent()
				w.popIndent()
				return fmt.Errorf("loop break-if: %w", err)
			}
			w.out.WriteString(") {\n")
			w.pushIndent()
			w.writeLine("break;")
			w.popIndent()
			w.writeLine("}")
		}
		w.popIndent()
		w.writeLine("}")
		w.writeLine("%s = false;", loopInitName)
	} else if hasContinuing && !w.options.ForceLoopBounding {
		// Without loop bounding, just write continuing after body
		// (handled below after body)
	}

	// Write main body
	if err := w.writeBlock(s.Body); err != nil {
		w.popIndent()
		return fmt.Errorf("loop body: %w", err)
	}

	// Write continuing block without force loop bounding
	if hasContinuing && !w.options.ForceLoopBounding {
		if len(s.Continuing) > 0 {
			if err := w.writeBlock(s.Continuing); err != nil {
				w.popIndent()
				return fmt.Errorf("loop continuing: %w", err)
			}
		}
		if s.BreakIf != nil {
			w.writeIndent()
			w.out.WriteString("if (")
			if err := w.writeExpression(*s.BreakIf); err != nil {
				w.popIndent()
				return fmt.Errorf("loop break-if: %w", err)
			}
			w.out.WriteString(") {\n")
			w.pushIndent()
			w.writeLine("break;")
			w.popIndent()
			w.writeLine("}")
		}
	}

	w.popIndent()
	w.writeLine("}")

	w.continueCtx.exitLoop()
	return nil
}

// writeBreakStatement writes a break statement.
func (w *Writer) writeBreakStatement() error {
	w.writeLine("break;")
	return nil
}

// writeContinueStatement writes a continue statement.
// Uses continueCtx to forward continue statements inside switches to enclosing loops.
func (w *Writer) writeContinueStatement() error {
	if variable := w.continueCtx.continueEncountered(); variable != "" {
		// Inside a switch within a loop: set the bool and break from the switch
		w.writeLine("%s = true;", variable)
		w.writeLine("break;")
	} else {
		// Normal continue
		w.writeLine("continue;")
	}
	return nil
}

// =============================================================================
// Return and Kill Statements
// =============================================================================

// writeReturnStatement writes a return statement.
// For EP functions returning structs with output struct conversion, matches Rust naga pattern:
//
//	const StructType var = expr;
//	const OutputType var_1 = { var.member1, var.member2, ... };
//	return var_1;
func (w *Writer) writeReturnStatement(s ir.StmtReturn) error {
	if s.Value == nil {
		w.writeLine("return;")
		return nil
	}

	// Check if return value is a struct type - needs special handling
	// Rust naga always wraps struct returns in a const variable
	returnType := w.getExpressionType(*s.Value)
	if returnType == nil {
		returnType = w.inferExpressionType(*s.Value)
	}
	if returnType != nil {
		resolvedInner := returnType.Inner
		if ptr, ok := resolvedInner.(ir.PointerType); ok {
			if int(ptr.Base) < len(w.module.Types) {
				resolvedInner = w.module.Types[ptr.Base].Inner
			}
		}
		if _, isStruct := resolvedInner.(ir.StructType); isStruct {
			// Get EP output struct if applicable
			var epOutput *entryPointBinding
			if w.currentEPIndex >= 0 {
				if epIO := w.entryPointIO[w.currentEPIndex]; epIO != nil {
					epOutput = epIO.output
				}
			}
			return w.writeStructReturn(s, epOutput)
		}
	}

	w.writeIndent()
	w.out.WriteString("return ")
	if err := w.writeExpression(*s.Value); err != nil {
		return fmt.Errorf("return value: %w", err)
	}
	w.out.WriteString(";\n")
	return nil
}

// writeStructReturn handles struct return value wrapping.
// Always wraps in const variable. If epOutput is non-nil, also converts to EP output struct.
func (w *Writer) writeStructReturn(s ir.StmtReturn, epOutput *entryPointBinding) error {
	// Get the type handle for the return expression
	typeHandle := w.getExpressionTypeHandle(*s.Value)
	if typeHandle == nil {
		// Fallback to direct return
		w.writeIndent()
		w.out.WriteString("return ")
		if err := w.writeExpression(*s.Value); err != nil {
			return err
		}
		w.out.WriteString(";\n")
		return nil
	}

	// Get the struct type name
	structName := w.typeNames[*typeHandle]
	if structName == "" {
		structName = fmt.Sprintf("type_%d", *typeHandle)
	}

	// Step 1: const StructType variable = expr;
	varName := w.namer.call(strings.ToLower(structName))
	w.writeIndent()
	fmt.Fprintf(&w.out, "const %s %s = ", structName, varName)
	if err := w.writeExpression(*s.Value); err != nil {
		return err
	}
	w.out.WriteString(";\n")

	// Step 2: If EP output struct, create conversion
	finalName := varName
	if epOutput != nil {
		finalName = w.namer.call(varName)
		w.writeIndent()
		fmt.Fprintf(&w.out, "const %s %s = { ", epOutput.tyName, finalName)
		for i, m := range epOutput.members {
			if i > 0 {
				w.out.WriteString(", ")
			}
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(*typeHandle), handle2: m.index}]
			if memberName == "" {
				memberName = fmt.Sprintf("member_%d", m.index)
			}
			fmt.Fprintf(&w.out, "%s.%s", varName, memberName)
		}
		w.out.WriteString(" };\n")
	}

	// Step 3: return final name;
	w.writeIndent()
	fmt.Fprintf(&w.out, "return %s;\n", finalName)
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

	// Matches Rust naga write_control_barrier: emit each barrier type independently.
	// SubGroup barrier is a no-op in HLSL (does not exist in DirectX).
	if hasStorage {
		w.writeLine("DeviceMemoryBarrierWithGroupSync();")
	}
	if hasWorkgroup {
		w.writeLine("GroupMemoryBarrierWithGroupSync();")
	}
	// SUB_GROUP: no-op in HLSL (matches Rust naga)
	if hasTexture {
		w.writeLine("DeviceMemoryBarrierWithGroupSync();")
	}
	_ = hasSubgroup

	return nil
}

// =============================================================================
// Store Statements
// =============================================================================

// writeStoreStatement writes a store statement.
func (w *Writer) writeStoreStatement(s ir.StmtStore) error {
	// Check if storing to a storage buffer pointer -> use ByteAddressBuffer Store
	if w.isStoragePointer(s.Pointer) {
		varHandle, err := w.fillAccessChain(s.Pointer)
		if err != nil {
			return fmt.Errorf("storage store: %w", err)
		}
		sv := storeValue{kind: storeValueExpression, expr: s.Value}
		return w.writeStorageStore(varHandle, sv, w.indent)
	}

	// Check for matCx2 struct member store — use SetMat/SetMatVec/SetMatScalar helpers.
	if handled, err := w.writeMatCx2StoreIfNeeded(s); handled {
		return err
	}

	// Check for dynamic column/scalar store on array-of-matCx2 struct members.
	// Uses __set_col_of_matCx2 / __set_el_of_matCx2 helpers.
	if handled, err := w.writeArrayMatCx2DynamicStoreIfNeeded(s); handled {
		return err
	}

	w.writeIndent()
	if err := w.writeExpression(s.Pointer); err != nil {
		return fmt.Errorf("store pointer: %w", err)
	}
	w.out.WriteString(" = ")

	// Cast RHS when storing to a struct member that is matCx2 or array-of-matCx2.
	// Matches Rust naga: get_inner_matrix_of_struct_array_member wrapping.
	castClose := w.writeMatCx2StoreCastIfNeeded(s.Pointer)

	if err := w.writeExpression(s.Value); err != nil {
		return fmt.Errorf("store value: %w", err)
	}
	w.out.WriteString(castClose)
	w.out.WriteString(";\n")
	return nil
}

// writeMatCx2StoreCastIfNeeded checks if the store target is a struct member that
// is matCx2 or array-of-matCx2, and if so writes a cast prefix.
// Returns the closing string to write after the value (e.g. ")").
// Matches Rust naga: wraps RHS in (__matCx2) or (__matCx2[N]) cast.
func (w *Writer) writeMatCx2StoreCastIfNeeded(pointer ir.ExpressionHandle) string {
	if w.currentFunction == nil {
		return ""
	}
	m := w.getInnerMatrixOfStructArrayMember(pointer)
	if m == nil || !m.isMatCx2() {
		return ""
	}

	// Determine the resolved type of the pointer to see if it's array or direct matrix
	resolved := w.getExpressionTypeInner(pointer)
	if resolved == nil {
		return ""
	}
	// Dereference pointer
	if ptr, ok := resolved.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			resolved = w.module.Types[ptr.Base].Inner
		}
	}

	cols := uint8(m.columns)
	if arr, ok := resolved.(ir.ArrayType); ok {
		// Array-of-matCx2: cast to __matCx2[size]
		size := ""
		if arr.Size.Constant != nil {
			size = fmt.Sprintf("[%d]", *arr.Size.Constant)
		}
		cast := fmt.Sprintf("(__mat%dx2%s)", cols, size)
		w.out.WriteString(cast)
		return ""
	}
	// Direct matCx2 member: cast to __matCx2
	cast := fmt.Sprintf("(__mat%dx2)", cols)
	w.out.WriteString(cast)
	return ""
}

// getInnerMatrixOfStructArrayMember walks an access chain to find if the target
// is a struct member that is matCx2 or array-of-matCx2.
// Matches Rust naga get_inner_matrix_of_struct_array_member.
func (w *Writer) getInnerMatrixOfStructArrayMember(handle ir.ExpressionHandle) *matrixTypeInfo {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}

	var arrayBase *ir.TypeHandle
	current := handle
	for {
		if int(current) >= len(w.currentFunction.Expressions) {
			return nil
		}

		// Resolve type (deref pointer)
		resolved := w.getExpressionTypeInner(current)
		if resolved == nil {
			return nil
		}
		if ptr, ok := resolved.(ir.PointerType); ok {
			if int(ptr.Base) < len(w.module.Types) {
				resolved = w.module.Types[ptr.Base].Inner
			}
		}

		switch resolved.(type) {
		case ir.MatrixType:
			// Found matrix in chain
		case ir.ArrayType:
			arr := resolved.(ir.ArrayType)
			ab := arr.Base
			arrayBase = &ab
		case ir.StructType:
			if arrayBase != nil {
				return getInnerMatrixData(w.module, *arrayBase)
			}
			return nil
		default:
			return nil
		}

		// Walk up access chain
		expr := w.currentFunction.Expressions[current].Kind
		switch e := expr.(type) {
		case ir.ExprAccess:
			current = e.Base
		case ir.ExprAccessIndex:
			current = e.Base
		default:
			return nil
		}
	}
}

// writeArrayMatCx2DynamicStoreIfNeeded checks if the store targets a dynamic column
// (or column+scalar) of a matCx2 inside an array-of-matCx2 struct member.
// If so, writes __set_col_of_matCx2 or __set_el_of_matCx2.
// Returns (true, err) if handled, (false, nil) if not.
func (w *Writer) writeArrayMatCx2DynamicStoreIfNeeded(s ir.StmtStore) (bool, error) {
	if w.currentFunction == nil {
		return false, nil
	}

	// Walk the pointer chain looking for dynamic Access on a matCx2 within array-of-matCx2
	// Pattern: [AccessIndex(scalar)] -> Access(dynamic_col) -> ... -> struct.array_member
	pointer := s.Pointer
	if int(pointer) >= len(w.currentFunction.Expressions) {
		return false, nil
	}

	// Check for optional scalar access on top (for __set_el).
	// Can be static (AccessIndex) or dynamic (Access).
	type scalarAccessInfo struct {
		isStatic bool
		static_  uint32
		dynamic  ir.ExpressionHandle
	}
	var scalarInfo *scalarAccessInfo
	expr := w.currentFunction.Expressions[pointer].Kind

	// Helper to check if parent chain leads to array-of-matCx2
	checkParent := func(parentBase ir.ExpressionHandle) bool {
		if int(parentBase) >= len(w.currentFunction.Expressions) {
			return false
		}
		parentExpr := w.currentFunction.Expressions[parentBase].Kind
		if acc, ok := parentExpr.(ir.ExprAccess); ok {
			if m := w.getInnerMatrixOfStructArrayMember(acc.Base); m != nil && m.isMatCx2() {
				baseInner := w.getExpressionTypeInner(acc.Base)
				if baseInner != nil {
					if ptr, ok := baseInner.(ir.PointerType); ok {
						if int(ptr.Base) < len(w.module.Types) {
							baseInner = w.module.Types[ptr.Base].Inner
						}
					}
					if _, ok := baseInner.(ir.MatrixType); ok {
						return true
					}
				}
			}
		}
		return false
	}

	if ai, ok := expr.(ir.ExprAccessIndex); ok {
		if checkParent(ai.Base) {
			scalarInfo = &scalarAccessInfo{isStatic: true, static_: ai.Index}
			pointer = ai.Base
		}
	} else if acc, ok := expr.(ir.ExprAccess); ok {
		if checkParent(acc.Base) {
			scalarInfo = &scalarAccessInfo{isStatic: false, dynamic: acc.Index}
			pointer = acc.Base
		}
	}

	// Now check for dynamic Access on matCx2
	if int(pointer) >= len(w.currentFunction.Expressions) {
		return false, nil
	}
	expr = w.currentFunction.Expressions[pointer].Kind
	acc, ok := expr.(ir.ExprAccess)
	if !ok {
		return false, nil
	}

	// Check if this Access is on a matCx2 within array-of-matCx2
	m := w.getInnerMatrixOfStructArrayMember(acc.Base)
	if m == nil || !m.isMatCx2() {
		return false, nil
	}

	// Verify base type is matrix
	baseInner := w.getExpressionTypeInner(acc.Base)
	if baseInner != nil {
		if ptr, ok := baseInner.(ir.PointerType); ok {
			if int(ptr.Base) < len(w.module.Types) {
				baseInner = w.module.Types[ptr.Base].Inner
			}
		}
	}
	if _, ok := baseInner.(ir.MatrixType); !ok {
		return false, nil
	}

	cols := uint8(m.columns)
	w.writeIndent()

	if scalarInfo != nil {
		// __set_el_of_matCx2(base, col_idx, scalar_idx, value)
		fmt.Fprintf(&w.out, "__set_el_of_mat%dx2(", cols)
		if err := w.writeExpression(acc.Base); err != nil {
			return true, err
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(acc.Index); err != nil {
			return true, err
		}
		w.out.WriteString(", ")
		if scalarInfo.isStatic {
			fmt.Fprintf(&w.out, "%d", scalarInfo.static_)
		} else {
			if err := w.writeExpression(scalarInfo.dynamic); err != nil {
				return true, err
			}
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(s.Value); err != nil {
			return true, err
		}
		w.out.WriteString(");\n")
		return true, nil
	}

	// __set_col_of_matCx2(base, col_idx, value)
	fmt.Fprintf(&w.out, "__set_col_of_mat%dx2(", cols)
	if err := w.writeExpression(acc.Base); err != nil {
		return true, err
	}
	w.out.WriteString(", ")
	if err := w.writeExpression(acc.Index); err != nil {
		return true, err
	}
	w.out.WriteString(", ")
	if err := w.writeExpression(s.Value); err != nil {
		return true, err
	}
	w.out.WriteString(");\n")
	return true, nil
}

// matCx2StoreIndex represents an index access in the chain above a matCx2 member.
type matCx2StoreIndex struct {
	isStatic bool
	value    ir.ExpressionHandle // for dynamic
	static_  uint32              // for static (AccessIndex)
}

// writeMatCx2StoreIfNeeded checks if the store targets a matCx2 struct member
// and writes the appropriate SetMat/SetMatVec/SetMatScalar helper call.
// Returns (true, err) if handled, (false, nil) if not a matCx2 store.
func (w *Writer) writeMatCx2StoreIfNeeded(s ir.StmtStore) (bool, error) {
	if w.currentFunction == nil {
		return false, nil
	}

	// Walk the access chain from the store pointer to find a matCx2 struct member.
	// Pattern: ptr -> [Access/AccessIndex for scalar] -> [Access/AccessIndex for vector] -> AccessIndex{struct, member_idx}
	matExprPtr, vectorIdx, scalarIdx := w.findMatCx2InAccessChain(s.Pointer)
	if matExprPtr == nil {
		return false, nil
	}
	matExpr := *matExprPtr

	// matExpr points to an AccessIndex on a struct where the member is matCx2
	ai, ok := w.currentFunction.Expressions[matExpr].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false, nil
	}

	// Find the struct type handle (deref pointer)
	structTyHandle := w.resolveExprTypeHandle(w.currentFunction, ai.Base)
	if structTyHandle == nil {
		return false, nil
	}
	tyH := *structTyHandle
	if ptr, ok := w.module.Types[tyH].Inner.(ir.PointerType); ok {
		tyH = ptr.Base
	}
	st, ok := w.module.Types[tyH].Inner.(ir.StructType)
	if !ok || int(ai.Index) >= len(st.Members) {
		return false, nil
	}
	member := st.Members[ai.Index]
	if member.Binding != nil {
		return false, nil
	}
	_, isMat := w.module.Types[member.Type].Inner.(ir.MatrixType)
	if !isMat {
		return false, nil
	}

	structName := w.typeNames[tyH]
	fieldName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(tyH), handle2: uint32(ai.Index)}]

	w.writeIndent()

	if vectorIdx == nil {
		// Direct matrix store: SetMatmOnBaz(base, value)
		fmt.Fprintf(&w.out, "SetMat%sOn%s(", fieldName, structName)
		if err := w.writeExpression(ai.Base); err != nil {
			return true, err
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(s.Value); err != nil {
			return true, err
		}
		w.out.WriteString(");\n")
		return true, nil
	}

	if vectorIdx.isStatic {
		// Static vector index: base.m_N [optionally .y or [scalar]]
		if err := w.writeExpression(ai.Base); err != nil {
			return true, err
		}
		fmt.Fprintf(&w.out, ".%s_%d", fieldName, vectorIdx.static_)
		if scalarIdx != nil {
			if scalarIdx.isStatic {
				// Rust naga uses [N] form for scalar access on decomposed matCx2 columns
				fmt.Fprintf(&w.out, "[%d]", scalarIdx.static_)
			} else {
				w.out.WriteByte('[')
				if err := w.writeExpression(scalarIdx.value); err != nil {
					return true, err
				}
				w.out.WriteByte(']')
			}
		}
		w.out.WriteString(" = ")
		if err := w.writeExpression(s.Value); err != nil {
			return true, err
		}
		w.out.WriteString(";\n")
		return true, nil
	}

	// Dynamic vector index
	if scalarIdx == nil {
		// SetMatVecmOnBaz(base, value, vec_idx)
		fmt.Fprintf(&w.out, "SetMatVec%sOn%s(", fieldName, structName)
		if err := w.writeExpression(ai.Base); err != nil {
			return true, err
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(s.Value); err != nil {
			return true, err
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(vectorIdx.value); err != nil {
			return true, err
		}
		w.out.WriteString(");\n")
		return true, nil
	}

	// Dynamic vector + scalar: SetMatScalarmOnBaz(base, value, vec_idx, scalar_idx)
	fmt.Fprintf(&w.out, "SetMatScalar%sOn%s(", fieldName, structName)
	if err := w.writeExpression(ai.Base); err != nil {
		return true, err
	}
	w.out.WriteString(", ")
	if err := w.writeExpression(s.Value); err != nil {
		return true, err
	}
	w.out.WriteString(", ")
	if err := w.writeExpression(vectorIdx.value); err != nil {
		return true, err
	}
	w.out.WriteString(", ")
	if scalarIdx.isStatic {
		fmt.Fprintf(&w.out, "%d", scalarIdx.static_)
	} else {
		if err := w.writeExpression(scalarIdx.value); err != nil {
			return true, err
		}
	}
	w.out.WriteString(");\n")
	return true, nil
}

// findMatCx2InAccessChain walks the store pointer access chain to find a matCx2 struct member.
// Returns (matExprHandle, vectorIdx, scalarIdx) where matExprHandle is the expression handle
// of the AccessIndex that points to the matCx2 member, or nil if not found.
// Matches Rust naga's find_matrix_in_access_chain.
func (w *Writer) findMatCx2InAccessChain(pointer ir.ExpressionHandle) (*ir.ExpressionHandle, *matCx2StoreIndex, *matCx2StoreIndex) {
	cur := pointer
	var vectorIdx, scalarIdx *matCx2StoreIndex

	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return nil, nil, nil
		}
		expr := w.currentFunction.Expressions[cur]

		// Check if this expression points to a struct member that is matCx2.
		// We must check the BASE type (not root variable type) to properly
		// distinguish struct member access from matrix column access.
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			baseTyH := w.resolveExprTypeHandle(w.currentFunction, ai.Base)
			if baseTyH != nil {
				bth := *baseTyH
				if ptr, ok := w.module.Types[bth].Inner.(ir.PointerType); ok {
					bth = ptr.Base
				}
				if st, ok := w.module.Types[bth].Inner.(ir.StructType); ok {
					if int(ai.Index) < len(st.Members) {
						member := st.Members[ai.Index]
						if member.Binding == nil {
							if mat, ok := w.module.Types[member.Type].Inner.(ir.MatrixType); ok && mat.Rows == 2 {
								_ = mat
								result := cur
								return &result, vectorIdx, scalarIdx
							}
						}
					}
				}
			}
		}

		// Move up the chain
		switch e := expr.Kind.(type) {
		case ir.ExprAccess:
			if vectorIdx == nil && scalarIdx == nil {
				vectorIdx = &matCx2StoreIndex{isStatic: false, value: e.Index}
			} else if scalarIdx == nil {
				scalarIdx = vectorIdx
				vectorIdx = &matCx2StoreIndex{isStatic: false, value: e.Index}
			} else {
				return nil, nil, nil
			}
			cur = e.Base
		case ir.ExprAccessIndex:
			if vectorIdx == nil && scalarIdx == nil {
				vectorIdx = &matCx2StoreIndex{isStatic: true, static_: e.Index}
			} else if scalarIdx == nil {
				scalarIdx = vectorIdx
				vectorIdx = &matCx2StoreIndex{isStatic: true, static_: e.Index}
			} else {
				return nil, nil, nil
			}
			cur = e.Base
		default:
			return nil, nil, nil
		}
	}
}

// resolveExprTypeHandle resolves the TYPE of an expression (not the root variable type).
// For AccessIndex/Access, it resolves the member/element type from the parent type.
// This is needed for accurate matCx2 detection in store chains.
func (w *Writer) resolveExprTypeHandle(fn *ir.Function, handle ir.ExpressionHandle) *ir.TypeHandle {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	expr := fn.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			ty := w.module.GlobalVariables[e.Variable].Type
			return &ty
		}
	case ir.ExprLocalVariable:
		if fn.LocalVars != nil && int(e.Variable) < len(fn.LocalVars) {
			ty := fn.LocalVars[e.Variable].Type
			return &ty
		}
	case ir.ExprFunctionArgument:
		if int(e.Index) < len(fn.Arguments) {
			ty := fn.Arguments[e.Index].Type
			return &ty
		}
	case ir.ExprAccessIndex:
		baseTyH := w.resolveExprTypeHandle(fn, e.Base)
		if baseTyH == nil {
			return nil
		}
		bth := *baseTyH
		// Dereference pointer
		if ptr, ok := w.module.Types[bth].Inner.(ir.PointerType); ok {
			bth = ptr.Base
		}
		switch inner := w.module.Types[bth].Inner.(type) {
		case ir.StructType:
			if int(e.Index) < len(inner.Members) {
				ty := inner.Members[e.Index].Type
				return &ty
			}
		case ir.ArrayType:
			ty := inner.Base
			return &ty
		case ir.VectorType:
			// Component access - need to find scalar type handle
			// For simplicity, return nil and let caller handle
			return nil
		case ir.MatrixType:
			// Column access - need to find vector type handle
			return nil
		}
	case ir.ExprAccess:
		return w.resolveExprTypeHandle(fn, e.Base)
	case ir.ExprLoad:
		return w.resolveExprTypeHandle(fn, e.Pointer)
	}
	return nil
}

// resolveVariableTypeHandle walks an expression chain to find the root variable
// and returns its type handle. Supports GlobalVariable, LocalVariable, FunctionArgument.
func (w *Writer) resolveVariableTypeHandle(fn *ir.Function, handle ir.ExpressionHandle) *ir.TypeHandle {
	cur := handle
	for {
		if int(cur) >= len(fn.Expressions) {
			return nil
		}
		expr := fn.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				ty := w.module.GlobalVariables[e.Variable].Type
				return &ty
			}
			return nil
		case ir.ExprLocalVariable:
			if fn.LocalVars != nil && int(e.Variable) < len(fn.LocalVars) {
				ty := fn.LocalVars[e.Variable].Type
				return &ty
			}
			return nil
		case ir.ExprFunctionArgument:
			if int(e.Index) < len(fn.Arguments) {
				ty := fn.Arguments[e.Index].Type
				return &ty
			}
			return nil
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		case ir.ExprLoad:
			cur = e.Pointer
		default:
			return nil
		}
	}
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
// Matches Rust naga's Statement::Atomic handling:
// - WorkGroup space: InterlockedXxx(pointer, ...)
// - Storage space: buffer.InterlockedXxx[64](byte_offset, ...)
// - Result declaration: type _eN; before the call
// - Subtract: negate value argument
// - CompareExchange: extra compare arg, .old_value, .exchanged follow-up
func (w *Writer) writeAtomicStatement(s ir.StmtAtomic) error {
	w.writeIndent()

	// Determine the atomic function suffix (Add, And, Or, Xor, Min, Max, Exchange, CompareExchange)
	funSuffix := atomicFunSuffix(s.Fun)

	// Extract compare expression for CompareExchange
	var compareExpr *ir.ExpressionHandle
	if xchg, ok := s.Fun.(ir.AtomicExchange); ok {
		compareExpr = xchg.Compare
	}

	// Declare result variable if needed (matches Rust: "type _eN; ")
	var resVarName string
	if s.Result != nil {
		resHandle := *s.Result
		resVarName = fmt.Sprintf("_e%d", resHandle)

		// Get the result type
		if int(resHandle) < len(w.currentFunction.ExpressionTypes) {
			resolution := &w.currentFunction.ExpressionTypes[resHandle]
			if resolution.Handle != nil {
				h := *resolution.Handle
				if int(h) < len(w.module.Types) {
					typeName := w.getTypeName(h)
					fmt.Fprintf(&w.out, "%s %s; ", typeName, resVarName)
				}
			} else if resolution.Value != nil {
				typeName := w.typeInnerToHLSLStr(resolution.Value)
				fmt.Fprintf(&w.out, "%s %s; ", typeName, resVarName)
			}
		}
		w.namedExpressions[resHandle] = resVarName
	}

	// Determine address space from pointer expression
	pointerSpace := w.getPointerAddressSpace(s.Pointer)

	// Determine width suffix for 64-bit atomics
	widthSuffix := ""
	valueInner := w.getExpressionTypeInner(s.Value)
	if valueInner != nil {
		switch t := valueInner.(type) {
		case ir.ScalarType:
			if t.Width == 8 {
				widthSuffix = "64"
			}
		}
	}

	switch pointerSpace {
	case ir.SpaceStorage:
		// Storage: buffer.InterlockedXxx[64](byte_offset, ...)
		varHandle, err := w.fillAccessChain(s.Pointer)
		if err != nil {
			return fmt.Errorf("atomic storage access chain: %w", err)
		}
		varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(varHandle)}]
		fmt.Fprintf(&w.out, "%s.Interlocked%s%s(", varName, funSuffix, widthSuffix)
		chain := w.tempAccessChain
		w.tempAccessChain = nil
		if err := w.writeStorageAddress(chain); err != nil {
			return err
		}
		w.tempAccessChain = chain
		w.writeAtomicTail(s.Fun, compareExpr, s.Value, resVarName)

	default:
		// WorkGroup (and other): InterlockedXxx(pointer_expr, ...)
		fmt.Fprintf(&w.out, "Interlocked%s(", funSuffix)
		if err := w.writeExpression(s.Pointer); err != nil {
			return fmt.Errorf("atomic pointer: %w", err)
		}
		w.writeAtomicTail(s.Fun, compareExpr, s.Value, resVarName)
	}

	// CompareExchange follow-up: .exchanged = (.old_value == compare)
	if compareExpr != nil && resVarName != "" {
		w.writeIndent()
		fmt.Fprintf(&w.out, "%s.exchanged = (%s.old_value == ", resVarName, resVarName)
		if err := w.writeExpression(*compareExpr); err != nil {
			return fmt.Errorf("atomic compare follow-up: %w", err)
		}
		w.out.WriteString(");\n")
	}

	return nil
}

// writeAtomicTail writes the remaining arguments of an atomic call after the first
// argument (pointer or byte offset). Matches Rust naga's emit_hlsl_atomic_tail.
func (w *Writer) writeAtomicTail(fun ir.AtomicFunction, compareExpr *ir.ExpressionHandle, value ir.ExpressionHandle, resVarName string) {
	if compareExpr != nil {
		w.out.WriteString(", ")
		_ = w.writeExpression(*compareExpr) // error ignored for tail
	}
	w.out.WriteString(", ")
	if _, ok := fun.(ir.AtomicSubtract); ok {
		w.out.WriteByte('-')
	}
	_ = w.writeExpression(value) // error ignored for tail
	if resVarName != "" {
		w.out.WriteString(", ")
		if compareExpr != nil {
			fmt.Fprintf(&w.out, "%s.old_value", resVarName)
		} else {
			w.out.WriteString(resVarName)
		}
	}
	w.out.WriteString(");\n")
}

// atomicFunSuffix returns the HLSL suffix for an atomic function.
// Matches Rust naga's AtomicFunction::to_hlsl_suffix.
func atomicFunSuffix(fun ir.AtomicFunction) string {
	switch f := fun.(type) {
	case ir.AtomicAdd, ir.AtomicSubtract:
		return "Add"
	case ir.AtomicAnd:
		return "And"
	case ir.AtomicInclusiveOr:
		return "Or"
	case ir.AtomicExclusiveOr:
		return "Xor"
	case ir.AtomicMin:
		return "Min"
	case ir.AtomicMax:
		return "Max"
	case ir.AtomicExchange:
		if f.Compare != nil {
			return "CompareExchange"
		}
		return "Exchange"
	default:
		return "Exchange"
	}
}

// getPointerAddressSpace resolves the address space for a pointer expression.
func (w *Writer) getPointerAddressSpace(handle ir.ExpressionHandle) ir.AddressSpace {
	if w.currentFunction == nil {
		return ir.SpaceFunction
	}
	if int(handle) < len(w.currentFunction.ExpressionTypes) {
		resolution := &w.currentFunction.ExpressionTypes[handle]
		var inner ir.TypeInner
		if resolution.Handle != nil {
			h := *resolution.Handle
			if int(h) < len(w.module.Types) {
				inner = w.module.Types[h].Inner
			}
		} else {
			inner = resolution.Value
		}
		if ptr, ok := inner.(ir.PointerType); ok {
			return ptr.Space
		}
	}
	// Fallback: walk access chain to global var
	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return ir.SpaceFunction
		}
		expr := w.currentFunction.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				return w.module.GlobalVariables[e.Variable].Space
			}
			return ir.SpaceFunction
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		default:
			return ir.SpaceFunction
		}
	}
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

	// If there's a result, declare a typed local variable and assign it.
	// Matches Rust naga: `const TYPE _eN[SIZE] = func();`
	if s.Result != nil {
		typeName := hlslTypeFloat // fallback
		arraySuffix := ""
		if int(s.Function) < len(w.module.Functions) {
			calledFn := &w.module.Functions[s.Function]
			if calledFn.Result != nil {
				typeName, arraySuffix = w.getTypeNameWithArraySuffix(calledFn.Result.Type)
			}
		}
		tempName := fmt.Sprintf("_e%d", *s.Result)
		w.namedExpressions[*s.Result] = tempName
		fmt.Fprintf(&w.out, "const %s %s%s = ", typeName, tempName, arraySuffix)
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
	case ir.RayQueryGenerateIntersection:
		return w.writeRayQueryGenerateIntersection(s.Query, f)
	case ir.RayQueryConfirmIntersection:
		return w.writeRayQueryConfirmIntersection(s.Query)
	default:
		return fmt.Errorf("unsupported ray query function: %T", s.Fun)
	}
}

// writeRayQueryInitialize writes ray query initialization.
// Matches Rust naga: rq.TraceRayInline(accel, desc.flags, desc.cull_mask, RayDescFromRayDesc_(desc))
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
	w.out.WriteString(", ")

	// Descriptor.flags
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.out.WriteString(".flags, ")

	// Descriptor.cull_mask
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.out.WriteString(".cull_mask, ")

	// RayDescFromRayDesc_(descriptor)
	w.out.WriteString("RayDescFromRayDesc_(")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return fmt.Errorf("ray query descriptor: %w", err)
	}
	w.out.WriteString("));\n")

	// Mark that we need the RayDescFromRayDesc_ helper
	w.needsRayDescHelper = true
	return nil
}

// writeRayQueryProceed writes ray query proceed.
// Matches Rust naga: `const bool _eN = rq.Proceed();`
func (w *Writer) writeRayQueryProceed(query ir.ExpressionHandle, f ir.RayQueryProceed) error {
	w.writeIndent()
	name := w.nameExpression(f.Result)
	fmt.Fprintf(&w.out, "const bool %s = ", name)
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

// writeRayQueryGenerateIntersection writes ray query generate intersection.
func (w *Writer) writeRayQueryGenerateIntersection(query ir.ExpressionHandle, f ir.RayQueryGenerateIntersection) error {
	w.writeIndent()
	if err := w.writeExpression(query); err != nil {
		return fmt.Errorf("ray query: %w", err)
	}
	w.out.WriteString(".CommitProceduralPrimitiveHit(")
	if err := w.writeExpression(f.HitT); err != nil {
		return fmt.Errorf("ray query hit_t: %w", err)
	}
	w.out.WriteString(");\n")
	return nil
}

// writeRayQueryConfirmIntersection writes ray query confirm intersection.
func (w *Writer) writeRayQueryConfirmIntersection(query ir.ExpressionHandle) error {
	w.writeIndent()
	if err := w.writeExpression(query); err != nil {
		return fmt.Errorf("ray query: %w", err)
	}
	w.out.WriteString(".CommitNonOpaqueTriangleHit();\n")
	return nil
}

// =============================================================================
// Function Body Integration
// =============================================================================

// writeFunctionBody writes the complete body of a function.
func (w *Writer) writeFunctionBody(fn *ir.Function) error {
	// Write local variables (use pre-registered names from registerNames)
	for localIdx, local := range fn.LocalVars {
		localName := w.names[nameKey{kind: nameKeyLocal, handle1: uint32(w.currentFuncHandle), handle2: uint32(localIdx)}]
		if localName == "" {
			localName = w.namer.callOr(local.Name, "local")
		}
		w.localNames[uint32(localIdx)] = localName
		// HLSL arrays: type name[size], not type[size] name
		localType, arraySuffix := w.getTypeNameWithArraySuffix(local.Type)

		// Rust naga always initializes locals: with init expression or (Type)0.
		// Exception: RayQuery variables are NOT zero-initialized (no init in HLSL).
		w.writeIndent()
		isRayQuery := strings.Contains(localType, "RayQuery")
		if !isRayQuery && int(local.Type) < len(w.module.Types) {
			_, isRayQuery = w.module.Types[local.Type].Inner.(ir.RayQueryType)
		}
		if isRayQuery {
			// RayQuery<RAY_FLAG_NONE> rq; (no initialization)
			fmt.Fprintf(&w.out, "%s %s%s;\n", localType, localName, arraySuffix)
		} else {
			fmt.Fprintf(&w.out, "%s %s%s = ", localType, localName, arraySuffix)
			if local.Init != nil {
				if err := w.writeExpression(*local.Init); err != nil {
					return fmt.Errorf("local var init: %w", err)
				}
			} else {
				// Zero initialize: (Type)0 matching Rust naga's write_default_init
				fmt.Fprintf(&w.out, "(%s%s)0", localType, arraySuffix)
			}
			w.out.WriteString(";\n")
		}
	}

	if len(fn.LocalVars) > 0 {
		// Rust naga writes just a newline (no indentation) after locals
		w.out.WriteByte('\n')
	}

	// Write function body statements
	return w.writeBlock(fn.Body)
}

// writeSubgroupBallotStatement writes a SubgroupBallot statement.
// Matches Rust naga: `const uint4 _eN = WaveActiveBallot(predicate);`
func (w *Writer) writeSubgroupBallotStatement(s ir.StmtSubgroupBallot) error {
	w.writeIndent()
	name := w.nameExpression(s.Result)
	fmt.Fprintf(&w.out, "const uint4 %s = WaveActiveBallot(", name)
	if s.Predicate != nil {
		if err := w.writeExpression(*s.Predicate); err != nil {
			return err
		}
	} else {
		w.out.WriteString("true")
	}
	w.out.WriteString(");\n")
	return nil
}

// writeSubgroupCollectiveOperationStatement writes a SubgroupCollectiveOperation statement.
// Matches Rust naga: `const TYPE _eN = WaveActiveOp(argument);`
func (w *Writer) writeSubgroupCollectiveOperationStatement(s ir.StmtSubgroupCollectiveOperation) error {
	w.writeIndent()
	name := w.nameExpression(s.Result)
	typeName := w.expressionTypeStr(s.Result)
	fmt.Fprintf(&w.out, "const %s %s = ", typeName, name)

	// InclusiveScan requires special handling: `arg OP WavePrefixOp(arg)`
	isInclusiveScan := s.CollectiveOp == ir.CollectiveInclusiveScan

	if isInclusiveScan {
		if err := w.writeExpression(s.Argument); err != nil {
			return err
		}
		switch s.Op {
		case ir.SubgroupOperationAdd:
			w.out.WriteString(" + WavePrefixSum(")
		case ir.SubgroupOperationMul:
			w.out.WriteString(" * WavePrefixProduct(")
		default:
			return fmt.Errorf("unsupported inclusive scan op: %d", s.Op)
		}
	} else {
		switch {
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationAll:
			w.out.WriteString("WaveActiveAllTrue(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationAny:
			w.out.WriteString("WaveActiveAnyTrue(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationAdd:
			w.out.WriteString("WaveActiveSum(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationMul:
			w.out.WriteString("WaveActiveProduct(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationMin:
			w.out.WriteString("WaveActiveMin(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationMax:
			w.out.WriteString("WaveActiveMax(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationAnd:
			w.out.WriteString("WaveActiveBitAnd(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationOr:
			w.out.WriteString("WaveActiveBitOr(")
		case s.CollectiveOp == ir.CollectiveReduce && s.Op == ir.SubgroupOperationXor:
			w.out.WriteString("WaveActiveBitXor(")
		case s.CollectiveOp == ir.CollectiveExclusiveScan && s.Op == ir.SubgroupOperationAdd:
			w.out.WriteString("WavePrefixSum(")
		case s.CollectiveOp == ir.CollectiveExclusiveScan && s.Op == ir.SubgroupOperationMul:
			w.out.WriteString("WavePrefixProduct(")
		default:
			return fmt.Errorf("unsupported subgroup collective op: %d/%d", s.CollectiveOp, s.Op)
		}
	}
	if err := w.writeExpression(s.Argument); err != nil {
		return err
	}
	w.out.WriteString(");\n")
	return nil
}

// writeSubgroupGatherStatement writes a SubgroupGather statement.
// Matches Rust naga: `const TYPE _eN = WaveReadLaneAt/First/QuadRead...(argument, index);`
func (w *Writer) writeSubgroupGatherStatement(s ir.StmtSubgroupGather) error {
	w.writeIndent()
	name := w.nameExpression(s.Result)
	typeName := w.expressionTypeStr(s.Result)
	fmt.Fprintf(&w.out, "const %s %s = ", typeName, name)

	switch mode := s.Mode.(type) {
	case ir.GatherBroadcastFirst:
		w.out.WriteString("WaveReadLaneFirst(")
		if err := w.writeExpression(s.Argument); err != nil {
			return err
		}

	case ir.GatherQuadBroadcast:
		w.out.WriteString("QuadReadLaneAt(")
		if err := w.writeExpression(s.Argument); err != nil {
			return err
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(mode.Index); err != nil {
			return err
		}

	case ir.GatherQuadSwap:
		switch mode.Direction {
		case ir.QuadDirectionX:
			w.out.WriteString("QuadReadAcrossX(")
		case ir.QuadDirectionY:
			w.out.WriteString("QuadReadAcrossY(")
		case ir.QuadDirectionDiagonal:
			w.out.WriteString("QuadReadAcrossDiagonal(")
		}
		if err := w.writeExpression(s.Argument); err != nil {
			return err
		}

	default:
		// Broadcast, Shuffle, ShuffleDown, ShuffleUp, ShuffleXor -> WaveReadLaneAt
		w.out.WriteString("WaveReadLaneAt(")
		if err := w.writeExpression(s.Argument); err != nil {
			return err
		}
		w.out.WriteString(", ")
		switch m := s.Mode.(type) {
		case ir.GatherBroadcast:
			if err := w.writeExpression(m.Index); err != nil {
				return err
			}
		case ir.GatherShuffle:
			if err := w.writeExpression(m.Index); err != nil {
				return err
			}
		case ir.GatherShuffleDown:
			w.out.WriteString("WaveGetLaneIndex() + ")
			if err := w.writeExpression(m.Delta); err != nil {
				return err
			}
		case ir.GatherShuffleUp:
			w.out.WriteString("WaveGetLaneIndex() - ")
			if err := w.writeExpression(m.Delta); err != nil {
				return err
			}
		case ir.GatherShuffleXor:
			w.out.WriteString("WaveGetLaneIndex() ^ ")
			if err := w.writeExpression(m.Mask); err != nil {
				return err
			}
		}
	}
	w.out.WriteString(");\n")
	return nil
}

// nameExpression assigns a baked name (_eN) to an expression and registers it.
func (w *Writer) nameExpression(handle ir.ExpressionHandle) string {
	name := fmt.Sprintf("_e%d", handle)
	w.namedExpressions[handle] = name
	return name
}

// expressionTypeStr returns the HLSL type string for an expression's result type.
func (w *Writer) expressionTypeStr(handle ir.ExpressionHandle) string {
	if w.currentFunction == nil {
		return "uint"
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return "uint"
	}
	res := &w.currentFunction.ExpressionTypes[handle]
	if res.Handle != nil {
		return w.getTypeName(*res.Handle)
	}
	if res.Value != nil {
		return w.typeInnerToHLSLStr(res.Value)
	}
	return "uint"
}
