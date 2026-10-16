package codegen

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/ir"
)

// ErrMissingBinding reports a resource with no BindingMap entry while
// Options.RequireBindings is set (Rust naga's Error::MissingBinding).
var ErrMissingBinding = errors.New("resource binding missing from BindingMap")

// bindingArrayKey identifies a binding array type whose size comes from the
// binding map rather than the module (Rust naga's LocalType::BindingArray).
type bindingArrayKey struct {
	base ir.TypeHandle
	size uint32
}

// resolveResourceBinding returns where binding goes in SPIR-V (Rust naga's
// resolve_resource_binding): its BindingMap entry or, unless RequireBindings
// is set, its WGSL @group/@binding.
func (b *Backend) resolveResourceBinding(binding ir.ResourceBinding) (BindingInfo, error) {
	if info, ok := b.options.BindingMap[binding]; ok {
		return info, nil
	}
	if b.options.RequireBindings {
		return BindingInfo{}, fmt.Errorf("%w: group %d, binding %d", ErrMissingBinding, binding.Group, binding.Binding)
	}
	return BindingInfo{DescriptorSet: binding.Group, Binding: binding.Binding}, nil
}

// remappedBindingArrayType returns the type for a binding array global whose
// BindingMap entry sets BindingArraySize: an array of that many elements,
// sized or unbounded in WGSL alike (Rust naga's write_global_variable).
// ok is false when the global keeps its declared type.
func (b *Backend) remappedBindingArrayType(global *ir.GlobalVariable, info BindingInfo) (id uint32, ok bool, err error) {
	if info.BindingArraySize == nil {
		return 0, false, nil
	}
	ba, isBindingArray := b.module.Types[global.Type].Inner.(ir.BindingArrayType)
	if !isBindingArray {
		return 0, false, nil
	}
	key := bindingArrayKey{base: ba.Base, size: *info.BindingArraySize}
	if id, ok := b.bindingArrayTypes[key]; ok {
		return id, true, nil
	}
	baseID, err := b.emitType(ba.Base)
	if err != nil {
		return 0, false, err
	}
	u32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	if err != nil {
		return 0, false, err
	}
	id = b.builder.AddTypeArray(baseID, b.builder.AddConstant(u32TypeID, key.size))
	if b.bindingArrayTypes == nil {
		b.bindingArrayTypes = make(map[bindingArrayKey]uint32)
	}
	b.bindingArrayTypes[key] = id
	return id, true, nil
}

// bindingArrayLayoutSize returns the BindingMap size of the binding array
// global that sequence indexes into, if any (Rust naga's
// binding_array_layout_size).
func (e *ExpressionEmitter) bindingArrayLayoutSize(sequence ir.ExpressionHandle) (uint32, bool) {
	handle, ok := e.originatingGlobal(sequence)
	if !ok {
		return 0, false
	}
	global := &e.backend.module.GlobalVariables[handle]
	if _, isBindingArray := e.backend.module.Types[global.Type].Inner.(ir.BindingArrayType); !isBindingArray || global.Binding == nil {
		return 0, false
	}
	info, err := e.backend.resolveResourceBinding(*global.Binding)
	if err != nil || info.BindingArraySize == nil {
		return 0, false
	}
	return *info.BindingArraySize, true
}

// originatingGlobal follows Access and AccessIndex bases back to the global
// variable a pointer is derived from (Rust naga's Function::originating_global).
func (e *ExpressionEmitter) originatingGlobal(pointer ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	for {
		switch k := e.function.Expressions[pointer].Kind.(type) {
		case ir.ExprAccess:
			pointer = k.Base
		case ir.ExprAccessIndex:
			pointer = k.Base
		case ir.ExprGlobalVariable:
			return k.Variable, true
		default:
			return 0, false
		}
	}
}
