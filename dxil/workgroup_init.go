package dxil

import (
	"slices"

	"github.com/gogpu/naga/ir"
)

// zeroInitWorkgroupMemory makes compute entry point ep store zero to every
// workgroup variable it uses before running its body: invocation 0 does the
// stores and a workgroup barrier holds the others until it is done, as the
// HLSL writer does with ZeroInitializeWorkgroupMemory. The stores go to each
// member of a struct and to an array's elements in a loop, so that a large
// array is not one store per scalar once scalarizeWorkgroupMemory splits the
// stores.
func zeroInitWorkgroupMemory(m *ir.Module, ep *ir.EntryPoint) {
	if ep.Stage != ir.StageCompute {
		return
	}
	var vars []ir.GlobalVariableHandle
	for h := range reachableGlobalsForEntry(m, ep) {
		if m.GlobalVariables[h].Space == ir.SpaceWorkGroup {
			vars = append(vars, h)
		}
	}
	if len(vars) == 0 {
		return
	}
	slices.Sort(vars)

	z := &workgroupZeroer{m: m, fn: &ep.Function, u32: typeHandle(m, ir.ScalarType{Kind: ir.ScalarUint, Width: 4})}
	var binding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinLocalInvocationIndex}
	z.fn.Arguments = append(z.fn.Arguments, ir.FunctionArgument{Name: "__local_invocation_index", Type: z.u32, Binding: &binding})
	index := z.add(ir.ExprFunctionArgument{Index: uint32(len(z.fn.Arguments) - 1)}) //nolint:gosec // argument count fits uint32
	isFirst, emitIsFirst := z.emitted(ir.ExprBinary{Op: ir.BinaryEqual, Left: index, Right: z.literal(0)})

	var stores ir.Block
	for _, h := range vars {
		ty := m.GlobalVariables[h].Type
		stores = append(stores, z.zero(z.add(ir.ExprGlobalVariable{Variable: h}), ty)...)
	}
	z.fn.Body = slices.Concat(ir.Block{
		emitIsFirst,
		{Kind: ir.StmtIf{Condition: isFirst, Accept: stores}},
		{Kind: ir.StmtControlBarrier{Flags: ir.BarrierWorkGroup}},
	}, z.fn.Body)
}

type workgroupZeroer struct {
	m   *ir.Module
	fn  *ir.Function
	u32 ir.TypeHandle
}

// zero returns the statements storing zero to all of the ty that ptr, a
// pointer into workgroup memory, points to.
func (z *workgroupZeroer) zero(ptr ir.ExpressionHandle, ty ir.TypeHandle) ir.Block {
	switch t := z.m.Types[ty].Inner.(type) {
	case ir.StructType:
		b := make(ir.Block, 0, len(t.Members))
		for i, member := range t.Members {
			field, emit := z.emitted(ir.ExprAccessIndex{Base: ptr, Index: uint32(i)})
			b = append(b, emit)
			b = append(b, z.zero(field, member.Type)...)
		}
		return b
	case ir.ArrayType:
		return z.zeroArray(ptr, t)
	case ir.AtomicType:
		ty = typeHandle(z.m, t.Scalar)
	}
	return ir.Block{{Kind: ir.StmtStore{Pointer: ptr, Value: z.add(ir.ExprZeroValue{Type: ty})}}}
}

// zeroArray returns a loop storing zero to each element of the array ptr
// points to.
func (z *workgroupZeroer) zeroArray(ptr ir.ExpressionHandle, t ir.ArrayType) ir.Block {
	init := z.literal(0)
	z.fn.LocalVars = append(z.fn.LocalVars, ir.LocalVariable{Name: "__zero_init_index", Type: z.u32, Init: &init})
	counter := z.add(ir.ExprLocalVariable{Variable: ir.LocalVariableHandle(len(z.fn.LocalVars) - 1)}) //nolint:gosec // local count fits uint32

	i, emitI := z.emitted(ir.ExprLoad{Pointer: counter})
	inBounds, emitInBounds := z.emitted(ir.ExprBinary{Op: ir.BinaryLess, Left: i, Right: z.literal(*t.Size.Constant)})
	elem, emitElem := z.emitted(ir.ExprAccess{Base: ptr, Index: i})
	body := slices.Concat(ir.Block{
		emitI,
		emitInBounds,
		{Kind: ir.StmtIf{Condition: inBounds, Reject: ir.Block{{Kind: ir.StmtBreak{}}}}},
		emitElem,
	}, z.zero(elem, t.Base))

	last, emitLast := z.emitted(ir.ExprLoad{Pointer: counter})
	next, emitNext := z.emitted(ir.ExprBinary{Op: ir.BinaryAdd, Left: last, Right: z.literal(1)})
	continuing := ir.Block{emitLast, emitNext, {Kind: ir.StmtStore{Pointer: counter, Value: next}}}
	return ir.Block{{Kind: ir.StmtLoop{Body: body, Continuing: continuing}}}
}

func (z *workgroupZeroer) add(kind ir.ExpressionKind) ir.ExpressionHandle {
	z.fn.Expressions = append(z.fn.Expressions, ir.Expression{Kind: kind})
	return ir.ExpressionHandle(len(z.fn.Expressions) - 1) //nolint:gosec // expression count fits uint32 by IR design
}

// emitted adds an expression that has to be emitted, with the Emit for it.
func (z *workgroupZeroer) emitted(kind ir.ExpressionKind) (ir.ExpressionHandle, ir.Statement) {
	h := z.add(kind)
	return h, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}}
}

func (z *workgroupZeroer) literal(v uint32) ir.ExpressionHandle {
	return z.add(ir.Literal{Value: ir.LiteralU32(v)})
}

// typeHandle returns the handle of an unnamed type inner in m, adding it
// when m doesn't have it.
func typeHandle(m *ir.Module, inner ir.TypeInner) ir.TypeHandle {
	for i, t := range m.Types {
		if t.Inner == inner {
			return ir.TypeHandle(i)
		}
	}
	m.Types = append(m.Types, ir.Type{Inner: inner})
	return ir.TypeHandle(len(m.Types) - 1) //nolint:gosec // type count fits uint32 by IR design
}
