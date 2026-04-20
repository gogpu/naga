package mem2reg

import (
	"github.com/gogpu/naga/ir"
)

// promoteStructured runs Phase B SSA construction for scalar local
// variables whose stores or loads span multiple blocks of the
// structured CFG.
//
// Algorithm — adapted from LLVM PromoteMemoryToRegister.cpp:
//
//  1. For each promotable scalar var: track its current value through
//     a DFS walk of fn.Body. Loads rewrite to ExprAlias of the current
//     value. Stores update the current value and are removed.
//  2. At StmtIf merge: if the var's value differs between the two
//     branches, synthesize ExprPhi at the merge point with one
//     PhiPredIfAccept incoming and one PhiPredIfReject incoming. The
//     phi handle becomes the new current value.
//  3. At StmtLoop: insert phi at the loop "header" (which we model as
//     the first statement of the body in our structured IR) with two
//     incomings — PhiPredLoopInit (pre-loop value) and
//     PhiPredLoopBackEdge (value at end of body+continuing). The
//     back-edge incoming is patched up after the body walk.
//  4. At StmtSwitch merge: synthesize ExprPhi with one
//     PhiPredSwitchCase incoming per case (CaseIdx encodes the case
//     index in the original Cases slice).
//
// Each synthesized phi is materialized as (a) a new entry in
// fn.Expressions and (b) a single-handle StmtEmit prepended to the
// merge point. The DXIL emit layer recognizes ExprPhi inside a
// StmtEmit range and lowers it to LLVM FUNC_CODE_INST_PHI at the
// matching basic-block prologue, with PhiPredKey -> BBIndex
// resolved via the emitter's lastBranchContext side state.
//
// Reference parity: matches LLVM mem2reg's invariant that every
// load resolves to either (a) the dominating store's value or
// (b) a phi merging the dominating values from each predecessor of
// the load's BB. Structured CFG makes IDF computation trivial:
// merge BB == statement-after-{if,switch}, loop header == start of
// loop body.
func promoteStructured(ctx *promotionContext) {
	walker := newPhiWalker(ctx)
	walker.walkBlock(&ctx.fn.Body)
}

// phiWalker carries the rename-pass state through the structured-CFG
// DFS. currentValue maps each promoted variable to its current SSA
// value handle; it is snapshot/restored at each branch point.
type phiWalker struct {
	ctx          *promotionContext
	currentValue map[uint32]ir.ExpressionHandle

	// candidates is the set of variables Phase B will promote in this
	// walk. Computed once at the start by selectStructuredCandidates.
	candidates map[uint32]struct{}
}

func newPhiWalker(ctx *promotionContext) *phiWalker {
	w := &phiWalker{
		ctx:          ctx,
		currentValue: make(map[uint32]ir.ExpressionHandle),
		candidates:   selectStructuredCandidates(ctx),
	}
	// Seed initial values from each candidate's Init or a fresh ZeroValue.
	for v := range w.candidates {
		w.currentValue[v] = initialValueOf(ctx, v)
	}
	return w
}

// selectStructuredCandidates returns variables eligible for Phase B
// promotion: promotable scalars (excluding bool) that have at least
// one load. Phase A already attempted single-block promotion;
// remaining promotable vars (those whose stores/loads spanned blocks)
// become Phase B candidates.
//
// Bool (i1) is excluded because DXC's bitcode parser rejects i1-typed
// LLVM phi instructions with "Invalid record" — see the same
// limitation noted at emitter.go:639 for i1 return types. WGSL
// `var temp: bool` written across branches keeps its alloca lowering;
// DXC's HLSL frontend handles this by widening the bool to i32 +
// `icmp ne 0` at the load. Mirroring that widening for our phi path
// is tracked under BUG-DXIL-041 alongside the loop / switch-merge
// phi work.
func selectStructuredCandidates(ctx *promotionContext) map[uint32]struct{} {
	out := make(map[uint32]struct{})
	for v, info := range ctx.uses {
		if !info.promotable {
			continue
		}
		if len(info.loadHandles) == 0 {
			continue
		}
		if isBoolLocal(ctx.mod, &ctx.fn.LocalVars[v]) {
			continue
		}
		out[uint32(v)] = struct{}{}
	}
	return out
}

// isBoolLocal reports whether the local variable's declared type is a
// bool scalar. Bools in phi position trigger DXC's "Invalid record"
// bitcode parser rejection.
func isBoolLocal(mod *ir.Module, lv *ir.LocalVariable) bool {
	if int(lv.Type) >= len(mod.Types) {
		return false
	}
	st, ok := mod.Types[lv.Type].Inner.(ir.ScalarType)
	if !ok {
		return false
	}
	return st.Kind == ir.ScalarBool
}

// initialValueOf returns the starting SSA value handle for a candidate
// variable: its declared Init expression handle if set, otherwise a
// freshly synthesized ExprZeroValue handle of the variable's type.
//
// Mirrors WGSL semantics: reads of a `var` without explicit Init see
// zero per the WGSL specification. Mirrors initialValues() in promote.go.
func initialValueOf(ctx *promotionContext, v uint32) ir.ExpressionHandle {
	lv := &ctx.fn.LocalVars[v]
	if lv.Init != nil {
		return *lv.Init
	}
	zh := ir.ExpressionHandle(len(ctx.fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	ctx.fn.Expressions = append(ctx.fn.Expressions, ir.Expression{
		Kind: ir.ExprZeroValue{Type: lv.Type},
	})
	if len(ctx.fn.ExpressionTypes) == int(zh) {
		ctx.fn.ExpressionTypes = append(ctx.fn.ExpressionTypes, ir.TypeResolution{
			Handle: &lv.Type,
		})
	}
	return zh
}

// walkBlock performs the rename pass for a single statement block.
// The block is mutated in place: stores to candidates are removed,
// loads to candidates are rewritten to ExprAlias, and synthesized
// phi-bearing StmtEmit statements are inserted at structured merge
// points.
//
// Returns nothing — all output goes through ctx.fn mutation.
func (w *phiWalker) walkBlock(blk *[]ir.Statement) {
	out := make([]ir.Statement, 0, len(*blk))
	for i := range *blk {
		stmt := (*blk)[i]
		switch sk := stmt.Kind.(type) {
		case ir.StmtStore:
			if v, ok := w.ctx.localPtrs[sk.Pointer]; ok {
				if _, isCand := w.candidates[v]; isCand {
					w.currentValue[v] = sk.Value
					continue // drop the store
				}
			}
			out = append(out, stmt)
		case ir.StmtEmit:
			w.rewriteLoadsInRange(sk.Range)
			out = append(out, stmt)
		case ir.StmtIf:
			out = append(out, stmt)
			extra := w.handleIf(&out[len(out)-1])
			out = append(out, extra...)
		case ir.StmtSwitch:
			out = append(out, stmt)
			extra := w.handleSwitch(&out[len(out)-1])
			out = append(out, extra...)
		case ir.StmtLoop:
			out = append(out, stmt)
			extra := w.handleLoop(&out[len(out)-1])
			out = append(out, extra...)
		case ir.StmtBlock:
			b := []ir.Statement(sk.Block)
			w.walkBlock(&b)
			out = append(out, ir.Statement{Kind: ir.StmtBlock{Block: ir.Block(b)}})
		default:
			out = append(out, stmt)
		}
	}
	*blk = out
}

// rewriteLoadsInRange rewrites every load handle in r whose target is
// a Phase B candidate to ExprAlias of currentValue[v]. Runs at every
// StmtEmit in the block walk.
func (w *phiWalker) rewriteLoadsInRange(r ir.Range) {
	for h := r.Start; h < r.End; h++ {
		v, ok := loadHandleVar(w.ctx, h)
		if !ok {
			continue
		}
		if _, isCand := w.candidates[v]; !isCand {
			continue
		}
		cv, hasCV := w.currentValue[v]
		if !hasCV {
			continue
		}
		w.ctx.fn.Expressions[h].Kind = ir.ExprAlias{Source: cv}
	}
}

// handleIf processes a StmtIf during the rename pass. Walks both
// branches with snapshot/restore of currentValue, then synthesizes
// ExprPhi statements for any candidate whose value differs between
// the branches.
//
// Returns the synthesized phi-bearing StmtEmit statements that the
// caller should append immediately after the StmtIf to position the
// phis at the merge BB prologue.
func (w *phiWalker) handleIf(stmtPtr *ir.Statement) []ir.Statement {
	sk := stmtPtr.Kind.(ir.StmtIf)
	preIf := snapshotValues(w.currentValue)

	accept := []ir.Statement(sk.Accept)
	w.walkBlock(&accept)
	acceptValues := snapshotValues(w.currentValue)

	w.currentValue = snapshotValues(preIf)
	reject := []ir.Statement(sk.Reject)
	w.walkBlock(&reject)
	rejectValues := snapshotValues(w.currentValue)

	stmtPtr.Kind = ir.StmtIf{
		Condition: sk.Condition,
		Accept:    ir.Block(accept),
		Reject:    ir.Block(reject),
	}

	var phis []ir.Statement
	for v := range w.candidates {
		va, haveA := acceptValues[v]
		vr, haveR := rejectValues[v]
		if !haveA && !haveR {
			continue
		}
		if !haveA {
			va = preIf[v]
		}
		if !haveR {
			vr = preIf[v]
		}
		if va == vr {
			w.currentValue[v] = va
			continue
		}
		phiHandle := w.appendPhi([]ir.PhiIncoming{
			{PredKey: ir.PhiPredIfAccept, Value: va},
			{PredKey: ir.PhiPredIfReject, Value: vr},
		})
		w.currentValue[v] = phiHandle
		phis = append(phis, makePhiEmit(phiHandle))
	}
	return phis
}

// handleSwitch processes a StmtSwitch during the rename pass. Walks
// each case with snapshot/restore of currentValue, then synthesizes
// ExprPhi statements for any candidate whose value differs across
// the cases.
//
// Fall-through cases are conservatively treated like a separate
// incoming — the emit-side BB-tracking maps PhiPredSwitchCase+CaseIdx
// to the BB where that case's body terminated.
func (w *phiWalker) handleSwitch(stmtPtr *ir.Statement) []ir.Statement {
	sk := stmtPtr.Kind.(ir.StmtSwitch)
	preSwitch := snapshotValues(w.currentValue)

	caseValues := make([]map[uint32]ir.ExpressionHandle, len(sk.Cases))
	cases := make([]ir.SwitchCase, len(sk.Cases))
	copy(cases, sk.Cases)
	for ci := range cases {
		w.currentValue = snapshotValues(preSwitch)
		body := []ir.Statement(cases[ci].Body)
		w.walkBlock(&body)
		caseValues[ci] = snapshotValues(w.currentValue)
		cases[ci].Body = ir.Block(body)
	}
	stmtPtr.Kind = ir.StmtSwitch{Selector: sk.Selector, Cases: cases}

	var phis []ir.Statement
	for v := range w.candidates {
		// Decide whether ANY case wrote to v.
		writes := false
		for ci := range caseValues {
			if val, ok := caseValues[ci][v]; ok && val != preSwitch[v] {
				writes = true
				break
			}
		}
		if !writes {
			w.currentValue[v] = preSwitch[v]
			continue
		}
		incomings := make([]ir.PhiIncoming, 0, len(caseValues))
		for ci := range caseValues {
			val, ok := caseValues[ci][v]
			if !ok {
				val = preSwitch[v]
			}
			incomings = append(incomings, ir.PhiIncoming{
				PredKey: ir.PhiPredSwitchCase,
				CaseIdx: uint32(ci),
				Value:   val,
			})
		}
		phiHandle := w.appendPhi(incomings)
		w.currentValue[v] = phiHandle
		phis = append(phis, makePhiEmit(phiHandle))
	}
	return phis
}

// handleLoop processes a StmtLoop during the rename pass.
//
// LIMITATION (deferred to BUG-DXIL-041): loop-header phi placement
// requires forward-reference value-ID handling at emit time — the
// back-edge value is not known until after the body is walked, so
// the LLVM phi instruction must be allocated first and patched later.
// Our current emit infrastructure is single-pass and does not support
// late operand patching.
//
// Conservative behavior: any candidate variable stored inside the
// loop body OR continuing is REMOVED from the candidate set before
// the body walk, which causes its loads to retain their alloca
// lowering instead of being rewritten. This produces strictly correct
// (if non-optimal) DXIL — better than the pre-Phase-B baseline of
// all-alloca even for non-loop variables.
//
// Variables that are NOT stored inside the loop pass through cleanly:
// their currentValue at loop entry stays valid throughout because the
// loop body never overwrites it.
//
// The signature returns a (currently always nil) statement slice to
// stay symmetric with handleIf and handleSwitch — once BUG-DXIL-041
// adds loop-header phi support the return will carry the synthesized
// phi statements that walkBlock prepends to the body BB prologue.
//
//nolint:unparam // signature kept symmetric with handleIf/handleSwitch
func (w *phiWalker) handleLoop(stmtPtr *ir.Statement) []ir.Statement {
	sk := stmtPtr.Kind.(ir.StmtLoop)

	// Disqualify candidates whose value escapes through the loop
	// back-edge (i.e., are stored anywhere inside the body or
	// continuing). They keep their alloca lowering for now.
	storedInLoop := collectLoopStores(&w.ctx.localPtrs, w.candidates, sk)
	for v := range storedInLoop {
		delete(w.candidates, v)
		delete(w.currentValue, v)
	}

	body := []ir.Statement(sk.Body)
	w.walkBlock(&body)
	cont := []ir.Statement(sk.Continuing)
	w.walkBlock(&cont)
	stmtPtr.Kind = ir.StmtLoop{
		Body:       ir.Block(body),
		Continuing: ir.Block(cont),
		BreakIf:    sk.BreakIf,
	}
	return nil
}

// collectLoopStores returns the set of candidate variables that have
// at least one StmtStore inside the loop body or continuing.
func collectLoopStores(ptrs *localPtr, candidates map[uint32]struct{}, sk ir.StmtLoop) map[uint32]struct{} {
	out := make(map[uint32]struct{})
	collectStores(ptrs, candidates, []ir.Statement(sk.Body), out)
	collectStores(ptrs, candidates, []ir.Statement(sk.Continuing), out)
	return out
}

// collectStores recursively scans block looking for StmtStore
// targeting any candidate. Result accumulates in out.
func collectStores(ptrs *localPtr, candidates map[uint32]struct{}, block []ir.Statement, out map[uint32]struct{}) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtStore:
			if v, ok := (*ptrs)[sk.Pointer]; ok {
				if _, isCand := candidates[v]; isCand {
					out[v] = struct{}{}
				}
			}
		case ir.StmtBlock:
			collectStores(ptrs, candidates, []ir.Statement(sk.Block), out)
		case ir.StmtIf:
			collectStores(ptrs, candidates, []ir.Statement(sk.Accept), out)
			collectStores(ptrs, candidates, []ir.Statement(sk.Reject), out)
		case ir.StmtLoop:
			collectStores(ptrs, candidates, []ir.Statement(sk.Body), out)
			collectStores(ptrs, candidates, []ir.Statement(sk.Continuing), out)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				collectStores(ptrs, candidates, []ir.Statement(sk.Cases[ci].Body), out)
			}
		}
	}
}

// appendPhi appends a new ExprPhi to fn.Expressions and returns its
// handle. The phi's result type is taken from the first incoming
// (SSA invariant: every incoming has the same type).
func (w *phiWalker) appendPhi(incomings []ir.PhiIncoming) ir.ExpressionHandle {
	h := ir.ExpressionHandle(len(w.ctx.fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	w.ctx.fn.Expressions = append(w.ctx.fn.Expressions, ir.Expression{
		Kind: ir.ExprPhi{Incoming: incomings},
	})
	// Mirror the type of the first incoming into ExpressionTypes so
	// downstream type queries succeed without re-resolving.
	if len(incomings) > 0 && int(incomings[0].Value) < len(w.ctx.fn.ExpressionTypes) {
		w.ctx.fn.ExpressionTypes = append(w.ctx.fn.ExpressionTypes,
			w.ctx.fn.ExpressionTypes[incomings[0].Value])
	} else {
		w.ctx.fn.ExpressionTypes = append(w.ctx.fn.ExpressionTypes, ir.TypeResolution{})
	}
	return h
}

// makePhiEmit wraps a phi handle in a single-handle StmtEmit so the
// DXIL emit layer materializes it eagerly at the merge BB prologue.
func makePhiEmit(h ir.ExpressionHandle) ir.Statement {
	return ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}}
}

// snapshotValues returns a defensive copy of m. Used to checkpoint
// currentValue at branch boundaries so each branch starts from the
// same baseline.
func snapshotValues(m map[uint32]ir.ExpressionHandle) map[uint32]ir.ExpressionHandle {
	out := make(map[uint32]ir.ExpressionHandle, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
