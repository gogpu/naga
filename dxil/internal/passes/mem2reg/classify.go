package mem2reg

import (
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// localPtr maps every ExpressionHandle whose Kind is
// ExprLocalVariable{i} to the variable index i. Built once per
// function so the rest of the pass can ask "is this handle a local-
// var pointer?" in O(1).
type localPtr map[ir.ExpressionHandle]uint32

// localUseInfo accumulates per-variable analysis used by the
// rewrite phase to decide whether to promote each LocalVariable
// and which fast path applies.
type localUseInfo struct {
	// promotable is the conservative AND of "well-formed uses" — a
	// variable is promotable iff its declared type is scalar AND
	// every use is well-formed (load.pointer or store.pointer,
	// never an AccessIndex base, function-call argument, atomic
	// pointer, ...). Single-block colocation is checked separately
	// inside Phase A's promote.go; the Phase B walker accepts
	// promotable but multi-block variables.
	promotable bool

	// promotableType reports whether the variable's declared type is
	// eligible for mem2reg (scalar or vector). Struct, matrix, array,
	// and atomic types require SROA decomposition first.
	promotableType bool

	// loadHandles lists every ExprLoad{ExprLocalVariable{i}}
	// expression handle in fn.Expressions. A separate set
	// (loadHandleSet) gives O(1) membership lookup during the
	// rewrite walk.
	loadHandles   []ir.ExpressionHandle
	loadHandleSet map[ir.ExpressionHandle]struct{}

	// storeCount tallies how many StmtStore statements anywhere in
	// the function target this variable. Used by the Phase A walker
	// to detect the "all stores in one block" condition.
	storeCount int
}

// classifyLocals analyses fn and returns one localUseInfo per
// LocalVariable. The slice is parallel to fn.LocalVars.
//
// Two sub-passes:
//  1. Walk fn.Expressions linearly. For every expression that
//     REFERENCES a local-var handle (i.e., contains a sub-handle
//     whose Kind is ExprLocalVariable), classify it as either a
//     legal use (ExprLoad with the local-var handle as Pointer)
//     or a disqualifying use (anything else).
//  2. Walk fn.Body recursively. Tally StmtStore statements per
//     variable; disqualify on any other statement that uses a
//     local-var handle as a pointer (StmtAtomic, StmtImageStore,
//     StmtCall arguments, ...).
//
// Single-block colocation is verified later, inline with the
// rewrite walk in promote.go, because it requires per-block context
// the classify pass intentionally avoids carrying.
func classifyLocals(mod *ir.Module, fn *ir.Function) []localUseInfo {
	uses := make([]localUseInfo, len(fn.LocalVars))
	for i := range uses {
		uses[i].promotableType = isPromotableLocal(mod, &fn.LocalVars[i])
		uses[i].promotable = uses[i].promotableType
	}
	if len(fn.LocalVars) == 0 {
		return uses
	}

	localPtrs := buildLocalPtrMap(fn)

	// After inlining, dead ExprLoad expressions may exist that reference
	// local-var handles but are outside any StmtEmit range (they are
	// template copies from the inliner's arg-spill pattern). These dead
	// loads inflate the total loadHandles count, causing Phase A's
	// single-block colocation check to fail even when all LIVE loads are
	// in one block. Build the emitted set BEFORE classification so only
	// live loads are counted, enabling Phase A direct promotion.
	emitted := collectEmittedHandles(fn)

	classifyExpressions(fn, localPtrs, uses, emitted)
	classifyStatements(fn.Body, localPtrs, uses)

	// Build per-variable fast-lookup load sets.
	for v := range uses {
		if !uses[v].promotable {
			continue
		}
		if len(uses[v].loadHandles) == 0 {
			// No loads at all — variable is dead-stored. Phase A
			// does not bother with pure dead-store elimination
			// (DCE in the bitcode emitter handles this naturally
			// once the alloca has zero uses on the load side).
			continue
		}
		set := make(map[ir.ExpressionHandle]struct{}, len(uses[v].loadHandles))
		for _, h := range uses[v].loadHandles {
			set[h] = struct{}{}
		}
		uses[v].loadHandleSet = set
	}

	return uses
}

// buildLocalPtrMap returns the set of expression handles that name
// a LocalVariable pointer.
func buildLocalPtrMap(fn *ir.Function) localPtr {
	m := make(localPtr, len(fn.LocalVars))
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			m[ir.ExpressionHandle(h)] = uint32(lv.Variable)
		}
	}
	return m
}

// classifyExpressions walks every expression in fn.Expressions and
// classifies references to local-var handles. The only legal form
// is ExprLoad{Pointer: <lv handle>}; any other reference disqualifies
// the variable from promotion.
//
// The emitted set limits which load handles are counted: only loads
// inside a StmtEmit range are live. After inlining, the arg-spill
// pattern creates ExprLoad expressions outside Emit ranges that are
// never evaluated (the callee's Emit ranges only cover the callee's
// own expressions, not the pre-allocated arg-load templates). Counting
// these dead loads would cause Phase A to reject the variable because
// the in-block load count wouldn't match the total load count.
//
// Non-load disqualifying references are checked regardless of Emit
// coverage because they represent address-taken or aliased uses that
// prevent promotion entirely.
func classifyExpressions(fn *ir.Function, localPtrs localPtr, uses []localUseInfo, emitted map[ir.ExpressionHandle]bool) {
	for h, expr := range fn.Expressions {
		hh := ir.ExpressionHandle(h)
		load, isLoad := expr.Kind.(ir.ExprLoad)
		irutil.ExprHandles(expr.Kind, func(operand ir.ExpressionHandle) {
			v, isLocal := localPtrs[operand]
			if !isLocal {
				return
			}
			if isLoad && operand == load.Pointer {
				// Only count loads that are inside an Emit range.
				if emitted[hh] {
					uses[v].loadHandles = append(uses[v].loadHandles, hh)
				}
				return
			}
			// Any other referencer (Access, AccessIndex, Compose,
			// math op, ...) is a disqualifying use.
			uses[v].promotable = false
		})
	}
}

// classifyStatements walks the statement tree under block and tallies
// StmtStore statements per variable. Any other use of a local-var handle
// by a statement (an operand other than a store's pointer: an atomic's
// pointer, an image's, a call argument, a returned or stored pointer, ...)
// disqualifies the variable.
func classifyStatements(block ir.Block, localPtrs localPtr, uses []localUseInfo) {
	for i := range block {
		kind := block[i].Kind
		store, isStore := kind.(ir.StmtStore)
		irutil.StmtExprHandles(kind, func(operand ir.ExpressionHandle) {
			v, isLocal := localPtrs[operand]
			if !isLocal {
				return
			}
			if isStore && operand == store.Pointer {
				uses[v].storeCount++
				return
			}
			uses[v].promotable = false
		})
		irutil.NestedBlocks(kind, func(b ir.Block) { classifyStatements(b, localPtrs, uses) })
	}
}

// isPromotableLocal reports whether the local variable's declared type
// is eligible for mem2reg promotion. Scalars (Float / Sint / Uint / Bool)
// and vectors qualify. Both have simple whole-value Store/Load semantics;
// component-level access (AccessIndex) is caught by the expression
// classifier as a disqualifying reference, so vector locals are only
// promoted when all uses are full-vector store/load.
//
// Struct, matrix, array, and atomic types require SROA decomposition
// first and remain excluded here.
func isPromotableLocal(mod *ir.Module, lv *ir.LocalVariable) bool {
	if int(lv.Type) >= len(mod.Types) {
		return false
	}
	switch mod.Types[lv.Type].Inner.(type) {
	case ir.ScalarType:
		return true
	default:
		return false
	}
}

// collectEmittedHandles gathers every expression handle that falls
// within a StmtEmit range anywhere in the function body (recursively
// including nested control-flow blocks). Expressions outside any Emit
// range are dead template copies and should not participate in
// Phase A promotion decisions.
func collectEmittedHandles(fn *ir.Function) map[ir.ExpressionHandle]bool {
	m := make(map[ir.ExpressionHandle]bool)
	collectEmitsInBlock(fn.Body, m)
	return m
}

func collectEmitsInBlock(block ir.Block, out map[ir.ExpressionHandle]bool) {
	for _, st := range block {
		switch sk := st.Kind.(type) {
		case ir.StmtEmit:
			for h := sk.Range.Start; h < sk.Range.End; h++ {
				out[h] = true
			}
		case ir.StmtBlock:
			collectEmitsInBlock(sk.Block, out)
		case ir.StmtIf:
			collectEmitsInBlock(sk.Accept, out)
			collectEmitsInBlock(sk.Reject, out)
		case ir.StmtLoop:
			collectEmitsInBlock(sk.Body, out)
			collectEmitsInBlock(sk.Continuing, out)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				collectEmitsInBlock(sk.Cases[ci].Body, out)
			}
		}
	}
}
