package viewid

import (
	"github.com/gogpu/naga/ir"
)

// taintOf returns the per-component taint for an expression, computing
// and memoizing if not already known.
func (s *analysisState) taintOf(h ir.ExpressionHandle) componentTaint {
	if t, ok := s.exprTaint[h]; ok {
		return t
	}
	t := s.computeTaint(h)
	s.exprTaint[h] = t
	return t
}

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // expression dispatch over every IR expression kind
func (s *analysisState) computeTaint(h ir.ExpressionHandle) componentTaint {
	fn := &s.ep.Function
	expr := fn.Expressions[h]

	// Determine the result type for sizing.
	resultN := s.resultScalarCount(h)

	switch k := expr.Kind.(type) {
	case ir.Literal, ir.ExprConstant, ir.ExprOverride, ir.ExprZeroValue:
		// Constants depend on nothing.
		return emptyTaint(resultN)

	case ir.ExprFunctionArgument:
		// Should have been pre-populated in initArgumentTaint. If we hit
		// this branch the argument wasn't bound to a signature element
		// (e.g., compute builtins, or globals via a synthetic arg).
		return emptyTaint(resultN)

	case ir.ExprGlobalVariable:
		// Globals (uniforms, storage, textures) don't feed signature
		// inputs. Outputs influenced purely by globals have empty input
		// taint, which is correct.
		return emptyTaint(resultN)

	case ir.ExprLocalVariable:
		// Pointer to a local variable. The dataflow value is produced
		// by ExprLoad of this pointer.
		// Return empty — the load expression picks up the variable's
		// current taint separately.
		return emptyTaint(resultN)

	case ir.ExprLoad:
		// If the pointer root is a local variable, return that var's
		// taint (optionally narrowed by a GEP path). Otherwise empty.
		rootVar, path, ok := s.rootLocalVar(k.Pointer)
		if !ok {
			return emptyTaint(resultN)
		}
		// Collect taint from any dynamic index steps. When the path has
		// dynamic steps (e.g. array[vertex_index]), the loaded value
		// depends on the index expression — even if the array itself is
		// constant-initialized (and thus has nil localVarTaint).
		dynIdxTaint := newScalarSet()
		hasDynamic := false
		for _, step := range path {
			if !step.isStatic {
				hasDynamic = true
				dynIdxTaint.addAll(s.taintOf(step.dynamic).union())
			}
		}
		varTaint := s.localVarTaint[rootVar]
		if varTaint == nil {
			// Local var was never stored to (uninitialized or initialized
			// only via LocalVariable.Init which is constant by WGSL rules).
			// The stored value has no input taint, but dynamic indices
			// into the var still contribute to the result.
			if !hasDynamic {
				return emptyTaint(resultN)
			}
			out := make(componentTaint, resultN)
			for i := range out {
				out[i] = dynIdxTaint.copy()
			}
			return out
		}
		lv := &fn.LocalVars[rootVar]
		varInner := s.irMod.Types[lv.Type].Inner
		start, count, precise := resolveFlatRange(s.irMod, varInner, path)
		if !precise || start+count > len(varTaint) {
			// Conservative: return union of all var scalars plus any
			// dynamic index taints.
			u := varTaint.union()
			if u == nil {
				u = newScalarSet()
			}
			if hasDynamic {
				u.addAll(dynIdxTaint)
			}
			out := make(componentTaint, resultN)
			for i := range out {
				out[i] = u.copy()
			}
			return out
		}
		out := make(componentTaint, resultN)
		for i := 0; i < resultN; i++ {
			if i < count {
				out[i] = varTaint[start+i].copy()
			} else {
				out[i] = newScalarSet()
			}
			if hasDynamic {
				out[i].addAll(dynIdxTaint)
			}
		}
		return out

	case ir.ExprAccessIndex:
		baseTaint := s.taintOf(k.Base)
		baseInner := s.innerOfExpr(k.Base)
		// For vector-typed bases, index selects one component.
		if _, ok := baseInner.(ir.VectorType); ok {
			out := make(componentTaint, 1)
			if int(k.Index) < len(baseTaint) {
				out[0] = baseTaint[k.Index].copy()
			} else {
				out[0] = newScalarSet()
			}
			return out
		}
		// For struct bases, pick out the member's flat range.
		if st, ok := baseInner.(ir.StructType); ok {
			if int(k.Index) >= len(st.Members) {
				return emptyTaint(resultN)
			}
			off := 0
			for i := 0; i < int(k.Index); i++ {
				mi := s.irMod.Types[st.Members[i].Type]
				off += totalScalarCount(s.irMod, mi.Inner)
			}
			memTy := s.irMod.Types[st.Members[k.Index].Type]
			memCount := totalScalarCount(s.irMod, memTy.Inner)
			out := make(componentTaint, memCount)
			for i := 0; i < memCount; i++ {
				if off+i < len(baseTaint) {
					out[i] = baseTaint[off+i].copy()
				} else {
					out[i] = newScalarSet()
				}
			}
			return out
		}
		// For matrix bases, index selects a column (vector).
		if mt, ok := baseInner.(ir.MatrixType); ok {
			rows := int(mt.Rows)
			out := make(componentTaint, rows)
			startIdx := int(k.Index) * rows
			for i := 0; i < rows; i++ {
				j := startIdx + i
				if j < len(baseTaint) {
					out[i] = baseTaint[j].copy()
				} else {
					out[i] = newScalarSet()
				}
			}
			return out
		}
		// For array bases, static index picks one element — but the
		// element is treated as a single scalar in our flat model.
		// Return the union of base taints as conservative.
		u := baseTaint.union()
		if u == nil {
			u = newScalarSet()
		}
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprAccess:
		// Dynamic index — taint of the result is conservative union of
		// base taints plus index taint.
		baseTaint := s.taintOf(k.Base)
		idxTaint := s.taintOf(k.Index)
		u := baseTaint.union()
		if u == nil {
			u = newScalarSet()
		}
		u.addAll(idxTaint.union())
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprSplat:
		valTaint := s.taintOf(k.Value)
		u := valTaint.union()
		if u == nil {
			u = newScalarSet()
		}
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprSwizzle:
		vecTaint := s.taintOf(k.Vector)
		out := make(componentTaint, int(k.Size))
		for i := 0; i < int(k.Size); i++ {
			c := int(k.Pattern[i])
			if c < len(vecTaint) {
				out[i] = vecTaint[c].copy()
			} else {
				out[i] = newScalarSet()
			}
		}
		return out

	case ir.ExprCompose:
		// Gather per-component taints from the sub-expressions. Each
		// sub-expression contributes its own scalar count worth of
		// components to the result.
		out := make(componentTaint, 0, resultN)
		for _, c := range k.Components {
			ct := s.taintOf(c)
			out = append(out, ct...)
		}
		// Pad or truncate to expected result size.
		if len(out) > resultN {
			out = out[:resultN]
		}
		for len(out) < resultN {
			out = append(out, newScalarSet())
		}
		return out

	case ir.ExprUnary:
		// Unary ops (negate, logical not, bitwise not) are all
		// component-wise on vectors: result[i] depends only on input[i].
		return s.taintOf(k.Expr).perComponentCopy(resultN)

	case ir.ExprBinary:
		lhsTaint := s.taintOf(k.Left)
		rhsTaint := s.taintOf(k.Right)
		// WGSL binary operators are component-wise on scalar/vector
		// operands: result[i] = lhs[i] op rhs[i]. But matrix multiply
		// (mat*vec, vec*mat, mat*mat) creates cross-component deps --
		// each output component depends on ALL input components.
		lhsInner := s.innerOfExpr(k.Left)
		rhsInner := s.innerOfExpr(k.Right)
		_, lhsIsMat := lhsInner.(ir.MatrixType)
		_, rhsIsMat := rhsInner.(ir.MatrixType)
		if lhsIsMat || rhsIsMat {
			// Matrix multiply: conservative union of all components.
			lhs := lhsTaint.union()
			rhs := rhsTaint.union()
			if lhs == nil {
				lhs = newScalarSet()
			}
			lhs.addAll(rhs)
			out := make(componentTaint, resultN)
			for i := range out {
				out[i] = lhs.copy()
			}
			return out
		}
		// Scalar/vector: component-wise propagation.
		return mergeComponentWise(lhsTaint, rhsTaint, resultN)

	case ir.ExprSelect:
		// WGSL select() with vector condition is component-wise:
		// result[i] = cond[i] ? accept[i] : reject[i].
		a := s.taintOf(k.Accept)
		b := s.taintOf(k.Reject)
		c := s.taintOf(k.Condition)
		return mergeSelectComponentWise(a, b, c, resultN)

	case ir.ExprDerivative:
		// ddx/ddy are component-wise on vectors: derivative of vec[i]
		// depends only on vec[i].
		return s.taintOf(k.Expr).perComponentCopy(resultN)

	case ir.ExprRelational:
		// Relational builtins (any, all, isNan, isInf, isNormal, isFinite)
		// collapse a vector to a scalar bool. The result depends on all
		// input components.
		return s.taintOf(k.Argument).copyAllAsUnion(resultN)

	case ir.ExprMath:
		u := s.taintOf(k.Arg).union()
		if u == nil {
			u = newScalarSet()
		}
		if k.Arg1 != nil {
			u.addAll(s.taintOf(*k.Arg1).union())
		}
		if k.Arg2 != nil {
			u.addAll(s.taintOf(*k.Arg2).union())
		}
		if k.Arg3 != nil {
			u.addAll(s.taintOf(*k.Arg3).union())
		}
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprAs:
		// Type casts are component-wise: vec4<i32>(vec4<f32>) casts
		// each component independently.
		return s.taintOf(k.Expr).perComponentCopy(resultN)

	case ir.ExprImageSample:
		// Sample coordinate and offsets feed image access but the result
		// is a sampled color independent of signature input dataflow
		// UNLESS the coordinate itself was derived from signature
		// inputs (e.g. UV from vertex). Be conservative: union of
		// coordinate + level + depth ref.
		u := newScalarSet()
		u.addAll(s.taintOf(k.Coordinate).union())
		if k.ArrayIndex != nil {
			u.addAll(s.taintOf(*k.ArrayIndex).union())
		}
		if k.DepthRef != nil {
			u.addAll(s.taintOf(*k.DepthRef).union())
		}
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprImageLoad:
		u := newScalarSet()
		u.addAll(s.taintOf(k.Coordinate).union())
		if k.ArrayIndex != nil {
			u.addAll(s.taintOf(*k.ArrayIndex).union())
		}
		out := make(componentTaint, resultN)
		for i := range out {
			out[i] = u.copy()
		}
		return out

	case ir.ExprImageQuery:
		return emptyTaint(resultN)

	case ir.ExprCallResult:
		// Without interprocedural analysis, fall back to "unknown" — mark
		// giveUp so the final output is conservative.
		s.giveUp = true
		return emptyTaint(resultN)

	case ir.ExprArrayLength:
		return emptyTaint(resultN)

	case ir.ExprAtomicResult, ir.ExprWorkGroupUniformLoadResult,
		ir.ExprRayQueryProceedResult, ir.ExprRayQueryGetIntersection,
		ir.ExprSubgroupBallotResult, ir.ExprSubgroupOperationResult:
		return emptyTaint(resultN)

	default:
		_ = k
		s.giveUp = true
		return emptyTaint(resultN)
	}
}

// copyAllAsUnion collapses a per-component taint into a single union and
// re-expands it to `n` components. Used for ops where per-component
// precision is genuinely lost (e.g. dot product, cross product, length).
func (c componentTaint) copyAllAsUnion(n int) componentTaint {
	u := c.union()
	if u == nil {
		u = newScalarSet()
	}
	out := make(componentTaint, n)
	for i := range out {
		out[i] = u.copy()
	}
	return out
}

// perComponentCopy preserves per-component taint for component-wise
// operations (unary, cast, derivative, relational). Each result
// component[i] inherits operand component[i]'s taint. If the operand
// has fewer components than the result, surplus components get empty
// taint.
func (c componentTaint) perComponentCopy(n int) componentTaint {
	out := make(componentTaint, n)
	for i := range out {
		if i < len(c) {
			out[i] = c[i].copy()
		} else {
			out[i] = newScalarSet()
		}
	}
	return out
}

// mergeComponentWise produces per-component union of two operands for
// component-wise binary operations (Add, Mul, etc.). When one operand
// is scalar (len=1) and the other is vector, the scalar is broadcast:
// result[i] = scalar union operand[i]. When both are vectors of the
// same size, result[i] = lhs[i] union rhs[i].
func mergeComponentWise(lhs, rhs componentTaint, n int) componentTaint {
	out := make(componentTaint, n)
	lScalar := len(lhs) == 1 && n > 1
	rScalar := len(rhs) == 1 && n > 1
	for i := range out {
		out[i] = newScalarSet()
		// Left operand contribution.
		if lScalar {
			out[i].addAll(lhs[0])
		} else if i < len(lhs) {
			out[i].addAll(lhs[i])
		}
		// Right operand contribution.
		if rScalar {
			out[i].addAll(rhs[0])
		} else if i < len(rhs) {
			out[i].addAll(rhs[i])
		}
	}
	return out
}

// mergeSelectComponentWise produces per-component union of accept,
// reject, and condition for component-wise select operations.
func mergeSelectComponentWise(accept, reject, cond componentTaint, n int) componentTaint {
	out := make(componentTaint, n)
	aScalar := len(accept) == 1 && n > 1
	rScalar := len(reject) == 1 && n > 1
	cScalar := len(cond) == 1 && n > 1
	for i := range out {
		out[i] = newScalarSet()
		if aScalar {
			out[i].addAll(accept[0])
		} else if i < len(accept) {
			out[i].addAll(accept[i])
		}
		if rScalar {
			out[i].addAll(reject[0])
		} else if i < len(reject) {
			out[i].addAll(reject[i])
		}
		if cScalar {
			out[i].addAll(cond[0])
		} else if i < len(cond) {
			out[i].addAll(cond[i])
		}
	}
	return out
}

// resultScalarCount returns the total scalar count of an expression's
// result type, looking up the module's expression type table.
func (s *analysisState) resultScalarCount(h ir.ExpressionHandle) int {
	inner := s.innerOfExpr(h)
	if inner == nil {
		return 1
	}
	n := totalScalarCount(s.irMod, inner)
	if n <= 0 {
		return 1
	}
	return n
}

// innerOfExpr returns the TypeInner of an expression, resolving
// TypeResolution's Handle or Value form. Returns nil if the expression
// has no resolved type.
func (s *analysisState) innerOfExpr(h ir.ExpressionHandle) ir.TypeInner {
	fn := &s.ep.Function
	if int(h) >= len(fn.ExpressionTypes) {
		return nil
	}
	tr := fn.ExpressionTypes[h]
	if tr.Handle != nil {
		th := *tr.Handle
		if int(th) < len(s.irMod.Types) {
			return s.irMod.Types[th].Inner
		}
	}
	return tr.Value
}
