package front

import (
	"sort"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// atomicAccess records what a module accesses with atomic operations, Rust
// naga's atomic_upgrade::Upgrades: the globals, and the struct fields of those
// globals' types, that have to become Atomic.
type atomicAccess struct {
	globals map[ir.GlobalVariableHandle]struct{}
	// fields maps a struct type to the indices of its members that are
	// accessed atomically.
	fields map[ir.TypeHandle]map[uint32]struct{}
}

// recordGlobal marks a global variable as atomically accessed.
func (a *atomicAccess) recordGlobal(handle ir.GlobalVariableHandle) {
	a.globals[handle] = struct{}{}
}

// recordField marks one member of a struct type.
func (a *atomicAccess) recordField(handle ir.TypeHandle, index uint32) {
	fields, ok := a.fields[handle]
	if !ok {
		fields = make(map[uint32]struct{})
		a.fields[handle] = fields
	}
	fields[index] = struct{}{}
}

// recordAtomicAccess walks a pointer expression down to the global variable it
// addresses and records every global and struct field on the way, Rust naga's
// record_atomic_access. It returns the type of the value the expression
// addresses.
func (p *parser) recordAtomicAccess(fs *functionState, handle ir.ExpressionHandle) (ir.TypeHandle, error) {
	if int(handle) >= len(fs.fn.Expressions) {
		return 0, errAtomicGlobalVariableMissing
	}
	switch kind := fs.fn.Expressions[handle].Kind.(type) {
	case ir.ExprAccess:
		// An OpAccess into an array of atomics addresses an element.
		ty, err := p.recordAtomicAccess(fs, kind.Base)
		if err != nil {
			return 0, err
		}
		array, ok := p.module.Types[ty].Inner.(ir.ArrayType)
		if !ok {
			// Rust naga panics here; untrusted input must not.
			return 0, errAtomicUnsupportedExpression
		}
		return array.Base, nil
	case ir.ExprAccessIndex:
		ty, err := p.recordAtomicAccess(fs, kind.Base)
		if err != nil {
			return 0, err
		}
		switch t := p.module.Types[ty].Inner.(type) {
		case ir.StructType:
			if int(kind.Index) >= len(t.Members) {
				return 0, errInvalidAccessIndex(kind.Index)
			}
			p.atomicUpgrades.recordField(ty, kind.Index)
			return t.Members[kind.Index].Type, nil
		case ir.ArrayType:
			return t.Base, nil
		}
		return 0, errAtomicUnsupportedExpression
	case ir.ExprGlobalVariable:
		if int(kind.Variable) >= len(p.module.GlobalVariables) {
			return 0, errAtomicGlobalVariableMissing
		}
		p.atomicUpgrades.recordGlobal(kind.Variable)
		return p.module.GlobalVariables[kind.Variable].Type, nil
	}
	return 0, errAtomicGlobalVariableMissing
}

// atomicBaseType returns the type the pointer expression addresses, Rust
// naga's get_exp_and_base_ty_handles.
func (p *parser) atomicBaseType(lookup lookupExpression) (ir.TypeHandle, error) {
	pointer, err := p.typeLookup(lookup.typeID)
	if err != nil {
		return 0, err
	}
	if !pointer.hasBase {
		return 0, errInvalidAccessType(lookup.typeID)
	}
	base, err := p.typeLookup(pointer.baseID)
	if err != nil {
		return 0, err
	}
	return base.handle, nil
}

// atomicStructType returns the predeclared __atomic_compare_exchange_result<T>
// struct type for a scalar, as the WGSL frontend builds it: the boolean comes
// first in the arena, then the scalar, then the named struct.
func (p *parser) atomicStructType(scalar ir.ScalarType) ir.TypeHandle {
	kindName := "Float"
	switch scalar.Kind {
	case ir.ScalarUint:
		kindName = "Uint"
	case ir.ScalarSint:
		kindName = "Sint"
	}
	name := "__atomic_compare_exchange_result<" + kindName + "," + strconv.Itoa(int(scalar.Width)) + ">"
	for index, ty := range p.module.Types {
		if ty.Name == name {
			return ir.TypeHandle(index)
		}
	}
	boolType := p.insertType("", ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	scalarType := p.insertType("", scalar)
	return p.insertType(name, ir.StructType{
		Members: []ir.StructMember{
			{Name: "old_value", Type: scalarType, Offset: 0},
			{Name: "exchanged", Type: boolType, Offset: uint32(scalar.Width)},
		},
		Span: uint32(scalar.Width) * 2,
	})
}

// atomicLoad parses OpAtomicLoad: a plain load of the (upgraded) atomic value,
// as Rust naga's frontend lowers it.
func (p *parser) atomicLoad(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	// The memory scope and semantics are not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	pointer, err := p.handleOf(pointerID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, ir.ExprLoad{Pointer: pointer}))
	_, err = p.recordAtomicAccess(fs, pointer)
	return err
}

// atomicStore parses OpAtomicStore into a plain store of the (upgraded) atomic
// value.
func (p *parser) atomicStore(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int) error {
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	// The memory scope and semantics are not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}
	pointer, err := p.handleOf(pointerID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	value, err := p.handleOf(valueID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	*block = append(*block, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: value}})
	consumer.startWith(fs.fn.Expressions)
	_, err = p.recordAtomicAccess(fs, pointer)
	return err
}

// atomicValue parses the atomic operations that take one value operand
// (OpAtomicExchange and the arithmetic and bitwise families), Rust naga's
// parse_atomic_expr_with_value.
func (p *parser) atomicValue(inst instruction, fun ir.AtomicFunction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	// The memory scope and semantics are not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}

	pointerLookup, err := p.lookupExpr(pointerID)
	if err != nil {
		return err
	}
	pointer, err := p.getExprHandle(pointerID, pointerLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	baseType, err := p.atomicBaseType(pointerLookup)
	if err != nil {
		return err
	}
	value, err := p.lookupExpr(valueID)
	if err != nil {
		return err
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprAtomicResult{Ty: baseType})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	consumer.startWith(fs.fn.Expressions)
	*block = append(*block, ir.Statement{Kind: ir.StmtAtomic{
		Pointer: pointer,
		Fun:     fun,
		Value:   value.handle,
		Result:  &result,
	}})
	_, err = p.recordAtomicAccess(fs, pointer)
	return err
}

// atomicIncrement parses OpAtomicIIncrement and OpAtomicIDecrement: an add or
// subtract of the literal one.
func (p *parser) atomicIncrement(inst instruction, subtract bool, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	// The memory scope and semantics are not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	if _, err := p.next(); err != nil {
		return err
	}

	pointerLookup, err := p.lookupExpr(pointerID)
	if err != nil {
		return err
	}
	pointer, err := p.getExprHandle(pointerID, pointerLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	baseType, err := p.atomicBaseType(pointerLookup)
	if err != nil {
		return err
	}

	// The result and the literal one are pre-emit expressions: the literal is
	// written outside every Emit range, as Rust naga's make_index_literal
	// writes it.
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprAtomicResult{Ty: baseType})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	value, err := p.atomicUnitLiteral(baseType, resultTypeID)
	if err != nil {
		return err
	}
	one := appendExpression(fs.fn, ir.Literal{Value: value})
	consumer.startWith(fs.fn.Expressions)
	fun := ir.AtomicFunction(ir.AtomicAdd{})
	if subtract {
		fun = ir.AtomicSubtract{}
	}
	*block = append(*block, ir.Statement{Kind: ir.StmtAtomic{
		Pointer: pointer,
		Fun:     fun,
		Value:   one,
		Result:  &result,
	}})
	_, err = p.recordAtomicAccess(fs, pointer)
	return err
}

// atomicUnitLiteral returns the literal one of the atomic's scalar type:
// opAtomicIIncrement and OpAtomicIDecrement have no value operand.
func (p *parser) atomicUnitLiteral(baseType ir.TypeHandle, resultTypeID uint32) (ir.LiteralValue, error) {
	if scalar, ok := p.module.Types[baseType].Inner.(ir.ScalarType); ok {
		switch scalar.Kind {
		case ir.ScalarUint:
			return ir.LiteralU32(1), nil
		case ir.ScalarSint:
			return ir.LiteralI32(1), nil
		}
	}
	return nil, errInvalidIndexType(resultTypeID)
}

// atomicCompareExchange parses OpAtomicCompareExchange: the statement's result
// is the predeclared struct, and the instruction's result is its old value.
func (p *parser) atomicCompareExchange(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	// The memory scope, and the equal and unequal semantics, are not carried
	// into the IR.
	for range 3 {
		if _, err := p.next(); err != nil {
			return err
		}
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}
	comparatorID, err := p.next()
	if err != nil {
		return err
	}

	pointerLookup, err := p.lookupExpr(pointerID)
	if err != nil {
		return err
	}
	pointer, err := p.getExprHandle(pointerID, pointerLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	baseType, err := p.atomicBaseType(pointerLookup)
	if err != nil {
		return err
	}
	// Both operands go through get_expr_handle: Rust naga spills a value
	// defined in a block that does not dominate this one.
	value, err := p.handleOf(valueID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	comparator, err := p.handleOf(comparatorID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	scalar, ok := p.module.Types[baseType].Inner.(ir.ScalarType)
	if !ok {
		return errAtomicCompareExchangeNonScalar
	}
	structType := p.atomicStructType(scalar)

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprAtomicResult{Ty: structType, Comparison: true})
	consumer.startWith(fs.fn.Expressions)
	oldValue := appendExpression(fs.fn, ir.ExprAccessIndex{Base: result, Index: 0})
	p.storeExpression(resultID, resultTypeID, blockID, oldValue)
	*block = append(*block, ir.Statement{Kind: ir.StmtAtomic{
		Pointer: pointer,
		Fun:     ir.AtomicExchange{Compare: &comparator},
		Value:   value,
		Result:  &result,
	}})
	_, err = p.recordAtomicAccess(fs, pointer)
	return err
}

// upgradeAtomics rewrites the types of the globals the module accesses
// atomically: every scalar leaf on the path of an atomic access becomes an
// Atomic, Rust naga's Module::upgrade_atomics.
func (p *parser) upgradeAtomics() error {
	if len(p.atomicUpgrades.globals) == 0 {
		return nil
	}
	handles := make([]ir.GlobalVariableHandle, 0, len(p.atomicUpgrades.globals))
	for handle := range p.atomicUpgrades.globals {
		handles = append(handles, handle)
	}
	sort.Slice(handles, func(i, j int) bool { return handles[i] < handles[j] })

	upgraded := make(map[ir.TypeHandle]ir.TypeHandle)
	for _, handle := range handles {
		if int(handle) >= len(p.module.GlobalVariables) {
			continue
		}
		global := &p.module.GlobalVariables[handle]
		if global.InitExpr != nil {
			return errAtomicGlobalInitUnsupported
		}
		newType, err := p.upgradeAtomicType(global.Type, upgraded)
		if err != nil {
			return err
		}
		global.Type = newType
	}
	return nil
}

// upgradeAtomicType returns the type equivalent to ty with its scalar leaves
// upgraded where the module accesses them atomically.
func (p *parser) upgradeAtomicType(ty ir.TypeHandle, upgraded map[ir.TypeHandle]ir.TypeHandle) (ir.TypeHandle, error) {
	if newType, ok := upgraded[ty]; ok {
		return newType, nil
	}
	if int(ty) >= len(p.module.Types) {
		return 0, errUnsupportedType(ty)
	}
	original := p.module.Types[ty]
	var inner ir.TypeInner
	switch t := original.Inner.(type) {
	case ir.ScalarType:
		inner = ir.AtomicType{Scalar: t}
	case ir.PointerType:
		base, err := p.upgradeAtomicType(t.Base, upgraded)
		if err != nil {
			return 0, err
		}
		t.Base = base
		inner = t
	case ir.ArrayType:
		base, err := p.upgradeAtomicType(t.Base, upgraded)
		if err != nil {
			return 0, err
		}
		t.Base = base
		inner = t
	case ir.StructType:
		fields, ok := p.atomicUpgrades.fields[ty]
		if !ok {
			// No field is accessed atomically; nothing to rewrite.
			return ty, nil
		}
		members := append([]ir.StructMember(nil), t.Members...)
		for index := range fields {
			if int(index) >= len(members) {
				return 0, errInvalidAccessIndex(index)
			}
			base, err := p.upgradeAtomicType(members[index].Type, upgraded)
			if err != nil {
				return 0, err
			}
			members[index].Type = base
		}
		t.Members = members
		inner = t
	case ir.BindingArrayType:
		base, err := p.upgradeAtomicType(t.Base, upgraded)
		if err != nil {
			return 0, err
		}
		t.Base = base
		inner = t
	default:
		return ty, nil
	}
	newType := p.insertType(original.Name, inner)
	upgraded[ty] = newType
	return newType, nil
}
