package mem2reg

import (
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
	classifyStatements(&fn.Body, localPtrs, uses)

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
			m[ir.ExpressionHandle(h)] = lv.Variable
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
		v, isUse := referencedLocalVar(expr.Kind, localPtrs)
		if !isUse {
			continue
		}
		if load, isLoad := expr.Kind.(ir.ExprLoad); isLoad {
			if _, ptrIsLV := localPtrs[load.Pointer]; ptrIsLV {
				// Only count loads that are inside an Emit range.
				if emitted[hh] {
					uses[v].loadHandles = append(uses[v].loadHandles, hh)
				}
				continue
			}
		}
		// Any other referencer (Access, AccessIndex, Compose,
		// math op, ...) is a disqualifying use.
		uses[v].promotable = false
	}
}

// classifyStatements walks the statement tree under blk and tallies
// StmtStore statements per variable. Disqualifies the variable on
// any other statement that uses a local-var handle as a pointer.
//
// The blk parameter is *[]ir.Statement so that callers can pass
// either fn.Body (declared as []ir.Statement) or a nested ir.Block
// (declared as []ir.Statement under a different name) without an
// awkward conversion.
func classifyStatements(blk *[]ir.Statement, localPtrs localPtr, uses []localUseInfo) {
	for i := range *blk {
		switch sk := (*blk)[i].Kind.(type) {
		case ir.StmtStore:
			if v, ok := localPtrs[sk.Pointer]; ok {
				uses[v].storeCount++
			}
		case ir.StmtAtomic:
			if v, ok := localPtrs[sk.Pointer]; ok {
				uses[v].promotable = false
			}
		case ir.StmtImageStore:
			if v, ok := localPtrs[sk.Image]; ok {
				uses[v].promotable = false
			}
		case ir.StmtCall:
			for _, a := range sk.Arguments {
				if v, ok := localPtrs[a]; ok {
					uses[v].promotable = false
				}
			}
		case ir.StmtBlock:
			// Block is []Statement; alias to the same slice header.
			b := []ir.Statement(sk.Block)
			classifyStatements(&b, localPtrs, uses)
		case ir.StmtIf:
			a := []ir.Statement(sk.Accept)
			r := []ir.Statement(sk.Reject)
			classifyStatements(&a, localPtrs, uses)
			classifyStatements(&r, localPtrs, uses)
		case ir.StmtLoop:
			b := []ir.Statement(sk.Body)
			c := []ir.Statement(sk.Continuing)
			classifyStatements(&b, localPtrs, uses)
			classifyStatements(&c, localPtrs, uses)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				cb := []ir.Statement(sk.Cases[ci].Body)
				classifyStatements(&cb, localPtrs, uses)
			}
		}
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

// referencedLocalVar reports whether the expression kind references
// a local-var pointer handle, returning the variable index if so.
//
// ExprLocalVariable itself is NOT reported: this function asks
// "does THIS expression USE some other handle that names a local
// var?" — the canonical use being ExprLoad{Pointer: <lv handle>}.
//
// Implementation: collect every sub-handle reachable from the
// expression kind into a small fixed-capacity buffer, then probe
// the localPtrs map. Splitting into "collect" + "probe" lets the
// dispatcher stay shallow and keeps complexity below the lint
// threshold without sacrificing exhaustive coverage.
func referencedLocalVar(kind ir.ExpressionKind, localPtrs localPtr) (uint32, bool) {
	var buf [16]ir.ExpressionHandle
	handles := collectExpressionHandles(kind, buf[:0])
	for _, h := range handles {
		if v, ok := localPtrs[h]; ok {
			return v, true
		}
	}
	return 0, false
}

// collectExpressionHandles returns the slice of every ExpressionHandle
// directly referenced by kind, appended onto out.
//
// Expression kinds that hold no sub-handles (literals, ExprConstant,
// ExprLocalVariable, ExprGlobalVariable, ExprFunctionArgument,
// ExprAtomicResult, ExprCallResult, ExprZeroValue, ExprOverride, ...)
// return out unchanged. Aggregate kinds (ExprCompose) append every
// component handle.
func collectExpressionHandles(kind ir.ExpressionKind, out []ir.ExpressionHandle) []ir.ExpressionHandle {
	switch k := kind.(type) {
	case ir.ExprLoad:
		out = append(out, k.Pointer)
	case ir.ExprAccess:
		out = append(out, k.Base, k.Index)
	case ir.ExprAccessIndex:
		out = append(out, k.Base)
	case ir.ExprBinary:
		out = append(out, k.Left, k.Right)
	case ir.ExprUnary:
		out = append(out, k.Expr)
	case ir.ExprSelect:
		out = append(out, k.Condition, k.Accept, k.Reject)
	case ir.ExprSplat:
		out = append(out, k.Value)
	case ir.ExprSwizzle:
		out = append(out, k.Vector)
	case ir.ExprCompose:
		out = append(out, k.Components...)
	case ir.ExprAs:
		out = append(out, k.Expr)
	case ir.ExprDerivative:
		out = append(out, k.Expr)
	case ir.ExprMath:
		out = append(out, k.Arg)
		out = appendOptional(out, k.Arg1)
		out = appendOptional(out, k.Arg2)
		out = appendOptional(out, k.Arg3)
	case ir.ExprRelational:
		out = append(out, k.Argument)
	case ir.ExprArrayLength:
		out = append(out, k.Array)
	case ir.ExprImageSample:
		out = append(out, k.Image, k.Sampler, k.Coordinate)
		out = appendOptional(out, k.ArrayIndex)
		out = appendOptional(out, k.Offset)
		out = appendOptional(out, k.DepthRef)
	case ir.ExprImageLoad:
		out = append(out, k.Image, k.Coordinate)
		out = appendOptional(out, k.ArrayIndex)
		out = appendOptional(out, k.Sample)
		out = appendOptional(out, k.Level)
	case ir.ExprImageQuery:
		out = append(out, k.Image)
	case ir.ExprAlias:
		out = append(out, k.Source)
	}
	return out
}

// appendOptional appends *h to out when h is non-nil; otherwise
// returns out unchanged. Saves a 4-line if-block at every nullable
// expression handle field.
func appendOptional(out []ir.ExpressionHandle, h *ir.ExpressionHandle) []ir.ExpressionHandle {
	if h != nil {
		return append(out, *h)
	}
	return out
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
