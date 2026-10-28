package dxil

import (
	"fmt"
	"slices"

	"github.com/gogpu/naga/ir"
)

// zeroInitWorkgroupMemory makes compute entry point ep store zero to every
// workgroup variable it uses before running its body: invocation 0 does the
// stores and a workgroup barrier holds the others until it is done, as the
// HLSL writer does with ZeroInitializeWorkgroupMemory. The stores go to each
// scalar and vector, an array's elements in a loop, since the emitter splits
// a store of a struct or an array only one level deep. A workgroup matrix is
// an error, since the emitter can't store to one yet.
func zeroInitWorkgroupMemory(m *ir.Module, ep *ir.EntryPoint) error {
	if ep.Stage != ir.StageCompute {
		return nil
	}
	var vars []ir.GlobalVariableHandle
	for h := range reachableGlobalsForEntry(m, ep) {
		if m.GlobalVariables[h].Space == ir.SpaceWorkGroup {
			vars = append(vars, h)
		}
	}
	if len(vars) == 0 {
		return nil
	}
	slices.Sort(vars)

	z := &workgroupZeroer{m: m, fn: &ep.Function, u32: typeHandle(m, ir.ScalarType{Kind: ir.ScalarUint, Width: 4})}
	var binding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinLocalInvocationIndex}
	z.fn.Arguments = append(z.fn.Arguments, ir.FunctionArgument{Name: "__local_invocation_index", Type: z.u32, Binding: &binding})
	index := z.add(ir.ExprFunctionArgument{Index: uint32(len(z.fn.Arguments) - 1)}, z.value(z.u32)) //nolint:gosec // argument count fits uint32
	isFirst, emitIsFirst := z.emitted(ir.ExprBinary{Op: ir.BinaryEqual, Left: index, Right: z.literal(0)}, z.boolean())

	var stores ir.Block
	for _, h := range vars {
		ty := m.GlobalVariables[h].Type
		b, err := z.zero(z.add(ir.ExprGlobalVariable{Variable: h}, z.pointer(ty, ir.SpaceWorkGroup)), ty)
		if err != nil {
			return fmt.Errorf("dxil: zero-initializing workgroup variable %q: %w", m.GlobalVariables[h].Name, err)
		}
		stores = append(stores, b...)
	}
	z.fn.Body = slices.Concat(ir.Block{
		emitIsFirst,
		{Kind: ir.StmtIf{Condition: isFirst, Accept: stores}},
		{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
	}, z.fn.Body)
	return nil
}

type workgroupZeroer struct {
	m   *ir.Module
	fn  *ir.Function
	u32 ir.TypeHandle
}

// zero returns the statements storing zero to all of the ty that ptr, a
// pointer into workgroup memory, points to.
func (z *workgroupZeroer) zero(ptr ir.ExpressionHandle, ty ir.TypeHandle) (ir.Block, error) {
	switch t := z.m.Types[ty].Inner.(type) {
	case ir.StructType:
		if z.flat(t) {
			return z.zeroFlatStruct(ptr, ty, t), nil
		}
		b := make(ir.Block, 0, len(t.Members))
		for i, member := range t.Members {
			field, emit := z.emitted(ir.ExprAccessIndex{Base: ptr, Index: uint32(i)}, z.pointer(member.Type, ir.SpaceWorkGroup))
			stores, err := z.zero(field, member.Type)
			if err != nil {
				return nil, err
			}
			b = append(b, emit)
			b = append(b, stores...)
		}
		return b, nil
	case ir.ArrayType:
		return z.zeroArray(ptr, t)
	case ir.MatrixType:
		return nil, fmt.Errorf("it holds a matrix, which DXIL can't store to in workgroup memory yet")
	case ir.AtomicType:
		ty = typeHandle(z.m, t.Scalar)
	}
	return ir.Block{{Kind: ir.StmtStore{Pointer: ptr, Value: z.add(ir.ExprZeroValue{Type: ty}, z.value(ty))}}}, nil
}

// flat reports whether every member of t is a scalar or a vector.
func (z *workgroupZeroer) flat(t ir.StructType) bool {
	for _, member := range t.Members {
		switch z.m.Types[member.Type].Inner.(type) {
		case ir.ScalarType, ir.VectorType:
		default:
			return false
		}
	}
	return true
}

// zeroFlatStruct stores a struct composed of zeros, as s = S(0u, ...) does.
// The emitter can store a whole struct where it can't index one of its
// members, in an element of a workgroup array.
func (z *workgroupZeroer) zeroFlatStruct(ptr ir.ExpressionHandle, ty ir.TypeHandle, t ir.StructType) ir.Block {
	components := make([]ir.ExpressionHandle, len(t.Members))
	for i, member := range t.Members {
		components[i] = z.add(ir.ExprZeroValue{Type: member.Type}, z.value(member.Type))
	}
	value, emit := z.emitted(ir.ExprCompose{Type: ty, Components: components}, z.value(ty))
	return ir.Block{emit, {Kind: ir.StmtStore{Pointer: ptr, Value: value}}}
}

// zeroArray returns a loop storing zero to each element of the array ptr
// points to.
func (z *workgroupZeroer) zeroArray(ptr ir.ExpressionHandle, t ir.ArrayType) (ir.Block, error) {
	init := z.literal(0)
	z.fn.LocalVars = append(z.fn.LocalVars, ir.LocalVariable{Name: "__zero_init_index", Type: z.u32, Init: &init})
	counter := z.add(ir.ExprLocalVariable{Variable: uint32(len(z.fn.LocalVars) - 1)}, z.pointer(z.u32, ir.SpaceFunction)) //nolint:gosec // local count fits uint32

	i, emitI := z.emitted(ir.ExprLoad{Pointer: counter}, z.value(z.u32))
	inBounds, emitInBounds := z.emitted(ir.ExprBinary{Op: ir.BinaryLess, Left: i, Right: z.literal(*t.Size.Constant)}, z.boolean())
	elem, emitElem := z.emitted(ir.ExprAccess{Base: ptr, Index: i}, z.pointer(t.Base, ir.SpaceWorkGroup))
	stores, err := z.zero(elem, t.Base)
	if err != nil {
		return nil, err
	}
	body := slices.Concat(ir.Block{
		emitI,
		emitInBounds,
		{Kind: ir.StmtIf{Condition: inBounds, Reject: ir.Block{{Kind: ir.StmtBreak{}}}}},
		emitElem,
	}, stores)

	last, emitLast := z.emitted(ir.ExprLoad{Pointer: counter}, z.value(z.u32))
	next, emitNext := z.emitted(ir.ExprBinary{Op: ir.BinaryAdd, Left: last, Right: z.literal(1)}, z.value(z.u32))
	continuing := ir.Block{emitLast, emitNext, {Kind: ir.StmtStore{Pointer: counter, Value: next}}}
	return ir.Block{{Kind: ir.StmtLoop{Body: body, Continuing: continuing}}}, nil
}

func (z *workgroupZeroer) add(kind ir.ExpressionKind, ty ir.TypeResolution) ir.ExpressionHandle {
	z.fn.Expressions = append(z.fn.Expressions, ir.Expression{Kind: kind})
	z.fn.ExpressionTypes = append(z.fn.ExpressionTypes, ty)
	return ir.ExpressionHandle(len(z.fn.Expressions) - 1) //nolint:gosec // expression count fits uint32 by IR design
}

// emitted adds an expression that has to be emitted, with the Emit for it.
func (z *workgroupZeroer) emitted(kind ir.ExpressionKind, ty ir.TypeResolution) (ir.ExpressionHandle, ir.Statement) {
	h := z.add(kind, ty)
	return h, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}}
}

func (z *workgroupZeroer) literal(v uint32) ir.ExpressionHandle {
	return z.add(ir.Literal{Value: ir.LiteralU32(v)}, z.value(z.u32))
}

func (z *workgroupZeroer) value(ty ir.TypeHandle) ir.TypeResolution {
	return ir.TypeResolution{Handle: &ty}
}

func (z *workgroupZeroer) boolean() ir.TypeResolution {
	return ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}
}

func (z *workgroupZeroer) pointer(base ir.TypeHandle, space ir.AddressSpace) ir.TypeResolution {
	return ir.TypeResolution{Value: ir.PointerType{Base: base, Space: space}}
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
