package irutil

import (
	"fmt"
	"slices"

	"github.com/gogpu/naga/ir"
)

// This file holds the one walker over the expression handles of an
// expression kind (mapExprHandles) and the one walker over the expression
// handles of a statement tree (blockMapper). Compaction marks and renumbers
// through them, inlining copies a callee body through them and
// pipeline-constant processing renumbers through RemapExpressionHandles and
// RemapStatementExpressionHandles, so a kind is handled in one place.
//
// Rust naga matches every kind exhaustively (compact/expressions.rs,
// compact/statements.rs) and the compiler rejects a missing one. Go has no
// exhaustive switch, so both walkers panic on a kind they have no case for,
// and TestWalkersCoverEveryKind builds every kind declared in package ir to
// prove there is none: a kind added to ir without a case here fails that
// test instead of keeping stale handles at run time.

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

// markInto returns a handle mapping that marks each handle it sees in used
// and keeps it, so the walkers serve as tracers too.
func markInto(used []bool) func(ir.ExpressionHandle) ir.ExpressionHandle {
	return func(h ir.ExpressionHandle) ir.ExpressionHandle {
		if int(h) < len(used) {
			used[h] = true
		}
		return h
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

// keepEmitRange leaves an Emit range alone: a tracer does not treat emitted
// expressions as used, as Rust naga's trace_block does not ("since
// evaluating expressions has no effect, we don't need to assume that
// everything emitted is live").
func keepEmitRange(r ir.Range) (ir.Range, bool) { return r, true }

// mapOpt maps an optional handle; the result never shares the pointer of h.
func mapOpt(h *ir.ExpressionHandle, f func(ir.ExpressionHandle) ir.ExpressionHandle) *ir.ExpressionHandle {
	if h == nil {
		return nil
	}
	v := f(*h)
	return &v
}

// mapExprHandles returns kind with f applied to every expression handle it
// holds. kind is not modified: a slice of handles is copied. It panics on a
// kind it has no case for.
func mapExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.ExpressionKind {
	if out, ok := mapValueExprHandles(kind, f); ok {
		return out
	}
	if out, ok := mapOperationExprHandles(kind, f); ok {
		return out
	}
	if out, ok := mapImageExprHandles(kind, f); ok {
		return out
	}
	panic(fmt.Sprintf("irutil: no expression-handle case for expression kind %T", kind))
}

// mapValueExprHandles maps the kinds that name, build or take apart a value;
// it reports false for any other kind.
func mapValueExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle) ir.ExpressionHandle) (ir.ExpressionKind, bool) {
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
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			comps[i] = f(c)
		}
		k.Components = comps
		return k, true
	case ir.ExprAccess:
		k.Base = f(k.Base)
		k.Index = f(k.Index)
		return k, true
	case ir.ExprAccessIndex:
		k.Base = f(k.Base)
		return k, true
	case ir.ExprSplat:
		k.Value = f(k.Value)
		return k, true
	case ir.ExprSwizzle:
		k.Vector = f(k.Vector)
		return k, true
	case ir.ExprLoad:
		k.Pointer = f(k.Pointer)
		return k, true
	case ir.ExprAlias:
		k.Source = f(k.Source)
		return k, true
	case ir.ExprPhi:
		incoming := make([]ir.PhiIncoming, len(k.Incoming))
		for i, in := range k.Incoming {
			in.Value = f(in.Value)
			incoming[i] = in
		}
		k.Incoming = incoming
		return k, true
	case ir.ExprArrayLength:
		k.Array = f(k.Array)
		return k, true
	case ir.ExprRayQueryGetIntersection:
		k.Query = f(k.Query)
		return k, true
	default:
		return nil, false
	}
}

// mapOperationExprHandles maps the operator, built-in function and
// conversion kinds; it reports false for any other kind.
func mapOperationExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle) ir.ExpressionHandle) (ir.ExpressionKind, bool) {
	switch k := kind.(type) {
	case ir.ExprUnary:
		k.Expr = f(k.Expr)
		return k, true
	case ir.ExprBinary:
		k.Left = f(k.Left)
		k.Right = f(k.Right)
		return k, true
	case ir.ExprSelect:
		k.Condition = f(k.Condition)
		k.Accept = f(k.Accept)
		k.Reject = f(k.Reject)
		return k, true
	case ir.ExprDerivative:
		k.Expr = f(k.Expr)
		return k, true
	case ir.ExprRelational:
		k.Argument = f(k.Argument)
		return k, true
	case ir.ExprMath:
		k.Arg = f(k.Arg)
		k.Arg1 = mapOpt(k.Arg1, f)
		k.Arg2 = mapOpt(k.Arg2, f)
		k.Arg3 = mapOpt(k.Arg3, f)
		return k, true
	case ir.ExprAs:
		k.Expr = f(k.Expr)
		return k, true
	default:
		return nil, false
	}
}

// mapImageExprHandles maps the image kinds; it reports false for any other
// kind.
func mapImageExprHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle) ir.ExpressionHandle) (ir.ExpressionKind, bool) {
	switch k := kind.(type) {
	case ir.ExprImageSample:
		k.Image = f(k.Image)
		k.Sampler = f(k.Sampler)
		k.Coordinate = f(k.Coordinate)
		k.ArrayIndex = mapOpt(k.ArrayIndex, f)
		k.Offset = mapOpt(k.Offset, f)
		k.Level = mapSampleLevel(k.Level, f)
		k.DepthRef = mapOpt(k.DepthRef, f)
		return k, true
	case ir.ExprImageLoad:
		k.Image = f(k.Image)
		k.Coordinate = f(k.Coordinate)
		k.ArrayIndex = mapOpt(k.ArrayIndex, f)
		k.Sample = mapOpt(k.Sample, f)
		k.Level = mapOpt(k.Level, f)
		return k, true
	case ir.ExprImageQuery:
		k.Image = f(k.Image)
		k.Query = mapImageQuery(k.Query, f)
		return k, true
	default:
		return nil, false
	}
}

// mapSampleLevel maps the expression handles of a sample level; nil stays
// nil.
func mapSampleLevel(level ir.SampleLevel, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.SampleLevel {
	switch l := level.(type) {
	case nil, ir.SampleLevelAuto, ir.SampleLevelZero:
		return level
	case ir.SampleLevelExact:
		l.Level = f(l.Level)
		return l
	case ir.SampleLevelBias:
		l.Bias = f(l.Bias)
		return l
	case ir.SampleLevelGradient:
		l.X = f(l.X)
		l.Y = f(l.Y)
		return l
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for sample level %T", level))
	}
}

// mapImageQuery maps the expression handles of an image query.
func mapImageQuery(query ir.ImageQuery, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.ImageQuery {
	switch q := query.(type) {
	case nil, ir.ImageQueryNumLevels, ir.ImageQueryNumLayers, ir.ImageQueryNumSamples:
		return query
	case ir.ImageQuerySize:
		q.Level = mapOpt(q.Level, f)
		return q
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for image query %T", query))
	}
}

// mapAtomicFunction maps the expression handles of an atomic function.
func mapAtomicFunction(fun ir.AtomicFunction, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.AtomicFunction {
	switch a := fun.(type) {
	case nil, ir.AtomicAdd, ir.AtomicSubtract, ir.AtomicAnd, ir.AtomicExclusiveOr,
		ir.AtomicInclusiveOr, ir.AtomicMin, ir.AtomicMax, ir.AtomicStore, ir.AtomicLoad:
		return fun
	case ir.AtomicExchange:
		a.Compare = mapOpt(a.Compare, f)
		return a
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for atomic function %T", fun))
	}
}

// mapRayQueryFunction maps the expression handles of a ray-query function.
func mapRayQueryFunction(fun ir.RayQueryFunction, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.RayQueryFunction {
	switch r := fun.(type) {
	case nil, ir.RayQueryTerminate, ir.RayQueryConfirmIntersection:
		return fun
	case ir.RayQueryInitialize:
		r.AccelerationStructure = f(r.AccelerationStructure)
		r.Descriptor = f(r.Descriptor)
		return r
	case ir.RayQueryProceed:
		r.Result = f(r.Result)
		return r
	case ir.RayQueryGenerateIntersection:
		r.HitT = f(r.HitT)
		return r
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for ray-query function %T", fun))
	}
}

// mapGatherMode maps the expression handles of a subgroup gather mode.
func mapGatherMode(mode ir.GatherMode, f func(ir.ExpressionHandle) ir.ExpressionHandle) ir.GatherMode {
	switch g := mode.(type) {
	case nil, ir.GatherBroadcastFirst, ir.GatherQuadSwap:
		return mode
	case ir.GatherBroadcast:
		g.Index = f(g.Index)
		return g
	case ir.GatherShuffle:
		g.Index = f(g.Index)
		return g
	case ir.GatherShuffleDown:
		g.Delta = f(g.Delta)
		return g
	case ir.GatherShuffleUp:
		g.Delta = f(g.Delta)
		return g
	case ir.GatherShuffleXor:
		g.Mask = f(g.Mask)
		return g
	case ir.GatherQuadBroadcast:
		g.Index = f(g.Index)
		return g
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for gather mode %T", mode))
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

// handles maps a slice of handles; without fresh it is the slice's own
// storage.
func (m *blockMapper) handles(hs []ir.ExpressionHandle) []ir.ExpressionHandle {
	out := hs
	if m.fresh && hs != nil {
		out = make([]ir.ExpressionHandle, len(hs))
	}
	for i, h := range hs {
		out[i] = m.handle(h)
	}
	return out
}

// statement maps one statement kind and reports whether it stays: the
// control-flow kinds here, the others in operation. It panics on a kind it
// has no case for.
func (m *blockMapper) statement(kind ir.StatementKind) (ir.StatementKind, bool) {
	f := m.handle
	switch s := kind.(type) {
	case nil:
		return nil, true
	case ir.StmtBreak, ir.StmtContinue, ir.StmtKill, ir.StmtControlBarrier, ir.StmtMemoryBarrier:
		return kind, true
	case ir.StmtEmit:
		r, keep := m.emit(s.Range)
		s.Range = r
		return s, keep
	case ir.StmtBlock:
		s.Block = m.block(s.Block)
		return s, true
	case ir.StmtIf:
		s.Condition = f(s.Condition)
		s.Accept = m.block(s.Accept)
		s.Reject = m.block(s.Reject)
		return s, true
	case ir.StmtSwitch:
		s.Selector = f(s.Selector)
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
		s.BreakIf = mapOpt(s.BreakIf, f)
		return s, true
	case ir.StmtReturn:
		s.Value = mapOpt(s.Value, f)
		return s, true
	default:
		return m.operation(kind), true
	}
}

// operation maps a statement kind that is not control flow; none of them is
// ever dropped. It panics on a kind it has no case for.
func (m *blockMapper) operation(kind ir.StatementKind) ir.StatementKind {
	f := m.handle
	switch s := kind.(type) {
	case ir.StmtStore:
		s.Pointer = f(s.Pointer)
		s.Value = f(s.Value)
		return s
	case ir.StmtImageStore:
		s.Image = f(s.Image)
		s.Coordinate = f(s.Coordinate)
		s.ArrayIndex = mapOpt(s.ArrayIndex, f)
		s.Value = f(s.Value)
		return s
	case ir.StmtAtomic:
		s.Pointer = f(s.Pointer)
		s.Fun = mapAtomicFunction(s.Fun, f)
		s.Value = f(s.Value)
		s.Result = mapOpt(s.Result, f)
		return s
	case ir.StmtImageAtomic:
		// Rust's compact skips fun here (`fun: _`), as its validator allows
		// no compare-exchange on an image; a Compare handle, if one is set,
		// is still an expression of this function and is mapped with the
		// rest.
		s.Image = f(s.Image)
		s.Coordinate = f(s.Coordinate)
		s.ArrayIndex = mapOpt(s.ArrayIndex, f)
		s.Fun = mapAtomicFunction(s.Fun, f)
		s.Value = f(s.Value)
		return s
	case ir.StmtWorkGroupUniformLoad:
		s.Pointer = f(s.Pointer)
		s.Result = f(s.Result)
		return s
	case ir.StmtCall:
		s.Arguments = m.handles(s.Arguments)
		s.Result = mapOpt(s.Result, f)
		return s
	case ir.StmtRayQuery:
		s.Query = f(s.Query)
		s.Fun = mapRayQueryFunction(s.Fun, f)
		return s
	case ir.StmtSubgroupBallot:
		s.Result = f(s.Result)
		s.Predicate = mapOpt(s.Predicate, f)
		return s
	case ir.StmtSubgroupCollectiveOperation:
		s.Argument = f(s.Argument)
		s.Result = f(s.Result)
		return s
	case ir.StmtSubgroupGather:
		s.Mode = mapGatherMode(s.Mode, f)
		s.Argument = f(s.Argument)
		s.Result = f(s.Result)
		return s
	default:
		panic(fmt.Sprintf("irutil: no expression-handle case for statement kind %T", kind))
	}
}
