package dxil

import "github.com/gogpu/naga/ir"

// restrictIndices clamps every run-time index into an array, matrix or
// vector of m that isn't in a storage or constant buffer to the last
// element, min(u32(i), n - 1), as the HLSL writer does with
// RestrictIndexing. D3D12 bounds checks storage buffer accesses itself, and
// a constant buffer binding clamps its own (BindTarget.RestrictIndexing).
func restrictIndices(m *ir.Module) {
	for i := range m.Functions {
		restrictFunctionIndices(m, &m.Functions[i])
	}
	for i := range m.EntryPoints {
		restrictFunctionIndices(m, &m.EntryPoints[i].Function)
	}
}

// restrictFunctionIndices clamps the indices of fn's accesses: those an Emit
// emits with an Emit of the clamps after it, and the others, which the
// emitter emits where they are used, with clamps it emits there too.
func restrictFunctionIndices(m *ir.Module, fn *ir.Function) {
	n := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	fn.Body = restrictBlock(m, fn, fn.Body)
	for h := ir.ExpressionHandle(0); h < n; h++ {
		restrictAccess(m, fn, h)
	}
}

// restrictBlock clamps the indices of the accesses each Emit of block
// emits, with an Emit of the clamps after it.
func restrictBlock(m *ir.Module, fn *ir.Function, block ir.Block) ir.Block {
	out := make(ir.Block, 0, len(block))
	for _, st := range block {
		switch k := st.Kind.(type) {
		case ir.StmtEmit:
			out = append(out, st)
			start := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
			for h := k.Range.Start; h < k.Range.End; h++ {
				restrictAccess(m, fn, h)
			}
			if end := ir.ExpressionHandle(len(fn.Expressions)); end > start { //nolint:gosec // as above
				out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: end}}})
			}
			continue
		case ir.StmtBlock:
			k.Block = restrictBlock(m, fn, k.Block)
			st.Kind = k
		case ir.StmtIf:
			k.Accept = restrictBlock(m, fn, k.Accept)
			k.Reject = restrictBlock(m, fn, k.Reject)
			st.Kind = k
		case ir.StmtSwitch:
			for i := range k.Cases {
				k.Cases[i].Body = restrictBlock(m, fn, k.Cases[i].Body)
			}
		case ir.StmtLoop:
			k.Body = restrictBlock(m, fn, k.Body)
			k.Continuing = restrictBlock(m, fn, k.Continuing)
			st.Kind = k
		}
		out = append(out, st)
	}
	return out
}

// restrictAccess makes the index of access h, if it can be out of bounds,
// min(u32(index), n - 1) in new expressions, unless it is one already.
func restrictAccess(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) {
	acc, ok := fn.Expressions[h].Kind.(ir.ExprAccess)
	if !ok {
		return
	}
	if clamp, isMath := fn.Expressions[acc.Index].Kind.(ir.ExprMath); isMath && clamp.Fun == ir.MathMin && acc.Index > h {
		return
	}
	count := indexCount(m, fn, acc.Base)
	if count == 0 {
		return
	}
	if v, isConst := constIndex(fn, acc.Index); isConst && v < uint64(count) {
		return
	}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	add := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		for len(fn.ExpressionTypes) < len(fn.Expressions) {
			fn.ExpressionTypes = append(fn.ExpressionTypes, ir.TypeResolution{})
		}
		fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
		fn.ExpressionTypes = append(fn.ExpressionTypes, ir.TypeResolution{Value: u32})
		return ir.ExpressionHandle(len(fn.Expressions) - 1) //nolint:gosec // expression count fits uint32 by IR design
	}
	index := add(ir.ExprAs{Expr: acc.Index, Kind: ir.ScalarUint})
	last := add(ir.Literal{Value: ir.LiteralU32(count - 1)})
	acc.Index = add(ir.ExprMath{Fun: ir.MathMin, Arg: index, Arg1: &last})
	fn.Expressions[h].Kind = acc
}

// indexCount is the number of elements a run-time index into base picks
// from, or 0 when it isn't clamped: base is in a storage or constant buffer,
// a binding array or a runtime-sized array.
func indexCount(m *ir.Module, fn *ir.Function, base ir.ExpressionHandle) uint32 {
	res, err := ir.ResolveExpressionType(m, fn, base)
	if err != nil {
		return 0
	}
	inner := ir.TypeResInner(m, res)
	switch t := inner.(type) {
	case ir.PointerType:
		if t.Space == ir.SpaceUniform || t.Space == ir.SpaceStorage || t.Space == ir.SpaceHandle {
			return 0
		}
		inner = m.Types[t.Base].Inner
	case ir.ValuePointerType:
		if t.Size == nil || t.Space == ir.SpaceUniform || t.Space == ir.SpaceStorage {
			return 0
		}
		return uint32(*t.Size)
	}
	switch t := inner.(type) {
	case ir.ArrayType:
		if t.Size.Constant != nil {
			return *t.Size.Constant
		}
	case ir.VectorType:
		return uint32(t.Size)
	case ir.MatrixType:
		return uint32(t.Columns)
	}
	return 0
}

// constIndex returns the value of index h when it is an integer literal,
// directly or through aliases.
func constIndex(fn *ir.Function, h ir.ExpressionHandle) (uint64, bool) {
	for {
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprAlias:
			h = k.Source
		case ir.Literal:
			switch v := k.Value.(type) {
			case ir.LiteralU32:
				return uint64(v), true
			case ir.LiteralI32:
				return uint64(v), v >= 0 //nolint:gosec // a negative value is reported as not a constant index
			case ir.LiteralAbstractInt:
				return uint64(v), v >= 0 //nolint:gosec // as above
			}
			return 0, false
		default:
			return 0, false
		}
	}
}
