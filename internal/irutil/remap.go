package irutil

import (
	"fmt"
	"slices"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// This file holds the one walker over the operand expression handles of an
// expression kind (walkExpr) and the one over those of a statement kind
// (walkStmt). Each is written once, generic over what it does with a handle
// (handleOp): map the handles into a rebuilt kind, as compaction, inlining
// and pipeline-constant processing do, or only read them, as the tracers of
// compaction and the DXIL backend's liveness and use scans do (ExprHandles,
// StmtExprHandles, BlockExprHandles). blockMapper carries the statement
// walker through a statement tree.
//
// Rust naga matches every kind exhaustively (compact/expressions.rs,
// compact/statements.rs) and the compiler rejects a missing one. Go has no
// exhaustive switch, so both walkers panic on a kind they have no case for,
// and TestWalkersCoverEveryKind builds every kind declared in package ir, and
// the DXIL backend's SSA kinds of package ssa, to prove there is none: a kind
// added without a case here fails that test instead of keeping stale handles
// at run time.

// RemapExpressionHandles returns kind with every operand handle h replaced by
// remap[h]; a handle outside remap is kept. Handles that name other arenas
// (constants, overrides, globals, locals, types) are not changed. kind itself
// is not modified.
func RemapExpressionHandles(kind ir.ExpressionKind, remap []ir.ExpressionHandle) ir.ExpressionKind {
	return mapExprHandles(kind, remapThrough(remap))
}

// RemapStatementExpressionHandles replaces, in place, every expression
// handle h in block and its nested blocks by remap[h]; a handle outside remap
// is kept. An Emit range is adjusted by its first and last expression, as
// Rust naga's adjust_stmt does.
func RemapStatementExpressionHandles(block []ir.Statement, remap []ir.ExpressionHandle) {
	rm := remapThrough(remap)
	m := blockMapper{handle: rm, emit: adjustEmitRange(rm)}
	m.block(block)
}

// ExprHandles calls f with every expression handle kind holds as an operand,
// in field order. Handles that name other arenas (constants, overrides,
// globals, locals, types) are not expression handles and are not visited.
// kind is not modified and nothing is allocated. It panics on a kind it has
// no case for.
func ExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle)) {
	walkExpr(kind, readHandles(f))
}

// StmtExprHandles calls f with every expression handle kind holds as an
// operand, in field order: not the range of an Emit, which says when
// expressions are evaluated and is no use of them, and not the handles of
// the statements in the blocks nested in kind (NestedBlocks reaches those).
// kind is not modified and nothing is allocated. It panics on a kind it has
// no case for.
func StmtExprHandles(kind ir.StatementKind, f func(ir.ExpressionHandle)) {
	walkStmt(kind, readHandles(f))
}

// NestedBlocks calls f with each block nested directly in kind: the block of
// a Block, the accept and reject blocks of an If, the body of every case of
// a Switch, and the body and continuing block of a Loop. Other kinds hold no
// block.
func NestedBlocks(kind ir.StatementKind, f func(ir.Block)) {
	switch s := kind.(type) {
	case ir.StmtBlock:
		f(s.Block)
	case ir.StmtIf:
		f(s.Accept)
		f(s.Reject)
	case ir.StmtSwitch:
		for i := range s.Cases {
			f(s.Cases[i].Body)
		}
	case ir.StmtLoop:
		f(s.Body)
		f(s.Continuing)
	default:
	}
}

// BlockExprHandles calls f with every operand handle (StmtExprHandles) of
// every statement in block and in the blocks nested in it. block is not
// modified.
func BlockExprHandles(block ir.Block, f func(ir.ExpressionHandle)) {
	var walk func(b ir.Block)
	walk = func(b ir.Block) {
		for i := range b {
			StmtExprHandles(b[i].Kind, f)
			NestedBlocks(b[i].Kind, walk)
		}
	}
	walk(block)
}

// RemapStatementExpressionHandlesEmits is RemapStatementExpressionHandles
// with the Emit ranges mapped by emit instead of by their first and last
// expression: a pass that turns one expression into several, or into an
// earlier one, knows which new expressions an old range stands for.
func RemapStatementExpressionHandlesEmits(block []ir.Statement, remap []ir.ExpressionHandle, emit func(ir.Range) ir.Range) {
	m := blockMapper{handle: remapThrough(remap), emit: func(r ir.Range) (ir.Range, bool) { return emit(r), true }}
	m.block(block)
}

// remapThrough returns the handle mapping of a remap table: remap[h] for a
// handle inside it, h itself for one outside.
func remapThrough(remap []ir.ExpressionHandle) func(ir.ExpressionHandle) ir.ExpressionHandle {
	return func(h ir.ExpressionHandle) ir.ExpressionHandle {
		if int(h) < len(remap) {
			return remap[h]
		}
		return h
	}
}

// markInto returns a visitor that marks each handle it sees in used; a
// handle outside used is ignored.
func markInto(used []bool) func(ir.ExpressionHandle) {
	return func(h ir.ExpressionHandle) {
		if int(h) < len(used) {
			used[h] = true
		}
	}
}

// adjustEmitRange maps an Emit range by its first and last expression, as
// Rust naga's adjust_stmt does: End is exclusive and may be the end of the
// arena, which a remap table has no entry for. An empty range is kept as is.
// The Emit statement always stays.
func adjustEmitRange(f func(ir.ExpressionHandle) ir.ExpressionHandle) func(ir.Range) (ir.Range, bool) {
	return func(r ir.Range) (ir.Range, bool) {
		if r.End > r.Start {
			first, last := f(r.Start), f(r.End-1)
			r = ir.Range{Start: first, End: last + 1}
		}
		return r, true
	}
}

// mapExprHandles returns kind with f applied to every expression handle it
// holds. kind is not modified: a slice of handles is copied. It panics on a
// kind it has no case for.
func mapExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.ExpressionKind {
	return walkExpr(kind, mapHandles{f: f})
}

// handleOp is what the walkers do with each operand handle they find.
type handleOp interface {
	// handle returns what h becomes; a reader returns h.
	handle(h ir.ExpressionHandle) ir.ExpressionHandle
	// rebuilds reports whether the walkers build each kind again with the
	// handles handle returns. A reader does not: the walkers then neither
	// copy nor allocate, and return the kind they were given.
	rebuilds() bool
	// inPlace reports whether a statement's slice of handles (a call's
	// arguments) is written over instead of copied. The expression walker
	// copies its slices whatever inPlace says: it never modifies its input.
	inPlace() bool
}

// mapHandles rebuilds every kind with the handles f returns.
type mapHandles struct {
	f     func(ir.ExpressionHandle) ir.ExpressionHandle
	write bool // statement slices are written over (blockMapper without fresh)
}

func (m mapHandles) handle(h ir.ExpressionHandle) ir.ExpressionHandle { return m.f(h) }
func (mapHandles) rebuilds() bool                                     { return true }
func (m mapHandles) inPlace() bool                                    { return m.write }

// readHandles only reads the handles, calling itself with each.
type readHandles func(ir.ExpressionHandle)

func (r readHandles) handle(h ir.ExpressionHandle) ir.ExpressionHandle {
	r(h)
	return h
}
func (readHandles) rebuilds() bool { return false }
func (readHandles) inPlace() bool  { return false }

// rebuilt returns k, the walker's copy of a kind with its handles mapped,
// when op rebuilds, and orig, the kind the walker was given, when op only
// reads: boxing k into the interface would allocate for nothing.
func rebuilt[O handleOp, I, K any](op O, orig I, k K) I {
	if !op.rebuilds() {
		return orig
	}
	return any(k).(I)
}

// opOpt hands an optional handle to op; a rebuilt handle never shares the
// pointer of h.
func opOpt[O handleOp](op O, h *ir.ExpressionHandle) *ir.ExpressionHandle {
	if h == nil {
		return nil
	}
	if !op.rebuilds() {
		op.handle(*h)
		return h
	}
	v := op.handle(*h)
	return &v
}

// opExprHandles hands the handles of an expression's slice to op; a rebuilt
// slice is a new one, as the expression walker never modifies its input.
func opExprHandles[O handleOp](op O, hs []ir.ExpressionHandle) []ir.ExpressionHandle {
	if !op.rebuilds() {
		for _, h := range hs {
			op.handle(h)
		}
		return hs
	}
	out := make([]ir.ExpressionHandle, len(hs))
	for i, h := range hs {
		out[i] = op.handle(h)
	}
	return out
}

// opStmtHandles hands the handles of a statement's slice to op; a rebuilt
// slice is written over when op works in place, and is a new one otherwise.
func opStmtHandles[O handleOp](op O, hs []ir.ExpressionHandle) []ir.ExpressionHandle {
	if !op.rebuilds() {
		for _, h := range hs {
			op.handle(h)
		}
		return hs
	}
	out := hs
	if !op.inPlace() && hs != nil {
		out = make([]ir.ExpressionHandle, len(hs))
	}
	for i, h := range hs {
		out[i] = op.handle(h)
	}
	return out
}

// walkExpr hands every operand handle of kind to op and returns the kind
// rebuilt, or kind itself when op only reads. It panics on a kind it has no
// case for.
func walkExpr[O handleOp](kind ir.ExpressionKind, op O) ir.ExpressionKind {
	if out, ok := walkValueExpr(kind, op); ok {
		return out
	}
	if out, ok := walkOperationExpr(kind, op); ok {
		return out
	}
	if out, ok := walkImageExpr(kind, op); ok {
		return out
	}
	panic(fmt.Sprintf("irutil: no expression-handle case for expression kind %T", kind))
}

// walkValueExpr walks the kinds that name, build or take apart a value; it
// reports false for any other kind.
func walkValueExpr[O handleOp](kind ir.ExpressionKind, op O) (ir.ExpressionKind, bool) {
	switch k := kind.(type) {
	case nil:
		return nil, true
	case ir.Literal, ir.ExprConstant, ir.ExprOverride, ir.ExprZeroValue,
		ir.ExprFunctionArgument, ir.ExprGlobalVariable, ir.ExprLocalVariable,
		ir.ExprCallResult, ir.ExprAtomicResult, ir.ExprWorkGroupUniformLoadResult,
		ir.ExprRayQueryProceedResult, ir.ExprSubgroupBallotResult,
		ir.ExprSubgroupOperationResult:
		// No operand expression: what they name lives in another arena or
		// is produced by a statement.
		return kind, true
	case ir.ExprCompose:
		k.Components = opExprHandles(op, k.Components)
		return rebuilt(op, kind, k), true
	case ir.ExprAccess:
		k.Base = op.handle(k.Base)
		k.Index = op.handle(k.Index)
		return rebuilt(op, kind, k), true
	case ir.ExprAccessIndex:
		k.Base = op.handle(k.Base)
		return rebuilt(op, kind, k), true
	case ir.ExprSplat:
		k.Value = op.handle(k.Value)
		return rebuilt(op, kind, k), true
	case ir.ExprSwizzle:
		k.Vector = op.handle(k.Vector)
		return rebuilt(op, kind, k), true
	case ir.ExprLoad:
		k.Pointer = op.handle(k.Pointer)
		return rebuilt(op, kind, k), true
	case ssa.Alias:
		k.Source = op.handle(k.Source)
		return rebuilt(op, kind, k), true
	case ssa.Phi:
		return walkPhi(kind, k, op), true
	case ir.ExprArrayLength:
		k.Array = op.handle(k.Array)
		return rebuilt(op, kind, k), true
	case ir.ExprRayQueryGetIntersection:
		k.Query = op.handle(k.Query)
		return rebuilt(op, kind, k), true
	default:
		return nil, false
	}
}

// walkPhi walks the incoming values of a phi; a rebuilt phi has a new slice
// of incomings.
func walkPhi[O handleOp](kind ir.ExpressionKind, k ssa.Phi, op O) ir.ExpressionKind {
	if !op.rebuilds() {
		for _, in := range k.Incoming {
			op.handle(in.Value)
		}
		return kind
	}
	incoming := make([]ssa.Incoming, len(k.Incoming))
	for i, in := range k.Incoming {
		in.Value = op.handle(in.Value)
		incoming[i] = in
	}
	k.Incoming = incoming
	return k
}

// walkOperationExpr walks the operator, built-in function and conversion
// kinds; it reports false for any other kind.
func walkOperationExpr[O handleOp](kind ir.ExpressionKind, op O) (ir.ExpressionKind, bool) {
	switch k := kind.(type) {
	case ir.ExprUnary:
		k.Expr = op.handle(k.Expr)
		return rebuilt(op, kind, k), true
	case ir.ExprBinary:
		k.Left = op.handle(k.Left)
		k.Right = op.handle(k.Right)
		return rebuilt(op, kind, k), true
	case ir.ExprSelect:
		k.Condition = op.handle(k.Condition)
		k.Accept = op.handle(k.Accept)
		k.Reject = op.handle(k.Reject)
		return rebuilt(op, kind, k), true
	case ir.ExprDerivative:
		k.Expr = op.handle(k.Expr)
		return rebuilt(op, kind, k), true
	case ir.ExprRelational:
		k.Argument = op.handle(k.Argument)
		return rebuilt(op, kind, k), true
	case ir.ExprMath:
		k.Arg = op.handle(k.Arg)
		k.Arg1 = opOpt(op, k.Arg1)
		k.Arg2 = opOpt(op, k.Arg2)
		k.Arg3 = opOpt(op, k.Arg3)
		return rebuilt(op, kind, k), true
	case ir.ExprAs:
		k.Expr = op.handle(k.Expr)
		return rebuilt(op, kind, k), true
	default:
		return nil, false
	}
}

// walkImageExpr walks the image kinds; it reports false for any other kind.
func walkImageExpr[O handleOp](kind ir.ExpressionKind, op O) (ir.ExpressionKind, bool) {
	switch k := kind.(type) {
	case ir.ExprImageSample:
		k.Image = op.handle(k.Image)
		k.Sampler = op.handle(k.Sampler)
		k.Coordinate = op.handle(k.Coordinate)
		k.ArrayIndex = opOpt(op, k.ArrayIndex)
		k.Offset = opOpt(op, k.Offset)
		k.Level = walkSampleLevel(k.Level, op)
		k.DepthRef = opOpt(op, k.DepthRef)
		return rebuilt(op, kind, k), true
	case ir.ExprImageLoad:
		k.Image = op.handle(k.Image)
		k.Coordinate = op.handle(k.Coordinate)
		k.ArrayIndex = opOpt(op, k.ArrayIndex)
		k.Sample = opOpt(op, k.Sample)
		k.Level = opOpt(op, k.Level)
		return rebuilt(op, kind, k), true
	case ir.ExprImageQuery:
		k.Image = op.handle(k.Image)
		k.Query = walkImageQuery(k.Query, op)
		return rebuilt(op, kind, k), true
	default:
		return nil, false
	}
}

// walkSampleLevel walks the expression handles of a sample level; nil stays
// nil.
func walkSampleLevel[O handleOp](level ir.SampleLevel, op O) ir.SampleLevel {
	switch l := level.(type) {
	case nil, ir.SampleLevelAuto, ir.SampleLevelZero:
		return level
	case ir.SampleLevelExact:
		l.Level = op.handle(l.Level)
		return rebuilt(op, level, l)
	case ir.SampleLevelBias:
		l.Bias = op.handle(l.Bias)
		return rebuilt(op, level, l)
	case ir.SampleLevelGradient:
		l.X = op.handle(l.X)
		l.Y = op.handle(l.Y)
		return rebuilt(op, level, l)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for sample level %T", level))
	}
}

// walkImageQuery walks the expression handles of an image query.
func walkImageQuery[O handleOp](query ir.ImageQuery, op O) ir.ImageQuery {
	switch q := query.(type) {
	case nil, ir.ImageQueryNumLevels, ir.ImageQueryNumLayers, ir.ImageQueryNumSamples:
		return query
	case ir.ImageQuerySize:
		q.Level = opOpt(op, q.Level)
		return rebuilt(op, query, q)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for image query %T", query))
	}
}

// walkAtomicFunction walks the expression handles of an atomic function.
func walkAtomicFunction[O handleOp](fun ir.AtomicFunction, op O) ir.AtomicFunction {
	switch a := fun.(type) {
	case nil, ir.AtomicAdd, ir.AtomicSubtract, ir.AtomicAnd, ir.AtomicExclusiveOr,
		ir.AtomicInclusiveOr, ir.AtomicMin, ir.AtomicMax:
		return fun
	case ir.AtomicExchange:
		a.Compare = opOpt(op, a.Compare)
		return rebuilt(op, fun, a)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for atomic function %T", fun))
	}
}

// walkRayQueryFunction walks the expression handles of a ray-query function.
func walkRayQueryFunction[O handleOp](fun ir.RayQueryFunction, op O) ir.RayQueryFunction {
	switch r := fun.(type) {
	case nil, ir.RayQueryTerminate, ir.RayQueryConfirmIntersection:
		return fun
	case ir.RayQueryInitialize:
		r.AccelerationStructure = op.handle(r.AccelerationStructure)
		r.Descriptor = op.handle(r.Descriptor)
		return rebuilt(op, fun, r)
	case ir.RayQueryProceed:
		r.Result = op.handle(r.Result)
		return rebuilt(op, fun, r)
	case ir.RayQueryGenerateIntersection:
		r.HitT = op.handle(r.HitT)
		return rebuilt(op, fun, r)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for ray-query function %T", fun))
	}
}

// walkGatherMode walks the expression handles of a subgroup gather mode.
func walkGatherMode[O handleOp](mode ir.GatherMode, op O) ir.GatherMode {
	switch g := mode.(type) {
	case nil, ir.GatherBroadcastFirst, ir.GatherQuadSwap:
		return mode
	case ir.GatherBroadcast:
		g.Index = op.handle(g.Index)
		return rebuilt(op, mode, g)
	case ir.GatherShuffle:
		g.Index = op.handle(g.Index)
		return rebuilt(op, mode, g)
	case ir.GatherShuffleDown:
		g.Delta = op.handle(g.Delta)
		return rebuilt(op, mode, g)
	case ir.GatherShuffleUp:
		g.Delta = op.handle(g.Delta)
		return rebuilt(op, mode, g)
	case ir.GatherShuffleXor:
		g.Mask = op.handle(g.Mask)
		return rebuilt(op, mode, g)
	case ir.GatherQuadBroadcast:
		g.Index = op.handle(g.Index)
		return rebuilt(op, mode, g)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for gather mode %T", mode))
	}
}

// walkStmt hands every operand handle of kind to op (not an Emit range, and
// not the statements of nested blocks) and returns the kind rebuilt, or kind
// itself when op only reads. A rebuilt kind keeps the blocks of kind. It
// panics on a kind it has no case for.
func walkStmt[O handleOp](kind ir.StatementKind, op O) ir.StatementKind {
	if out, ok := walkControlStmt(kind, op); ok {
		return out
	}
	return walkOperationStmt(kind, op)
}

// walkControlStmt walks the control-flow kinds and Emit; it reports false
// for any other kind.
func walkControlStmt[O handleOp](kind ir.StatementKind, op O) (ir.StatementKind, bool) {
	switch s := kind.(type) {
	case nil:
		return nil, true
	case ir.StmtEmit, ir.StmtBlock, ir.StmtBreak, ir.StmtContinue, ir.StmtKill,
		ir.StmtControlBarrier, ir.StmtMemoryBarrier:
		// No operand: an Emit range says when expressions are evaluated,
		// and a block's statements are walked one by one.
		return kind, true
	case ir.StmtIf:
		s.Condition = op.handle(s.Condition)
		return rebuilt(op, kind, s), true
	case ir.StmtSwitch:
		s.Selector = op.handle(s.Selector)
		return rebuilt(op, kind, s), true
	case ir.StmtLoop:
		s.BreakIf = opOpt(op, s.BreakIf)
		return rebuilt(op, kind, s), true
	case ir.StmtReturn:
		s.Value = opOpt(op, s.Value)
		return rebuilt(op, kind, s), true
	default:
		return nil, false
	}
}

// walkOperationStmt walks a statement kind that is not control flow. It
// panics on a kind it has no case for.
func walkOperationStmt[O handleOp](kind ir.StatementKind, op O) ir.StatementKind {
	switch s := kind.(type) {
	case ir.StmtStore:
		s.Pointer = op.handle(s.Pointer)
		s.Value = op.handle(s.Value)
		return rebuilt(op, kind, s)
	case ir.StmtImageStore:
		s.Image = op.handle(s.Image)
		s.Coordinate = op.handle(s.Coordinate)
		s.ArrayIndex = opOpt(op, s.ArrayIndex)
		s.Value = op.handle(s.Value)
		return rebuilt(op, kind, s)
	case ir.StmtAtomic:
		s.Pointer = op.handle(s.Pointer)
		s.Fun = walkAtomicFunction(s.Fun, op)
		s.Value = op.handle(s.Value)
		s.Result = opOpt(op, s.Result)
		return rebuilt(op, kind, s)
	case ir.StmtImageAtomic:
		// Rust's compact skips fun here (`fun: _`), as its validator allows
		// no compare-exchange on an image; a Compare handle, if one is set,
		// is still an expression of this function and is walked with the
		// rest.
		s.Image = op.handle(s.Image)
		s.Coordinate = op.handle(s.Coordinate)
		s.ArrayIndex = opOpt(op, s.ArrayIndex)
		s.Fun = walkAtomicFunction(s.Fun, op)
		s.Value = op.handle(s.Value)
		return rebuilt(op, kind, s)
	case ir.StmtWorkGroupUniformLoad:
		s.Pointer = op.handle(s.Pointer)
		s.Result = op.handle(s.Result)
		return rebuilt(op, kind, s)
	case ir.StmtCall:
		s.Arguments = opStmtHandles(op, s.Arguments)
		s.Result = opOpt(op, s.Result)
		return rebuilt(op, kind, s)
	case ir.StmtRayQuery:
		s.Query = op.handle(s.Query)
		s.Fun = walkRayQueryFunction(s.Fun, op)
		return rebuilt(op, kind, s)
	case ir.StmtSubgroupBallot:
		s.Result = op.handle(s.Result)
		s.Predicate = opOpt(op, s.Predicate)
		return rebuilt(op, kind, s)
	case ir.StmtSubgroupCollectiveOperation:
		s.Argument = op.handle(s.Argument)
		s.Result = op.handle(s.Result)
		return rebuilt(op, kind, s)
	case ir.StmtSubgroupGather:
		s.Mode = walkGatherMode(s.Mode, op)
		s.Argument = op.handle(s.Argument)
		s.Result = op.handle(s.Result)
		return rebuilt(op, kind, s)
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for statement kind %T", kind))
	}
}

// blockMapper rewrites the expression handles of a statement tree: handle
// maps each handle, and emit maps the range of an Emit statement and reports
// whether the statement stays (compaction drops an Emit with nothing left in
// it). Without fresh it writes the result over the blocks it is given, as
// compaction and pipeline-constant processing want; with fresh it never
// writes into them and builds a new tree, as inlining a callee body that
// stays in the module needs.
type blockMapper struct {
	handle func(ir.ExpressionHandle) ir.ExpressionHandle
	emit   func(ir.Range) (ir.Range, bool)
	fresh  bool
}

// block returns b with its statements mapped and the dropped Emit
// statements left out; without fresh it is b's own storage.
func (m *blockMapper) block(b ir.Block) ir.Block {
	if b == nil {
		return nil
	}
	out := b[:0]
	if m.fresh {
		out = make(ir.Block, 0, len(b))
	}
	for _, s := range b {
		kind, keep := m.statement(s.Kind)
		if !keep {
			continue
		}
		s.Kind = kind
		out = append(out, s)
	}
	return out
}

// statement maps one statement kind and reports whether it stays: the
// operands through walkStmt, then the nested blocks. It panics on a kind it
// has no case for.
func (m *blockMapper) statement(kind ir.StatementKind) (ir.StatementKind, bool) {
	if s, ok := kind.(ir.StmtEmit); ok {
		r, keep := m.emit(s.Range)
		s.Range = r
		return s, keep
	}
	kind = walkStmt(kind, mapHandles{f: m.handle, write: !m.fresh})
	switch s := kind.(type) {
	case ir.StmtBlock:
		s.Block = m.block(s.Block)
		return s, true
	case ir.StmtIf:
		s.Accept = m.block(s.Accept)
		s.Reject = m.block(s.Reject)
		return s, true
	case ir.StmtSwitch:
		if m.fresh {
			s.Cases = slices.Clone(s.Cases)
		}
		for i := range s.Cases {
			s.Cases[i].Body = m.block(s.Cases[i].Body)
		}
		return s, true
	case ir.StmtLoop:
		s.Body = m.block(s.Body)
		s.Continuing = m.block(s.Continuing)
		return s, true
	default:
		return kind, true
	}
}
