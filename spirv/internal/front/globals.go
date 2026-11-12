package front

import (
	"github.com/gogpu/naga/ir"
)

// lookupVariable is one entry of the SPIR-V global variable table: the IR
// global, and for an entry point input or output the argument or result the
// wrapper uses (Rust naga's LookupVariable and Variable).
type lookupVariable struct {
	class  variableClass
	handle ir.GlobalVariableHandle
	typeID uint32
	// argument is the entry point argument an Input variable becomes.
	argument ir.FunctionArgument
	// result is the entry point result an Output variable becomes.
	result ir.FunctionResult
}

// parseGlobalVariable parses OpVariable. An Input or Output variable becomes a
// private global plus the entry point argument or result the wrapper reads or
// writes it through.
func (p *parser) parseGlobalVariable(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	storageClass, err := p.next()
	if err != nil {
		return err
	}
	var init *ir.ExpressionHandle
	if inst.wc > 4 {
		if err := inst.expect(5); err != nil {
			return err
		}
		initID, err := p.next()
		if err != nil {
			return err
		}
		constant, err := p.constantLookup(initID)
		if err != nil {
			return err
		}
		handle := p.appendGlobalExpression(constant.kind.expressionKind())
		init = &handle
	}

	dec := p.takeDecoration(id)
	original, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	ty := original.handle

	originalInner := p.module.Types[original.handle].Inner
	if pointer, ok := originalInner.(ir.PointerType); ok {
		ty = pointer.Base
	}

	if _, ok := originalInner.(ir.BindingArrayType); ok {
		// parseTypeArray guessed that an array of images or samplers is a
		// binding array; a variable of one needs a resource binding.
		if dec.resourceBinding() == nil {
			return errNonBindingArrayOfImageOrSamplers
		}
	}

	if image, ok := p.module.Types[ty].Inner.(ir.ImageType); ok && image.Class == ir.ImageClassStorage {
		// A SPIR-V storage image type has no access, and one type is shared by
		// every variable of it: the access comes from the variable's
		// decorations, so the type is copied with it applied.
		image.StorageAccess = dec.storageAccess()
		ty = p.insertType("", image)
	}

	extClass, storageAccess, err := p.variableClass(ty, storageClass)
	if err != nil {
		return err
	}

	var global ir.GlobalVariable
	lookup := lookupVariable{typeID: typeID}
	switch extClass.class {
	case classInput:
		binding, err := dec.ioBinding()
		if err != nil {
			return err
		}
		argumentType := ty
		if builtin, ok := binding.(ir.BuiltinBinding); ok {
			if unsigned, ok := p.unsignedBuiltinType(builtin.Builtin, ty); ok {
				argumentType = unsigned
			}
		}
		global = ir.GlobalVariable{
			Name:  dec.name,
			Space: ir.SpacePrivate,
			Type:  ty,
		}
		lookup.class = classInput
		lookup.argument = ir.FunctionArgument{
			Name:    dec.name,
			Type:    argumentType,
			Binding: &binding,
		}
	case classOutput:
		binding, _ := dec.ioBinding()
		var initExpr *ir.ExpressionHandle
		if bb, ok := binding.(ir.BuiltinBinding); ok {
			handle := p.generateDefaultBuiltIn(&bb.Builtin, ty)
			initExpr = &handle
		} else if binding == nil {
			if handle, ok := p.defaultStructInit(ty); ok {
				initExpr = &handle
			}
		}
		global = ir.GlobalVariable{
			Name:     dec.name,
			Space:    ir.SpacePrivate,
			Type:     ty,
			InitExpr: initExpr,
		}
		lookup.class = classOutput
		lookup.result = ir.FunctionResult{Type: ty}
		if binding != nil {
			lookup.result.Binding = &binding
		}
	default:
		space := extClass.space
		var access ir.StorageAccess
		if space == ir.SpaceStorage {
			access = storageAccess & dec.storageAccess()
		}
		global = ir.GlobalVariable{
			Name:     dec.name,
			Space:    space,
			Binding:  dec.resourceBinding(),
			Type:     ty,
			InitExpr: init,
			Access:   access,
		}
		lookup.class = classGlobal
	}

	lookup.handle = ir.GlobalVariableHandle(len(p.module.GlobalVariables))
	p.module.GlobalVariables = append(p.module.GlobalVariables, global)
	p.lookupVariable[id] = lookup
	return nil
}

// variableClass maps a variable's pointee type and storage class to its IR
// home, with the storage access of a BufferBlock struct applied.
func (p *parser) variableClass(ty ir.TypeHandle, storageClass uint32) (extendedClass, ir.StorageAccess, error) {
	if access, ok := p.lookupStorageBufferTypes[ty]; ok {
		return extendedClass{class: classGlobal, space: ir.SpaceStorage}, access, nil
	}
	extClass, err := mapStorageClass(storageClass)
	if err != nil {
		return extendedClass{}, 0, err
	}
	if extClass.class == classGlobal && extClass.space == ir.SpaceStorage {
		return extClass, ir.StorageAccessLoad | ir.StorageAccessStore, nil
	}
	return extClass, 0, nil
}

// unsignedBuiltinType returns the unsigned type an input built-in is read as:
// SPIR-V declares the integer ones signed, the IR reads them unsigned. It
// returns false when the built-in keeps its type.
func (p *parser) unsignedBuiltinType(builtin ir.BuiltinValue, ty ir.TypeHandle) (ir.TypeHandle, bool) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	var unsigned ir.TypeInner
	switch builtin {
	case ir.BuiltinBaseInstance, ir.BuiltinBaseVertex, ir.BuiltinInstanceIndex,
		ir.BuiltinSampleIndex, ir.BuiltinVertexIndex, ir.BuiltinPrimitiveIndex,
		ir.BuiltinLocalInvocationIndex:
		unsigned = u32
	case ir.BuiltinGlobalInvocationID, ir.BuiltinLocalInvocationID,
		ir.BuiltinWorkGroupID, ir.BuiltinWorkGroupSize:
		unsigned = ir.VectorType{Size: ir.Vec3, Scalar: u32}
	case ir.BuiltinBarycentricNoPerspective:
		unsigned = ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	default:
		return 0, false
	}
	kind, ok := scalarKind(p.module.Types[ty].Inner)
	if !ok || kind != ir.ScalarSint {
		return 0, false
	}
	return p.insertType("", unsigned), true
}

// defaultStructInit builds the default value of an output struct: each member
// gets its built-in's default, or a zero value.
func (p *parser) defaultStructInit(ty ir.TypeHandle) (ir.ExpressionHandle, bool) {
	structType, ok := p.module.Types[ty].Inner.(ir.StructType)
	if !ok {
		return 0, false
	}
	components := make([]ir.ExpressionHandle, len(structType.Members))
	for i, member := range structType.Members {
		var builtin *ir.BuiltinValue
		if member.Binding != nil {
			if binding, ok := (*member.Binding).(ir.BuiltinBinding); ok {
				builtin = &binding.Builtin
			}
		}
		components[i] = p.generateDefaultBuiltIn(builtin, member.Type)
	}
	return p.appendGlobalExpression(ir.ExprCompose{Type: ty, Components: components}), true
}
