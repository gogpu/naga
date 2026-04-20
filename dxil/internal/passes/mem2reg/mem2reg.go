// Package mem2reg promotes function-scope local variables of scalar type
// to SSA values, eliminating the corresponding alloca / load / store
// triples that the DXIL emitter would otherwise produce.
//
// This pass mirrors the LLVM "Promote Memory To Register" transform
// (lib/Transforms/Utils/PromoteMemoryToRegister.cpp), specialized to
// naga IR's structured control flow. Reference behavior:
//
//   - For each LocalVariable whose every use is a direct Load/Store
//     (no GEP, no AccessIndex chain, no pointer escape), the alloca
//     is considered "promotable".
//   - Phase A (this implementation) only promotes variables whose
//     entire set of stores AND loads is confined to a single Block
//     of the structured CFG. The rewrite walks that block in
//     textual order tracking a per-variable current value:
//   - StmtStore updates the current value and is removed
//     from the statement stream.
//   - Each load expression is rewritten in place to an
//     ExprAlias pointing at the current value (or, when the
//     load precedes every store, at the variable's Init
//     expression or a synthesized ExprZeroValue).
//
// Phase A scope:
//   - Scalar-typed local variables only. Aggregate types (vector,
//     matrix, struct, array, atomic) keep their alloca-form
//     lowering because the existing emit pipeline has many
//     aggregate-specific paths that depend on the alloca pointer.
//   - Single-block colocation only. Multi-block cases that would
//     require phi insertion at structured-CFG merge points are left
//     as alloca; they are tracked separately for Phase B.
//
// Phase B (deferred — tracked as BUG-DXIL-040) will add ExprPhi
// expressions placed at structured-CFG merge points (if-merge,
// loop-header, switch-merge), the corresponding emit-side support
// for LLVM phi instructions in basic-block prologue position, and
// the bitcode writer extension for sign-rotated VBR encoding of
// phi value operands.
//
// References:
//   - LLVM Mem2Reg: https://llvm.org/docs/Passes.html#mem2reg
//   - DXC reference: clang -O2 lowers HLSL locals through this same
//     LLVM pass before DXIL bitcode emission, which is why DXC
//     golden output has zero allocas for the same shaders we still
//     emit them.
package mem2reg

import (
	"github.com/gogpu/naga/ir"
)

// Run promotes promotable scalar local variables in fn to SSA form.
//
// On success the function's Body and Expressions arrays are mutated
// in place:
//   - Promoted-variable loads (ExprLoad{ExprLocalVariable{i}}) are
//     rewritten to ExprAlias pointing at the value last stored
//     into the variable (or, on first read, at the variable's
//     Init expression or a synthesized ExprZeroValue).
//   - Stores into promoted variables are removed from the
//     statement stream.
//   - The LocalVariable slot itself stays in fn.LocalVars (so
//     existing index-based references remain valid) but no
//     expression references it any more — the emitter's
//     reachability check skips allocas with zero remaining uses.
//
// The pass is idempotent: a second invocation is a no-op because
// every alloca that survives the first pass has at least one
// address-taken use (which means it failed the promotability
// predicate to begin with).
//
// On any internal failure the pass leaves fn untouched. Callers
// may safely ignore the returned error and proceed with the
// original IR; mem2reg is a pure optimization, not a correctness
// requirement.
func Run(mod *ir.Module, fn *ir.Function) error {
	if mod == nil || fn == nil {
		return nil
	}
	if len(fn.LocalVars) == 0 {
		return nil
	}

	uses := classifyLocals(mod, fn)
	if !anyPromotable(uses) {
		return nil
	}

	ctx := &promotionContext{
		mod:       mod,
		fn:        fn,
		uses:      uses,
		localPtrs: buildLocalPtrMap(fn),
	}
	// Phase A: single-block promotion (no phi). Handles the easy
	// cases without touching control flow and produces ExprAlias
	// rewrites. After this pass, vars whose stores/loads spanned
	// multiple blocks remain unpromoted with their alloca path.
	promoteBlocks(ctx, &fn.Body)

	// Phase B: structured-CFG SSA construction for remaining
	// multi-block scalar candidates. Inserts ExprPhi at if/switch
	// merge points and at loop headers, plus the rename pass that
	// rewrites loads to ExprAlias of the appropriate phi or
	// dominating store value.
	//
	// Re-classify to refresh the load handle set (Phase A may have
	// already consumed some loads via ExprAlias rewrite, so the
	// remaining loadHandles list is what Phase B should target).
	// Phase B: structured-CFG SSA construction with if-merge AND
	// switch-merge phi insertion. Loop-header phis are disqualified
	// (variables stored inside loop bodies keep their alloca lowering)
	// because the back-edge value is a forward reference at emit time
	// and we lack the deferred-emit infrastructure to fix-up phi
	// operands after walking the body. Bool / i1 variables are also
	// disqualified because DXC's bitcode parser rejects i1-typed phi
	// instructions with "Invalid record" — same limitation noted at
	// emitter.go:639 for i1 return types.
	//
	// Loop-header phi and i1 widening tracked under BUG-DXIL-041.
	uses2 := classifyLocals(mod, fn)
	if !anyPromotable(uses2) {
		return nil
	}
	ctx2 := &promotionContext{
		mod:       mod,
		fn:        fn,
		uses:      uses2,
		localPtrs: buildLocalPtrMap(fn),
	}
	promoteStructured(ctx2)
	return nil
}

// anyPromotable reports whether at least one variable passed
// classification. Avoids the rewrite walk's overhead when nothing
// can be promoted.
func anyPromotable(uses []localUseInfo) bool {
	for i := range uses {
		if uses[i].promotable && len(uses[i].loadHandles) > 0 {
			return true
		}
	}
	return false
}
