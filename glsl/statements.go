// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

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
		// Sometimes we must render Continue as a break (inside do-while switches).
		// See continue_forward.go for details.
		if variable := w.continueCtx.continueEncountered(); variable != "" {
			w.writeLine("%s = true;", variable)
			w.writeLine("break;")
		} else {
			w.writeLine("continue;")
		}
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

	case ir.StmtImageAtomic:
		return w.writeImageAtomic(k)

	case ir.StmtAtomic:
		return w.writeAtomic(k)

	case ir.StmtCall:
		return w.writeCall(k)

	case ir.StmtWorkGroupUniformLoad:
		return w.writeWorkGroupUniformLoad(k)

	case ir.StmtRayQuery:
		return w.writeRayQuery(k)

	case ir.StmtSubgroupBallot:
		return w.writeSubgroupBallot(k)
	case ir.StmtSubgroupCollectiveOperation:
		return w.writeSubgroupCollective(k)
	case ir.StmtSubgroupGather:
		return w.writeSubgroupGather(k)

	default:
		return fmt.Errorf("unsupported statement kind: %T", kind)
	}
}

// writeEmit writes an emit statement (materializes expressions).
// Expressions in NamedExpressions (from let bindings and phony assignments)
// are always emitted with their user-given names. Other expressions use _eN.
func (w *Writer) writeEmit(emit ir.StmtEmit) error {
	for handle := emit.Range.Start; handle < emit.Range.End; handle++ {
		if err := w.maybeEmitExpression(handle); err != nil {
			return err
		}
	}
	return nil
}

// shouldBakeExpression determines whether an expression needs to be baked
// into a temporary variable. Matches Rust naga's baking logic:
//   - Named expressions → always bake (they represent WGSL let/var names)
//   - Expressions with bake_ref_count==1 (Load, ImageSample, ImageLoad, Derivative) → always bake
//   - Expressions used multiple times → bake (we approximate via needBakeExpression set)
//   - Access/AccessIndex → never bake
//   - Everything else → don't bake (inline at use site)
func (w *Writer) shouldBakeExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}

	// Named expressions are baked, UNLESS the expression is a Literal
	// (after process_overrides const-eval, expressions like `let a = !override_bool`
	// become Literal(true) and should inline, not bake).
	if w.currentFunction.NamedExpressions != nil {
		if _, ok := w.currentFunction.NamedExpressions[handle]; ok {
			if _, isLit := w.currentFunction.Expressions[handle].Kind.(ir.Literal); !isLit {
				return true
			}
		}
	}

	// Explicitly marked for baking
	if _, ok := w.needBakeExpression[handle]; ok {
		return true
	}

	// Expression types that always need baking (bake_ref_count == 1)
	expr := &w.currentFunction.Expressions[handle]
	switch expr.Kind.(type) {
	case ir.ExprLoad:
		return true
	case ir.ExprImageSample:
		return true
	case ir.ExprImageLoad:
		return true
	case ir.ExprDerivative:
		return true
	}

	return false
}

// maybeEmitExpression decides whether to bake an expression into a temporary
// or leave it for inlining. Matches Rust naga's Emit statement handling.
func (w *Writer) maybeEmitExpression(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
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

	// Only bake if the expression should be baked
	if !w.shouldBakeExpression(handle) {
		return nil
	}

	// Pre-emit clamped LOD for Restrict-policy ImageLoad on mipmapped sampled images.
	// Rust naga emits this as a separate statement BEFORE baking the ImageLoad expression.
	// Pattern: int _eN_clamped_lod = clamp(level, 0, textureQueryLevels(image) - 1);
	if w.options.BoundsCheckPolicies.ImageLoad == BoundsCheckRestrict {
		if err := w.maybeEmitClampedLod(handle); err != nil {
			return err
		}
	}

	// Determine expression type using ExpressionTypes.
	// Use base type + array suffix for C-style array declarations.
	resolution := &w.currentFunction.ExpressionTypes[handle]
	baseType := ""
	arraySuffix := ""
	if resolution.Handle != nil {
		baseType = w.getBaseTypeName(*resolution.Handle)
		arraySuffix = w.getArraySuffix(*resolution.Handle)
	} else if resolution.Value != nil {
		if at, ok := resolution.Value.(ir.ArrayType); ok {
			baseType = w.typeInnerToGLSL(w.module.Types[at.Base].Inner)
			if at.Size.Constant != nil {
				arraySuffix = fmt.Sprintf("[%d]", *at.Size.Constant)
			} else {
				arraySuffix = "[]"
			}
		} else {
			baseType = w.typeInnerToGLSL(resolution.Value)
		}
	}
	// Fallback: if ExpressionTypes didn't populate this slot, try resolving
	// the type dynamically from the expression itself.
	if baseType == "" && w.module != nil && w.currentFunction != nil {
		resolved, err := ir.ResolveExpressionType(w.module, w.currentFunction, handle)
		if err == nil {
			if resolved.Handle != nil {
				baseType = w.getBaseTypeName(*resolved.Handle)
				arraySuffix = w.getArraySuffix(*resolved.Handle)
			} else if resolved.Value != nil {
				if at, ok := resolved.Value.(ir.ArrayType); ok {
					baseType = w.typeInnerToGLSL(w.module.Types[at.Base].Inner)
					if at.Size.Constant != nil {
						arraySuffix = fmt.Sprintf("[%d]", *at.Size.Constant)
					} else {
						arraySuffix = "[]"
					}
				} else {
					baseType = w.typeInnerToGLSL(resolved.Value)
				}
			}
		}
	}
	if baseType == "" {
		return nil // No type info — skip baking
	}

	// Determine name: use IR named expression or auto-generated _eN
	tempName := fmt.Sprintf("_e%d", handle)
	if w.currentFunction.NamedExpressions != nil {
		if irName, ok := w.currentFunction.NamedExpressions[handle]; ok {
			tempName = w.namer.call(irName)
		}
	}

	// Write the expression BEFORE registering the name, so that
	// writeExpression expands the actual expression rather than
	// returning the name itself (which would produce "int _e5 = _e5;").
	exprStr, err := w.writeExpression(handle)
	if err != nil {
		return err
	}
	w.writeLine("%s %s%s = %s;", baseType, tempName, arraySuffix, exprStr)

	// Cache the name for subsequent references
	w.namedExpressions[handle] = tempName
	return nil
}

// isPointerExpression returns true if the expression has a pointer type.
// Pointer expressions are never baked — they are address-space references.
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

func (w *Writer) isPointerExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}

	// Type-based check
	resolution := &w.currentFunction.ExpressionTypes[handle]
	if resolution.Handle != nil && w.module != nil {
		if int(*resolution.Handle) < len(w.module.Types) {
			_, isPtr := w.module.Types[*resolution.Handle].Inner.(ir.PointerType)
			if isPtr {
				return true
			}
		}
	}
	if resolution.Value != nil {
		if _, isPtr := resolution.Value.(ir.PointerType); isPtr {
			return true
		}
	}

	// Structural check: AccessIndex/Access on a variable reference is an l-value.
	// These must not be baked — they are store targets (struct field pointers).
	// Matches Rust naga's needs_baking which treats these as Named::Access.
	if int(handle) < len(w.currentFunction.Expressions) {
		switch k := w.currentFunction.Expressions[handle].Kind.(type) {
		case ir.ExprAccessIndex:
			return w.isVariableReference(k.Base) || w.isPointerExpression(k.Base)
		case ir.ExprAccess:
			return w.isVariableReference(k.Base) || w.isPointerExpression(k.Base)
		}
	}

	return false
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
// When all cases except the last are empty fall-through, emit as
// do { body } while(false); instead (workaround for wgpu#4514).
func (w *Writer) writeSwitch(switchStmt ir.StmtSwitch) error {
	// Check if this is a "one body" switch — all cases except last are empty fall-through.
	// Rust naga converts these to do { } while(false) for GLSL compatibility.
	if w.isSingleBodySwitch(switchStmt) {
		return w.writeSwitchAsDoWhile(switchStmt)
	}

	selector, err := w.writeExpression(switchStmt.Selector)
	if err != nil {
		return err
	}

	w.writeLine("switch(%s) {", selector)
	w.pushIndent()

	for _, switchCase := range switchStmt.Cases {
		// Rust naga: braces only when case has body or is not fall-through
		writeBraces := !(switchCase.FallThrough && len(switchCase.Body) == 0)

		switch v := switchCase.Value.(type) {
		case ir.SwitchValueI32:
			if writeBraces {
				w.writeLine("case %d: {", int32(v))
			} else {
				w.writeLine("case %d:", int32(v))
			}
		case ir.SwitchValueU32:
			if writeBraces {
				w.writeLine("case %du: {", uint32(v))
			} else {
				w.writeLine("case %du:", uint32(v))
			}
		case ir.SwitchValueDefault:
			if writeBraces {
				w.writeLine("default: {")
			} else {
				w.writeLine("default:")
			}
		}

		if writeBraces {
			w.pushIndent()
			if err := w.writeBlock(switchCase.Body); err != nil {
				return err
			}
			if !switchCase.FallThrough && !blockEndsWithTerminator(switchCase.Body) {
				w.writeLine("break;")
			}
			w.popIndent()
			w.writeLine("}")
		}
	}

	w.popIndent()
	w.writeLine("}")
	return nil
}

// isSingleBodySwitch checks if all cases except the last are empty fall-through.
func (w *Writer) isSingleBodySwitch(switchStmt ir.StmtSwitch) bool {
	if len(switchStmt.Cases) == 0 {
		return false
	}
	for i := 0; i < len(switchStmt.Cases)-1; i++ {
		c := &switchStmt.Cases[i]
		if !c.FallThrough || len(c.Body) > 0 {
			return false
		}
	}
	return true
}

// writeSwitchAsDoWhile emits a single-body switch as do { body } while(false).
// Uses continue_forward to handle continue statements inside the do-while,
// since the do-while loop would otherwise capture them.
func (w *Writer) writeSwitchAsDoWhile(switchStmt ir.StmtSwitch) error {
	// Enter switch for continue forwarding (only matters inside a loop)
	if variable := w.continueCtx.enterSwitch(w.namer); variable != "" {
		w.writeLine("bool %s = false;", variable)
	}

	w.writeLine("do {")
	w.pushIndent()

	// Write the last case body
	if last := &switchStmt.Cases[len(switchStmt.Cases)-1]; len(last.Body) > 0 {
		if err := w.writeBlock(last.Body); err != nil {
			return err
		}
	}

	w.popIndent()
	w.writeLine("} while(false);")

	// Handle any forwarded continue statements
	result := w.continueCtx.exitSwitch()
	switch result.kind {
	case exitContinue:
		w.writeLine("if (%s) {", result.variable)
		w.pushIndent()
		w.writeLine("continue;")
		w.popIndent()
		w.writeLine("}")
	case exitBreak:
		w.writeLine("if (%s) {", result.variable)
		w.pushIndent()
		w.writeLine("break;")
		w.popIndent()
		w.writeLine("}")
	}

	return nil
}

// writeLoop writes a loop statement.
// Matches Rust naga's GLSL emission pattern:
//   - Simple loop (no continuing, no break_if): while(true) { body }
//   - Loop with continuing/break_if: uses loop_init gate pattern:
//     bool loop_init = true;
//     while(true) {
//     if (!loop_init) { <continuing>; if (<break_if>) { break; } }
//     loop_init = false;
//     <body>
//     }
func (w *Writer) writeLoop(loop ir.StmtLoop) error {
	w.continueCtx.enterLoop()

	hasContinuing := len(loop.Continuing) > 0
	hasBreakIf := loop.BreakIf != nil

	if hasContinuing || hasBreakIf {
		// Loops with continuing block or break-if use the loop_init gate pattern
		gateName := w.namer.call("loop_init")
		w.writeLine("bool %s = true;", gateName)
		w.writeLine("while(true) {")
		w.pushIndent()

		// Continuing block runs on every iteration except the first
		w.writeLine("if (!%s) {", gateName)
		w.pushIndent()

		if hasContinuing {
			if err := w.writeBlock(loop.Continuing); err != nil {
				return err
			}
		}

		if hasBreakIf {
			condition, err := w.writeExpression(*loop.BreakIf)
			if err != nil {
				return err
			}
			w.writeLine("if (%s) {", condition)
			w.pushIndent()
			w.writeLine("break;")
			w.popIndent()
			w.writeLine("}")
		}

		w.popIndent()
		w.writeLine("}")
		w.writeLine("%s = false;", gateName)
	} else {
		// Simple loop — no continuing, no break-if
		w.writeLine("while(true) {")
		w.pushIndent()
	}

	// Write body
	if err := w.writeBlock(loop.Body); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")
	w.continueCtx.exitLoop()
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
		// For vertex position output, add coordinate space adjustment and point size
		if b.Builtin == ir.BuiltinPosition {
			if w.options.WriterFlags&WriterFlagAdjustCoordinateSpace != 0 {
				w.writeLine("gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);")
			}
			if w.options.WriterFlags&WriterFlagForcePointSize != 0 {
				w.writeLine("gl_PointSize = 1.0;")
			}
		}
		w.writeLine("return;")
	case ir.LocationBinding:
		// Use the varying output name matching writeVaryingDeclarations
		ep := w.getSelectedEntryPoint()
		varName := w.varyingName(int(b.Location), ep.Stage, true)
		w.writeLine("%s = %s;", varName, value)
		w.writeLine("return;")
	default:
		w.writeLine("return %s;", value)
	}
	return nil
}

// writeStructReturn expands a struct return value into individual output assignments.
// The return value can be:
// - An ExprCompose (constructing the struct from individual values)
// - A local variable or other expression referencing a struct
func (w *Writer) writeStructReturn(ret ir.StmtReturn, info *epStructInfo) error {
	// Rust naga: for Compose expressions, create _tmp_return struct, then assign members.
	// For other expressions, evaluate once and assign members.
	tmpName := "_tmp_return"
	if w.currentFunction != nil && int(*ret.Value) < len(w.currentFunction.Expressions) {
		if _, ok := w.currentFunction.Expressions[*ret.Value].Kind.(ir.ExprCompose); ok {
			// Create temp struct from Compose
			structName := w.getTypeName(info.structType)
			composeStr, err := w.writeExpression(*ret.Value)
			if err != nil {
				return err
			}
			w.writeLine("%s %s = %s;", structName, tmpName, composeStr)

			// Assign each member from temp
			if int(info.structType) < len(w.module.Types) {
				if st, ok := w.module.Types[info.structType].Inner.(ir.StructType); ok {
					for memberIdx, memberInfo := range info.members {
						if memberIdx >= len(st.Members) {
							break
						}
						memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(info.structType), handle2: uint32(memberIdx)}]
						w.writeLine("%s = %s.%s;", memberInfo.glslName, tmpName, memberName)
					}
				}
			}
			w.writeCoordinateAdjustIfNeeded(info)
			w.writeLine("return;")
			return nil
		}
	}

	// General case: evaluate expression once and assign members.
	value, err := w.writeExpression(*ret.Value)
	if err != nil {
		return err
	}

	if int(info.structType) < len(w.module.Types) {
		if st, ok := w.module.Types[info.structType].Inner.(ir.StructType); ok {
			for memberIdx, memberInfo := range info.members {
				if memberIdx >= len(st.Members) {
					break
				}
				memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(info.structType), handle2: uint32(memberIdx)}]
				w.writeLine("%s = %s.%s;", memberInfo.glslName, value, memberName)
			}
			w.writeCoordinateAdjustIfNeeded(info)
			w.writeLine("return;")
			return nil
		}
	}

	w.writeLine("return %s;", value)
	return nil
}

// writeCoordinateAdjustIfNeeded adds gl_Position coordinate space adjustment
// and optional gl_PointSize for vertex struct returns.
func (w *Writer) writeCoordinateAdjustIfNeeded(info *epStructInfo) {
	for _, member := range info.members {
		if member.isBuiltin && member.builtinName == "gl_Position" {
			if w.options.WriterFlags&WriterFlagAdjustCoordinateSpace != 0 {
				w.writeLine("gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);")
			}
			if w.options.WriterFlags&WriterFlagForcePointSize != 0 {
				w.writeLine("gl_PointSize = 1.0;")
			}
			return
		}
	}
}

// writeBarrier writes a control barrier statement.
// Matches Rust naga's write_control_barrier: memory barriers first, then barrier().
// Memory barriers: STORAGE→memoryBarrierBuffer, WORK_GROUP→memoryBarrierShared,
// SUB_GROUP→subgroupMemoryBarrier, TEXTURE→memoryBarrierImage.
func (w *Writer) writeBarrier(barrier ir.StmtBarrier) error {
	// Write memory barriers based on flags
	if barrier.Flags&ir.BarrierStorage != 0 {
		w.writeLine("memoryBarrierBuffer();")
	}
	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		w.writeLine("memoryBarrierShared();")
	}
	if barrier.Flags&ir.BarrierSubGroup != 0 {
		w.writeLine("subgroupMemoryBarrier();")
	}
	if barrier.Flags&ir.BarrierTexture != 0 {
		w.writeLine("memoryBarrierImage();")
	}
	// Execution barrier always follows memory barriers
	w.writeLine("barrier();")
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
// Matches Rust naga: uses write_texture_coord for coordinate vector construction,
// including array index merging and uint-to-int conversion.
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

	imgType := w.resolveImageType(imgStore.Image)
	coordStr := w.buildTextureCoord(coordinate, imgStore.Coordinate, imgStore.ArrayIndex, imgType)

	w.writeLine("imageStore(%s, %s, %s);", image, coordStr, value)
	return nil
}

// writeImageAtomic writes an image atomic operation statement.
// Matches Rust naga's write_image_atomic: imageAtomicFun(image, coord, value);
func (w *Writer) writeImageAtomic(imgAtomic ir.StmtImageAtomic) error {
	image, err := w.writeExpression(imgAtomic.Image)
	if err != nil {
		return err
	}
	coordinate, err := w.writeExpression(imgAtomic.Coordinate)
	if err != nil {
		return err
	}
	value, err := w.writeExpression(imgAtomic.Value)
	if err != nil {
		return err
	}

	// Map atomic function to GLSL imageAtomic name
	funStr := ""
	switch imgAtomic.Fun.(type) {
	case ir.AtomicAdd:
		funStr = "Add"
	case ir.AtomicSubtract:
		funStr = "Add" // Subtract emulated via atomicAdd with negated value
	case ir.AtomicAnd:
		funStr = "And"
	case ir.AtomicInclusiveOr:
		funStr = "Or"
	case ir.AtomicExclusiveOr:
		funStr = "Xor"
	case ir.AtomicMin:
		funStr = "Min"
	case ir.AtomicMax:
		funStr = "Max"
	case ir.AtomicExchange:
		funStr = "Exchange"
	default:
		return fmt.Errorf("unsupported image atomic function: %T", imgAtomic.Fun)
	}

	// Handle Subtract by negating value
	if _, isSub := imgAtomic.Fun.(ir.AtomicSubtract); isSub {
		value = fmt.Sprintf("-%s", value)
	}

	// Build coordinate (including array index if present)
	coord := coordinate
	if imgAtomic.ArrayIndex != nil {
		arrayIdx, err := w.writeExpression(*imgAtomic.ArrayIndex)
		if err != nil {
			return err
		}
		coord = fmt.Sprintf("ivec3(%s, %s)", coordinate, arrayIdx)
	}

	w.writeLine("imageAtomic%s(%s, %s, %s);", funStr, image, coord, value)
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
		// GLSL doesn't have atomicSub, use atomicAdd with negated value.
		// Rust naga emits "-" before the expression without parentheses:
		// atomicAdd(ptr, -1u) not atomicAdd(ptr, -(1u))
		funcName = "atomicAdd"
		value = fmt.Sprintf("-%s", value)
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
		tempName := fmt.Sprintf("_e%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		// Determine type from ExpressionTypes
		typeName := "uint"
		if w.currentFunction != nil && int(*atomic.Result) < len(w.currentFunction.ExpressionTypes) {
			res := &w.currentFunction.ExpressionTypes[*atomic.Result]
			if res.Handle != nil {
				typeName = w.getTypeName(*res.Handle)
			}
		}
		w.writeLine("%s %s = %s(%s, %s);", typeName, tempName, funcName, pointer, value)
	} else {
		w.writeLine("%s(%s, %s);", funcName, pointer, value)
	}
	return nil
}

// writeAtomicCompareExchange writes an atomic compare-exchange operation.
// Matches Rust naga: declares struct result, calls atomicCompSwap, sets .exchanged.
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
		tempName := fmt.Sprintf("_e%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName

		// Determine result struct type from ExpressionTypes
		resultType := "_atomic_compare_exchange_result"
		if w.currentFunction != nil && int(*atomic.Result) < len(w.currentFunction.ExpressionTypes) {
			res := &w.currentFunction.ExpressionTypes[*atomic.Result]
			if res.Handle != nil {
				resultType = w.getTypeName(*res.Handle)
			}
		}

		// Rust pattern: declare struct, call atomicCompSwap, set exchanged
		w.writeLine("%s %s; %s.old_value = atomicCompSwap(%s, %s, %s);",
			resultType, tempName, tempName, pointer, compareVal, exchangeVal)
		w.writeLine("%s.exchanged = (%s.old_value == %s);", tempName, tempName, compareVal)
	} else {
		w.writeLine("atomicCompSwap(%s, %s, %s);", pointer, compareVal, exchangeVal)
	}
	return nil
}

// writeCall writes a function call statement.
func (w *Writer) writeCall(call ir.StmtCall) error {
	// Get function name
	funcName := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(call.Function)}]

	// Write arguments, filtering out sampler args (GLSL uses combined texture-sampler).
	// Matches Rust naga: filter_map(|(i, arg)| { if callee.args[i].ty is Sampler { None } else { Some } })
	callee := &w.module.Functions[call.Function]
	argStrs := make([]string, 0, len(call.Arguments))
	for i, arg := range call.Arguments {
		// Skip sampler arguments
		if i < len(callee.Arguments) {
			argType := callee.Arguments[i].Type
			if int(argType) < len(w.module.Types) {
				if _, isSampler := w.module.Types[argType].Inner.(ir.SamplerType); isSampler {
					continue
				}
			}
		}
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
		tempName := fmt.Sprintf("_e%d", *call.Result)
		w.namedExpressions[*call.Result] = tempName
		// Determine type from ExpressionTypes — use C-style array syntax
		baseType := "/* type */"
		arraySuffix := ""
		if w.currentFunction != nil && int(*call.Result) < len(w.currentFunction.ExpressionTypes) {
			resolution := &w.currentFunction.ExpressionTypes[*call.Result]
			if resolution.Handle != nil {
				baseType = w.getBaseTypeName(*resolution.Handle)
				arraySuffix = w.getArraySuffix(*resolution.Handle)
			}
		}
		w.writeLine("%s %s%s = %s;", baseType, tempName, arraySuffix, callExpr)
	} else {
		w.writeLine("%s;", callExpr)
	}
	return nil
}

// writeSubgroupBallot writes a subgroup ballot statement.
func (w *Writer) writeSubgroupBallot(s ir.StmtSubgroupBallot) error {
	typeName := "uvec4"
	if w.currentFunction != nil && int(s.Result) < len(w.currentFunction.ExpressionTypes) {
		if res := &w.currentFunction.ExpressionTypes[s.Result]; res.Handle != nil {
			typeName = w.getTypeName(*res.Handle)
		}
	}
	tempName := fmt.Sprintf("_e%d", s.Result)
	w.namedExpressions[s.Result] = tempName

	predicate := "true"
	if s.Predicate != nil {
		p, err := w.writeExpression(*s.Predicate)
		if err != nil {
			return err
		}
		predicate = p
	}
	w.writeLine("%s %s = subgroupBallot(%s);", typeName, tempName, predicate)
	return nil
}

// writeSubgroupCollective writes a subgroup collective operation.
func (w *Writer) writeSubgroupCollective(s ir.StmtSubgroupCollectiveOperation) error {
	typeName := "uint"
	if w.currentFunction != nil && int(s.Result) < len(w.currentFunction.ExpressionTypes) {
		if res := &w.currentFunction.ExpressionTypes[s.Result]; res.Handle != nil {
			typeName = w.getTypeName(*res.Handle)
		}
	}
	tempName := fmt.Sprintf("_e%d", s.Result)
	w.namedExpressions[s.Result] = tempName

	arg, err := w.writeExpression(s.Argument)
	if err != nil {
		return err
	}

	funcName := "subgroupAdd" // default
	switch s.CollectiveOp {
	case ir.CollectiveReduce:
		switch s.Op {
		case ir.SubgroupOperationAll:
			funcName = "subgroupAll"
		case ir.SubgroupOperationAny:
			funcName = "subgroupAny"
		case ir.SubgroupOperationAdd:
			funcName = "subgroupAdd"
		case ir.SubgroupOperationMul:
			funcName = "subgroupMul"
		case ir.SubgroupOperationMax:
			funcName = "subgroupMax"
		case ir.SubgroupOperationMin:
			funcName = "subgroupMin"
		case ir.SubgroupOperationAnd:
			funcName = "subgroupAnd"
		case ir.SubgroupOperationOr:
			funcName = "subgroupOr"
		case ir.SubgroupOperationXor:
			funcName = "subgroupXor"
		}
	case ir.CollectiveExclusiveScan:
		switch s.Op {
		case ir.SubgroupOperationAdd:
			funcName = "subgroupExclusiveAdd"
		case ir.SubgroupOperationMul:
			funcName = "subgroupExclusiveMul"
		}
	case ir.CollectiveInclusiveScan:
		switch s.Op {
		case ir.SubgroupOperationAdd:
			funcName = "subgroupInclusiveAdd"
		case ir.SubgroupOperationMul:
			funcName = "subgroupInclusiveMul"
		}
	}

	w.writeLine("%s %s = %s(%s);", typeName, tempName, funcName, arg)
	return nil
}

// writeSubgroupGather writes a subgroup gather statement.
func (w *Writer) writeSubgroupGather(s ir.StmtSubgroupGather) error {
	typeName := "uint"
	if w.currentFunction != nil && int(s.Result) < len(w.currentFunction.ExpressionTypes) {
		if res := &w.currentFunction.ExpressionTypes[s.Result]; res.Handle != nil {
			typeName = w.getTypeName(*res.Handle)
		}
	}
	tempName := fmt.Sprintf("_e%d", s.Result)
	w.namedExpressions[s.Result] = tempName

	arg, err := w.writeExpression(s.Argument)
	if err != nil {
		return err
	}

	switch m := s.Mode.(type) {
	case ir.GatherBroadcastFirst:
		w.writeLine("%s %s = subgroupBroadcastFirst(%s);", typeName, tempName, arg)
	case ir.GatherBroadcast:
		idx, idxErr := w.writeExpression(m.Index)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupBroadcast(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherShuffle:
		idx, idxErr := w.writeExpression(m.Index)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupShuffle(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherShuffleDown:
		idx, idxErr := w.writeExpression(m.Delta)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupShuffleDown(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherShuffleUp:
		idx, idxErr := w.writeExpression(m.Delta)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupShuffleUp(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherShuffleXor:
		idx, idxErr := w.writeExpression(m.Mask)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupShuffleXor(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherQuadBroadcast:
		idx, idxErr := w.writeExpression(m.Index)
		if idxErr != nil {
			return idxErr
		}
		w.writeLine("%s %s = subgroupQuadBroadcast(%s, %s);", typeName, tempName, arg, idx)
	case ir.GatherQuadSwap:
		funcName := "subgroupQuadSwapHorizontal"
		switch m.Direction {
		case ir.QuadDirectionY:
			funcName = "subgroupQuadSwapVertical"
		case ir.QuadDirectionDiagonal:
			funcName = "subgroupQuadSwapDiagonal"
		}
		w.writeLine("%s %s = %s(%s);", typeName, tempName, funcName, arg)
	default:
		w.writeLine("%s %s = subgroupBroadcastFirst(%s);", typeName, tempName, arg)
	}
	return nil
}

// writeWorkGroupUniformLoad writes a workgroup uniform load.
// Matches Rust naga: memoryBarrierShared + barrier, load, memoryBarrierShared + barrier.
func (w *Writer) writeWorkGroupUniformLoad(load ir.StmtWorkGroupUniformLoad) error {
	// First control barrier
	w.writeLine("memoryBarrierShared();")
	w.writeLine("barrier();")

	// Create result variable with proper type
	tempName := fmt.Sprintf("_e%d", load.Result)
	w.namedExpressions[load.Result] = tempName

	pointer, err := w.writeExpression(load.Pointer)
	if err != nil {
		return err
	}

	// Determine type from ExpressionTypes
	typeName := "int" // fallback
	if w.currentFunction != nil && int(load.Result) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[load.Result]
		if res.Handle != nil {
			typeName = w.getTypeName(*res.Handle)
		}
	}
	w.writeLine("%s %s = %s;", typeName, tempName, pointer)

	// Second control barrier
	w.writeLine("memoryBarrierShared();")
	w.writeLine("barrier();")
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

// maybeEmitClampedLod checks if an expression is an ImageLoad on a mipmapped sampled
// image with Restrict policy, and if so, emits a clamped LOD variable BEFORE the
// expression is baked. This matches Rust naga's write_clamped_lod (writer.rs:3774).
// Pattern: int _eN_clamped_lod = clamp(level, 0, textureQueryLevels(image) - 1);
func (w *Writer) maybeEmitClampedLod(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := &w.currentFunction.Expressions[handle]
	imgLoad, ok := expr.Kind.(ir.ExprImageLoad)
	if !ok || imgLoad.Level == nil {
		return nil
	}
	// Only for sampled (texelFetch) images, not storage
	imgType := w.resolveImageType(imgLoad.Image)
	if imgType == nil || imgType.Class != ir.ImageClassSampled {
		return nil
	}
	// Multisampled images don't have LOD
	if imgType.Multisampled {
		return nil
	}

	image, err := w.writeExpression(imgLoad.Image)
	if err != nil {
		return err
	}
	levelExpr, err := w.writeExpression(*imgLoad.Level)
	if err != nil {
		return err
	}

	w.writeLine("int _e%d_clamped_lod = clamp(%s, 0, textureQueryLevels(%s) - 1);",
		handle, levelExpr, image)
	return nil
}
