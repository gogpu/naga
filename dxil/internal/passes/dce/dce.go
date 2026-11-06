// Package dce implements dead code elimination for DXIL shader functions.
//
// This pass removes expressions and statements that do not transitively
// contribute to any observable side effect: stores to global outputs,
// UAV writes, image stores, atomics, barriers, function calls, or
// shader returns.
//
// The algorithm is mark-and-sweep, inspired by LLVM's DeadCodeElimination
// pass (which DXC runs after mem2reg) and Mesa's nir_opt_dce:
//
//  1. Mark: walk the statement tree, find "roots" — statements with
//     observable side effects. Transitively mark every expression
//     reachable from those roots as live.
//
//  2. Identify dead locals: a local variable is dead if none of its
//     loads are reachable from any live root. Its stores are dead too.
//
//  3. Sweep: walk the statement tree again, removing dead stores and
//     shrinking StmtEmit ranges to exclude dead expressions.
//
// Dead control-flow elimination (ADCE-style): a condition, selector or
// break-if is live only when the construct it controls holds live content
// (controlDeps). An if or a switch without live content is removed, and
// so is a loop, as DXC's LoopDeletion removes a loop with no effect
// whatever its trip count (HLSL forward progress).
//
// References:
//   - DXC pipeline: DxilLinker.cpp:1284 (mem2reg -> SimplifyInst -> CFGSimplify -> DCE -> GlobalDCE)
//   - Mesa NIR: nir_opt_dce (mark-and-sweep, iterative until stable)
//   - LLVM: lib/Transforms/Scalar/DCE.cpp (worklist-based trivially-dead removal)
package dce

import (
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// Run eliminates dead code from fn. It must be called AFTER mem2reg
// so that promoted locals have already been converted to SSA aliases,
// leaving only genuinely-needed alloca/load/store triples and dead
// remnants.
//
// The pass is conservative: anything it cannot prove dead is kept.
// On any internal inconsistency it returns without modifying fn,
// making it safe to call unconditionally.
func Run(mod *ir.Module, fn *ir.Function) {
	if mod == nil || fn == nil || len(fn.Expressions) == 0 {
		return
	}

	// Liveness, then the ifs, switches and loops with nothing live in them
	// go, as DXC runs CFGSimplification and LoopDeletion before DCE
	// (DxilLinker.cpp:1282-1284). A condition is live when its construct
	// holds live content (controlDeps). Liveness is computed afresh once a
	// construct goes, so a condition shared with a construct that stays
	// stays live; taking liveness back from a condition instead missed
	// statement uses (P1-DXIL-061). Removing a construct can leave another
	// with nothing live, so this repeats until nothing more goes.
	live, deadLocals := liveness(mod, fn)
	for {
		body, dropped := dropDeadControlFlow(mod, fn, fn.Body, deadLocals, live, buildLocalPtrMap(fn))
		if !dropped {
			break
		}
		fn.Body = body
		live, deadLocals = liveness(mod, fn)
	}

	// Sweep dead statements and shrink emit ranges.
	// Even when no dead locals exist, the sweep still shrinks emit ranges
	// for dead expressions. Without this, shaders that compute into
	// only let-bindings (zero locals, zero outputs) retain all dead
	// instructions in the DXIL body, diverging from DXC which runs
	// LLVM DCE and strips them.
	fn.Body = sweepBlock(fn, fn.Body, deadLocals, live)
}

// liveness returns which expressions of fn are live and which locals are
// dead: everything a side-effecting root reaches, through the locals and
// pure calls that feed it.
func liveness(mod *ir.Module, fn *ir.Function) ([]bool, map[uint32]bool) {
	// Phase 1: mark expressions reachable from side-effecting roots.
	live := markLive(mod, fn)

	// Phase 2: identify dead local variables.
	deadLocals := findDeadLocals(fn, live)

	// Phase 2b: for live locals, mark the values stored INTO them as
	// live. The initial mark pass skips local stores because they're
	// not roots. But if the local itself is live (has a live load),
	// the stored value must also be live so its emit range is preserved.
	// A value stored into one local can load another, which is live
	// then too, so this runs until no more locals come alive.
	//
	// Phase 2b-fixup: after local-store propagation, some ExprCallResult
	// handles may have become live through local store chains. Retroactively
	// mark the arguments of those calls so the sweep keeps them.
	//
	// Both only ever add liveness, and each can feed the other (a call's
	// argument loads a local; a local holds a call's result; a call's
	// result is the argument of the call before it), so they repeat until
	// nothing more comes alive, as Mesa's nir_opt_dce iterates its worklist.
	// Stopping earlier left the arguments of the first calls of a chain of
	// pure calls dead.
	var mark func(h ir.ExpressionHandle)
	mark = func(h ir.ExpressionHandle) {
		if int(h) >= len(live) || live[h] {
			return
		}
		live[h] = true
		irutil.ExprHandles(fn.Expressions[h].Kind, mark)
	}
	//
	// Phase 2c: the conditions of the constructs that hold live content
	// (controlDeps). A condition loads locals and calls pure
	// functions like any other live value, and a store it makes live can
	// make another construct live, so it takes part in the same fixed point.
	localPtrs := buildLocalPtrMap(fn)
	for {
		before := countLive(live)
		markLiveLocalStoreValues(fn, live, deadLocals)
		propagateCallResultLiveness(fn.Body, live, mark)
		deadLocals = findDeadLocals(fn, live)
		cd := controlDeps{mod: mod, fn: fn, live: live, deadLocals: deadLocals, localPtrs: localPtrs, mark: mark}
		cd.block(fn.Body, false, false)
		if countLive(live) == before {
			break
		}
	}
	return live, deadLocals
}

// controlDeps decides which constructs hold live content and, when mark is
// set, marks the condition, selector or break-if of each such construct
// live. This is the control-dependence half of LLVM's aggressive DCE: a
// branch is live only when something live depends on which way it goes.
// DXC's LLVM 3.7 ADCE keeps every terminator and leaves the rest to
// SimplifyCFG and LoopDeletion (DxilLinker.cpp:1282-1284); in a structured
// IR the three come down to this one walk.
//
// What a construct holds that makes it live:
//   - a statement with an observable effect: a store to a global, an
//     atomic, an image store or atomic, a barrier, a kill, a return, a
//     workgroup uniform load, a ray query, a subgroup operation, a call to a
//     function with side effects;
//   - a store to a live local, a live pure call, an emit with a live
//     expression (a header phi read after its loop is one);
//   - a phi right after it that is live (feedsLivePhi);
//   - a break or continue whose target, the innermost loop or switch, is
//     live by what else it holds: leaving a live construct early decides
//     what of it runs.
//
// A loop with none of it goes whole, as LLVM's LoopDeletion removes it: the
// HLSL change there (LoopDeletion.cpp, "remove loops even cannot solve the
// trip count") drops the termination check, since HLSL loops make forward
// progress. A loop with a return in it has a second exit and stays, as
// LoopDeletion keeps a loop with more than one exit block.
type controlDeps struct {
	mod        *ir.Module
	fn         *ir.Function
	live       []bool
	deadLocals map[uint32]bool
	localPtrs  map[ir.ExpressionHandle]uint32
	// mark marks an expression and its operands live; nil only asks.
	mark func(ir.ExpressionHandle)
}

// block reports whether block holds live content. breakLive and
// continueLive say whether the targets of a break and of a continue in it
// are live.
func (cd *controlDeps) block(block ir.Block, breakLive, continueLive bool) bool {
	holds := false
	for i := range block {
		// Every statement is visited, so the conditions in it are marked
		// even after the block is known to be live.
		if cd.stmt(block, i, breakLive, continueLive) {
			holds = true
		}
	}
	return holds
}

// stmt reports whether block[i] is, or holds, live content.
func (cd *controlDeps) stmt(block ir.Block, i int, breakLive, continueLive bool) bool {
	switch sk := block[i].Kind.(type) {
	case ir.StmtBlock:
		return cd.block(sk.Block, breakLive, continueLive)
	case ir.StmtIf:
		accept := cd.block(sk.Accept, breakLive, continueLive)
		reject := cd.block(sk.Reject, breakLive, continueLive)
		holds := accept || reject || feedsLivePhi(cd.fn, block, i, cd.live)
		if holds {
			cd.markLive(sk.Condition)
		}
		return holds
	case ir.StmtSwitch:
		holds := cd.switchCases(sk, false, continueLive) || feedsLivePhi(cd.fn, block, i, cd.live)
		if holds {
			// Now that the switch is live, a break out of it counts.
			cd.switchCases(sk, true, continueLive)
			cd.markLive(sk.Selector)
		}
		return holds
	case ir.StmtLoop:
		return cd.loop(sk) || feedsLivePhi(cd.fn, block, i, cd.live)
	default:
		return cd.simpleStmt(sk, breakLive, continueLive)
	}
}

// simpleStmt reports whether a statement that holds no block is live
// content.
func (cd *controlDeps) simpleStmt(kind ir.StatementKind, breakLive, continueLive bool) bool {
	switch sk := kind.(type) {
	case ir.StmtEmit:
		for h := sk.Range.Start; h < sk.Range.End; h++ {
			if isLiveInRange(h, cd.live) {
				return true
			}
		}
		return false
	case ir.StmtStore:
		if varIdx, isLocal := isLocalStore(cd.fn, sk.Pointer, cd.localPtrs); isLocal {
			return !cd.deadLocals[varIdx]
		}
		return true
	case ir.StmtCall:
		return calleeHasSideEffects(cd.mod, sk.Function) || sweepCallIsLive(sk, cd.live)
	case ir.StmtBreak:
		return breakLive
	case ir.StmtContinue:
		return continueLive
	default:
		// Returns, kills, barriers, atomics, image stores and atomics,
		// workgroup uniform loads, ray queries, subgroup operations and any
		// kind added later: observable, so live.
		return true
	}
}

func (cd *controlDeps) switchCases(sk ir.StmtSwitch, breakLive, continueLive bool) bool {
	holds := false
	for ci := range sk.Cases {
		if cd.block(sk.Cases[ci].Body, breakLive, continueLive) {
			holds = true
		}
	}
	return holds
}

// loop reports whether a loop holds live content other than its own
// breaks and continues; when it does, it marks its break-if and, walking
// the body again with them counted, the conditions of the ifs that leave
// it. A break or continue in a loop always targets that loop or one nested
// in it, so the answer does not depend on what is around the loop.
func (cd *controlDeps) loop(sk ir.StmtLoop) bool {
	body := cd.block(sk.Body, false, false)
	cont := cd.block(sk.Continuing, false, false)
	if !body && !cont {
		return false
	}
	cd.block(sk.Body, true, true)
	cd.block(sk.Continuing, true, true)
	if sk.BreakIf != nil {
		cd.markLive(*sk.BreakIf)
	}
	return true
}

func (cd *controlDeps) markLive(h ir.ExpressionHandle) {
	if cd.mark != nil {
		cd.mark(h)
	}
}

func countLive(live []bool) int {
	n := 0
	for _, l := range live {
		if l {
			n++
		}
	}
	return n
}

// markLive performs the mark phase. It walks the statement tree and
// for every root statement with observable side effects, transitively
// marks all referenced expressions as live.
//
// A root is any statement that writes to something observable:
//   - StmtStore targeting a GLOBAL variable (output, UAV, workgroup memory)
//   - StmtReturn with a value
//   - StmtImageStore, StmtImageAtomic
//   - StmtAtomic on a global
//   - StmtControlBarrier (execution side effect)
//   - StmtKill (terminates invocation)
//   - StmtCall (may have arbitrary side effects)
//   - StmtWorkGroupUniformLoad (barrier + load)
//   - StmtSubgroupBallot, StmtSubgroupCollectiveOperation, StmtSubgroupGather
//   - StmtRayQuery
//
// StmtStore targeting a LOCAL variable is NOT a root — it only
// becomes live if the stored value is itself consumed by a root
// (transitively via a load of that local).
func markLive(mod *ir.Module, fn *ir.Function) []bool {
	live := make([]bool, len(fn.Expressions))

	// Build a map from ExprLocalVariable handle -> variable index
	// so we can distinguish local stores from global stores.
	localPtrs := buildLocalPtrMap(fn)

	// Mark helper: recursively mark an expression and all its
	// sub-expressions as live.
	var mark func(h ir.ExpressionHandle)
	mark = func(h ir.ExpressionHandle) {
		if int(h) >= len(live) || live[h] {
			return
		}
		live[h] = true
		// Transitively mark all sub-expressions.
		irutil.ExprHandles(fn.Expressions[h].Kind, mark)
	}

	// Walk statements and mark roots.
	markBlockRoots(mod, fn, fn.Body, localPtrs, mark)

	// Fixup: if a pure call's ExprCallResult was marked live by a
	// downstream consumer, retroactively mark the call's arguments.
	propagateCallResultLiveness(fn.Body, live, mark)

	return live
}

// propagateCallResultLiveness scans for StmtCalls whose Result is
// live (consumed downstream) but whose arguments weren't marked
// (pure callee, initially skipped). Marks those arguments live.
func propagateCallResultLiveness(block ir.Block, live []bool, mark func(ir.ExpressionHandle)) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtCall:
			if sk.Result != nil && int(*sk.Result) < len(live) && live[*sk.Result] {
				for _, a := range sk.Arguments {
					mark(a)
				}
			}
		case ir.StmtIf:
			propagateCallResultLiveness(sk.Accept, live, mark)
			propagateCallResultLiveness(sk.Reject, live, mark)
		case ir.StmtLoop:
			propagateCallResultLiveness(sk.Body, live, mark)
			propagateCallResultLiveness(sk.Continuing, live, mark)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				propagateCallResultLiveness(sk.Cases[ci].Body, live, mark)
			}
		case ir.StmtBlock:
			propagateCallResultLiveness(sk.Block, live, mark)
		}
	}
}

// markLiveLocalStoreValues walks the statement tree a second time.
// For every StmtStore that targets a live local (not in deadLocals),
// it transitively marks the stored value expression as live.
//
// The initial mark phase skips local stores because they aren't
// side-effecting roots. But when a local is live (its load feeds a
// live root), the values written to it must also be live so their
// emit ranges are preserved and the emitter can resolve the value IDs.
func markLiveLocalStoreValues(fn *ir.Function, live []bool, deadLocals map[uint32]bool) {
	localPtrs := buildLocalPtrMap(fn)

	var mark func(h ir.ExpressionHandle)
	mark = func(h ir.ExpressionHandle) {
		if int(h) >= len(live) || live[h] {
			return
		}
		live[h] = true
		irutil.ExprHandles(fn.Expressions[h].Kind, mark)
	}

	markLiveStoresBlock(fn, fn.Body, localPtrs, deadLocals, mark)
}

// markLiveStoresBlock recurses through a block marking store values
// for live locals.
func markLiveStoresBlock(
	fn *ir.Function,
	block ir.Block,
	localPtrs map[ir.ExpressionHandle]uint32,
	deadLocals map[uint32]bool,
	mark func(ir.ExpressionHandle),
) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtStore:
			varIdx, isLocal := isLocalStore(fn, sk.Pointer, localPtrs)
			if isLocal && !deadLocals[varIdx] {
				// Live local — mark the stored value as live.
				mark(sk.Value)
			}
		case ir.StmtIf:
			markLiveStoresBlock(fn, sk.Accept, localPtrs, deadLocals, mark)
			markLiveStoresBlock(fn, sk.Reject, localPtrs, deadLocals, mark)
		case ir.StmtLoop:
			markLiveStoresBlock(fn, sk.Body, localPtrs, deadLocals, mark)
			markLiveStoresBlock(fn, sk.Continuing, localPtrs, deadLocals, mark)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				markLiveStoresBlock(fn, sk.Cases[ci].Body, localPtrs, deadLocals, mark)
			}
		case ir.StmtBlock:
			markLiveStoresBlock(fn, sk.Block, localPtrs, deadLocals, mark)
		}
	}
}

// markBlockRoots walks a block, calling mark for every expression
// handle referenced by side-effecting (root) statements.
func markBlockRoots(
	mod *ir.Module,
	fn *ir.Function,
	block ir.Block,
	localPtrs map[ir.ExpressionHandle]uint32,
	mark func(ir.ExpressionHandle),
) {
	for i := range block {
		markStmtRoots(mod, fn, block[i].Kind, localPtrs, mark)
	}
}

// markStmtRoots marks expression handles referenced by a single
// statement if that statement is a side-effecting root, and the roots of
// the blocks nested in it.
//
// Every statement is a root with all its operands (irutil.StmtExprHandles)
// but a store to a local and a call to a pure function. That covers:
//   - stores to globals (output, UAV, workgroup), image stores and image
//     atomics, returns, workgroup uniform loads, ray queries and subgroup
//     operations;
//   - atomics, even on a local: unlike a local store they are not left to
//     the dead-local fixup (atomics on locals are not expected after
//     mem2reg; classify.go disqualifies such variables);
//   - barriers and kills, which have no operand but are always live;
//   - not control flow: a construct's condition, selector or break-if is
//     left to controlDeps, which marks it when the construct holds live
//     content; only the roots nested in the construct are marked here.
func markStmtRoots(
	mod *ir.Module,
	fn *ir.Function,
	kind ir.StatementKind,
	localPtrs map[ir.ExpressionHandle]uint32,
	mark func(ir.ExpressionHandle),
) {
	switch sk := kind.(type) {
	case ir.StmtEmit:
		// Emit ranges define evaluation timing, not side effects. The
		// expressions within the range become live only if some root
		// references them (StmtExprHandles does not visit the range).
	case ir.StmtStore:
		if _, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); isLocal {
			// Store to local (including field stores via AccessIndex
			// chains) — not a root by itself. It becomes live only if
			// the local's loads are marked live (dead-local fixup).
			return
		}
	case ir.StmtCall:
		if !calleeHasSideEffects(mod, sk.Function) {
			// Pure callees: don't mark anything. If the Result is
			// consumed by a live expression, mark() will reach
			// ExprCallResult via dependency chain. The fixup pass
			// (propagateCallResultLiveness) then marks the call's
			// arguments. The sweep drops calls where neither args
			// nor Result are live.
			return
		}
	case ir.StmtIf, ir.StmtSwitch, ir.StmtLoop:
		// A condition, selector or break-if is live only when the
		// construct it controls holds live content: controlDeps
		// marks it then. Only the roots nested in the construct count here.
		irutil.NestedBlocks(kind, func(b ir.Block) { markBlockRoots(mod, fn, b, localPtrs, mark) })
		return
	}
	irutil.StmtExprHandles(kind, mark)
	irutil.NestedBlocks(kind, func(b ir.Block) { markBlockRoots(mod, fn, b, localPtrs, mark) })
}

// dropDeadControlFlow returns block without the dead constructs, inner
// ones first, and whether it dropped any. An if or a switch is dead when it
// will be empty once swept, or when its condition or selector is not live:
// controlDeps marks it live for every construct
// that holds live content. A loop is dead when it holds no live content
// (controlDeps.loop, read-only here). A construct whose branches feed a
// live phi after it stays (feedsLivePhi).
func dropDeadControlFlow(
	mod *ir.Module,
	fn *ir.Function,
	block ir.Block,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) (ir.Block, bool) {
	cd := controlDeps{mod: mod, fn: fn, live: live, deadLocals: deadLocals, localPtrs: localPtrs}
	out := make(ir.Block, 0, len(block))
	dropped := false
	nested := func(b ir.Block) ir.Block {
		b, d := dropDeadControlFlow(mod, fn, b, deadLocals, live, localPtrs)
		dropped = dropped || d
		return b
	}
	for i := range block {
		st := block[i]
		switch sk := st.Kind.(type) {
		case ir.StmtIf:
			sk.Accept, sk.Reject = nested(sk.Accept), nested(sk.Reject)
			empty := blockWillBeEmpty(fn, sk.Accept, deadLocals, live, localPtrs) &&
				blockWillBeEmpty(fn, sk.Reject, deadLocals, live, localPtrs)
			if (empty || !isLiveInRange(sk.Condition, live)) && !feedsLivePhi(fn, block, i, live) {
				dropped = true
				continue
			}
			st.Kind = sk

		case ir.StmtSwitch:
			cases := make([]ir.SwitchCase, len(sk.Cases))
			allEmpty := true
			for ci, c := range sk.Cases {
				c.Body = nested(c.Body)
				cases[ci] = c
				if !blockWillBeEmpty(fn, c.Body, deadLocals, live, localPtrs) {
					allEmpty = false
				}
			}
			if (allEmpty || !isLiveInRange(sk.Selector, live)) && !feedsLivePhi(fn, block, i, live) {
				dropped = true
				continue
			}
			sk.Cases = cases
			st.Kind = sk

		case ir.StmtLoop:
			if !cd.loop(sk) && !feedsLivePhi(fn, block, i, live) {
				dropped = true
				continue
			}
			sk.Body, sk.Continuing = nested(sk.Body), nested(sk.Continuing)
			st.Kind = sk

		case ir.StmtBlock:
			sk.Block = nested(sk.Block)
			st.Kind = sk
		}
		out = append(out, st)
	}
	return out, dropped
}

// feedsLivePhi reports whether the emits right after block[i] hold a live
// ssa.Phi. mem2reg places a phi there and takes its incomings from the
// branches of block[i], so that if or switch has to stay even when its
// branches end up empty.
func feedsLivePhi(fn *ir.Function, block ir.Block, i int, live []bool) bool {
	for _, st := range block[i+1:] {
		emit, ok := st.Kind.(ir.StmtEmit)
		if !ok {
			return false
		}
		for h := emit.Range.Start; h < emit.Range.End; h++ {
			if _, isPhi := fn.Expressions[h].Kind.(ssa.Phi); isPhi && live[h] {
				return true
			}
		}
	}
	return false
}

// blockWillBeEmpty predicts whether a block will be empty after
// the sweep phase runs: all its statements are dead stores and dead emit
// ranges. dropDeadControlFlow has removed the dead ifs and switches in it
// first, so one that is still there stays.
func blockWillBeEmpty(
	fn *ir.Function,
	block ir.Block,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) bool {
	for i := range block {
		if stmtSurvivesSweep(fn, block[i].Kind, deadLocals, live, localPtrs) {
			return false
		}
	}
	return true
}

// stmtSurvivesSweep predicts whether a single statement will survive
// the sweep phase.
func stmtSurvivesSweep(
	fn *ir.Function,
	kind ir.StatementKind,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) bool {
	switch sk := kind.(type) {
	case ir.StmtStore:
		if varIdx, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); isLocal && deadLocals[varIdx] {
			return false
		}
		return true
	case ir.StmtEmit:
		newRange := shrinkEmitRange(sk.Range, live)
		return newRange.Start < newRange.End
	default:
		// Ifs and switches still here stay (see blockWillBeEmpty); the
		// others are kept conservatively.
		return true
	}
}

// findDeadLocals identifies local variables whose loads are all dead
// (not reachable from any side-effecting root).
//
// A local is dead iff EVERY ExprLoad that reads from its
// ExprLocalVariable handle is in a non-live expression slot.
// When a local is dead, its stores can be removed.
func findDeadLocals(fn *ir.Function, live []bool) map[uint32]bool {
	// Map ExprLocalVariable handle -> variable index.
	localPtrs := buildLocalPtrMap(fn)

	// For each local, track: has any live load? has any load at all?
	type localInfo struct {
		hasLoad     bool
		hasLiveLoad bool
	}
	info := make([]localInfo, len(fn.LocalVars))

	for h, expr := range fn.Expressions {
		load, isLoad := expr.Kind.(ir.ExprLoad)
		if !isLoad {
			continue
		}
		// Check direct LocalVariable pointer, then follow
		// AccessIndex/Access chains for field loads.
		varIdx, isLocalPtr := localPtrs[load.Pointer]
		if !isLocalPtr {
			varIdx, isLocalPtr = resolveLocalVar(fn, load.Pointer)
		}
		if !isLocalPtr {
			continue
		}
		info[varIdx].hasLoad = true
		if live[h] {
			info[varIdx].hasLiveLoad = true
		}
	}

	dead := make(map[uint32]bool)
	for i := range info {
		// A local is dead if it has loads but none of them are live,
		// OR if it has no loads at all (write-only local).
		if !info[i].hasLiveLoad {
			dead[uint32(i)] = true
		}
	}

	// Now propagate: mark stores to dead locals as not needing
	// their values to be live. This can cause cascading deadness
	// for expressions only used by those stores.
	// We handle this by simply not marking those stores' values
	// in the sweep phase — the expressions will remain in the
	// array but their emit ranges will be shrunk.

	return dead
}

// sweepBlock walks a block and returns a new block with dead
// statements removed and emit ranges adjusted.
func sweepBlock(fn *ir.Function, block ir.Block, deadLocals map[uint32]bool, live []bool) ir.Block {
	localPtrs := buildLocalPtrMap(fn)
	return doSweepBlock(fn, block, deadLocals, live, localPtrs)
}

func doSweepBlock(
	fn *ir.Function,
	block ir.Block,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) ir.Block {
	result := make(ir.Block, 0, len(block))

	for i := range block {
		stmt := block[i]
		switch sk := stmt.Kind.(type) {
		case ir.StmtStore:
			// Remove stores to dead locals, including field stores
			// that go through AccessIndex/Access chains.
			if varIdx, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); isLocal && deadLocals[varIdx] {
				continue // dead store — skip
			}
			result = append(result, stmt)

		case ir.StmtEmit:
			// Shrink emit range to exclude dead expressions.
			newRange := shrinkEmitRange(sk.Range, live)
			if newRange.Start >= newRange.End {
				continue // entire range is dead — skip
			}
			result = append(result, ir.Statement{Kind: ir.StmtEmit{Range: newRange}})

		case ir.StmtIf:
			result = append(result, sweepIf(fn, sk, deadLocals, live, localPtrs))

		case ir.StmtLoop:
			if swept, keep := sweepLoop(fn, sk, deadLocals, live, localPtrs); keep {
				result = append(result, swept)
			}

		case ir.StmtSwitch:
			result = append(result, sweepSwitch(fn, sk, deadLocals, live, localPtrs))

		case ir.StmtCall:
			if sweepCallIsLive(sk, live) {
				result = append(result, stmt)
			}

		case ir.StmtBlock:
			result = append(result, ir.Statement{Kind: ir.StmtBlock{
				Block: doSweepBlock(fn, sk.Block, deadLocals, live, localPtrs),
			}})

		default:
			// All other statements pass through unchanged.
			result = append(result, stmt)
		}
	}

	return result
}

// sweepIf sweeps an if-statement's branches. The if itself stays:
// dropDeadControlFlow already removed the ifs with nothing live in them.
func sweepIf(
	fn *ir.Function,
	sk ir.StmtIf,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) ir.Statement {
	return ir.Statement{Kind: ir.StmtIf{
		Condition: sk.Condition,
		Accept:    doSweepBlock(fn, sk.Accept, deadLocals, live, localPtrs),
		Reject:    doSweepBlock(fn, sk.Reject, deadLocals, live, localPtrs),
	}}
}

// sweepLoop recursively sweeps a loop's body and continuing block and
// reports whether the loop should be kept. It eliminates loops whose body
// has no observable side effects after sweeping — this handles the
// synthetic wrapper loops from the early-return inline pattern.
func sweepLoop(
	fn *ir.Function,
	sk ir.StmtLoop,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) (ir.Statement, bool) {
	body := doSweepBlock(fn, sk.Body, deadLocals, live, localPtrs)
	cont := doSweepBlock(fn, sk.Continuing, deadLocals, live, localPtrs)
	if loopBodyIsDead(body, cont, sk.BreakIf, live) {
		return ir.Statement{}, false
	}
	return ir.Statement{Kind: ir.StmtLoop{
		Body:       body,
		Continuing: cont,
		BreakIf:    sk.BreakIf,
	}}, true
}

// sweepCallIsLive reports whether a call statement should be kept.
// A call is live if any argument or its Result expression is live.
// For impure callees markStmtRoots marked everything; for pure callees
// Result is live only if consumed by a downstream live expression.
func sweepCallIsLive(sk ir.StmtCall, live []bool) bool {
	for _, a := range sk.Arguments {
		if int(a) < len(live) && live[a] {
			return true
		}
	}
	if sk.Result != nil && int(*sk.Result) < len(live) && live[*sk.Result] {
		return true
	}
	return false
}

// loopBodyIsDead reports whether a loop (after sweeping) has no observable
// side effects and can be eliminated entirely. A loop is dead when:
//   - Its body contains only StmtBreak/StmtContinue (loop terminators) and
//     no side-effecting statements.
//   - Its continuing block is empty.
//   - Its BreakIf condition (if any) is not live.
//
// This handles the synthetic wrapper loops from the early-return inline
// pattern: after DCE eliminates dead ret-slot stores and dead
// switches/ifs, only the break statement remains.
func loopBodyIsDead(body, cont ir.Block, breakIf *ir.ExpressionHandle, live []bool) bool {
	if len(cont) > 0 {
		return false
	}
	if breakIf != nil && int(*breakIf) < len(live) && live[*breakIf] {
		return false
	}
	return blockOnlyHasTerminators(body)
}

// blockOnlyHasTerminators reports whether a block contains only loop/switch
// terminators (break/continue) with no other observable statements. Empty
// blocks also qualify. This recurses into nested if/switch/loop/block to
// check that ALL paths lead only to terminators.
func blockOnlyHasTerminators(block ir.Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtBreak, ir.StmtContinue:
			// Terminators — allowed.
		case ir.StmtIf:
			if !blockOnlyHasTerminators(sk.Accept) || !blockOnlyHasTerminators(sk.Reject) {
				return false
			}
		case ir.StmtSwitch:
			for j := range sk.Cases {
				if !blockOnlyHasTerminators(sk.Cases[j].Body) {
					return false
				}
			}
		case ir.StmtLoop:
			if !blockOnlyHasTerminators(sk.Body) || !blockOnlyHasTerminators(sk.Continuing) {
				return false
			}
		case ir.StmtBlock:
			if !blockOnlyHasTerminators(sk.Block) {
				return false
			}
		default:
			// Any other statement (store, emit, call, atomic, etc.) means
			// the loop has side effects and must be kept.
			return false
		}
	}
	return true
}

// sweepSwitch sweeps a StmtSwitch's cases. The switch itself stays:
// dropDeadControlFlow already removed the switches with nothing live in
// them.
func sweepSwitch(
	fn *ir.Function,
	sk ir.StmtSwitch,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) ir.Statement {
	newCases := make([]ir.SwitchCase, len(sk.Cases))
	for ci := range sk.Cases {
		newCases[ci] = ir.SwitchCase{
			Value:       sk.Cases[ci].Value,
			Body:        doSweepBlock(fn, sk.Cases[ci].Body, deadLocals, live, localPtrs),
			FallThrough: sk.Cases[ci].FallThrough,
		}
	}
	return ir.Statement{Kind: ir.StmtSwitch{
		Selector: sk.Selector,
		Cases:    newCases,
	}}
}

// shrinkEmitRange returns a new Range that excludes leading and
// trailing dead expressions. If the entire range is dead, the
// returned range has Start >= End.
func shrinkEmitRange(r ir.Range, live []bool) ir.Range {
	start := r.Start
	end := r.End

	// Trim leading dead expressions.
	for start < end && !isLiveInRange(start, live) {
		start++
	}

	// Trim trailing dead expressions.
	for end > start && !isLiveInRange(end-1, live) {
		end--
	}

	// Check if there are dead expressions in the middle.
	// If so, we can't remove them from a single contiguous range.
	// Keep the range as-is — the dead expressions in the middle
	// will still be emitted but they won't affect correctness
	// since nothing reads them.
	return ir.Range{Start: start, End: end}
}

// isLiveInRange checks if the expression at handle h is live.
func isLiveInRange(h ir.ExpressionHandle, live []bool) bool {
	if int(h) >= len(live) {
		return true // out of range — conservative: keep
	}
	return live[h]
}

// calleeHasSideEffects reports whether calling the function at handle
// fh can produce observable side effects. If the callee is pure (only
// local stores, no globals, no atomics, no barriers, no image stores),
// a StmtCall to it with an unused result is dead and can be eliminated.
//
// Conservative: returns true for any callee we can't inspect (out of
// range, entry points, or callee that itself calls other functions).
func calleeHasSideEffects(mod *ir.Module, fh ir.FunctionHandle) bool {
	if mod == nil || int(fh) >= len(mod.Functions) {
		return true
	}
	fn := &mod.Functions[fh]
	return blockHasSideEffects(fn, fn.Body)
}

func blockHasSideEffects(fn *ir.Function, block ir.Block) bool {
	localPtrs := buildLocalPtrMap(fn)
	return blockHasSideEffectsWalk(block, localPtrs, fn)
}

func blockHasSideEffectsWalk(block ir.Block, localPtrs map[ir.ExpressionHandle]uint32, fn *ir.Function) bool {
	for i := range block {
		if stmtHasSideEffects(block[i], localPtrs, fn) {
			return true
		}
	}
	return false
}

// stmtHasSideEffects reports whether a single statement has observable
// side effects (non-local stores, atomics, barriers, calls, image ops).
// For control flow statements it recurses into nested blocks.
//
// Every statement kind has a case: TestStmtHasSideEffectsEveryKind fails for
// a kind added to package ir without one, and the default keeps such a kind
// (a side effect) until it has.
func stmtHasSideEffects(stmt ir.Statement, localPtrs map[ir.ExpressionHandle]uint32, fn *ir.Function) bool {
	switch sk := stmt.Kind.(type) {
	case ir.StmtStore:
		_, isLocal := isLocalStore(fn, sk.Pointer, localPtrs)
		return !isLocal
	case ir.StmtAtomic, ir.StmtImageStore, ir.StmtImageAtomic,
		ir.StmtControlBarrier, ir.StmtMemoryBarrier, ir.StmtKill,
		ir.StmtCall, ir.StmtWorkGroupUniformLoad, ir.StmtRayQuery:
		return true
	case ir.StmtEmit, ir.StmtReturn, ir.StmtBreak, ir.StmtContinue,
		ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation, ir.StmtSubgroupGather:
		// Evaluation and control flow inside the callee, and subgroup
		// operations, which only compute their result: nothing a caller
		// observes unless it uses the call's result.
		return false
	case ir.StmtIf:
		return blockHasSideEffectsWalk(sk.Accept, localPtrs, fn) || blockHasSideEffectsWalk(sk.Reject, localPtrs, fn)
	case ir.StmtLoop:
		return blockHasSideEffectsWalk(sk.Body, localPtrs, fn) || blockHasSideEffectsWalk(sk.Continuing, localPtrs, fn)
	case ir.StmtSwitch:
		for ci := range sk.Cases {
			if blockHasSideEffectsWalk(sk.Cases[ci].Body, localPtrs, fn) {
				return true
			}
		}
		return false
	case ir.StmtBlock:
		return blockHasSideEffectsWalk(sk.Block, localPtrs, fn)
	default:
		return true
	}
}

// buildLocalPtrMap returns a map from ExprLocalVariable expression
// handle to the local variable index it references.
func buildLocalPtrMap(fn *ir.Function) map[ir.ExpressionHandle]uint32 {
	m := make(map[ir.ExpressionHandle]uint32, len(fn.LocalVars))
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			m[ir.ExpressionHandle(h)] = uint32(lv.Variable)
		}
	}
	return m
}

// resolveLocalVar follows AccessIndex and Access chains from a pointer
// expression to find the base LocalVariable, if any. Stores and loads
// to struct fields go through AccessIndex(LocalVariable(i), fieldIdx)
// or deeper chains like AccessIndex(AccessIndex(LV, f1), f2). Without
// this traversal, such pointer paths are misclassified as global-store
// roots, preventing DCE from recognizing the local as dead.
//
// Returns the variable index and true if the chain ultimately reaches
// an ExprLocalVariable; returns (0, false) otherwise.
func resolveLocalVar(fn *ir.Function, h ir.ExpressionHandle) (uint32, bool) {
	const maxDepth = 16 // guard against pathological chains
	for depth := 0; depth < maxDepth; depth++ {
		if int(h) >= len(fn.Expressions) {
			return 0, false
		}
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprLocalVariable:
			return uint32(k.Variable), true
		case ir.ExprAccessIndex:
			h = k.Base
		case ir.ExprAccess:
			h = k.Base
		default:
			return 0, false
		}
	}
	return 0, false
}

// isLocalStore checks whether a store pointer targets a local variable,
// either directly (ExprLocalVariable) or via AccessIndex/Access chains
// (e.g., field stores into a struct local). Returns the variable index
// and true if the pointer resolves to a local.
func isLocalStore(fn *ir.Function, pointer ir.ExpressionHandle, localPtrs map[ir.ExpressionHandle]uint32) (uint32, bool) {
	// Fast path: direct local variable pointer.
	if varIdx, ok := localPtrs[pointer]; ok {
		return varIdx, true
	}
	// Slow path: follow AccessIndex/Access chains.
	return resolveLocalVar(fn, pointer)
}
