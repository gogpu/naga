package codegen

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/unsupported"
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
// After writing all statements, it un-emits expressions that were baked
// during this block, matching Rust naga's put_block behavior.
// This ensures baked names don't leak out of their block scope.
func (w *Writer) writeBlock(block ir.Block) error {
	for _, stmt := range block {
		if err := w.writeStatement(stmt); err != nil {
			return err
		}
	}
	// Un-emit: remove all expressions that were emitted in this block
	// from namedExpressions. This matches Rust naga MSL backend behavior
	// where baked expression names are scoped to their containing block.
	for _, stmt := range block {
		if emit, ok := stmt.Kind.(ir.StmtEmit); ok {
			for h := emit.Range.Start; h < emit.Range.End; h++ {
				delete(w.namedExpressions, h)
			}
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
		// Rust naga skips empty blocks entirely
		if len(k.Block) == 0 {
			return nil
		}
		w.WriteLine("{")
		w.PushIndent()
		if err := w.writeBlock(k.Block); err != nil {
			return err
		}
		w.PopIndent()
		w.WriteLine("}")
		return nil

	case ir.StmtIf:
		return w.writeIf(k)

	case ir.StmtSwitch:
		return w.writeSwitch(k)

	case ir.StmtLoop:
		return w.writeLoop(k)

	case ir.StmtBreak:
		w.WriteLine("break;")
		return nil

	case ir.StmtContinue:
		w.WriteLine("continue;")
		return nil

	case ir.StmtReturn:
		return w.writeReturn(k)

	case ir.StmtKill:
		w.WriteLine("%sdiscard_fragment();", Namespace)
		return nil

	case ir.StmtControlBarrier:
		return w.writeBarrier(k)

	case ir.StmtMemoryBarrier:
		// Metal has no barrier without execution sync; Rust naga writes
		// both statements with write_barrier (back/msl/writer.rs:4133).
		return w.writeBarrier(ir.StmtControlBarrier(k))

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
		return w.writeSubgroupCollectiveOperation(k)

	case ir.StmtSubgroupGather:
		return w.writeSubgroupGather(k)

	default:
		return unsupported.Errorf("unsupported statement kind: %T", kind)
	}
}

// writeEmit writes an emit statement (materializes expressions).
// Matches Rust naga behavior: expressions in NamedExpressions (from let bindings
// and phony assignments) are always baked with their user-given names.
// Other expressions are baked only if they are control-flow dependent (Load,
// ImageSample, ImageLoad, Derivative) or used multiple times (needBakeExpression).
// Pointer expressions are never baked (they are address-space references).
//
// Phony expressions (from _ = expr) that are side-effect-free are skipped,
// matching Rust naga where such expressions are constant-folded and never
// appear in Emit ranges.
func (w *Writer) writeEmit(emit ir.StmtEmit) error {
	for handle := emit.Range.Start; handle < emit.Range.End; handle++ {
		// Don't bake pointer expressions (matches Rust naga)
		if w.isPointerExpression(handle) {
			continue
		}

		// Don't bake variable reference expressions — they act as pointers
		// in the WGSL Load Rule model. The ExprLoad wrapping them is baked instead.
		if w.isVariableReference(handle) {
			continue
		}

		// For Metal >= 2.1, Dot4I8Packed/Dot4U8Packed need intermediate
		// packed_char4/packed_uchar4 reinterpreted variables emitted before the
		// dot product expression. Matches Rust naga put_casting_to_packed_chars.
		if !w.options.LangVersion.Less(Version2_1) {
			if err := w.writeDot4PackedCharsIfNeeded(handle); err != nil {
				return err
			}
		}

		// For Restrict bounds check policy, emit clamped_lod_eN local variable
		// before the image load expression. This variable is referenced by the
		// restrict-mode read call. Matches Rust naga put_image_load_restrict.
		if err := w.writeImageLoadClampedLodIfNeeded(handle); err != nil {
			return err
		}

		// Skip expressions already baked by an earlier statement (e.g., atomic results).
		// Matches Rust naga: atomic results are emitted inline with the atomic statement.
		if _, alreadyBaked := w.namedExpressions[handle]; alreadyBaked {
			continue
		}

		// Check if this expression has a name from the IR (let binding or phony)
		if name, ok := w.getIRNamedExpression(handle); ok {
			// Skip phony expressions that have no side effects.
			// In Rust naga, these get constant-folded at lowering time and
			// never appear in Emit ranges. Our lowerer keeps them as runtime
			// expressions, so we filter them here instead.
			if name == "phony" && w.isPureExpression(handle) {
				continue
			}
			if err := w.bakeExpressionWithName(handle, name); err != nil {
				return err
			}
			continue
		}

		if w.shouldBakeExpression(handle) {
			if err := w.bakeExpression(handle); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeImageLoadClampedLodIfNeeded emits a clamped LOD local variable before an
// ImageLoad expression when the Restrict bounds check policy is active.
// For 2D/3D images with mip levels, Rust naga emits:
//
//	uint clamped_lod_eN = metal::min(uint(level), image.get_num_mip_levels() - 1);
//
// This variable is then used in the restrict-mode read call for both coordinate
// clamping and the level argument.
func (w *Writer) writeImageLoadClampedLodIfNeeded(handle ir.ExpressionHandle) error {
	if w.options.BoundsCheckPolicies.ImageLoad != backend.BoundsCheckRestrict {
		return nil
	}
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := &w.currentFunction.Expressions[handle]
	load, ok := expr.Kind.(ir.ExprImageLoad)
	if !ok {
		return nil
	}
	// Only need clamped_lod for non-multisampled images with a level parameter
	// and dimension != 1D (1D doesn't use level in restrict mode).
	if load.Level == nil {
		return nil
	}
	imgType := w.getImageType(load.Image)
	if imgType != nil && (imgType.Multisampled || imgType.Dim == ir.Dim1D) {
		return nil
	}

	varName := fmt.Sprintf("clamped_lod_e%d", handle)
	w.WriteIndent()
	w.write("uint %s = %smin(uint(", varName, Namespace)
	if err := w.writeExpression(*load.Level); err != nil {
		return err
	}
	w.write("), ")
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".get_num_mip_levels() - 1);\n")
	return nil
}

// writeDot4PackedCharsIfNeeded emits packed_char4/packed_uchar4 reinterpreted variable
// declarations when the expression is a Dot4I8Packed or Dot4U8Packed math operation
// on Metal >= 2.1. These intermediate variables are used by the dot product expression.
// Matches Rust naga put_casting_to_packed_chars.
func (w *Writer) writeDot4PackedCharsIfNeeded(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}

	expr := &w.currentFunction.Expressions[handle]
	mathExpr, ok := expr.Kind.(ir.ExprMath)
	if !ok {
		return nil
	}

	var packedType string
	switch mathExpr.Fun {
	case ir.MathDot4I8Packed:
		packedType = mslPackedChar4
	case ir.MathDot4U8Packed:
		packedType = mslPackedUchar4
	default:
		return nil
	}

	// Emit reinterpreted variable for each argument
	args := []ir.ExpressionHandle{mathExpr.Arg}
	if mathExpr.Arg1 != nil {
		args = append(args, *mathExpr.Arg1)
	}

	for _, arg := range args {
		varName := w.reinterpretedVarName(packedType, arg)
		w.WriteIndent()
		w.write("%s %s = as_type<%s>(", packedType, varName, packedType)
		if err := w.writeExpression(arg); err != nil {
			return err
		}
		w.write(");\n")
	}

	return nil
}

// isPureExpression returns true if the expression has no side effects.
// Pure expressions include literals, binary/unary operations on pure operands,
// selects, composes, zero values, constants, and access operations.
// Expressions with side effects include function calls, image samples/loads,
// derivatives, and atomics.
func (w *Writer) isPureExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal, ir.ExprConstant, ir.ExprZeroValue, ir.ExprSplat:
		return true
	case ir.ExprBinary:
		return w.isPureExpression(k.Left) && w.isPureExpression(k.Right)
	case ir.ExprUnary:
		return w.isPureExpression(k.Expr)
	case ir.ExprSelect:
		return w.isPureExpression(k.Condition) &&
			w.isPureExpression(k.Accept) &&
			w.isPureExpression(k.Reject)
	case ir.ExprCompose:
		for _, c := range k.Components {
			if !w.isPureExpression(c) {
				return false
			}
		}
		return true
	case ir.ExprAs:
		return w.isPureExpression(k.Expr)
	case ir.ExprAccessIndex:
		return w.isPureExpression(k.Base)
	case ir.ExprAccess:
		return w.isPureExpression(k.Base) && w.isPureExpression(k.Index)
	case ir.ExprSwizzle:
		return w.isPureExpression(k.Vector)
	case ir.ExprLocalVariable, ir.ExprGlobalVariable, ir.ExprFunctionArgument:
		return true
	default:
		// Function calls, image ops, derivatives, loads, atomics — not pure
		return false
	}
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

// isPointerExpression returns true if the expression has a pointer type.
// Pointer expressions are never baked — they are address-space references.
func (w *Writer) isPointerExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	resolution := w.exprType(handle)
	if resolution.Handle != nil && w.module != nil {
		if int(*resolution.Handle) < len(w.module.Types) {
			_, isPtr := w.module.Types[*resolution.Handle].Inner.(ir.PointerType)
			if isPtr {
				return true
			}
		}
	}
	if resolution.Value != nil {
		_, isPtr := resolution.Value.(ir.PointerType)
		if isPtr {
			return true
		}
	}
	// Fallback: check if this is an Access/AccessIndex chain rooted in a
	// variable reference (GlobalVariable, LocalVariable, FunctionArgument).
	// Such chains are pointer expressions even if our type resolution
	// doesn't fully track pointer types through access chains.
	return w.isAccessChainOnVariable(handle)
}

// isAccessChainOnVariable checks if an expression is an Access or AccessIndex
// chain that ultimately roots in a variable reference (Global/Local/Argument).
// This identifies pointer expressions that our type resolution may not mark
// as PointerType, matching Rust naga's is_reference check.
func (w *Writer) isAccessChainOnVariable(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	for {
		if int(handle) >= len(w.currentFunction.Expressions) {
			return false
		}
		expr := &w.currentFunction.Expressions[handle]
		switch k := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			return true
		case ir.ExprLocalVariable:
			return true
		case ir.ExprFunctionArgument:
			// Check if the argument type is a pointer
			if int(k.Index) < len(w.currentFunction.Arguments) {
				argType := w.currentFunction.Arguments[k.Index].Type
				if int(argType) < len(w.module.Types) {
					_, isPtr := w.module.Types[argType].Inner.(ir.PointerType)
					return isPtr
				}
			}
			return false
		case ir.ExprAccessIndex:
			handle = k.Base
		case ir.ExprAccess:
			handle = k.Base
		default:
			return false
		}
	}
}

// getIRNamedExpression checks if an expression has a user-given name in the IR
// (from let bindings or phony assignments). Returns the name and true if found.
func (w *Writer) getIRNamedExpression(handle ir.ExpressionHandle) (string, bool) {
	if w.currentFunction == nil || w.currentFunction.NamedExpressions == nil {
		return "", false
	}
	name, ok := w.currentFunction.NamedExpressions[handle]
	return name, ok
}

// collectNeedBakeExpressions scans the function body for expressions that need
// to be baked (assigned to temporaries). Matches Rust naga's
// collect_needs_bake_expressions pass which uses ref_count + bake_ref_count.
//
// Baking rules (from Rust naga):
//   - Access/AccessIndex: never bake (always inline)
//   - ImageSample/ImageLoad/Derivative/Load: always bake (ref_count >= 1)
//   - Everything else: bake if referenced 2+ times
//   - Dot4 packed: both arguments baked (used 4x in expansion)
func (w *Writer) collectNeedBakeExpressions(fn *ir.Function) {
	// Count references: how many times each expression handle is used by others
	refCounts := make([]int, len(fn.Expressions))
	for i := range fn.Expressions {
		expr := &fn.Expressions[i]
		for _, ref := range exprRefs(expr.Kind) {
			if int(ref) < len(refCounts) {
				refCounts[ref]++
			}
		}
	}
	// Also count references from statements (Call arguments, Store values, etc.)
	w.countStmtExprRefs(fn.Body, refCounts)

	for i := range fn.Expressions {
		handle := ir.ExpressionHandle(i)
		expr := &fn.Expressions[i]

		minRefCount := exprBakeRefCount(expr.Kind)
		if minRefCount <= refCounts[i] {
			w.needBakeExpression[handle] = struct{}{}
		}

		// Math functions whose expansion reads an argument several times bake
		// it, as Rust's collect_needs_bake_expressions: written inline the
		// argument is computed again each time, and the expansion's operators
		// bind into it (`a & b < 0` is a & (b < 0)).
		if mathExpr, ok := expr.Kind.(ir.ExprMath); ok {
			switch mathExpr.Fun {
			case ir.MathDot4I8Packed, ir.MathDot4U8Packed:
				if !w.isLiteralExpression(fn, mathExpr.Arg) {
					w.needBakeExpression[mathExpr.Arg] = struct{}{}
				}
				if mathExpr.Arg1 != nil {
					if !w.isLiteralExpression(fn, *mathExpr.Arg1) {
						w.needBakeExpression[*mathExpr.Arg1] = struct{}{}
					}
				}
			case ir.MathFirstLeadingBit:
				w.needBakeExpression[mathExpr.Arg] = struct{}{}
			case ir.MathPack4xI8, ir.MathPack4xU8, ir.MathPack4xI8Clamp, ir.MathPack4xU8Clamp,
				ir.MathUnpack4xI8, ir.MathUnpack4xU8:
				// The polyfill below MSL 2.1 reads the argument four times.
				if w.options.LangVersion.Less(Version2_1) {
					w.needBakeExpression[mathExpr.Arg] = struct{}{}
				}
			case ir.MathExtractBits:
				// Only the offset is read twice.
				if mathExpr.Arg1 != nil {
					w.needBakeExpression[*mathExpr.Arg1] = struct{}{}
				}
			case ir.MathInsertBits:
				// Only the offset is read twice.
				if mathExpr.Arg2 != nil {
					w.needBakeExpression[*mathExpr.Arg2] = struct{}{}
				}
			case ir.MathSign:
				// Metal's sign takes floats only; the integer one is written
				// inline and reads its argument twice.
				sint := false
				switch inner := w.resolutionInner(w.fnExprType(fn, handle)).(type) {
				case ir.ScalarType:
					sint = inner.Kind == ir.ScalarSint
				case ir.VectorType:
					sint = inner.Scalar.Kind == ir.ScalarSint
				}
				if sint {
					w.needBakeExpression[mathExpr.Arg] = struct{}{}
				}
			}
		}
	}
}

// exprBakeRefCount returns the minimum reference count for baking an expression.
// Matches Rust naga's Expression::bake_ref_count().
func exprBakeRefCount(kind ir.ExpressionKind) int {
	switch kind.(type) {
	case ir.ExprAccess, ir.ExprAccessIndex:
		return int(^uint(0) >> 1) // MAX: never bake
	case ir.ExprImageSample, ir.ExprImageLoad, ir.ExprDerivative, ir.ExprLoad:
		return 1 // always bake
	default:
		return 2 // bake if used 2+ times
	}
}

// exprRefs returns all expression handles referenced by an expression kind.
func exprRefs(kind ir.ExpressionKind) []ir.ExpressionHandle {
	var refs []ir.ExpressionHandle
	switch k := kind.(type) {
	case ir.ExprAccess:
		refs = append(refs, k.Base, k.Index)
	case ir.ExprAccessIndex:
		refs = append(refs, k.Base)
	case ir.ExprSplat:
		refs = append(refs, k.Value)
	case ir.ExprSwizzle:
		refs = append(refs, k.Vector)
	case ir.ExprCompose:
		refs = append(refs, k.Components...)
	case ir.ExprLoad:
		refs = append(refs, k.Pointer)
	case ir.ExprUnary:
		refs = append(refs, k.Expr)
	case ir.ExprBinary:
		refs = append(refs, k.Left, k.Right)
	case ir.ExprSelect:
		refs = append(refs, k.Condition, k.Accept, k.Reject)
	case ir.ExprAs:
		refs = append(refs, k.Expr)
	case ir.ExprMath:
		refs = append(refs, k.Arg)
		if k.Arg1 != nil {
			refs = append(refs, *k.Arg1)
		}
		if k.Arg2 != nil {
			refs = append(refs, *k.Arg2)
		}
		if k.Arg3 != nil {
			refs = append(refs, *k.Arg3)
		}
	case ir.ExprImageSample:
		refs = append(refs, k.Image, k.Sampler, k.Coordinate)
		if k.ArrayIndex != nil {
			refs = append(refs, *k.ArrayIndex)
		}
		// Level is a SampleLevel interface — extract refs from concrete types
		switch lv := k.Level.(type) {
		case ir.SampleLevelExact:
			refs = append(refs, lv.Level)
		case ir.SampleLevelBias:
			refs = append(refs, lv.Bias)
		case ir.SampleLevelGradient:
			refs = append(refs, lv.X, lv.Y)
		}
		if k.DepthRef != nil {
			refs = append(refs, *k.DepthRef)
		}
		if k.Offset != nil {
			refs = append(refs, *k.Offset)
		}
	case ir.ExprImageLoad:
		refs = append(refs, k.Image, k.Coordinate)
		if k.ArrayIndex != nil {
			refs = append(refs, *k.ArrayIndex)
		}
		if k.Sample != nil {
			refs = append(refs, *k.Sample)
		}
		if k.Level != nil {
			refs = append(refs, *k.Level)
		}
	case ir.ExprImageQuery:
		refs = append(refs, k.Image)
	case ir.ExprRelational:
		refs = append(refs, k.Argument)
	case ir.ExprArrayLength:
		refs = append(refs, k.Array)
	case ir.ExprRayQueryGetIntersection:
		refs = append(refs, k.Query)
	}
	return refs
}

// countStmtExprRefs counts expression references from statements.
func (w *Writer) countStmtExprRefs(stmts []ir.Statement, refCounts []int) {
	incr := func(h ir.ExpressionHandle) {
		if int(h) < len(refCounts) {
			refCounts[h]++
		}
	}
	for _, stmt := range stmts {
		switch k := stmt.Kind.(type) {
		case ir.StmtEmit:
			// Emit doesn't add refs
		case ir.StmtStore:
			incr(k.Pointer)
			incr(k.Value)
		case ir.StmtCall:
			for _, arg := range k.Arguments {
				incr(arg)
			}
		case ir.StmtIf:
			incr(k.Condition)
			w.countStmtExprRefs(k.Accept, refCounts)
			w.countStmtExprRefs(k.Reject, refCounts)
		case ir.StmtSwitch:
			incr(k.Selector)
			for _, c := range k.Cases {
				w.countStmtExprRefs(c.Body, refCounts)
			}
		case ir.StmtLoop:
			w.countStmtExprRefs(k.Body, refCounts)
			w.countStmtExprRefs(k.Continuing, refCounts)
			if k.BreakIf != nil {
				incr(*k.BreakIf)
			}
		case ir.StmtReturn:
			if k.Value != nil {
				incr(*k.Value)
			}
		case ir.StmtImageStore:
			incr(k.Image)
			incr(k.Coordinate)
			incr(k.Value)
			if k.ArrayIndex != nil {
				incr(*k.ArrayIndex)
			}
		case ir.StmtAtomic:
			incr(k.Pointer)
			incr(k.Value)
		case ir.StmtRayQuery:
			incr(k.Query)
			switch rqf := k.Fun.(type) {
			case ir.RayQueryInitialize:
				incr(rqf.AccelerationStructure)
				// The descriptor is referenced 7+ times in the expanded MSL code
				// (flags checks, ray construction, cull_mask). Count enough to
				// trigger baking.
				for range 7 {
					incr(rqf.Descriptor)
				}
			case ir.RayQueryProceed:
				incr(rqf.Result)
			case ir.RayQueryGenerateIntersection:
				incr(rqf.HitT)
			}
		}
	}
}

// findGuardedIndices scans all expressions to find indices used in RZSW-policy accesses.
// These indices will be used twice in the generated code (once in the bounds check condition,
// once in the actual access) so they need to be baked into temporaries.
// Matches Rust naga's find_checked_indexes.
func (w *Writer) findGuardedIndices(fn *ir.Function) {
	if !policiesContain(w.options.BoundsCheckPolicies, backend.BoundsCheckReadZeroSkipWrite) {
		return
	}
	for i := range fn.Expressions {
		expr := &fn.Expressions[i]
		switch k := expr.Kind.(type) {
		case ir.ExprAccess:
			if w.chooseBoundsCheckPolicy(k.Base) == backend.BoundsCheckReadZeroSkipWrite {
				if w.accessNeedsCheck(k.Base, k.Index) {
					w.guardedIndices[k.Index] = struct{}{}
				}
			}
		}
	}
}

// accessNeedsCheck returns true if a dynamic access to base with the given index
// needs a runtime bounds check (i.e., the index is not statically known to be in bounds).
// Matches Rust naga's access_needs_check.
func (w *Writer) accessNeedsCheck(base, index ir.ExpressionHandle) bool {
	length, isDynamic := w.indexableLengthOrDynamic(base)
	if isDynamic {
		return true // dynamic arrays always need checking
	}
	if length == 0 {
		return false // unknown, skip
	}
	// Check if index is a known constant
	if w.currentFunction != nil && int(index) < len(w.currentFunction.Expressions) {
		indexExpr := &w.currentFunction.Expressions[index]
		if lit, ok := indexExpr.Kind.(ir.Literal); ok {
			var indexVal uint32
			switch v := lit.Value.(type) {
			case ir.LiteralI32:
				if int32(v) < 0 {
					return true // negative index always needs check
				}
				indexVal = uint32(v)
			case ir.LiteralU32:
				indexVal = uint32(v)
			default:
				return true
			}
			if indexVal < length {
				return false // statically in bounds
			}
		}
	}
	return true
}

// isLiteralExpression returns true if the expression at the given handle is a Literal.
func (w *Writer) isLiteralExpression(fn *ir.Function, handle ir.ExpressionHandle) bool {
	if int(handle) >= len(fn.Expressions) {
		return false
	}
	_, ok := fn.Expressions[handle].Kind.(ir.Literal)
	return ok
}

// shouldBakeExpression returns true if the expression should be assigned to a
// named temporary variable. Matches Rust naga's bake_ref_count logic:
//   - Load: always bake (control-flow dependent, ref_count threshold = 1)
//   - ImageSample, ImageLoad: always bake (control-flow dependent)
//   - Derivative: always bake (control-flow dependent)
//   - Expressions in needBakeExpression: bake (used multiple times)
//   - Pointer expressions: never bake
//   - Everything else: inline (don't bake)
func (w *Writer) shouldBakeExpression(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}

	// Check if explicitly marked for baking
	if _, ok := w.needBakeExpression[handle]; ok {
		return true
	}

	// Check if this is a guarded index (RZSW bounds check).
	// These are used twice: once in the condition, once in the access.
	if _, ok := w.guardedIndices[handle]; ok {
		return true
	}

	// Check expression kind
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
	default:
		return false
	}
}

// emitBreakIfSubExpressions walks the break-if condition expression and bakes
// any Load sub-expressions. This matches Rust naga behavior where break-if
// sub-expressions are emitted (Loads are baked) before the if check, but the
// condition itself is written inline using the original variable names.
func (w *Writer) emitBreakIfSubExpressions(condHandle ir.ExpressionHandle) error {
	if w.currentFunction == nil || int(condHandle) >= len(w.currentFunction.Expressions) {
		return nil
	}

	expr := &w.currentFunction.Expressions[condHandle]

	// For Binary expressions, check both operands
	if binary, ok := expr.Kind.(ir.ExprBinary); ok {
		if err := w.emitBreakIfSubExpressions(binary.Left); err != nil {
			return err
		}
		if err := w.emitBreakIfSubExpressions(binary.Right); err != nil {
			return err
		}
		return nil
	}

	// Bake Load expressions (matches Rust bake_ref_count=1 for Load)
	if _, ok := expr.Kind.(ir.ExprLoad); ok {
		if _, alreadyBaked := w.namedExpressions[condHandle]; !alreadyBaked {
			if err := w.bakeExpression(condHandle); err != nil {
				return err
			}
			// Remove from namedExpressions so the condition writes
			// the original variable name, not the baked name.
			// The baked statement is still emitted as a side effect.
			delete(w.namedExpressions, condHandle)
		}
	}

	return nil
}

// bakeExpression creates a temporary variable for an expression with an auto-generated name.
// If its type has no MSL name, the expression
// is skipped and will be inlined at use site instead.
func (w *Writer) bakeExpression(handle ir.ExpressionHandle) error {
	return w.bakeExpressionImpl(handle, fmt.Sprintf("_e%d", handle), false)
}

// bakeExpressionWithName creates a temporary variable for an expression with a user-given name.
// The namer is used for collision avoidance (e.g., "phony" -> "phony", "phony_1", etc.).
func (w *Writer) bakeExpressionWithName(handle ir.ExpressionHandle, name string) error {
	return w.bakeExpressionImpl(handle, name, true)
}

// bakeExpressionImpl is the shared implementation for baking an expression.
// If useNamer is true, the name is passed through the namer for collision avoidance.
func (w *Writer) bakeExpressionImpl(handle ir.ExpressionHandle, name string, useNamer bool) error {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}

	// Determine expression type
	resolution := w.exprType(handle)
	typeName := ""
	if resolution.Handle != nil {
		typeName = w.writeTypeName(*resolution.Handle, StorageAccess(0))
	} else if resolution.Value != nil {
		typeName = typeInnerToMSL(resolution.Value)
	}
	if typeName == "" {
		return nil // A type MSL can't name: the expression is inlined at its use
	}

	tempName := name
	if useNamer {
		tempName = w.namer.call(name)
	}
	w.namedExpressions[handle] = tempName

	// Write the declaration using writeExpressionInline which does NOT
	// look up namedExpressions (avoiding self-reference).
	w.WriteIndent()
	w.write("%s %s = ", typeName, tempName)
	if err := w.writeExpressionInline(handle); err != nil {
		return err
	}
	w.write(";\n")
	return nil
}

// writeIf writes an if statement.
func (w *Writer) writeIf(ifStmt ir.StmtIf) error {
	w.WriteIndent()
	w.write("if (")
	if err := w.writeExpression(ifStmt.Condition); err != nil {
		return err
	}
	w.write(") {\n")
	w.PushIndent()
	if err := w.writeBlock(ifStmt.Accept); err != nil {
		return err
	}
	w.PopIndent()

	if len(ifStmt.Reject) > 0 {
		w.WriteLine("} else {")
		w.PushIndent()
		if err := w.writeBlock(ifStmt.Reject); err != nil {
			return err
		}
		w.PopIndent()
	}

	w.WriteLine("}")
	return nil
}

// writeSwitch writes a switch statement.
// Matches Rust naga's MSL output: no space before paren, case bodies wrapped
// in braces with break inside.
// Fallthrough cases emit label only (no braces), matching Rust's pattern:
//
//	case 3:
//	case 4: {
//	    body...
//	    break;
//	}
func (w *Writer) writeSwitch(switchStmt ir.StmtSwitch) error {
	w.WriteIndent()
	w.write("switch(")
	if err := w.writeExpression(switchStmt.Selector); err != nil {
		return err
	}
	w.write(") {\n")
	w.PushIndent()

	for _, switchCase := range switchStmt.Cases {
		if switchCase.FallThrough {
			// Fallthrough: emit label only, no braces, no body.
			// Rust naga emits "case N:" on its own line for fallthrough.
			switch v := switchCase.Value.(type) {
			case ir.SwitchValueI32:
				w.WriteLine("case %d:", int32(v))
			case ir.SwitchValueU32:
				w.WriteLine("case %du:", uint32(v))
			case ir.SwitchValueDefault:
				w.WriteLine("default:")
			}
			continue
		}

		// Non-fallthrough: emit label with opening brace, body, break, closing brace.
		switch v := switchCase.Value.(type) {
		case ir.SwitchValueI32:
			w.WriteLine("case %d: {", int32(v))
		case ir.SwitchValueU32:
			w.WriteLine("case %du: {", uint32(v))
		case ir.SwitchValueDefault:
			w.WriteLine("default: {")
		}

		w.PushIndent()
		if err := w.writeBlock(switchCase.Body); err != nil {
			return err
		}
		// Only emit break if the last statement is not a terminator (Return, Break, Continue, Kill).
		// This matches Rust naga's MSL writer (writer.rs:3697).
		if !blockEndsWithTerminator(switchCase.Body) {
			w.WriteLine("break;")
		}
		w.PopIndent()
		w.WriteLine("}")
	}

	w.PopIndent()
	w.WriteLine("}")
	return nil
}

// writeLoop writes a loop statement.
// Matches Rust naga's loop output including force_loop_bounding and
// the loop_init gate pattern for continuing/break-if blocks.
func (w *Writer) writeLoop(loop ir.StmtLoop) error {
	hasContinuing := len(loop.Continuing) > 0 || loop.BreakIf != nil

	// Generate loop bound variable name and declaration if force_loop_bounding is enabled.
	// Counts down from (2^32-1, 2^32-1) giving 2^64 iterations before forced break.
	var loopBoundName string
	if w.options.ForceLoopBounding {
		loopBoundName = w.namer.call("loop_bound")
		w.WriteLine("uint2 %s = uint2(4294967295u);", loopBoundName)
	}

	// Generate loop_init gate variable if there's a continuing/break-if block.
	// The gate skips the continuing block on the first iteration.
	var gateName string
	if hasContinuing {
		gateName = w.namer.call("loop_init")
		w.WriteLine("bool %s = true;", gateName)
	}

	w.WriteLine("while(true) {")
	w.PushIndent()

	// Loop bound check+decrement at the top of the loop body.
	if w.options.ForceLoopBounding {
		w.WriteLine("if (%sall(%s == uint2(0u))) { break; }", Namespace, loopBoundName)
		w.WriteLine("%s -= uint2(%s.y == 0u, 1u);", loopBoundName, loopBoundName)
	}

	// Continuing block + break-if, gated by !loop_init (skip on first iteration).
	if hasContinuing {
		w.WriteLine("if (!%s) {", gateName)
		w.PushIndent()

		// Write continuing block statements
		if len(loop.Continuing) > 0 {
			if err := w.writeBlock(loop.Continuing); err != nil {
				return err
			}
		}

		// Write break-if condition.
		// The break-if sub-expressions were already emitted by the continuing
		// block's Emit statement, and then un-emitted by writeBlock. So
		// writeExpression will resolve Load expressions to their original
		// variable names, matching Rust naga behavior.
		if loop.BreakIf != nil {
			w.WriteIndent()
			w.write("if (")
			if err := w.writeExpression(*loop.BreakIf); err != nil {
				return err
			}
			w.write(") {\n")
			w.PushIndent()
			w.WriteLine("break;")
			w.PopIndent()
			w.WriteLine("}")
		}

		w.PopIndent()
		w.WriteLine("}")
		w.WriteLine("%s = false;", gateName)
	}

	// Write body
	if err := w.writeBlock(loop.Body); err != nil {
		return err
	}

	w.PopIndent()
	w.WriteLine("}")
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
		// Simple type with output struct (e.g., float4 with [[position]] or [[color(0)]])
		// Use aggregate initialization: return StructName { expr };
		outputStructName := w.getOutputStructName()
		w.WriteIndent()
		w.write("return %s { ", outputStructName)
		if err := w.writeOutputMember(w.currentFunction.Result.Binding, func() error { return w.writeExpression(value) }); err != nil {
			return false, err
		}
		// Add 1.0 for forced point size in vertex shaders
		if w.entryPointStage == ir.StageVertex && w.options.AllowAndForcePointSize {
			w.write(", 1.0")
		}
		w.writeSampleMaskValue(false)
		w.write(" };\n")
		return true, nil
	}

	// Matches Rust naga pattern:
	//   const auto _tmp = <expr>;
	//   return OutputStruct { _tmp.member1, _tmp.member2 };
	outputStructName := w.getOutputStructName()

	w.WriteIndent()
	w.write("const auto _tmp = ")
	if err := w.writeExpression(value); err != nil {
		return false, err
	}
	w.write(";\n")

	w.WriteIndent()
	w.write("return %s {", outputStructName)

	isFirst := true
	for memberIdx, member := range st.Members {
		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(typeHandle), handle2: uint32(memberIdx)})
		comma := ","
		if isFirst {
			comma = ""
			isFirst = false
		}
		w.write("%s ", comma)
		// The output member is a raw array: copy the wrapped one element by
		// element, as Rust naga does.
		if _, n, ok := w.clipDistancesArray(member); ok {
			w.write("{")
			for j := range n {
				if j != 0 {
					w.write(", ")
				}
				w.write("_tmp.%s.inner[%d]", memberName, j)
			}
			w.write("}")
			continue
		}
		if err := w.writeOutputMember(member.Binding, func() error {
			w.write("_tmp.%s", memberName)
			return nil
		}); err != nil {
			return false, err
		}
	}

	// Add 1.0 for forced point size in vertex shaders.
	if w.entryPointStage == ir.StageVertex && w.options.AllowAndForcePointSize {
		// Check if the result type already has a PointSize member
		hasPointSize := false
		for _, member := range st.Members {
			if member.Binding != nil {
				if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinPointSize {
					hasPointSize = true
					break
				}
			}
		}
		if !hasPointSize {
			if !isFirst {
				w.write(",")
			}
			w.write(" 1.0")
		}
	}
	w.writeSampleMaskValue(isFirst)

	w.write(" };\n")
	return true, nil
}

// writeReturn writes a return statement.
func (w *Writer) writeReturn(ret ir.StmtReturn) error {
	if ret.Value == nil {
		if w.voidOutputStruct != "" {
			w.writeVoidOutputReturn()
			return nil
		}
		w.WriteLine("return;")
		return nil
	}

	handled, err := w.writeEntryPointOutputReturn(*ret.Value)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	w.WriteIndent()
	w.write("return ")
	if err := w.writeExpression(*ret.Value); err != nil {
		return err
	}
	w.write(";\n")
	return nil
}

// writeBarrier writes a barrier statement.
// Rust naga reference: writer.rs ~line 7624-7657, uses {NAMESPACE}::threadgroup_barrier
// and {NAMESPACE}::mem_flags for fully qualified Metal calls.
func (w *Writer) writeBarrier(barrier ir.StmtControlBarrier) error {
	// Metal uses different barrier functions based on the memory being synchronized.
	// Both the function and mem_flags must be fully qualified with the metal:: namespace.
	if barrier.Flags&ir.BarrierSubGroup != 0 {
		// Subgroup barrier uses simdgroup_barrier instead of threadgroup_barrier.
		w.WriteLine("%ssimdgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)
	}
	if barrier.Flags&ir.BarrierWorkGroup != 0 {
		w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)
	}
	if barrier.Flags&ir.BarrierStorage != 0 {
		w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_device);", Namespace, Namespace)
	}
	if barrier.Flags&ir.BarrierTexture != 0 {
		w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_texture);", Namespace, Namespace)
	}
	if barrier.Flags == 0 {
		// Pure execution barrier
		w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_none);", Namespace, Namespace)
	}
	return nil
}

// writeStore writes a store statement.
// For atomic pointers, emits metal::atomic_store_explicit (matching Rust naga's put_store).
// For ReadZeroSkipWrite policy, wraps in if(bounds_check) { ... } (matching Rust naga's put_store).
func (w *Writer) writeStore(store ir.StmtStore) error {
	// Check RZSW policy first — stores skip if out of bounds.
	policy := w.chooseBoundsCheckPolicy(store.Pointer)
	if policy == backend.BoundsCheckReadZeroSkipWrite {
		check, ok, err := w.buildRZSWBoundsCheck(store.Pointer)
		if err != nil {
			return err
		}
		if ok {
			w.WriteIndent()
			w.write("if (%s) {\n", check)
			w.Indent++
			w.insideRZSW = true
			err := w.writeUncheckedStore(store)
			w.insideRZSW = false
			w.Indent--
			if err != nil {
				return err
			}
			w.WriteIndent()
			w.write("}\n")
			return nil
		}
	}

	return w.writeUncheckedStore(store)
}

// writeUncheckedStore writes a store statement without RZSW bounds checking.
func (w *Writer) writeUncheckedStore(store ir.StmtStore) error {
	// Check if the pointer target is an atomic type.
	// Rust naga emits StmtStore (not StmtAtomic) for atomicStore(),
	// and the MSL backend detects atomic pointers at render time.
	if w.isAtomicPointer(store.Pointer) {
		if err := w.validateAtomicOperation(store.Pointer, "store", false); err != nil {
			return err
		}
		w.WriteIndent()
		w.write("%satomic_store_explicit(&", Namespace)
		if err := w.writeExpression(store.Pointer); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(store.Value); err != nil {
			return err
		}
		w.write(", %smemory_order_relaxed);\n", Namespace)
		return nil
	}

	w.WriteIndent()
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

// validateAtomicOperation enforces the subset of 64-bit atomics supported by
// the MSL backend. This matches Rust naga's SHADER_INT64_ATOMIC_MIN_MAX
// capability: only result-discarded min/max operations in storage address
// space are supported. MSL has no SHADER_INT64_ATOMIC_ALL_OPS equivalent.
//
// name is the operation's WGSL-facing name in the error: an atomic function's
// (atomicFunctionName), or "load" or "store" for an atomic load or store,
// which are a Load and a Store through a pointer to atomic<T> and no atomic
// function, as in Rust naga. resultDiscardedMinMax says the operation is a
// min or max whose result is not used.
func (w *Writer) validateAtomicOperation(pointer ir.ExpressionHandle, name string, resultDiscardedMinMax bool) error {
	if w.currentFunction == nil {
		return nil
	}

	scalar := w.resolveAtomicScalarFromPointer(w.currentFunction, pointer)
	if scalar == nil || scalar.Width != 8 {
		return nil
	}

	if resultDiscardedMinMax {
		if space, ok := w.getPointerAddressSpace(pointer); ok && space == ir.SpaceStorage {
			return nil
		}
	}

	return fmt.Errorf(
		"64-bit atomic %s is unsupported by MSL; only result-discarded min/max in the storage address space are supported",
		name,
	)
}

// atomicFunctionName returns the WGSL-facing name used in MSL capability
// errors. Keeping these stable makes backend errors actionable to callers.
func atomicFunctionName(fun ir.AtomicFunction) string {
	switch f := fun.(type) {
	case ir.AtomicAdd:
		return "add"
	case ir.AtomicSubtract:
		return "subtract"
	case ir.AtomicAnd:
		return "and"
	case ir.AtomicExclusiveOr:
		return "xor"
	case ir.AtomicInclusiveOr:
		return "or"
	case ir.AtomicMin:
		return "min"
	case ir.AtomicMax:
		return "max"
	case ir.AtomicExchange:
		if f.Compare != nil {
			return "compare exchange"
		}
		return "exchange"
	default:
		return fmt.Sprintf("%T", fun)
	}
}

// isAtomicPointer checks if an expression is a pointer to an atomic type.
// Matches Rust naga's TypeInner::is_atomic_pointer.
// Note: our type resolution strips the pointer wrapper for access chains,
// so we check both Pointer{base: Atomic} and direct Atomic type.
func (w *Writer) isAtomicPointer(handle ir.ExpressionHandle) bool {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return false
	}
	// Check direct Pointer -> Atomic
	if pt, ok := typeInner.(ir.PointerType); ok {
		if int(pt.Base) < len(w.module.Types) {
			if _, isAtomic := w.module.Types[pt.Base].Inner.(ir.AtomicType); isAtomic {
				return true
			}
		}
	}
	// Our type resolution for AccessIndex through pointers strips the pointer wrapper,
	// returning the member type directly. So also check if the resolved type IS atomic.
	if _, isAtomic := typeInner.(ir.AtomicType); isAtomic {
		return true
	}
	// Check via type handle
	if th := w.getExpressionTypeHandle(handle); th != nil {
		if int(*th) < len(w.module.Types) {
			if _, isAtomic := w.module.Types[*th].Inner.(ir.AtomicType); isAtomic {
				return true
			}
		}
	}
	return false
}

// writeImageStore writes an image store statement.
// Coordinate type is based on image dimension (uint for 1D, metal::uint2 for 2D, etc.).
func (w *Writer) writeImageStore(imgStore ir.StmtImageStore) error {
	w.WriteIndent()
	if err := w.writeExpression(imgStore.Image); err != nil {
		return err
	}
	w.write(".write(")

	// Value
	if err := w.writeExpression(imgStore.Value); err != nil {
		return err
	}

	// Coordinate (type based on image dimension)
	imgType := w.getImageType(imgStore.Image)
	w.write(", ")
	if err := w.writeCoordCast(imgStore.Coordinate, imgType); err != nil {
		return err
	}

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

// writeImageAtomic writes an image atomic operation statement.
func (w *Writer) writeImageAtomic(imgAtomic ir.StmtImageAtomic) error {
	w.WriteIndent()
	if err := w.writeExpression(imgAtomic.Image); err != nil {
		return err
	}

	// Determine if this is a 64-bit atomic (uses different MSL names)
	is64bit := false
	if valType := w.getExpressionType(imgAtomic.Value); valType != nil {
		if sc, ok := valType.(ir.ScalarType); ok && sc.Width == 8 {
			is64bit = true
		}
	}

	var opName string
	if is64bit {
		switch imgAtomic.Fun.(type) {
		case ir.AtomicMin:
			opName = "min"
		case ir.AtomicMax:
			opName = "max"
		default:
			return unsupported.Errorf("unsupported 64-bit image atomic operation")
		}
	} else {
		switch imgAtomic.Fun.(type) {
		case ir.AtomicAdd:
			opName = "fetch_add"
		case ir.AtomicSubtract:
			opName = "fetch_sub"
		case ir.AtomicAnd:
			opName = "fetch_and"
		case ir.AtomicInclusiveOr:
			opName = "fetch_or"
		case ir.AtomicExclusiveOr:
			opName = "fetch_xor"
		case ir.AtomicMin:
			opName = "fetch_min"
		case ir.AtomicMax:
			opName = "fetch_max"
		case ir.AtomicExchange:
			opName = "exchange"
		default:
			return unsupported.Errorf("unsupported image atomic operation")
		}
	}

	w.write(".atomic_%s(", opName)

	// Coordinate (cast to uint)
	imgType := w.getImageType(imgAtomic.Image)
	if err := w.writeCoordCast(imgAtomic.Coordinate, imgType); err != nil {
		return err
	}

	w.write(", ")

	// Value
	if err := w.writeExpression(imgAtomic.Value); err != nil {
		return err
	}

	w.write(");\n")
	return nil
}

// writeAtomic writes an atomic operation statement.
func (w *Writer) writeAtomic(atomic ir.StmtAtomic) error {
	minMax := false
	switch atomic.Fun.(type) {
	case ir.AtomicMin, ir.AtomicMax:
		minMax = true
	}
	if err := w.validateAtomicOperation(atomic.Pointer, atomicFunctionName(atomic.Fun), minMax && atomic.Result == nil); err != nil {
		return err
	}

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
		// When result is not used, use void atomic_min_explicit (Rust naga pattern).
		if atomic.Result == nil {
			funcName = "atomic_min_explicit"
		} else {
			funcName = "atomic_fetch_min_explicit"
		}
	case ir.AtomicMax:
		// When result is not used, use void atomic_max_explicit (Rust naga pattern).
		if atomic.Result == nil {
			funcName = "atomic_max_explicit"
		} else {
			funcName = "atomic_fetch_max_explicit"
		}
	case ir.AtomicExchange:
		if f.Compare != nil {
			// Compare-and-exchange
			return w.writeAtomicCompareExchange(atomic, f)
		}
		funcName = "atomic_exchange_explicit"
	default:
		return unsupported.Errorf("unsupported atomic function: %T", atomic.Fun)
	}

	w.WriteIndent()

	// If there's a result, assign it.
	// Rust naga uses the concrete type (not 'auto') and standard expression naming.
	if atomic.Result != nil {
		resultType := "uint"
		scalar := w.getExpressionScalarType(atomic.Value)
		if scalar != nil {
			resultType = scalarTypeName(*scalar)
		}
		tempName := fmt.Sprintf("_e%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.write("%s %s = ", resultType, tempName)
	}

	checked, err := w.writeAtomicBoundsCheck(atomic.Pointer)
	if err != nil {
		return err
	}

	// Rust naga uses metal:: prefix and address-of operator for the pointer.
	w.write("%s%s(&", Namespace, funcName)
	if err := w.writeExpression(atomic.Pointer); err != nil {
		if checked {
			w.insideRZSW = false
		}
		return err
	}
	w.write(", ")
	if err := w.writeExpression(atomic.Value); err != nil {
		if checked {
			w.insideRZSW = false
		}
		return err
	}
	w.write(", %smemory_order_relaxed)", Namespace)

	if checked {
		w.insideRZSW = false
		w.write(" : DefaultConstructible()")
	}
	w.write(";\n")
	return nil
}

// writeAtomicCompareExchange writes an atomic compare-exchange operation.
// Rust naga emits: _atomic_compare_exchange_result_Uint_4_ _eN = naga_atomic_compare_exchange_weak_explicit(&ptr, cmp, val);
// The template helper function is emitted at the top of the file.
func (w *Writer) writeAtomicCompareExchange(atomic ir.StmtAtomic, exchange ir.AtomicExchange) error {
	// Determine the scalar type of the atomic value.
	scalar := w.getExpressionScalarType(atomic.Value)
	if scalar == nil {
		scalar = &ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	}
	w.registerAtomicCompareExchange(*scalar)
	structName := atomicCompareExchangeVariant{scalar: *scalar}.atomicExchangeStructName()

	w.WriteIndent()
	if atomic.Result != nil {
		tempName := fmt.Sprintf("_e%d", *atomic.Result)
		w.namedExpressions[*atomic.Result] = tempName
		w.write("%s %s = ", structName, tempName)
	}
	checked, err := w.writeAtomicBoundsCheck(atomic.Pointer)
	if err != nil {
		return err
	}
	if checked {
		defer func() { w.insideRZSW = false }()
	}
	w.write("naga_atomic_compare_exchange_weak_explicit(&")
	if err := w.writeExpression(atomic.Pointer); err != nil {
		return err
	}
	w.write(", ")
	if err := w.writeExpression(*exchange.Compare); err != nil {
		return err
	}
	w.write(", ")
	if err := w.writeExpression(atomic.Value); err != nil {
		return err
	}
	w.write(")")
	if checked {
		w.write(" : DefaultConstructible()")
	}
	w.write(";\n")
	return nil
}

// writeAtomicBoundsCheck writes "CONDITION ? " in front of an atomic call whose
// pointer needs ReadZeroSkipWrite checks, and sets insideRZSW so the pointer
// itself is written unchecked; the caller closes with " : DefaultConstructible()"
// and clears insideRZSW. The condition comes from buildRZSWBoundsCheck, the walk
// loads and stores use, as Rust naga's put_bounds_checks serves atomics too.
func (w *Writer) writeAtomicBoundsCheck(pointer ir.ExpressionHandle) (bool, error) {
	if w.chooseBoundsCheckPolicy(pointer) != backend.BoundsCheckReadZeroSkipWrite {
		return false, nil
	}
	check, ok, err := w.buildRZSWBoundsCheck(pointer)
	if err != nil || !ok {
		return false, err
	}
	w.write("%s ? ", check)
	w.insideRZSW = true
	return true, nil
}

// writeCall writes a function call statement.
func (w *Writer) writeCall(call ir.StmtCall) error {
	w.WriteIndent()

	// Assign result if needed
	if call.Result != nil {
		typeName := "auto"
		if typeHandle := w.getExpressionTypeHandle(*call.Result); typeHandle != nil {
			typeName = w.writeTypeName(*typeHandle, StorageAccess(0))
		}
		tempName := fmt.Sprintf("_e%d", *call.Result)
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

	// Pass-through global resources (textures, samplers, buffers)
	hasArgs := len(call.Arguments) > 0
	if globals, ok := w.funcPassThroughGlobals[call.Function]; ok {
		for _, gHandle := range globals {
			if hasArgs {
				w.write(", ")
			}
			name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: gHandle})
			w.write("%s", name)
			hasArgs = true
		}
	}

	// Pass-through _buffer_sizes for helper functions that access runtime-sized arrays
	if w.funcNeedsBufferSizes(call.Function) {
		if hasArgs {
			w.write(", ")
		}
		w.write("_buffer_sizes")
	}

	w.write(");\n")
	return nil
}

// writeWorkGroupUniformLoad writes a workgroup uniform load.
// Matches Rust naga: barrier, typed load into named variable, barrier.
// The result is named via namer.call("") which produces "unnamed", "unnamed_1", etc.
func (w *Writer) writeWorkGroupUniformLoad(load ir.StmtWorkGroupUniformLoad) error {
	// First barrier
	w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)

	// The result type for the variable declaration, named as Rust's
	// start_baking_expression names a resolution: a handle by its type's
	// name, a value (an atomic's scalar) by its own.
	typeName := ""
	if res := w.exprType(load.Result); res.Handle != nil {
		typeName = w.writeTypeName(*res.Handle, StorageAccess(0))
	} else if res.Value != nil {
		typeName = typeInnerToMSL(res.Value)
	}
	if typeName == "" {
		typeName = "auto"
	}

	// Generate the variable name via namer.call("").
	// Matches Rust naga: self.namer.call("") which produces "unnamed", "unnamed_1", etc.
	varName := w.namer.call("")
	w.namedExpressions[load.Result] = varName

	// Write: TYPE VARNAME = LOAD_EXPR;
	w.WriteIndent()
	w.write("%s %s = ", typeName, varName)

	// Use writeLoad pattern: write the pointer's access chain directly.
	policy := w.chooseBoundsCheckPolicy(load.Pointer)
	prevPolicy, prevHas := w.currentAccessPolicy, w.hasAccessPolicy
	w.currentAccessPolicy, w.hasAccessPolicy = policy, true
	err := w.writeExpression(load.Pointer)
	w.currentAccessPolicy, w.hasAccessPolicy = prevPolicy, prevHas
	if err != nil {
		return err
	}
	w.write(";\n")

	// Second barrier
	w.WriteLine("%sthreadgroup_barrier(%smem_flags::mem_threadgroup);", Namespace, Namespace)
	return nil
}

// writeRayQuery writes a ray query statement.
func (w *Writer) writeRayQuery(rayQuery ir.StmtRayQuery) error {
	switch f := rayQuery.Fun.(type) {
	case ir.RayQueryInitialize:
		return w.writeRayQueryInit(rayQuery.Query, f)

	case ir.RayQueryProceed:
		// bool _eN = query.ready;
		tempName := fmt.Sprintf("_e%d", f.Result)
		w.namedExpressions[f.Result] = tempName
		w.WriteIndent()
		w.write("bool %s = ", tempName)
		if err := w.writeExpression(rayQuery.Query); err != nil {
			return err
		}
		w.write(".ready;\n")

	case ir.RayQueryTerminate:
		// RAY_QUERY_MODERN_SUPPORT=false: just set ready = false
		w.WriteIndent()
		if err := w.writeExpression(rayQuery.Query); err != nil {
			return err
		}
		w.write(".ready = false;\n")

	case ir.RayQueryGenerateIntersection:
		// RAY_QUERY_MODERN_SUPPORT=false: not supported, skip
		_ = f

	case ir.RayQueryConfirmIntersection:
		// RAY_QUERY_MODERN_SUPPORT=false: not supported, skip

	default:
		return unsupported.Errorf("unsupported ray query function: %T", rayQuery.Fun)
	}
	return nil
}

// writeRayQueryInit writes the RayQuery Initialize statement.
// Matches Rust naga MSL backend: sets intersector properties, calls intersect, sets ready=true.
func (w *Writer) writeRayQueryInit(query ir.ExpressionHandle, f ir.RayQueryInitialize) error {
	const rtNs = "metal::raytracing"
	// Ensure the descriptor is baked as a local variable before use.
	// In Rust naga, the descriptor Compose is always in an Emit range
	// before the RayQuery Init statement. Our IR may emit it after.
	// The descriptor is used 7+ times in the expanded MSL, so it must be a local.
	if _, alreadyBaked := w.namedExpressions[f.Descriptor]; !alreadyBaked {
		name := fmt.Sprintf("_e%d", f.Descriptor)
		// Resolve the descriptor type
		typeName := ""
		if w.currentFunction != nil && int(f.Descriptor) < len(w.currentFunction.Expressions) {
			if compose, ok := w.currentFunction.Expressions[f.Descriptor].Kind.(ir.ExprCompose); ok {
				typeName = w.writeTypeName(compose.Type, StorageAccess(0))
			}
		}
		if typeName == "" {
			if res := w.exprType(f.Descriptor); res.Handle != nil {
				typeName = w.writeTypeName(*res.Handle, StorageAccess(0))
			}
		}
		if typeName != "" {
			w.namedExpressions[f.Descriptor] = name
			w.WriteIndent()
			w.write("%s %s = ", typeName, name)
			if err := w.writeExpressionInline(f.Descriptor); err != nil {
				return err
			}
			w.write(";\n")
		}
	}
	// assume_geometry_type(triangle)
	w.WriteIndent()
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersector.assume_geometry_type(%s::geometry_type::triangle);\n", rtNs)
	// set_opacity_cull_mode: CULL_OPAQUE=64, CULL_NO_OPAQUE=128
	w.WriteIndent()
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersector.set_opacity_cull_mode((")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".flags & 64) != 0 ? %s::opacity_cull_mode::opaque : (", rtNs)
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".flags & 128) != 0 ? %s::opacity_cull_mode::non_opaque : %s::opacity_cull_mode::none);\n", rtNs, rtNs)
	// force_opacity: OPAQUE=1, NO_OPAQUE=2
	w.WriteIndent()
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersector.force_opacity((")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".flags & 1) != 0 ? %s::forced_opacity::opaque : (", rtNs)
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".flags & 2) != 0 ? %s::forced_opacity::non_opaque : %s::forced_opacity::none);\n", rtNs, rtNs)
	// accept_any_intersection: TERMINATE_ON_FIRST_HIT=4
	w.WriteIndent()
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersector.accept_any_intersection((")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".flags & 4) != 0);\n")
	// intersection = intersector.intersect(ray(...), acs, cull_mask);    query.ready = true;
	w.WriteIndent()
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersection = ")
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".intersector.intersect(%s::ray(", rtNs)
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".origin, ")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".dir, ")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".tmin, ")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".tmax), ")
	if err := w.writeExpression(f.AccelerationStructure); err != nil {
		return err
	}
	w.write(", ")
	if err := w.writeExpression(f.Descriptor); err != nil {
		return err
	}
	w.write(".cull_mask);    ")
	if err := w.writeExpression(query); err != nil {
		return err
	}
	w.write(".ready = true;\n")
	return nil
}

// writeSubgroupBallot writes a subgroup ballot statement.
// Metal: metal::uint4 result = metal::uint4((uint64_t)metal::simd_ballot(predicate), 0, 0, 0);
func (w *Writer) writeSubgroupBallot(ballot ir.StmtSubgroupBallot) error {
	w.WriteIndent()

	// Register the result name
	var tempName string
	if name, ok := w.namedExpressions[ballot.Result]; ok {
		tempName = name
	} else {
		tempName = w.allocateUnnamedVar()
		w.namedExpressions[ballot.Result] = tempName
	}

	w.write("%suint4 %s = %suint4(", Namespace, tempName, Namespace)

	if ballot.Predicate != nil {
		// simd_ballot returns a simd_vote, cast to uint64_t
		w.write("(uint64_t)%ssimd_ballot(", Namespace)
		if err := w.writeExpression(*ballot.Predicate); err != nil {
			return err
		}
		w.write(")")
	} else {
		// No predicate means ballot for all active lanes (true)
		w.write("(uint64_t)%ssimd_ballot(true)", Namespace)
	}

	w.write(", 0, 0, 0);\n")
	return nil
}

// writeSubgroupCollectiveOperation writes a subgroup collective operation statement.
// Maps SubgroupOperation + CollectiveOperation to metal::simd_* functions.
func (w *Writer) writeSubgroupCollectiveOperation(op ir.StmtSubgroupCollectiveOperation) error {
	w.WriteIndent()

	// Determine MSL function name
	var funcName string
	switch op.Op {
	case ir.SubgroupOperationAll:
		funcName = "simd_all"
	case ir.SubgroupOperationAny:
		funcName = "simd_any"
	case ir.SubgroupOperationAdd:
		switch op.CollectiveOp {
		case ir.CollectiveReduce:
			funcName = "simd_sum"
		case ir.CollectiveExclusiveScan:
			funcName = "simd_prefix_exclusive_sum"
		case ir.CollectiveInclusiveScan:
			funcName = "simd_prefix_inclusive_sum"
		}
	case ir.SubgroupOperationMul:
		switch op.CollectiveOp {
		case ir.CollectiveReduce:
			funcName = "simd_product"
		case ir.CollectiveExclusiveScan:
			funcName = "simd_prefix_exclusive_product"
		case ir.CollectiveInclusiveScan:
			funcName = "simd_prefix_inclusive_product"
		}
	case ir.SubgroupOperationMin:
		funcName = "simd_min"
	case ir.SubgroupOperationMax:
		funcName = "simd_max"
	case ir.SubgroupOperationAnd:
		funcName = "simd_and"
	case ir.SubgroupOperationOr:
		funcName = "simd_or"
	case ir.SubgroupOperationXor:
		funcName = "simd_xor"
	default:
		return unsupported.Errorf("unsupported subgroup operation: %d", op.Op)
	}

	// Write result assignment
	resultType := w.getExpressionMSLTypeName(op.Result)
	tempName := w.allocateUnnamedVar()
	w.namedExpressions[op.Result] = tempName

	w.write("%s %s = %s%s(", resultType, tempName, Namespace, funcName)
	if err := w.writeExpression(op.Argument); err != nil {
		return err
	}
	w.write(");\n")
	return nil
}

// writeSubgroupGather writes a subgroup gather statement.
// Maps GatherMode to metal::simd_broadcast, simd_shuffle, etc.
func (w *Writer) writeSubgroupGather(gather ir.StmtSubgroupGather) error {
	w.WriteIndent()

	// Write result assignment
	resultType := w.getExpressionMSLTypeName(gather.Result)
	tempName := w.allocateUnnamedVar()
	w.namedExpressions[gather.Result] = tempName

	w.write("%s %s = ", resultType, tempName)

	switch m := gather.Mode.(type) {
	case ir.GatherBroadcastFirst:
		w.write("%ssimd_broadcast_first(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherBroadcast:
		w.write("%ssimd_broadcast(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Index); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherShuffle:
		w.write("%ssimd_shuffle(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Index); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherShuffleDown:
		w.write("%ssimd_shuffle_down(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Delta); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherShuffleUp:
		w.write("%ssimd_shuffle_up(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Delta); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherShuffleXor:
		w.write("%ssimd_shuffle_xor(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Mask); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherQuadBroadcast:
		w.write("%squad_broadcast(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(m.Index); err != nil {
			return err
		}
		w.write(")")

	case ir.GatherQuadSwap:
		// QuadSwapX = xor 1, QuadSwapY = xor 2, QuadSwapDiagonal = xor 3
		var xorMask uint32
		switch m.Direction {
		case ir.QuadDirectionX:
			xorMask = 1
		case ir.QuadDirectionY:
			xorMask = 2
		case ir.QuadDirectionDiagonal:
			xorMask = 3
		}
		w.write("%squad_shuffle_xor(", Namespace)
		if err := w.writeExpression(gather.Argument); err != nil {
			return err
		}
		w.write(", %du)", xorMask)

	default:
		return unsupported.Errorf("unsupported gather mode: %T", gather.Mode)
	}

	w.write(";\n")
	return nil
}

// getExpressionMSLTypeName returns the MSL type name for an expression based on its resolved type.
func (w *Writer) getExpressionMSLTypeName(handle ir.ExpressionHandle) string {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return "uint"
	}
	switch t := typeInner.(type) {
	case ir.ScalarType:
		return scalarTypeName(t)
	case ir.VectorType:
		return vectorTypeName(t)
	case ir.MatrixType:
		return matrixTypeName(t)
	default:
		return "uint"
	}
}

// allocateUnnamedVar returns a new unnamed variable name.
func (w *Writer) allocateUnnamedVar() string {
	name := fmt.Sprintf("unnamed")
	if w.unnamedCount > 0 {
		name = fmt.Sprintf("unnamed_%d", w.unnamedCount)
	}
	w.unnamedCount++
	return name
}
