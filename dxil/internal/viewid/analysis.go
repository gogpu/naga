package viewid

import (
	"github.com/gogpu/naga/ir"
)

// scalarSet is a set of input-scalar indices that influence some value.
// Implemented as map[uint32]struct{} for flexibility; shader sig sizes
// are small enough (rarely >32 scalars) that map overhead is negligible.
type scalarSet map[uint32]struct{}

func newScalarSet() scalarSet { return make(scalarSet) }

func (s scalarSet) add(v uint32) { s[v] = struct{}{} }
func (s scalarSet) addAll(other scalarSet) {
	for v := range other {
		s[v] = struct{}{}
	}
}
func (s scalarSet) copy() scalarSet {
	out := make(scalarSet, len(s))
	for v := range s {
		out[v] = struct{}{}
	}
	return out
}

// componentTaint holds per-component taint for an expression result. For
// scalar expressions the slice has length 1; for vectors up to 4; for
// matrices cols*rows; for structs/arrays: per-flattened-scalar taint.
type componentTaint []scalarSet

// union returns the union of all component taints — conservative fallback
// for when we can't track per-component precision.
func (c componentTaint) union() scalarSet {
	if len(c) == 0 {
		return nil
	}
	out := c[0].copy()
	for i := 1; i < len(c); i++ {
		out.addAll(c[i])
	}
	return out
}

// analysisState holds the mutable state of a single Analyze call.
type analysisState struct {
	irMod   *ir.Module
	ep      *ir.EntryPoint
	inputs  []SigElement
	outputs []SigElement

	// exprTaint[h] is the per-component taint of expression h. Populated
	// lazily on first reference.
	exprTaint map[ir.ExpressionHandle]componentTaint

	// localVarTaint[varIdx] is the current per-flattened-scalar taint of
	// local variable varIdx. Updated on StmtStore, read on ExprLoad of
	// an ExprLocalVariable. Written fields flow into the current value.
	//
	// Conservative merge on loops / branches: taint unions on each
	// store, never shrinks.
	localVarTaint map[uint32]componentTaint

	// outputTaint[sigIdx][col] = scalar taint that reaches the given
	// output signature element's column. Populated at Return or
	// storeOutput-equivalent points.
	outputTaint [][]scalarSet

	// Tainted flag: true if the entire analysis should fall back to
	// "all inputs influence all outputs" due to an unhandled construct.
	giveUp bool
}

func newAnalysisState(irMod *ir.Module, ep *ir.EntryPoint, inputs, outputs []SigElement) *analysisState {
	s := &analysisState{
		irMod:         irMod,
		ep:            ep,
		inputs:        inputs,
		outputs:       outputs,
		exprTaint:     make(map[ir.ExpressionHandle]componentTaint),
		localVarTaint: make(map[uint32]componentTaint),
		outputTaint:   make([][]scalarSet, len(outputs)),
	}
	for i := range outputs {
		s.outputTaint[i] = make([]scalarSet, outputs[i].NumChannels)
		for c := range s.outputTaint[i] {
			s.outputTaint[i][c] = newScalarSet()
		}
	}
	return s
}

// analyze is the entry point for the per-entry-point walk.
func (s *analysisState) analyze() {
	// Initialize function-argument taint from input signature elements.
	s.initArgumentTaint()

	// Walk the function body.
	s.walkBlock(s.ep.Function.Body)

	// If we hit an unhandled construct anywhere, conservatively mark
	// every output as depending on every input.
	if s.giveUp {
		s.markEverythingDependsOnEverything()
	}
}

// markEverythingDependsOnEverything is the fallback when the analyzer
// encounters a construct it cannot trace precisely.
func (s *analysisState) markEverythingDependsOnEverything() {
	allInputs := newScalarSet()
	for i := range s.inputs {
		for c := uint32(0); c < s.inputs[i].NumChannels; c++ {
			allInputs.add(s.inputs[i].ScalarStart + c)
		}
	}
	for outSigIdx := range s.outputTaint {
		for col := range s.outputTaint[outSigIdx] {
			s.outputTaint[outSigIdx][col] = allInputs.copy()
		}
	}
}

// initArgumentTaint populates exprTaint for every ExprFunctionArgument
// with per-scalar taint from the input signature elements.
func (s *analysisState) initArgumentTaint() {
	fn := &s.ep.Function
	// Walk arguments the same way collectGraphicsSignatures does, so the
	// sig-element index we track matches the emitter's and PSV0's
	// ordering.
	sigIdx := 0
	for argIdx := range fn.Arguments {
		arg := &fn.Arguments[argIdx]

		// Find the expression handle for this argument.
		argHandle, ok := s.findArgHandle(uint32(argIdx))
		if !ok {
			continue
		}

		argType := s.irMod.Types[arg.Type]
		if arg.Binding != nil {
			// Direct binding on argument.
			taint := s.taintForSig(sigIdx)
			sigIdx++
			// Replicate scalar taint across argument's component count.
			s.exprTaint[argHandle] = expandTaintToType(s.irMod, argType.Inner, taint)
			continue
		}

		// Struct-typed argument with per-member bindings.
		st, isStruct := argType.Inner.(ir.StructType)
		if !isStruct {
			continue
		}

		// Build a per-flattened-scalar taint for the struct and ALSO
		// pre-register any ExprAccessIndex(arg, k) so member reads pick
		// up the right per-member taint without re-running the argument
		// walk.
		memberTaints := make([]componentTaint, len(st.Members))
		for memIdx := range st.Members {
			member := &st.Members[memIdx]
			if member.Binding == nil {
				// Unbound member (unusual but possible in struct types
				// shared with non-entry-point functions). Zero taint.
				memberTy := s.irMod.Types[member.Type]
				n := totalScalarCount(s.irMod, memberTy.Inner)
				memberTaints[memIdx] = emptyTaint(n)
				continue
			}
			taint := s.taintForSig(sigIdx)
			sigIdx++
			memberTy := s.irMod.Types[member.Type]
			memberTaints[memIdx] = expandTaintToType(s.irMod, memberTy.Inner, taint)
		}

		// Flatten the struct for the argument expression's taint.
		var flat componentTaint
		for memIdx := range memberTaints {
			flat = append(flat, memberTaints[memIdx]...)
		}
		s.exprTaint[argHandle] = flat

		// Pre-register AccessIndex(arg, k) expressions so later walks
		// find the correct member taint without re-building.
		for exprIdx := range fn.Expressions {
			ai, ok := fn.Expressions[exprIdx].Kind.(ir.ExprAccessIndex)
			if !ok || ai.Base != argHandle {
				continue
			}
			if int(ai.Index) >= len(memberTaints) {
				continue
			}
			s.exprTaint[ir.ExpressionHandle(exprIdx)] = memberTaints[ai.Index]
		}
	}
}

// findArgHandle finds the ExpressionHandle for a FunctionArgument with
// the given index. Mirrors emit.findArgExprHandle, duplicated here to
// avoid cross-package dependency.
func (s *analysisState) findArgHandle(idx uint32) (ir.ExpressionHandle, bool) {
	fn := &s.ep.Function
	for i := range fn.Expressions {
		if fa, ok := fn.Expressions[i].Kind.(ir.ExprFunctionArgument); ok && fa.Index == idx {
			return ir.ExpressionHandle(i), true
		}
	}
	return 0, false
}

// taintForSig returns the per-scalar taint of signature element sigIdx —
// a scalarSet containing every input scalar owned by that element.
func (s *analysisState) taintForSig(sigIdx int) scalarSet {
	if sigIdx < 0 || sigIdx >= len(s.inputs) {
		return newScalarSet()
	}
	elem := s.inputs[sigIdx]
	out := newScalarSet()
	if elem.SystemManaged {
		return out
	}
	for c := uint32(0); c < elem.NumChannels; c++ {
		out.add(elem.ScalarStart + c)
	}
	return out
}

// expandTaintToType turns a single scalarSet (the full taint of a
// signature element) into a per-component componentTaint for a type.
// Every component inherits the same scalar set (conservative for
// within-element tracking).
func expandTaintToType(irMod *ir.Module, inner ir.TypeInner, taint scalarSet) componentTaint {
	n := totalScalarCount(irMod, inner)
	out := make(componentTaint, n)

	// For vectors, distribute scalar taint per-component precisely: the
	// i-th component inherits the i-th scalar from the taint set (if the
	// taint set is sorted). This matches the loadInput(sigID, col=i)
	// emission model.
	if vt, ok := inner.(ir.VectorType); ok {
		sorted := sortedScalars(taint)
		for i := 0; i < int(vt.Size) && i < n; i++ {
			out[i] = newScalarSet()
			if i < len(sorted) {
				out[i].add(sorted[i])
			}
		}
		return out
	}

	if _, ok := inner.(ir.ScalarType); ok {
		if n >= 1 {
			out[0] = taint.copy()
		}
		return out
	}

	// For everything else (matrix, array, nested struct), every
	// component inherits the full taint — conservative.
	for i := range out {
		out[i] = taint.copy()
	}
	return out
}

// sortedScalars returns the scalars in ascending order.
func sortedScalars(s scalarSet) []uint32 {
	out := make([]uint32, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	// Small N — insertion sort.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// emptyTaint returns a componentTaint of length n with empty (zero-dep)
// scalarSets.
func emptyTaint(n int) componentTaint {
	out := make(componentTaint, n)
	for i := range out {
		out[i] = newScalarSet()
	}
	return out
}

// totalScalarCount is local copy of emit.totalScalarCount to avoid an
// import cycle. Mirrors the same recursive shape.
func totalScalarCount(irMod *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		// Arrays count as 1 element (the array pointer/handle), matching
		// emit.totalScalarCount which treats them as opaque for flat
		// struct member offsets.
		return 1
	case ir.StructType:
		total := 0
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			total += totalScalarCount(irMod, memberTy.Inner)
		}
		return total
	default:
		return 1
	}
}
