package mem2reg

import (
	"github.com/gogpu/naga/ir"
)

// promotionContext bundles the per-function state needed by the
// recursive block walker.
type promotionContext struct {
	mod  *ir.Module
	fn   *ir.Function
	uses []localUseInfo
	// localPtrs maps every ExprLocalVariable handle to its variable
	// index — same map computed by classifyLocals, kept for the
	// rewrite walk to identify load/store targets.
	localPtrs localPtr
}

// promoteBlocks walks every Block in fn.Body recursively and
// promotes any variable whose entire set of stores AND loads is
// confined to a single Block.
//
// Approach (single block at a time):
//
//   - When entering a Block, inspect every StmtStore and every
//     StmtEmit range to discover which promotable variables have
//     uses INSIDE this block.
//   - For each candidate, verify the global use counts match what
//     we observed in this block (i.e., this block holds ALL the
//     variable's stores AND ALL the variable's loads). If so,
//     promote it.
//   - Promotion: walk the block once more in textual order. Track
//     the current value per promoted variable. On StmtEmit, for
//     each load handle in the range whose target is a promoted
//     variable, rewrite fn.Expressions[h].Kind to ExprAlias{Source:
//     currentValue}. On StmtStore, update currentValue and mark
//     the statement for removal.
//   - After the walk, drop the marked StmtStore statements from
//     the block in place.
//
// Nested blocks are walked AFTER the parent block is processed so
// that any variable promoted at a parent level is gone before
// children try to promote it again (idempotent — children would
// classify it as having no remaining uses).
func promoteBlocks(ctx *promotionContext, blk *[]ir.Statement) {
	promoteWithinBlock(ctx, blk)
	// Recurse into nested blocks. We rebuild nested-block slices
	// when promotion deletes statements; afterward we write the
	// new slice back into the parent statement so subsequent
	// emit walks see the mutated body.
	for i := range *blk {
		switch sk := (*blk)[i].Kind.(type) {
		case ir.StmtBlock:
			b := []ir.Statement(sk.Block)
			promoteBlocks(ctx, &b)
			(*blk)[i].Kind = ir.StmtBlock{Block: ir.Block(b)}
		case ir.StmtIf:
			a := []ir.Statement(sk.Accept)
			r := []ir.Statement(sk.Reject)
			promoteBlocks(ctx, &a)
			promoteBlocks(ctx, &r)
			(*blk)[i].Kind = ir.StmtIf{
				Condition: sk.Condition,
				Accept:    ir.Block(a),
				Reject:    ir.Block(r),
			}
		case ir.StmtLoop:
			b := []ir.Statement(sk.Body)
			c := []ir.Statement(sk.Continuing)
			promoteBlocks(ctx, &b)
			promoteBlocks(ctx, &c)
			(*blk)[i].Kind = ir.StmtLoop{
				Body:       ir.Block(b),
				Continuing: ir.Block(c),
				BreakIf:    sk.BreakIf,
			}
		case ir.StmtSwitch:
			cases := make([]ir.SwitchCase, len(sk.Cases))
			copy(cases, sk.Cases)
			for ci := range cases {
				cb := []ir.Statement(cases[ci].Body)
				promoteBlocks(ctx, &cb)
				cases[ci].Body = ir.Block(cb)
			}
			(*blk)[i].Kind = ir.StmtSwitch{Selector: sk.Selector, Cases: cases}
		}
	}
}

// promoteWithinBlock identifies and promotes variables whose entire
// set of uses lives inside blk. Returns silently if no candidates.
func promoteWithinBlock(ctx *promotionContext, blk *[]ir.Statement) {
	// Step 1: count uses per variable inside this block only.
	localStores, localLoads := countLocalUses(ctx, blk)
	if len(localStores) == 0 && len(localLoads) == 0 {
		return
	}

	// Step 2: select candidates whose global use counts match the
	// in-block counts (all stores and all loads contained here).
	candidates := selectBlockCandidates(ctx, localStores, localLoads)
	if len(candidates) == 0 {
		return
	}

	// Step 3: walk the block in textual order, rewriting loads and
	// marking stores for deletion.
	rewriteBlock(ctx, blk, candidates)
}

// countLocalUses tallies, per variable, how many StmtStore
// statements and how many ExprLoad expression-handle uses appear
// within blk (excluding nested control-flow children).
//
// A "load use within blk" means: there is a StmtEmit in blk whose
// range covers an expression handle that is a load of the variable.
// Loads referenced from nested blocks are NOT counted here, which
// is exactly what we need to enforce single-block colocation.
func countLocalUses(ctx *promotionContext, blk *[]ir.Statement) (stores, loads map[uint32]int) {
	stores = make(map[uint32]int)
	loads = make(map[uint32]int)
	for i := range *blk {
		switch sk := (*blk)[i].Kind.(type) {
		case ir.StmtStore:
			if v, ok := ctx.localPtrs[sk.Pointer]; ok {
				if ctx.uses[v].promotable {
					stores[v]++
				}
			}
		case ir.StmtEmit:
			for h := sk.Range.Start; h < sk.Range.End; h++ {
				v, ok := loadHandleVar(ctx, h)
				if !ok {
					continue
				}
				loads[v]++
			}
		}
	}
	return stores, loads
}

// loadHandleVar returns the variable index a given expression
// handle loads, if it is a load of a promotable local variable.
func loadHandleVar(ctx *promotionContext, h ir.ExpressionHandle) (uint32, bool) {
	if int(h) >= len(ctx.fn.Expressions) {
		return 0, false
	}
	load, isLoad := ctx.fn.Expressions[h].Kind.(ir.ExprLoad)
	if !isLoad {
		return 0, false
	}
	v, ok := ctx.localPtrs[load.Pointer]
	if !ok {
		return 0, false
	}
	if !ctx.uses[v].promotable {
		return 0, false
	}
	if _, isPromotedLoad := ctx.uses[v].loadHandleSet[h]; !isPromotedLoad {
		return 0, false
	}
	return v, true
}

// selectBlockCandidates returns the set of variables whose entire
// per-function store and load counts are matched by the in-block
// counts (i.e., the block contains every store and every load).
func selectBlockCandidates(ctx *promotionContext, localStores, localLoads map[uint32]int) map[uint32]struct{} {
	out := make(map[uint32]struct{})
	for v, info := range ctx.uses {
		if !info.promotable {
			continue
		}
		vu := uint32(v)
		// Need every store in this block.
		if localStores[vu] != info.storeCount {
			continue
		}
		// Need every load in this block. Use len(loadHandles) as
		// the total load count (each load handle counts once).
		if localLoads[vu] != len(info.loadHandles) {
			continue
		}
		// Skip variables with zero loads — they're dead-stored
		// and the bitcode emitter handles them by simply not
		// referencing the alloca after promotion. Promoting
		// them anyway does no harm but adds no benefit; skip
		// for simplicity.
		if len(info.loadHandles) == 0 {
			continue
		}
		out[vu] = struct{}{}
	}
	return out
}

// rewriteBlock walks blk in textual order and performs the actual
// promotion: per-variable currentValue tracking, load rewrites to
// ExprAlias, and store removal.
//
// On entry currentValue[v] holds either the variable's Init handle
// (when set) or a freshly synthesized ExprZeroValue handle. Each
// StmtStore updates the value; each load found in a StmtEmit range
// gets ExprAlias{Source: currentValue[v]}.
//
// Stores targeting promoted variables are removed by collecting
// keep-indices and slicing the block in place at the end.
func rewriteBlock(ctx *promotionContext, blk *[]ir.Statement, candidates map[uint32]struct{}) {
	// Initialize currentValue per candidate from the variable's
	// Init expression, or synthesize ExprZeroValue if uninitialized.
	currentValue := initialValues(ctx, candidates)

	// Collect indices of statements that survive promotion.
	keep := make([]int, 0, len(*blk))

	for i := range *blk {
		switch sk := (*blk)[i].Kind.(type) {
		case ir.StmtStore:
			if v, ok := ctx.localPtrs[sk.Pointer]; ok {
				if _, isCand := candidates[v]; isCand {
					currentValue[v] = sk.Value
					// Drop this statement (do not append to keep).
					continue
				}
			}
			keep = append(keep, i)
		case ir.StmtEmit:
			rewriteEmitRange(ctx, sk.Range, candidates, currentValue)
			keep = append(keep, i)
		default:
			keep = append(keep, i)
		}
	}

	// If we dropped any statements, rebuild the block slice in
	// textual order.
	if len(keep) != len(*blk) {
		newBlk := make([]ir.Statement, 0, len(keep))
		for _, idx := range keep {
			newBlk = append(newBlk, (*blk)[idx])
		}
		*blk = newBlk
	}
}

// initialValues returns the initial currentValue map for promotion.
// Each candidate variable maps to its Init expression handle if set,
// otherwise to a freshly appended ExprZeroValue expression handle of
// the variable's declared type.
//
// Loads that occur BEFORE the first store inside the block resolve
// to this initial value, matching WGSL semantics (reads of a `var`
// without explicit Init see zero per the WGSL specification).
func initialValues(ctx *promotionContext, candidates map[uint32]struct{}) map[uint32]ir.ExpressionHandle {
	out := make(map[uint32]ir.ExpressionHandle, len(candidates))
	for v := range candidates {
		lv := &ctx.fn.LocalVars[v]
		if lv.Init != nil {
			out[v] = *lv.Init
			continue
		}
		// Append an ExprZeroValue at the end of fn.Expressions.
		// Note: appended expressions are not covered by any
		// existing StmtEmit range, so they would not be naturally
		// "evaluated" by the emitter. The DXIL emit layer treats
		// ExprZeroValue (and the alias chain leading to it) as a
		// constant — emitZeroValue is called lazily from
		// emitExpression on demand and does not require StmtEmit
		// pre-evaluation. This matches how zero-init lookups
		// happen for unused let-bindings elsewhere.
		zh := ir.ExpressionHandle(len(ctx.fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
		ctx.fn.Expressions = append(ctx.fn.Expressions, ir.Expression{
			Kind: ir.ExprZeroValue{Type: lv.Type},
		})
		// Keep ExpressionTypes parallel to Expressions.
		if len(ctx.fn.ExpressionTypes) == int(zh) {
			ctx.fn.ExpressionTypes = append(ctx.fn.ExpressionTypes, ir.TypeResolution{
				Handle: &lv.Type,
			})
		}
		out[v] = zh
	}
	return out
}

// rewriteEmitRange walks one StmtEmit's expression range. For each
// handle that loads a promoted variable, rewrites the expression
// in fn.Expressions to ExprAlias{Source: currentValue[v]}.
func rewriteEmitRange(ctx *promotionContext, r ir.Range, candidates map[uint32]struct{}, currentValue map[uint32]ir.ExpressionHandle) {
	for h := r.Start; h < r.End; h++ {
		v, ok := loadHandleVar(ctx, h)
		if !ok {
			continue
		}
		if _, isCand := candidates[v]; !isCand {
			continue
		}
		cv, hasCV := currentValue[v]
		if !hasCV {
			continue
		}
		ctx.fn.Expressions[h].Kind = ir.ExprAlias{Source: cv}
	}
}
