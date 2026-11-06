package mem2reg

import (
	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// promoteStructured runs Phase B SSA construction for scalar local
// variables whose stores or loads span multiple blocks of the
// structured CFG.
//
// Algorithm — adapted from LLVM PromoteMemoryToRegister.cpp:
//
//  1. For each promotable scalar var: track its current value through
//     a DFS walk of fn.Body. Loads rewrite to ssa.Alias of the current
//     value. Stores update the current value and are removed.
//  2. At StmtIf merge: if the var's value differs between the two
//     branches, synthesize ssa.Phi at the merge point with one
//     ssa.PredIfAccept incoming and one ssa.PredIfReject incoming. The
//     phi handle becomes the new current value.
//  3. At StmtLoop: insert phi at the loop "header" (which we model as
//     the first statement of the body in our structured IR) with two
//     incomings — ssa.PredLoopInit (pre-loop value) and
//     ssa.PredLoopBackEdge (value at end of body+continuing). The
//     back-edge incoming is patched up after the body walk.
//  4. At StmtSwitch merge: synthesize ssa.Phi with one
//     ssa.PredSwitchCase incoming per case (CaseIdx encodes the case
//     index in the original Cases slice).
//
// Each synthesized phi is materialized as (a) a new entry in
// fn.Expressions and (b) a single-handle StmtEmit prepended to the
// merge point. The DXIL emit layer recognizes ssa.Phi inside a
// StmtEmit range and lowers it to LLVM FUNC_CODE_INST_PHI at the
// matching basic-block prologue, with ssa.PredKey -> BBIndex
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
	walker.walkBlock((*[]ir.Statement)(&ctx.fn.Body))
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
	for _, v := range sortedVars(w.candidates) {
		w.currentValue[v] = initialValueOf(ctx, v)
	}
	return w
}

// selectStructuredCandidates returns variables eligible for Phase B
// promotion: promotable scalars that have at least one load. Phase A
// already attempted single-block promotion; remaining promotable vars
// (those whose stores/loads spanned blocks) become Phase B candidates.
//
// Bools are candidates too: a register bool is i1 in DXIL, and DXC's
// mem2reg leaves `phi i1` at a merge (its output for a short-circuit
// `||` chain, or for atomics.wgsl). Only memory holds bools widened to
// i32; a bool kept in an alloca is the shape DXC never emits.
func selectStructuredCandidates(ctx *promotionContext) map[uint32]struct{} {
	out := make(map[uint32]struct{})
	for v, info := range ctx.uses {
		if !info.promotable {
			continue
		}
		if len(info.loadHandles) == 0 {
			continue
		}
		out[uint32(v)] = struct{}{}
	}
	// A variable stored inside a loop keeps its alloca (no loop-header
	// phis yet, see handleLoop). Decide that before the walk: the walk
	// drops each candidate's stores as it goes, so a store before the
	// loop (`var j = i;` lowers to one) was already gone when the loop
	// disqualified the variable, and the loop then read an
	// uninitialized alloca.
	dropLoopStoredCandidates(&ctx.localPtrs, out, ctx.fn.Body)
	dropUnmergeableSwitchCandidates(&ctx.localPtrs, out, ctx.fn.Body)
	return out
}

// dropUnmergeableSwitchCandidates removes from candidates every variable
// stored inside a switch whose merge has predecessors a ssa.PredSwitchCase
// incoming cannot name: a break that is not at the end of its case (it
// branches to the merge from a block inside the case), a fall-through case
// (its end branches to the next case), or a missing default (the last
// comparison branches to the merge). Those variables keep their alloca,
// which is always right.
func dropUnmergeableSwitchCandidates(ptrs *localPtr, candidates map[uint32]struct{}, block []ir.Statement) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtSwitch:
			if !switchMergeIsCaseEnds(sk) {
				stored := make(map[uint32]struct{})
				for ci := range sk.Cases {
					collectStores(ptrs, candidates, []ir.Statement(sk.Cases[ci].Body), stored)
				}
				for v := range stored {
					delete(candidates, v)
				}
			}
			for ci := range sk.Cases {
				dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Cases[ci].Body)
			}
		case ir.StmtBlock:
			dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Block)
		case ir.StmtIf:
			dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Accept)
			dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Reject)
		case ir.StmtLoop:
			dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Body)
			dropUnmergeableSwitchCandidates(ptrs, candidates, sk.Continuing)
		}
	}
}

// switchMergeIsCaseEnds reports whether the only predecessors of the
// switch's merge are the end blocks of its cases.
func switchMergeIsCaseEnds(sk ir.StmtSwitch) bool {
	hasDefault := false
	for ci := range sk.Cases {
		c := &sk.Cases[ci]
		if _, ok := c.Value.(ir.SwitchValueDefault); ok {
			hasDefault = true
		}
		if c.FallThrough || hasInnerBreak(c.Body, true) {
			return false
		}
	}
	return hasDefault
}

// hasInnerBreak reports whether block holds a break that leaves the switch
// around it from anywhere but the end of the case (tail reports whether
// block's last statement is the end of the case). Breaks inside a nested
// loop or switch leave that construct and do not count.
func hasInnerBreak(block []ir.Statement, tail bool) bool {
	for i := range block {
		last := tail && i == len(block)-1
		switch sk := block[i].Kind.(type) {
		case ir.StmtBreak:
			if !last {
				return true
			}
		case ir.StmtBlock:
			if hasInnerBreak(sk.Block, last) {
				return true
			}
		case ir.StmtIf:
			if hasInnerBreak(sk.Accept, false) || hasInnerBreak(sk.Reject, false) {
				return true
			}
		}
	}
	return false
}

// dropLoopStoredCandidates removes from candidates every variable stored
// inside any loop of block, at any depth.
func dropLoopStoredCandidates(ptrs *localPtr, candidates map[uint32]struct{}, block []ir.Statement) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtLoop:
			for v := range collectLoopStores(ptrs, candidates, sk) {
				delete(candidates, v)
			}
		case ir.StmtBlock:
			dropLoopStoredCandidates(ptrs, candidates, sk.Block)
		case ir.StmtIf:
			dropLoopStoredCandidates(ptrs, candidates, sk.Accept)
			dropLoopStoredCandidates(ptrs, candidates, sk.Reject)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				dropLoopStoredCandidates(ptrs, candidates, sk.Cases[ci].Body)
			}
		}
	}
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
	return zh
}

// blockExit is how control leaves the end of a block, in the emitter's
// terms (emit/statements.go): a block that ends with a break, continue or
// return ends its basic block with that branch, so it is no predecessor of
// the enclosing if's merge; a break is the one exit that still reaches a
// switch's merge, from the case's own end block.
type blockExit uint8

const (
	exitFallsThrough blockExit = iota
	exitBreak
	exitOther // continue or return
)

// walkBlock performs the rename pass for a single statement block.
// The block is mutated in place: stores to candidates are removed,
// loads to candidates are rewritten to ssa.Alias, and synthesized
// phi-bearing StmtEmit statements are inserted at structured merge
// points. Statements after a break, continue or return never run and
// are dropped, as the emitter drops them.
func (w *phiWalker) walkBlock(blk *[]ir.Statement) blockExit {
	out := make([]ir.Statement, 0, len(*blk))
	exit := exitFallsThrough
	for i := range *blk {
		stmt := (*blk)[i]
		switch sk := stmt.Kind.(type) {
		case ir.StmtBreak:
			out = append(out, stmt)
			exit = exitBreak
		case ir.StmtContinue, ir.StmtReturn:
			out = append(out, stmt)
			exit = exitOther
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
			exit = w.walkBlock(&b)
			out = append(out, ir.Statement{Kind: ir.StmtBlock{Block: ir.Block(b)}})
		default:
			out = append(out, stmt)
		}
		if exit != exitFallsThrough {
			break
		}
	}
	*blk = out
	return exit
}

// rewriteLoadsInRange rewrites every load handle in r whose target is
// a Phase B candidate to ssa.Alias of currentValue[v]. Runs at every
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
		w.ctx.fn.Expressions[h].Kind = ssa.Alias{Source: cv}
	}
}

// handleIf processes a StmtIf during the rename pass. Walks both
// branches with snapshot/restore of currentValue, then synthesizes
// ssa.Phi statements for any candidate whose value differs between
// the branches.
//
// Returns the synthesized phi-bearing StmtEmit statements that the
// caller should append immediately after the StmtIf to position the
// phis at the merge BB prologue.
func (w *phiWalker) handleIf(stmtPtr *ir.Statement) []ir.Statement {
	sk := stmtPtr.Kind.(ir.StmtIf)
	preIf := snapshotValues(w.currentValue)

	accept := []ir.Statement(sk.Accept)
	acceptExit := w.walkBlock(&accept)
	acceptValues := snapshotValues(w.currentValue)

	w.currentValue = snapshotValues(preIf)
	reject := []ir.Statement(sk.Reject)
	rejectExit := w.walkBlock(&reject)
	rejectValues := snapshotValues(w.currentValue)

	stmtPtr.Kind = ir.StmtIf{
		Condition: sk.Condition,
		Accept:    ir.Block(accept),
		Reject:    ir.Block(reject),
	}

	// A branch that leaves with a break, continue or return is no
	// predecessor of the merge: the merge sees only the other branch's
	// values, and a phi with an incoming from it would be invalid.
	switch {
	case acceptExit != exitFallsThrough && rejectExit != exitFallsThrough:
		// The merge is unreachable; any value is right there.
		w.currentValue = snapshotValues(preIf)
		return nil
	case acceptExit != exitFallsThrough:
		w.currentValue = rejectValues
		return nil
	case rejectExit != exitFallsThrough:
		w.currentValue = acceptValues
		return nil
	}

	var phis []ir.Statement
	for _, v := range sortedVars(w.candidates) {
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
		phiHandle := w.appendPhi([]ssa.Incoming{
			{PredKey: ssa.PredIfAccept, Value: va},
			{PredKey: ssa.PredIfReject, Value: vr},
		})
		w.currentValue[v] = phiHandle
		phis = append(phis, makePhiEmit(phiHandle))
	}
	return phis
}

// handleSwitch processes a StmtSwitch during the rename pass. Walks
// each case with snapshot/restore of currentValue, then synthesizes
// ssa.Phi statements for any candidate whose value differs across
// the cases.
//
// Fall-through cases are conservatively treated like a separate
// incoming — the emit-side BB-tracking maps ssa.PredSwitchCase+CaseIdx
// to the BB where that case's body terminated.
func (w *phiWalker) handleSwitch(stmtPtr *ir.Statement) []ir.Statement {
	sk := stmtPtr.Kind.(ir.StmtSwitch)
	preSwitch := snapshotValues(w.currentValue)

	caseValues := make([]map[uint32]ir.ExpressionHandle, len(sk.Cases))
	reachesMerge := make([]bool, len(sk.Cases))
	cases := make([]ir.SwitchCase, len(sk.Cases))
	copy(cases, sk.Cases)
	for ci := range cases {
		w.currentValue = snapshotValues(preSwitch)
		body := []ir.Statement(cases[ci].Body)
		// A case's end block branches to the merge when the case falls
		// off its end or ends with a break; a continue or return leaves
		// elsewhere. A fall-through case branches to the next case
		// instead (candidates stored in such a switch were dropped).
		reachesMerge[ci] = w.walkBlock(&body) != exitOther && !cases[ci].FallThrough
		caseValues[ci] = snapshotValues(w.currentValue)
		cases[ci].Body = ir.Block(body)
	}
	stmtPtr.Kind = ir.StmtSwitch{Selector: sk.Selector, Cases: cases}

	var phis []ir.Statement
	for _, v := range sortedVars(w.candidates) {
		incomings := switchMergeIncomings(v, caseValues, reachesMerge, preSwitch)
		switch {
		case len(incomings) == 0:
			// No case reaches the merge: it is unreachable.
			w.currentValue[v] = preSwitch[v]
		case allSameValue(incomings):
			// One value along every edge (the one before the switch, or
			// one every case stored): no phi.
			w.currentValue[v] = incomings[0].Value
		default:
			phiHandle := w.appendPhi(incomings)
			w.currentValue[v] = phiHandle
			phis = append(phis, makePhiEmit(phiHandle))
		}
	}
	return phis
}

// switchMergeIncomings returns v's value along each edge into the switch's
// merge, one per case that reaches it.
func switchMergeIncomings(v uint32, caseValues []map[uint32]ir.ExpressionHandle, reachesMerge []bool, preSwitch map[uint32]ir.ExpressionHandle) []ssa.Incoming {
	incomings := make([]ssa.Incoming, 0, len(caseValues))
	for ci := range caseValues {
		if !reachesMerge[ci] {
			continue
		}
		val, ok := caseValues[ci][v]
		if !ok {
			val = preSwitch[v]
		}
		incomings = append(incomings, ssa.Incoming{
			PredKey: ssa.PredSwitchCase,
			CaseIdx: uint32(ci),
			Value:   val,
		})
	}
	return incomings
}

func allSameValue(incomings []ssa.Incoming) bool {
	for _, in := range incomings[1:] {
		if in.Value != incomings[0].Value {
			return false
		}
	}
	return true
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

// appendPhi appends a new ssa.Phi to fn.Expressions and returns its
// handle. The typifier gives the phi the type of its incoming values
// (SSA invariant: every incoming has the same type).
func (w *phiWalker) appendPhi(incomings []ssa.Incoming) ir.ExpressionHandle {
	h := ir.ExpressionHandle(len(w.ctx.fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	w.ctx.fn.Expressions = append(w.ctx.fn.Expressions, ir.Expression{
		Kind: ssa.Phi{Incoming: incomings},
	})
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
