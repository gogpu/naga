// Package ssa holds the two expression kinds the DXIL backend adds to a
// function when it promotes local variables to SSA values: Alias and Phi.
//
// They are not part of the IR data model. Rust naga's IR has no such kinds,
// no frontend produces them, and no public path sees them: the DXIL backend
// creates them in its mem2reg pass (dxil/internal/passes/mem2reg) on its own
// copy of the module, which dxil.Compile clones from the caller's, and only
// the DXIL passes and emitter read them. The internal helpers that the DXIL
// backend shares with the rest of naga (the typifier and the IR walkers of
// internal/irutil) handle them too, so the DXIL passes can type and walk a
// promoted function with the same code as any other.
//
// The validator (valid.Validate), compaction (naga.Compact) and the other
// backends never see an Alias or a Phi; the snapshot test
// TestSSAKindsStayInDXIL compiles the whole corpus and checks it. Should one
// reach them anyway, they report an unknown expression kind instead of
// accepting it.
package ssa

import "github.com/gogpu/naga/ir"

// Alias stands for another expression of the same function: its value is
// the value of Source.
//
// mem2reg rewrites a load of a promoted local variable into an Alias of the
// value last stored into the variable (or, before the first store, of the
// variable's initializer or a zero value), so the emit pipeline keeps
// walking the function's expression arena unchanged. The DXIL emitter
// resolves an Alias to the value ID of its source and emits no instruction
// for it. This is the value substitution of LLVM's PromoteMemoryToRegister:
// once an alloca is promoted, every load uses the dominating store's value
// and the load is erased.
//
// ir.ExpressionKind is sealed by an unexported method, so a kind declared
// outside package ir can only satisfy it by embedding a kind declared there.
// Alias embeds ir.ExprRayQueryProceedResult, which has no fields, for that
// alone: the embedded value carries no meaning, and a type switch on
// ir.ExprRayQueryProceedResult never matches an Alias, whose dynamic type is
// Alias.
type Alias struct {
	ir.ExprRayQueryProceedResult

	// Source is the expression this one stands for.
	Source ir.ExpressionHandle
}

// PredKey identifies the structured-CFG edge a Phi incoming value flows
// along. LLVM's PromoteMemoryToRegister rename pass tracks the incoming
// values per predecessor basic block; in naga's structured IR a predecessor
// is one of a few named edges of an if, a switch or a loop.
type PredKey uint8

const (
	// PredIfAccept is the end of StmtIf.Accept.
	PredIfAccept PredKey = iota
	// PredIfReject is the end of StmtIf.Reject, or the value before the if
	// when Reject is empty or does not store to the variable.
	PredIfReject
	// PredLoopInit is the edge into the loop header from before the loop.
	PredLoopInit
	// PredLoopBackEdge is the back edge into the loop header: the end of
	// StmtLoop.Continuing, or of StmtLoop.Body when Continuing is empty.
	PredLoopBackEdge
	// PredSwitchCase is the end of a switch case; Incoming.CaseIdx says
	// which one.
	PredSwitchCase
	// PredFallThrough is the value from before a construct (for example into
	// a switch merge when no case stores to the variable).
	PredFallThrough
)

// Incoming is one (predecessor, value) pair of a Phi. CaseIdx is meaningful
// only when PredKey is PredSwitchCase.
type Incoming struct {
	PredKey PredKey
	CaseIdx uint32
	Value   ir.ExpressionHandle
}

// Phi merges the values a promoted local variable has on the structured-CFG
// edges into a merge point.
//
// mem2reg places a Phi at the merge after an if or a switch and at the header
// of a loop when the variable is stored on more than one incoming path: the
// iterated dominance frontier of LLVM's PromoteMemoryToRegister, which a
// structured CFG makes trivial. The DXIL emitter lowers it to LLVM's
// FUNC_CODE_INST_PHI at the start of the merge block, with the value ID of
// each incoming taken from the snapshot made at the end of its predecessor.
//
// Phi embeds ir.ExprRayQueryProceedResult only to satisfy the sealed
// ir.ExpressionKind, as Alias does; the embedded value carries no meaning,
// and a type switch on ir.ExprRayQueryProceedResult never matches a Phi.
type Phi struct {
	ir.ExprRayQueryProceedResult

	// Incoming holds one value per predecessor edge.
	Incoming []Incoming
}
