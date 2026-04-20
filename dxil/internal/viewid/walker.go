package viewid

import (
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// walkBlock recursively walks a statement block, updating local variable
// taint and collecting output taint at return statements.
func (s *analysisState) walkBlock(block ir.Block) {
	for i := range block {
		if s.giveUp {
			return
		}
		s.walkStatement(&block[i])
	}
}

func (s *analysisState) walkStatement(stmt *ir.Statement) {
	switch k := stmt.Kind.(type) {
	case ir.StmtEmit:
		// No action — expression evaluation is lazy in our analyzer.

	case ir.StmtBlock:
		s.walkBlock(k.Block)

	case ir.StmtIf:
		_ = s.taintOf(k.Condition) // pull condition's taint for reach set
		s.walkBlock(k.Accept)
		s.walkBlock(k.Reject)

	case ir.StmtSwitch:
		_ = s.taintOf(k.Selector)
		for ci := range k.Cases {
			s.walkBlock(k.Cases[ci].Body)
		}

	case ir.StmtLoop:
		// Conservative: walk once. Loops that modify outputs per-iteration
		// are handled by the taint-union-on-store rule.
		s.walkBlock(k.Body)
		s.walkBlock(k.Continuing)
		if k.BreakIf != nil {
			_ = s.taintOf(*k.BreakIf)
		}

	case ir.StmtBreak, ir.StmtContinue, ir.StmtKill, ir.StmtBarrier:
		// No dataflow effect for our purposes.

	case ir.StmtReturn:
		s.handleReturn(k)

	case ir.StmtStore:
		s.handleStore(k)

	case ir.StmtImageStore:
		// Writing to storage textures does not contribute to the entry
		// point's signature outputs (storage is a separate sink).

	case ir.StmtAtomic, ir.StmtImageAtomic:
		// Atomic ops can return a value; if that value feeds an output
		// we'll see it via the Result ExprAtomicResult handle. The
		// return value's taint is conservative (union of pointer + value).

	case ir.StmtWorkGroupUniformLoad:
		// Result expression will be picked up via taintOf on read.

	case ir.StmtCall:
		// User-function calls — without interprocedural analysis we
		// can't trace input→output deps across function boundaries.
		// Give up and mark everything dependent.
		s.giveUp = true

	case ir.StmtRayQuery:
		// Ray queries touch external state; any result feeds conservative
		// deps through taintOf on the result expression.

	case ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation, ir.StmtSubgroupGather:
		// Subgroup ops — result expressions pick up taints on read.

	default:
		// Unknown statement — be safe.
		s.giveUp = true
	}
}

// handleReturn processes a Return statement, distributing the return
// value's per-component taint to the output signature elements.
func (s *analysisState) handleReturn(ret ir.StmtReturn) {
	if ret.Value == nil {
		return
	}
	fn := &s.ep.Function
	if fn.Result == nil {
		return
	}
	resultType := s.irMod.Types[fn.Result.Type]

	valTaint := s.taintOf(*ret.Value)

	// Direct (non-struct) return: single output sig element, components
	// map linearly to output scalars.
	if st, okStruct := resultType.Inner.(ir.StructType); okStruct {
		s.distributeStructReturn(valTaint, st)
		return
	}
	// Non-struct return → output signature element index 0.
	if len(s.outputTaint) == 0 {
		return
	}
	for c := range s.outputTaint[0] {
		if c < len(valTaint) {
			s.outputTaint[0][c].addAll(valTaint[c])
		}
	}
}

// distributeStructReturn maps a struct-valued return's flat component
// taint into per-output-sig-element columns.
//
// The return value's components are laid out in DECLARATION order
// (Compose/Return places them in the source struct field order). The
// caller's outputs slice is in PACKED INTERFACE order (locations first,
// builtins last — see backend.SortedMemberIndices, BUG-DXIL-029). Walk
// the members in interface order so outputTaint[sigIdx] receives the
// taint from the corresponding flat component span in valTaint.
func (s *analysisState) distributeStructReturn(valTaint componentTaint, st ir.StructType) {
	memberSizes := make([]int, len(st.Members))
	memberOffsets := make([]int, len(st.Members))
	off := 0
	for mi := range st.Members {
		mt := s.irMod.Types[st.Members[mi].Type]
		memberSizes[mi] = totalScalarCount(s.irMod, mt.Inner)
		memberOffsets[mi] = off
		off += memberSizes[mi]
	}

	sigIdx := 0
	for _, mi := range backend.SortedMemberIndices(st.Members) {
		m := &st.Members[mi]
		if m.Binding == nil {
			continue
		}
		if sigIdx >= len(s.outputTaint) {
			sigIdx++
			continue
		}
		flatIdx := memberOffsets[mi]
		n := memberSizes[mi]
		cols := len(s.outputTaint[sigIdx])
		for c := 0; c < cols && c < n; c++ {
			if flatIdx+c < len(valTaint) {
				s.outputTaint[sigIdx][c].addAll(valTaint[flatIdx+c])
			}
		}
		sigIdx++
	}
}

// handleStore processes a StmtStore. For the purposes of this analyzer
// we only care about stores into local variables (tracked via
// localVarTaint) — stores into global memory don't contribute to sig
// outputs. Storing into a struct member of a local var merges the
// value's taint into the relevant component of the local var's taint.
func (s *analysisState) handleStore(store ir.StmtStore) {
	fn := &s.ep.Function
	valTaint := s.taintOf(store.Value)

	// Trace the pointer chain to identify the root local variable and
	// which component of it is being written.
	rootVar, subPath, ok := s.rootLocalVar(store.Pointer)
	if !ok {
		// Store into something we can't trace (global, pointer-typed
		// call result, etc.). Be safe — ignore.
		return
	}

	existing := s.localVarTaint[rootVar]
	localVar := &fn.LocalVars[rootVar]
	varTy := s.irMod.Types[localVar.Type]
	total := totalScalarCount(s.irMod, varTy.Inner)

	if existing == nil {
		existing = emptyTaint(total)
	}

	// Determine the flat scalar range affected by the store based on
	// subPath. If we couldn't resolve a static range, merge into the
	// whole variable.
	start, count, precise := resolveFlatRange(s.irMod, varTy.Inner, subPath)
	if !precise || start+count > total {
		// Conservative: merge value taint into every component.
		united := valTaint.union()
		if united == nil {
			united = newScalarSet()
		}
		for i := range existing {
			existing[i].addAll(united)
		}
		s.localVarTaint[rootVar] = existing
		return
	}

	// Precise: write each value-component into the matching variable
	// component (union — don't drop prior taint since control flow may
	// have written it in an earlier branch).
	for i := 0; i < count; i++ {
		var src scalarSet
		if i < len(valTaint) {
			src = valTaint[i]
		} else if len(valTaint) > 0 {
			src = valTaint[len(valTaint)-1]
		}
		if src != nil {
			existing[start+i].addAll(src)
		}
	}
	s.localVarTaint[rootVar] = existing
}

// rootLocalVar walks a pointer expression chain (ExprLocalVariable →
// ExprAccessIndex → ExprAccess) back to the root local variable, and
// returns the path of indices from the root to the pointee. Returns
// ok=false if the pointer is not rooted at a local variable.
func (s *analysisState) rootLocalVar(ptr ir.ExpressionHandle) (uint32, []pathStep, bool) {
	fn := &s.ep.Function
	var path []pathStep
	cursor := ptr
	for {
		expr := fn.Expressions[cursor]
		switch k := expr.Kind.(type) {
		case ir.ExprLocalVariable:
			// Reverse the path we accumulated (root→leaf order).
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return k.Variable, path, true
		case ir.ExprAccessIndex:
			path = append(path, pathStep{isStatic: true, staticIdx: k.Index})
			cursor = k.Base
		case ir.ExprAccess:
			path = append(path, pathStep{isStatic: false, dynamic: k.Index})
			cursor = k.Base
		default:
			return 0, nil, false
		}
	}
}

// pathStep represents one link in a pointer GEP chain rooted at a local
// variable. Static steps carry an index literal; dynamic steps carry the
// index expression handle (used for conservative taint merging).
type pathStep struct {
	isStatic  bool
	staticIdx uint32
	dynamic   ir.ExpressionHandle
}

// resolveFlatRange walks a GEP path from a root type and computes the
// flat scalar range [start, start+count) the path selects. Returns
// precise=false if any step is dynamic (unknown index) or if the type
// chain includes an array (which we don't flatten).
func resolveFlatRange(irMod *ir.Module, rootInner ir.TypeInner, path []pathStep) (start, count int, precise bool) {
	inner := rootInner
	start = 0
	precise = true
	for _, step := range path {
		if !step.isStatic {
			return 0, 0, false
		}
		switch t := inner.(type) {
		case ir.StructType:
			if int(step.staticIdx) >= len(t.Members) {
				return 0, 0, false
			}
			// Flat offset = sum of prior members' flat sizes.
			off := 0
			for i := 0; i < int(step.staticIdx); i++ {
				mi := irMod.Types[t.Members[i].Type]
				off += totalScalarCount(irMod, mi.Inner)
			}
			start += off
			inner = irMod.Types[t.Members[step.staticIdx].Type].Inner
		case ir.VectorType:
			if int(step.staticIdx) >= int(t.Size) {
				return 0, 0, false
			}
			start += int(step.staticIdx)
			inner = ir.ScalarType{Kind: t.Scalar.Kind, Width: t.Scalar.Width}
		case ir.ArrayType:
			// Arrays are opaque in our flat model.
			return 0, 0, false
		default:
			return 0, 0, false
		}
	}
	count = totalScalarCount(irMod, inner)
	return start, count, precise
}
