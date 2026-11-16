package front

// This file synthesizes the default uniform block of the GL 3.30 core and
// ES 3.00 dialects (ADR-013). GLSL outside the Vulkan profile lets a shader
// declare uniforms outside any block, and the IR has no resource for such a
// declaration: every non-opaque resource is a descriptor (group, binding).
// Every plain uniform of a module therefore becomes one member of one generated
// uniform block, `gl_DefaultUniformBlock`, in source declaration order with
// std140 offsets — the layout a GL driver's own lowering of the default block
// uses, so the block can be filled from the reported offsets.
//
// The block is built as the declarations are lowered, not in a pass after them:
// a function body may read a plain uniform declared above it, and lowering a
// reference needs the member's type in the module (blockSelectMemberLoads). A
// later declaration only appends a member; the indices of the earlier ones do
// not move.
//
// Vulkan GLSL keeps the refusal: its profile has no default uniform block and
// glslang refuses the declaration too.

import (
	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// defaultBlockName names the synthesized block in the module and is the key a
// caller puts in ParseOptions.ResourceBindings to place it. The GLSL
// specification reserves the gl_ prefix for the implementation, so no valid
// shader identifier can collide with it (glslang: "'gl_DefaultUniformBlock' :
// identifiers starting with "gl_" are reserved").
const defaultBlockName = frontend.DefaultBlockName

// defaultBlock is the state of the synthesis: the block's type, global and
// binding, its layout so far, and the members' names.
type defaultBlock struct {
	// binding is where the block is bound: the slot
	// ParseOptions.ResourceBindings gives the block's name, else the next free
	// slot of group 0 at the first plain declaration (bindings.go). That
	// declaration claims it, so the one-counter model of design §11 places the
	// block in source order among the module's other resources.
	binding ir.ResourceBinding
	// ty is the block's type in the module: a struct whose members grow with
	// each plain uniform appended (registry.Replace keeps the handle).
	ty ir.TypeHandle
	// global is the block's uniform global in the module.
	global ir.GlobalVariableHandle
	// offset is the end of the last member, the start of the next one once it
	// is rounded up to that member's alignment.
	offset uint32
	// align is the block's alignment: the largest member alignment.
	align uint32
	// report is the block as ParseInfo reports it, kept in step with the layout.
	report frontend.DefaultBlockInfo
}

// addPlainUniform records one plain uniform declarator of a GL 3.30 core or
// ES 3.00 source as a member of the synthesized block, creating the block at
// the first one, and makes the name resolve in c and in every later function
// body.
func (f *Frontend) addPlainUniform(c *Context, name string, sym *glslSymbol, ty ir.TypeHandle, span ir.Span) *frontError {
	if name == defaultBlockName {
		return errSemantic(span, "%q : the name is reserved for the block a GL 3.30 core or ES 3.00 source's plain uniforms are collected into",
			defaultBlockName)
	}
	// A plain uniform is a module-scope name like any other: a reserved or
	// repeated one is refused here (module_names.go), and the module-wide table
	// is what keeps the struct's members distinct — the MSL and HLSL backends
	// write the struct as it is, so a duplicate would be broken source.
	if err := f.declareModuleName(name, span); err != nil {
		return err
	}
	block := f.createDefaultBlock(span)

	// std140 places the member at its alignment, which rounds a vec3, a
	// matCxR column and an array up to 16 bytes (calculateOffset, offset.go).
	info := f.calculateOffset(ty, span, structLayoutStd140)
	offset := roundUp(info.align, block.offset)
	block.align = max(info.align, block.align)
	block.offset = offset + info.span

	members := append(f.blockMembers(block), ir.StructMember{Name: name, Type: info.ty, Offset: offset})
	structType := ir.StructType{Members: members, Span: roundUp(block.align, block.offset)}
	if !f.registry.Replace(block.ty, defaultBlockName, structType) {
		return errInternal("the default uniform block's type cannot grow", span)
	}
	f.mod.Types = f.registry.GetTypes()

	block.report.Size = structType.Span
	block.report.Members = append(block.report.Members, frontend.DefaultBlockMember{
		Name:   name,
		Offset: offset,
		Size:   info.span,
	})

	if sym != nil {
		lookup := globalLookup{
			kind:     lookupBlockSelect,
			variable: block.global,
			index:    uint32(len(members) - 1),
			mutable:  true,
			entryArg: -1,
		}
		if err := c.addGlobal(namedLookup{name: name, sym: sym, lookup: lookup}); err != nil {
			return err
		}
		f.globalVars = append(f.globalVars, namedLookup{name: name, sym: sym, lookup: lookup})
	}
	return nil
}

// createDefaultBlock returns the module's synthesized block, creating it on the
// first plain uniform declaration: its binding, its type and its global. The
// type starts empty and grows one member at a time.
func (f *Frontend) createDefaultBlock(span ir.Span) *defaultBlock {
	if f.defaultBlock != nil {
		return f.defaultBlock
	}
	binding, reason := f.bindings.forResource(defaultBlockName, 0)
	if reason != "" {
		f.addError(errSemantic(span, "%s", reason))
	}
	block := &defaultBlock{
		binding: binding,
		align:   1,
	}
	block.ty = f.registerNamedType(defaultBlockName, ir.StructType{})
	block.global = ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
	f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
		Name:    defaultBlockName,
		Space:   ir.SpaceUniform,
		Binding: &block.binding,
		Type:    block.ty,
	})
	block.report = frontend.DefaultBlockInfo{
		Name:    defaultBlockName,
		Binding: binding,
	}
	f.defaultBlock = block
	f.defaultBlockReport = &block.report
	return block
}

// blockMembers returns the block's members as they are in the module now: the
// struct the type registry holds, which addPlainUniform replaces with one more
// member.
func (f *Frontend) blockMembers(block *defaultBlock) []ir.StructMember {
	structType, ok := f.mod.Types[block.ty].Inner.(ir.StructType)
	if !ok {
		return nil
	}
	// A fresh slice: the registry's struct is the value the caller sees, and
	// appending to it must not rewrite what is already registered.
	members := make([]ir.StructMember, len(structType.Members), len(structType.Members)+1)
	copy(members, structType.Members)
	return members
}

// lowerPlainUniformList records the declarators of one plain uniform
// declaration as members of the synthesized default block. A member has no
// initializer in the IR — the GL API supplies a uniform's value — so a
// declarator's initializer is not lowered; glslang accepts it in these dialects
// and gives it no effect either.
func (f *Frontend) lowerPlainUniformList(c *Context, used *layoutUse, d *declVarList, base ir.TypeHandle) *frontError {
	// A plain uniform takes no layout qualifier. GL 3.30 core and ES 3.00 have
	// no layout(binding), and packing is a block's property, not a variable's:
	// glslang, "'layout' : cannot specify packing on a variable declaration".
	if entry, ok := used.take(qualifierKey{kind: layoutStructure}); ok {
		return errMessage(entry.span, "'layout' : cannot specify packing on a variable declaration")
	}
	f.rejectResourceLayoutQualifiers(used)
	for i := range d.Declarators {
		decl := &d.Declarators[i]
		ty, err := f.resolveTypeArrays(base, decl.Arrays)
		if err != nil {
			return err
		}
		if err := f.refuseUnsizedPlainUniform(decl.Name, ty, decl.Span); err != nil {
			return err
		}
		if err := f.addPlainUniform(c, decl.Name, decl.Sym, ty, decl.Span); err != nil {
			return err
		}
	}
	used.unusedErrors()
	return nil
}

// refuseUnsizedPlainUniform refuses a plain uniform whose type is or contains an
// array of unknown length: the generated block's member needs a fixed std140
// stride, which only a known element count gives. glslang accepts the
// declaration (`uniform float w[];` compiles in 330 core), but the descriptor
// model has no length to give the buffer, so the refusal is recorded as a
// deviation with this message.
func (f *Frontend) refuseUnsizedPlainUniform(name string, ty ir.TypeHandle, span ir.Span) *frontError {
	for {
		arr, ok := f.typeInner(ty).(ir.ArrayType)
		if !ok {
			return nil
		}
		if arr.Size.Constant == nil {
			return errSemantic(span,
				"plain uniform %q has an array of unknown length: naga's descriptor model needs a fixed length to lay the default uniform block out",
				name)
		}
		ty = arr.Base
	}
}
