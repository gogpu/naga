package dxil

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// restrictIndices clamps run-time indices to the last element, min(u32(i),
// n - 1), as the HLSL writer does: under the Index policy Restrict every
// index into an array, matrix or vector that isn't in a storage or constant
// buffer, and under the BindingArray policy Restrict every index into a
// binding array, to its size or its BindTarget.BindingArraySize. D3D12
// bounds checks storage buffer accesses itself, and a constant buffer
// binding clamps its own (BindTarget.RestrictIndexing, Buffer policy). An
// unbounded binding array without a size is an error.
func restrictIndices(m *ir.Module, opts Options) error {
	r := &restricter{m: m, policies: opts.BoundsCheckPolicies, bindings: opts.BindingMap}
	if r.policies.Index != backend.BoundsCheckRestrict && r.policies.BindingArray != backend.BoundsCheckRestrict {
		return nil
	}
	for i := range m.Functions {
		restrictFunctionIndices(r, &m.Functions[i])
	}
	for i := range m.EntryPoints {
		restrictFunctionIndices(r, &m.EntryPoints[i].Function)
	}
	return r.err
}

// restricter holds what restrictIndices needs, and its first error.
type restricter struct {
	m        *ir.Module
	policies backend.BoundsCheckPolicies
	bindings BindingMap
	err      error
	// types types the function being clamped as it was before; the clamps
	// only read the types of access bases it had.
	types *typifier.Typifier
}

// count is the number of elements a run-time index into base is clamped to,
// or 0 when it isn't clamped.
func (r *restricter) count(fn *ir.Function, base ir.ExpressionHandle) uint32 {
	if gv, ok := bindingArrayGlobal(r.m, fn, base); ok {
		if r.policies.BindingArray != backend.BoundsCheckRestrict {
			return 0
		}
		return r.bindingArraySize(gv)
	}
	if r.policies.Index != backend.BoundsCheckRestrict {
		return 0
	}
	return indexCount(r.m, r.types.Get(base))
}

// bindingArraySize is the number of elements of binding array gv: its
// type's size, or its BindTarget.BindingArraySize. Without either it records
// an error and returns 0.
func (r *restricter) bindingArraySize(gv ir.GlobalVariableHandle) uint32 {
	global := r.m.GlobalVariables[gv]
	if size := r.m.Types[global.Type].Inner.(ir.BindingArrayType).Size; size != nil {
		return *size
	}
	if global.Binding != nil {
		tgt, found := r.bindings[ir.ResourceBinding{Group: global.Binding.Group, Binding: global.Binding.Binding}]
		if found && tgt.BindingArraySize != nil {
			return *tgt.BindingArraySize
		}
	}
	if r.err == nil {
		r.err = fmt.Errorf("dxil: bounds check: the length of an unbounded binding array comes from "+
			"BindingMap (BindingArraySize), and %q has none; set it or use BoundsCheckUnchecked for BindingArray", global.Name)
	}
	return 0
}

// bindingArrayGlobal returns the global base is when it is a binding array.
func bindingArrayGlobal(m *ir.Module, fn *ir.Function, base ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	for {
		switch k := fn.Expressions[base].Kind.(type) {
		case ir.ExprAlias:
			base = k.Source
		case ir.ExprGlobalVariable:
			_, ok := m.Types[m.GlobalVariables[k.Variable].Type].Inner.(ir.BindingArrayType)
			return k.Variable, ok
		default:
			return 0, false
		}
	}
}

// restrictFunctionIndices clamps the indices of fn's accesses: those an Emit
// emits with an Emit of the clamps after it, and the others, which the
// emitter emits where they are used, with clamps it emits there too.
func restrictFunctionIndices(r *restricter, fn *ir.Function) {
	types, err := typifier.Function(r.m, fn)
	if err != nil {
		if r.err == nil {
			r.err = fmt.Errorf("dxil: bounds check: %w", err)
		}
		return
	}
	r.types = types
	n := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	fn.Body = restrictBlock(r, fn, fn.Body)
	for h := ir.ExpressionHandle(0); h < n; h++ {
		restrictAccess(r, fn, h)
	}
}

// restrictBlock clamps the indices of the accesses each Emit of block
// emits, with an Emit of the clamps after it.
func restrictBlock(r *restricter, fn *ir.Function, block ir.Block) ir.Block {
	out := make(ir.Block, 0, len(block))
	for _, st := range block {
		switch k := st.Kind.(type) {
		case ir.StmtEmit:
			out = append(out, st)
			start := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
			for h := k.Range.Start; h < k.Range.End; h++ {
				restrictAccess(r, fn, h)
			}
			if end := ir.ExpressionHandle(len(fn.Expressions)); end > start { //nolint:gosec // as above
				out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: end}}})
			}
			continue
		case ir.StmtBlock:
			k.Block = restrictBlock(r, fn, k.Block)
			st.Kind = k
		case ir.StmtIf:
			k.Accept = restrictBlock(r, fn, k.Accept)
			k.Reject = restrictBlock(r, fn, k.Reject)
			st.Kind = k
		case ir.StmtSwitch:
			for i := range k.Cases {
				k.Cases[i].Body = restrictBlock(r, fn, k.Cases[i].Body)
			}
		case ir.StmtLoop:
			k.Body = restrictBlock(r, fn, k.Body)
			k.Continuing = restrictBlock(r, fn, k.Continuing)
			st.Kind = k
		}
		out = append(out, st)
	}
	return out
}

// restrictAccess makes the index of access h, if it can be out of bounds,
// min(u32(index), n - 1) in new expressions, unless it is one already.
func restrictAccess(r *restricter, fn *ir.Function, h ir.ExpressionHandle) {
	acc, ok := fn.Expressions[h].Kind.(ir.ExprAccess)
	if !ok {
		return
	}
	if clamp, isMath := fn.Expressions[acc.Index].Kind.(ir.ExprMath); isMath && clamp.Fun == ir.MathMin && acc.Index > h {
		return
	}
	count := r.count(fn, acc.Base)
	if count == 0 {
		return
	}
	if v, isConst := constIndex(fn, acc.Index); isConst && v < uint64(count) {
		return
	}
	add := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
		return ir.ExpressionHandle(len(fn.Expressions) - 1) //nolint:gosec // expression count fits uint32 by IR design
	}
	index := add(ir.ExprAs{Expr: acc.Index, Kind: ir.ScalarUint})
	last := add(ir.Literal{Value: ir.LiteralU32(count - 1)})
	acc.Index = add(ir.ExprMath{Fun: ir.MathMin, Arg: index, Arg1: &last})
	fn.Expressions[h].Kind = acc
}

// indexCount is the number of elements a run-time index into base picks
// from, or 0 when it isn't clamped: base is in a storage or constant buffer,
// a binding array or a runtime-sized array. res is the type of base.
func indexCount(m *ir.Module, res ir.TypeResolution) uint32 {
	inner := res.Inner(m.Types)
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
