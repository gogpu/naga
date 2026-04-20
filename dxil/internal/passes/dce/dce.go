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
// Dead control-flow elimination (ADCE-style): after sweeping, a
// StmtIf or StmtSwitch whose sub-blocks are ALL empty and whose
// condition/selector is not marked live is removed entirely. This
// matches DXC's LLVM ADCE behavior. StmtLoop is kept even with an
// empty body (infinite loop = observable side effect).
//
// References:
//   - DXC pipeline: DxilLinker.cpp:1284 (mem2reg -> SimplifyInst -> CFGSimplify -> DCE -> GlobalDCE)
//   - Mesa NIR: nir_opt_dce (mark-and-sweep, iterative until stable)
//   - LLVM: lib/Transforms/Scalar/DCE.cpp (worklist-based trivially-dead removal)
package dce

import (
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

	// Phase 1: mark expressions reachable from side-effecting roots.
	live := markLive(mod, fn)

	// Phase 2: identify dead local variables.
	deadLocals := findDeadLocals(fn, live)

	// Phase 2b: for live locals, mark the values stored INTO them as
	// live. The initial mark pass skips local stores because they're
	// not roots. But if the local itself is live (has a live load),
	// the stored value must also be live so its emit range is preserved.
	markLiveLocalStoreValues(fn, live, deadLocals)

	// Phase 2b-fixup: after local-store propagation, some ExprCallResult
	// handles may have become live through local store chains. Retroactively
	// mark the arguments of those calls so the sweep keeps them.
	var mark func(h ir.ExpressionHandle)
	mark = func(h ir.ExpressionHandle) {
		if int(h) >= len(live) || live[h] {
			return
		}
		live[h] = true
		visitExprHandles(fn.Expressions[h].Kind, mark)
	}
	propagateCallResultLiveness(fn.Body, live, mark)

	// Phase 2c: unmark control-flow conditions whose sub-blocks
	// contain no live content. After phases 1-2b, the live array
	// reflects full liveness. A StmtIf whose branches have only
	// dead stores and dead emits is itself dead; unmarking its
	// condition lets the sweep phase drop it entirely and
	// transitively kills the condition's sub-expressions
	// (e.g., threadId loads feeding an if that does nothing).
	localPtrs := buildLocalPtrMap(fn)
	unmarkDeadControlFlow(fn, fn.Body, deadLocals, live, localPtrs)

	// Phase 3: sweep dead statements and shrink emit ranges.
	// Even when no dead locals exist, the sweep still shrinks emit ranges
	// for dead expressions. Without this, shaders that compute into
	// only let-bindings (zero locals, zero outputs) retain all dead
	// instructions in the DXIL body, diverging from DXC which runs
	// LLVM DCE and strips them.
	fn.Body = sweepBlock(fn, fn.Body, deadLocals, live)
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
//   - StmtBarrier (execution side effect)
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
		visitExprHandles(fn.Expressions[h].Kind, mark)
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
		visitExprHandles(fn.Expressions[h].Kind, mark)
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
// statement if that statement is a side-effecting root.
//
//nolint:gocyclo,cyclop,funlen // exhaustive statement-kind dispatch
func markStmtRoots(
	mod *ir.Module,
	fn *ir.Function,
	kind ir.StatementKind,
	localPtrs map[ir.ExpressionHandle]uint32,
	mark func(ir.ExpressionHandle),
) {
	switch sk := kind.(type) {
	case ir.StmtStore:
		if _, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); isLocal {
			// Store to local (including field stores via AccessIndex
			// chains) — not a root by itself. It becomes live only if
			// the local's loads are marked live (dead-local fixup).
			return
		}
		// Store to global (output, UAV, workgroup): root.
		mark(sk.Pointer)
		mark(sk.Value)

	case ir.StmtReturn:
		if sk.Value != nil {
			mark(*sk.Value)
		}

	case ir.StmtImageStore:
		mark(sk.Image)
		mark(sk.Coordinate)
		mark(sk.Value)
		if sk.ArrayIndex != nil {
			mark(*sk.ArrayIndex)
		}

	case ir.StmtImageAtomic:
		mark(sk.Image)
		mark(sk.Coordinate)
		mark(sk.Value)
		if sk.ArrayIndex != nil {
			mark(*sk.ArrayIndex)
		}

	case ir.StmtAtomic:
		if _, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); isLocal {
			// Atomic on local — unusual, but should be safe to
			// treat like a local store. However, be conservative:
			// atomics on locals are not expected after mem2reg
			// (classify.go disqualifies such vars). Mark as live.
			mark(sk.Pointer)
			mark(sk.Value)
			if sk.Result != nil {
				mark(*sk.Result)
			}
			return
		}
		mark(sk.Pointer)
		mark(sk.Value)
		if sk.Result != nil {
			mark(*sk.Result)
		}

	case ir.StmtBarrier:
		// Execution barrier — always live (side effect).
		// No expression operands to mark.

	case ir.StmtKill:
		// Discard/kill — always live.

	case ir.StmtCall:
		if calleeHasSideEffects(mod, sk.Function) {
			for _, a := range sk.Arguments {
				mark(a)
			}
			if sk.Result != nil {
				mark(*sk.Result)
			}
		}
		// Pure callees: don't mark anything. If the Result is
		// consumed by a live expression, mark() will reach
		// ExprCallResult via dependency chain. The fixup pass
		// (propagateCallResultLiveness) then marks the call's
		// arguments. The sweep drops calls where neither args
		// nor Result are live.

	case ir.StmtWorkGroupUniformLoad:
		mark(sk.Pointer)
		mark(sk.Result)

	case ir.StmtRayQuery:
		mark(sk.Query)
		markRayQueryFunctionHandles(sk.Fun, mark)

	case ir.StmtSubgroupBallot:
		mark(sk.Result)
		if sk.Predicate != nil {
			mark(*sk.Predicate)
		}

	case ir.StmtSubgroupCollectiveOperation:
		mark(sk.Argument)
		mark(sk.Result)

	case ir.StmtSubgroupGather:
		mark(sk.Argument)
		mark(sk.Result)

	// Control flow — recurse into sub-blocks and mark conditions.
	// Conditions are marked unconditionally here to ensure local
	// stores inside branches are correctly tracked. Dead control
	// flow (empty branches) is eliminated in the sweep phase after
	// full liveness is computed.
	case ir.StmtIf:
		mark(sk.Condition)
		markBlockRoots(mod, fn, sk.Accept, localPtrs, mark)
		markBlockRoots(mod, fn, sk.Reject, localPtrs, mark)

	case ir.StmtLoop:
		markBlockRoots(mod, fn, sk.Body, localPtrs, mark)
		markBlockRoots(mod, fn, sk.Continuing, localPtrs, mark)
		if sk.BreakIf != nil {
			mark(*sk.BreakIf)
		}

	case ir.StmtSwitch:
		mark(sk.Selector)
		for ci := range sk.Cases {
			markBlockRoots(mod, fn, sk.Cases[ci].Body, localPtrs, mark)
		}

	case ir.StmtBlock:
		markBlockRoots(mod, fn, sk.Block, localPtrs, mark)

	case ir.StmtEmit:
		// Emit ranges define evaluation timing, not side effects.
		// The expressions within the range become live only if
		// some root references them.

	case ir.StmtBreak, ir.StmtContinue:
		// Control flow — no expression operands.
	}
}

// unmarkDeadControlFlow walks the body and for each StmtIf whose
// accept and reject blocks will be empty after sweeping, unmarks the
// condition expression and all its transitive sub-expressions. This
// lets the sweep phase drop the entire if-statement and its condition
// computation (e.g., threadId loads for an empty-body if).
//
// The check uses blockWillBeEmpty, which simulates what the sweep
// phase would produce for a block using the already-computed live
// array and dead locals.
func unmarkDeadControlFlow(
	fn *ir.Function,
	block ir.Block,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtIf:
			// Recurse first so nested dead control flow is handled.
			unmarkDeadControlFlow(fn, sk.Accept, deadLocals, live, localPtrs)
			unmarkDeadControlFlow(fn, sk.Reject, deadLocals, live, localPtrs)

			if blockWillBeEmpty(fn, sk.Accept, deadLocals, live, localPtrs) &&
				blockWillBeEmpty(fn, sk.Reject, deadLocals, live, localPtrs) {
				// Both branches dead — unmark the condition.
				unmarkExpr(fn, sk.Condition, live)
			}

		case ir.StmtSwitch:
			allEmpty := true
			for ci := range sk.Cases {
				unmarkDeadControlFlow(fn, sk.Cases[ci].Body, deadLocals, live, localPtrs)
				if !blockWillBeEmpty(fn, sk.Cases[ci].Body, deadLocals, live, localPtrs) {
					allEmpty = false
				}
			}
			if allEmpty {
				unmarkExpr(fn, sk.Selector, live)
			}

		case ir.StmtLoop:
			unmarkDeadControlFlow(fn, sk.Body, deadLocals, live, localPtrs)
			unmarkDeadControlFlow(fn, sk.Continuing, deadLocals, live, localPtrs)

		case ir.StmtBlock:
			unmarkDeadControlFlow(fn, sk.Block, deadLocals, live, localPtrs)
		}
	}
}

// unmarkExpr unmarks an expression and all sub-expressions that are
// only live because of the given handle. It only unmarks expressions
// that have no other live consumer.
func unmarkExpr(fn *ir.Function, h ir.ExpressionHandle, live []bool) {
	if int(h) >= len(live) || !live[h] {
		return
	}
	// Only unmark if no other live expression references this one.
	if hasOtherLiveConsumer(fn, h, live) {
		return
	}
	live[h] = false
	visitExprHandles(fn.Expressions[h].Kind, func(sub ir.ExpressionHandle) {
		unmarkExpr(fn, sub, live)
	})
}

// hasOtherLiveConsumer checks if any live expression references h.
// Used to ensure we don't unmark expressions that feed into other
// live computations beyond the dead control flow.
func hasOtherLiveConsumer(fn *ir.Function, target ir.ExpressionHandle, live []bool) bool {
	for h, expr := range fn.Expressions {
		if !live[h] || ir.ExpressionHandle(h) == target {
			continue
		}
		found := false
		visitExprHandles(expr.Kind, func(sub ir.ExpressionHandle) {
			if sub == target {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

// blockWillBeEmpty predicts whether a block will be empty after
// the sweep phase runs. A block is empty if all its statements
// will be removed: dead stores, dead emit ranges, and dead control flow.
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
	case ir.StmtIf:
		acceptEmpty := blockWillBeEmpty(fn, sk.Accept, deadLocals, live, localPtrs)
		rejectEmpty := blockWillBeEmpty(fn, sk.Reject, deadLocals, live, localPtrs)
		return !(acceptEmpty && rejectEmpty && !live[sk.Condition])
	case ir.StmtSwitch:
		allEmpty := true
		for ci := range sk.Cases {
			if !blockWillBeEmpty(fn, sk.Cases[ci].Body, deadLocals, live, localPtrs) {
				allEmpty = false
			}
		}
		return !(allEmpty && !live[sk.Selector])
	default:
		return true // conservatively keep
	}
}

// markRayQueryFunctionHandles marks expression handles within a
// RayQueryFunction variant.
func markRayQueryFunctionHandles(fun ir.RayQueryFunction, mark func(ir.ExpressionHandle)) {
	switch f := fun.(type) {
	case ir.RayQueryInitialize:
		mark(f.AccelerationStructure)
		mark(f.Descriptor)
	case ir.RayQueryProceed:
		mark(f.Result)
	case ir.RayQueryGenerateIntersection:
		mark(f.HitT)
	case ir.RayQueryTerminate, ir.RayQueryConfirmIntersection:
		// No expression operands.
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
			if swept, keep := sweepIf(fn, sk, deadLocals, live, localPtrs); keep {
				result = append(result, swept)
			}

		case ir.StmtLoop:
			if swept, keep := sweepLoop(fn, sk, deadLocals, live, localPtrs); keep {
				result = append(result, swept)
			}

		case ir.StmtSwitch:
			if swept, keep := sweepSwitch(fn, sk, deadLocals, live, localPtrs); keep {
				result = append(result, swept)
			}

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

// sweepIf recursively sweeps an if-statement's branches and reports
// whether the statement should be kept. It eliminates the entire
// if-statement when both branches are empty after sweeping AND the
// condition expression has no side effects (DXC's ADCE does the same).
func sweepIf(
	fn *ir.Function,
	sk ir.StmtIf,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) (ir.Statement, bool) {
	accept := doSweepBlock(fn, sk.Accept, deadLocals, live, localPtrs)
	reject := doSweepBlock(fn, sk.Reject, deadLocals, live, localPtrs)
	if len(accept) == 0 && len(reject) == 0 && !live[sk.Condition] {
		return ir.Statement{}, false
	}
	return ir.Statement{Kind: ir.StmtIf{
		Condition: sk.Condition,
		Accept:    accept,
		Reject:    reject,
	}}, true
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

// sweepSwitch sweeps a StmtSwitch and returns the swept statement.
// Returns (stmt, true) if the switch survives, (_, false) if eliminated.
func sweepSwitch(
	fn *ir.Function,
	sk ir.StmtSwitch,
	deadLocals map[uint32]bool,
	live []bool,
	localPtrs map[ir.ExpressionHandle]uint32,
) (ir.Statement, bool) {
	newCases := make([]ir.SwitchCase, len(sk.Cases))
	allEmpty := true
	for ci := range sk.Cases {
		newCases[ci] = ir.SwitchCase{
			Value:       sk.Cases[ci].Value,
			Body:        doSweepBlock(fn, sk.Cases[ci].Body, deadLocals, live, localPtrs),
			FallThrough: sk.Cases[ci].FallThrough,
		}
		if len(newCases[ci].Body) > 0 {
			allEmpty = false
		}
	}
	// Eliminate switch when all case bodies are empty and
	// the selector is dead (no side effects feed it).
	if allEmpty && !live[sk.Selector] {
		return ir.Statement{}, false
	}
	return ir.Statement{Kind: ir.StmtSwitch{
		Selector: sk.Selector,
		Cases:    newCases,
	}}, true
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
func stmtHasSideEffects(stmt ir.Statement, localPtrs map[ir.ExpressionHandle]uint32, fn *ir.Function) bool {
	switch sk := stmt.Kind.(type) {
	case ir.StmtStore:
		if _, isLocal := isLocalStore(fn, sk.Pointer, localPtrs); !isLocal {
			return true
		}
	case ir.StmtAtomic:
		return true
	case ir.StmtImageStore, ir.StmtImageAtomic:
		return true
	case ir.StmtBarrier, ir.StmtKill:
		return true
	case ir.StmtCall:
		return true
	case ir.StmtWorkGroupUniformLoad:
		return true
	case ir.StmtRayQuery:
		return true
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
	case ir.StmtBlock:
		return blockHasSideEffectsWalk(sk.Block, localPtrs, fn)
	}
	return false
}

// buildLocalPtrMap returns a map from ExprLocalVariable expression
// handle to the local variable index it references.
func buildLocalPtrMap(fn *ir.Function) map[ir.ExpressionHandle]uint32 {
	m := make(map[ir.ExpressionHandle]uint32, len(fn.LocalVars))
	for h, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			m[ir.ExpressionHandle(h)] = lv.Variable
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
			return k.Variable, true
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

// visitExprHandles invokes f for every ExpressionHandle field inside
// the given expression kind. This mirrors the visitExpressionHandles
// in emit/statements.go but lives here to avoid a circular import.
//
//nolint:gocyclo,cyclop,funlen // exhaustive expression-kind dispatch
func visitExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle)) {
	switch k := kind.(type) {
	case ir.ExprLoad:
		f(k.Pointer)
	case ir.ExprAlias:
		f(k.Source)
	case ir.ExprAccess:
		f(k.Base)
		f(k.Index)
	case ir.ExprAccessIndex:
		f(k.Base)
	case ir.ExprBinary:
		f(k.Left)
		f(k.Right)
	case ir.ExprUnary:
		f(k.Expr)
	case ir.ExprSelect:
		f(k.Condition)
		f(k.Accept)
		f(k.Reject)
	case ir.ExprSplat:
		f(k.Value)
	case ir.ExprSwizzle:
		f(k.Vector)
	case ir.ExprCompose:
		for _, c := range k.Components {
			f(c)
		}
	case ir.ExprAs:
		f(k.Expr)
	case ir.ExprDerivative:
		f(k.Expr)
	case ir.ExprMath:
		f(k.Arg)
		if k.Arg1 != nil {
			f(*k.Arg1)
		}
		if k.Arg2 != nil {
			f(*k.Arg2)
		}
		if k.Arg3 != nil {
			f(*k.Arg3)
		}
	case ir.ExprRelational:
		f(k.Argument)
	case ir.ExprArrayLength:
		f(k.Array)
	case ir.ExprImageSample:
		f(k.Image)
		f(k.Sampler)
		f(k.Coordinate)
		if k.ArrayIndex != nil {
			f(*k.ArrayIndex)
		}
		if k.Offset != nil {
			f(*k.Offset)
		}
		if k.DepthRef != nil {
			f(*k.DepthRef)
		}
	case ir.ExprImageLoad:
		f(k.Image)
		f(k.Coordinate)
		if k.ArrayIndex != nil {
			f(*k.ArrayIndex)
		}
		if k.Sample != nil {
			f(*k.Sample)
		}
		if k.Level != nil {
			f(*k.Level)
		}
	case ir.ExprImageQuery:
		f(k.Image)
	case ir.ExprPhi:
		for _, inc := range k.Incoming {
			f(inc.Value)
		}
	}
}
