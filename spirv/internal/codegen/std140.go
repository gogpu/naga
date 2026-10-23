// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Two-row matrices in uniform buffers.
//
// Vulkan lays out a uniform buffer with its extended (std140) rules, which put
// the columns of a matrix at least 16 bytes apart, where WGSL puts those of a
// matCx2 8 bytes apart (4 for f16), also in the uniform address space: a
// MatrixStride of 8 in a Uniform block is only valid with
// uniformBufferStandardLayout. Like Rust naga (back/spv/mod.rs), the backend
// declares each type a uniform buffer holds that contains a two-row matrix as
// a std140-compatible type: a matrix member of a struct becomes one vector
// member per column, a matrix elsewhere a struct of its columns, and structs
// and arrays containing them compatible types of their own. Uniform buffers
// are read-only, so only loads convert: a load of such a type goes through a
// function converting it to the regular type, a matrix member is loaded
// column by column, a column of a matrix member is addressed in its struct,
// and a column selected by a dynamic index is taken from the loaded matrix by
// a function switching on the index.

// std140Type is the std140-compatible declaration of a type.
type std140Type struct {
	id uint32
	// memberIndices maps the members of a struct to their indices in the
	// declaration, in which a matrix member has one per column.
	memberIndices []uint32
}

func twoRowMatrix(inner ir.TypeInner) (ir.MatrixType, bool) {
	m, ok := inner.(ir.MatrixType)
	return m, ok && m.Rows == ir.Vec2
}

// declareStd140Types declares the std140-compatible types of the uniform
// buffers of the module.
func (b *Backend) declareStd140Types() error {
	for _, global := range b.module.GlobalVariables {
		if global.Space == ir.SpaceUniform {
			if err := b.declareStd140(global.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

// declareStd140 declares the std140-compatible type of ty, when it needs
// one, after those of the types it holds; b.std140Types has it then.
func (b *Backend) declareStd140(ty ir.TypeHandle) error {
	if _, ok := b.std140Types[ty]; ok {
		return nil
	}
	var info *std140Type
	switch t := b.module.Types[ty].Inner.(type) {
	case ir.MatrixType:
		if t.Rows != ir.Vec2 {
			return nil
		}
		columnTypeID, err := b.emitInlineType(ir.VectorType{Size: t.Rows, Scalar: t.Scalar})
		if err != nil {
			return err
		}
		members := make([]uint32, t.Columns)
		for c := range members {
			members[c] = columnTypeID
		}
		id := b.builder.AddTypeStruct(members...)
		for c := range members {
			b.builder.AddMemberDecorate(id, uint32(c), DecorationOffset, uint32(c)*2*uint32(t.Scalar.Width))
			if b.options.Debug {
				b.builder.AddMemberName(id, uint32(c), fmt.Sprintf("col%d", c))
			}
		}
		info = &std140Type{id: id}
	case ir.ArrayType:
		if err := b.declareStd140(t.Base); err != nil {
			return err
		}
		base := b.std140Types[t.Base]
		if base == nil {
			return nil
		}
		if t.Size.Constant == nil {
			return fmt.Errorf("a uniform buffer holds a runtime-sized array")
		}
		u32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		if err != nil {
			return err
		}
		id := b.builder.AddTypeArray(base.id, b.builder.AddConstant(u32TypeID, *t.Size.Constant))
		b.builder.AddDecorate(id, DecorationArrayStride, t.Stride)
		info = &std140Type{id: id}
	case ir.StructType:
		needed := false
		for _, m := range t.Members {
			if _, ok := twoRowMatrix(b.module.Types[m.Type].Inner); ok {
				needed = true
				continue
			}
			if err := b.declareStd140(m.Type); err != nil {
				return err
			}
			needed = needed || b.std140Types[m.Type] != nil
		}
		if !needed {
			return nil
		}
		info = &std140Type{}
		var members []uint32
		for _, m := range t.Members {
			info.memberIndices = append(info.memberIndices, uint32(len(members)))
			if mat, ok := twoRowMatrix(b.module.Types[m.Type].Inner); ok {
				columnTypeID, err := b.emitInlineType(ir.VectorType{Size: mat.Rows, Scalar: mat.Scalar})
				if err != nil {
					return err
				}
				for range mat.Columns {
					members = append(members, columnTypeID)
				}
			} else if member := b.std140Types[m.Type]; member != nil {
				members = append(members, member.id)
			} else {
				memberTypeID, err := b.emitType(m.Type)
				if err != nil {
					return err
				}
				members = append(members, memberTypeID)
			}
		}
		info.id = b.builder.AddTypeStruct(members...)
		for i, m := range t.Members {
			index := info.memberIndices[i]
			if mat, ok := twoRowMatrix(b.module.Types[m.Type].Inner); ok {
				for c := range uint32(mat.Columns) {
					b.builder.AddMemberDecorate(info.id, index+c, DecorationOffset, m.Offset+c*2*uint32(mat.Scalar.Width))
					if b.options.Debug && m.Name != "" {
						b.builder.AddMemberName(info.id, index+c, fmt.Sprintf("%s_col%d", m.Name, c))
					}
				}
			} else if b.std140Types[m.Type] != nil {
				b.builder.AddMemberDecorate(info.id, index, DecorationOffset, m.Offset)
				if b.options.Debug && m.Name != "" {
					b.builder.AddMemberName(info.id, index, m.Name)
				}
			} else {
				b.decorateStructMember(info.id, index, m)
			}
		}
	default:
		return nil
	}
	if b.options.Debug {
		b.builder.AddName(info.id, "std140_"+b.typeDebugName(ty))
	}
	b.std140Types[ty] = info
	return nil
}

// typeDebugName names ty as Rust naga's debug format does: a struct by its
// name, any other type in WGSL syntax.
func (b *Backend) typeDebugName(ty ir.TypeHandle) string {
	if _, ok := b.module.Types[ty].Inner.(ir.StructType); ok {
		return b.module.Types[ty].Name
	}
	return b.typeInnerDebugName(b.module.Types[ty].Inner)
}

func (b *Backend) typeInnerDebugName(inner ir.TypeInner) string {
	scalar := func(s ir.ScalarType) string {
		switch s.Kind {
		case ir.ScalarFloat:
			return fmt.Sprintf("f%d", 8*int(s.Width))
		case ir.ScalarSint:
			return fmt.Sprintf("i%d", 8*int(s.Width))
		case ir.ScalarUint:
			return fmt.Sprintf("u%d", 8*int(s.Width))
		case ir.ScalarBool:
			return "bool"
		}
		return "?"
	}
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalar(t)
	case ir.VectorType:
		return fmt.Sprintf("vec%d<%s>", t.Size, scalar(t.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("mat%dx%d<%s>", t.Columns, t.Rows, scalar(t.Scalar))
	case ir.ArrayType:
		if t.Size.Constant == nil {
			return fmt.Sprintf("array<%s>", b.typeDebugName(t.Base))
		}
		return fmt.Sprintf("array<%s, %d>", b.typeDebugName(t.Base), *t.Size.Constant)
	}
	return fmt.Sprintf("%T", inner)
}

// std140DataTypeID returns the type of the data of a global inside its wrapper:
// the std140-compatible type of a uniform buffer that has one.
func (b *Backend) std140DataTypeID(global *ir.GlobalVariable) (uint32, error) {
	if global.Space == ir.SpaceUniform {
		if info := b.std140Types[global.Type]; info != nil {
			return info.id, nil
		}
	}
	return b.emitType(global.Type)
}

// emitStd140Wrappers writes the functions the uniform buffer access expr
// needs, in the order Rust naga's write_wrapped_functions does.
func (b *Backend) emitStd140Wrappers(fn *ir.Function, expr ir.ExpressionKind) error {
	switch k := expr.(type) {
	case ir.ExprLoad:
		if ty, ok := uniformPointee(b.module, fn, k.Pointer); ok && b.std140Types[ty] != nil {
			_, err := b.std140Convert(ty)
			return err
		}
	case ir.ExprAccess:
		ty, ok := uniformPointee(b.module, fn, k.Base)
		if !ok {
			return nil
		}
		if _, isMatrix := twoRowMatrix(b.module.Types[ty].Inner); !isMatrix {
			return nil
		}
		if _, err := b.matCx2GetColumn(ty); err != nil {
			return err
		}
		if !isUniformMatCx2Member(b.module, fn, k.Base) {
			_, err := b.std140Convert(ty)
			return err
		}
	}
	return nil
}

// std140Convert returns the function converting a value of the
// std140-compatible type of ty to ty, which it writes the first time.
func (b *Backend) std140Convert(ty ir.TypeHandle) (uint32, error) {
	if id, ok := b.std140ConvertIDs[ty]; ok {
		return id, nil
	}
	info := b.std140Types[ty]
	if info == nil {
		return 0, fmt.Errorf("type %d has no std140-compatible declaration", ty)
	}
	functionID := b.builder.AllocID()
	b.std140ConvertIDs[ty] = functionID
	if b.options.Debug {
		b.builder.AddName(functionID, b.typeDebugName(ty)+"_from_std140")
	}
	returnTypeID, err := b.emitType(ty)
	if err != nil {
		return 0, err
	}
	var fb FunctionBuilder
	fb.Signature = Instruction{Opcode: OpFunction, Words: []uint32{returnTypeID, functionID, uint32(FunctionControlNone),
		b.getFuncType(returnTypeID, []uint32{info.id})}}
	paramID := b.builder.AllocID()
	fb.Parameters = []Instruction{{Opcode: OpFunctionParameter, Words: []uint32{info.id, paramID}}}
	block := NewBlock(b.builder.AllocID())
	extract := func(id, typeID, index uint32) {
		block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{typeID, id, paramID, index}})
	}
	call := func(id, typeID, function, arg uint32) {
		block.Push(Instruction{Opcode: OpFunctionCall, Words: []uint32{typeID, id, function, arg}})
	}
	construct := func(typeID uint32, constituents []uint32) uint32 {
		id := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpCompositeConstruct, Words: append([]uint32{typeID, id}, constituents...)})
		return id
	}
	columns := func(mat ir.MatrixType, first uint32) ([]uint32, error) {
		columnTypeID, err := b.emitInlineType(ir.VectorType{Size: mat.Rows, Scalar: mat.Scalar})
		if err != nil {
			return nil, err
		}
		ids := make([]uint32, mat.Columns)
		for c := range ids {
			ids[c] = b.builder.AllocID()
			extract(ids[c], columnTypeID, first+uint32(c))
		}
		return ids, nil
	}

	var resultID uint32
	switch t := b.module.Types[ty].Inner.(type) {
	case ir.MatrixType:
		ids, err := columns(t, 0)
		if err != nil {
			return 0, err
		}
		resultID = construct(returnTypeID, ids)
	case ir.ArrayType:
		elementConvert, err := b.std140Convert(t.Base)
		if err != nil {
			return 0, err
		}
		elementTypeID, err := b.emitType(t.Base)
		if err != nil {
			return 0, err
		}
		elements := make([]uint32, *t.Size.Constant)
		for i := range elements {
			std140Element := b.builder.AllocID()
			extract(std140Element, b.std140Types[t.Base].id, uint32(i))
			elements[i] = b.builder.AllocID()
			call(elements[i], elementTypeID, elementConvert, std140Element)
		}
		resultID = construct(returnTypeID, elements)
	case ir.StructType:
		members := make([]uint32, len(t.Members))
		next := uint32(0)
		for i, m := range t.Members {
			members[i] = b.builder.AllocID()
			memberTypeID, err := b.emitType(m.Type)
			if err != nil {
				return 0, err
			}
			if mat, ok := twoRowMatrix(b.module.Types[m.Type].Inner); ok {
				ids, err := columns(mat, next)
				if err != nil {
					return 0, err
				}
				next += uint32(mat.Columns)
				block.Push(Instruction{Opcode: OpCompositeConstruct, Words: append([]uint32{memberTypeID, members[i]}, ids...)})
				continue
			}
			if member := b.std140Types[m.Type]; member != nil {
				memberConvert, err := b.std140Convert(m.Type)
				if err != nil {
					return 0, err
				}
				std140Member := b.builder.AllocID()
				extract(std140Member, member.id, next)
				call(members[i], memberTypeID, memberConvert, std140Member)
			} else {
				extract(members[i], memberTypeID, next)
			}
			next++
		}
		resultID = construct(returnTypeID, members)
	default:
		return 0, fmt.Errorf("type %T has no std140-compatible declaration", t)
	}
	fb.Consume(block, Instruction{Opcode: OpReturnValue, Words: []uint32{resultID}})
	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	return functionID, nil
}

// matCx2GetColumn returns the function returning the column of a two-row
// matrix of type ty its u32 argument selects, which it writes the first time.
// An index out of bounds is handled by the Index bounds check policy, as in
// Rust naga: the last column for Restrict, zero for ReadZeroSkipWrite.
func (b *Backend) matCx2GetColumn(ty ir.TypeHandle) (uint32, error) {
	if id, ok := b.matCx2ColumnIDs[ty]; ok {
		return id, nil
	}
	mat, ok := twoRowMatrix(b.module.Types[ty].Inner)
	if !ok {
		return 0, fmt.Errorf("type %d is not a two-row matrix", ty)
	}
	functionID := b.builder.AllocID()
	b.matCx2ColumnIDs[ty] = functionID
	if b.options.Debug {
		b.builder.AddName(functionID, b.typeDebugName(ty)+"_get_column")
	}
	matrixTypeID, err := b.emitType(ty)
	if err != nil {
		return 0, err
	}
	u32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	if err != nil {
		return 0, err
	}
	columnTypeID, err := b.emitInlineType(ir.VectorType{Size: mat.Rows, Scalar: mat.Scalar})
	if err != nil {
		return 0, err
	}
	matrixParam, indexParam := b.builder.AllocID(), b.builder.AllocID()
	var fb FunctionBuilder
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{matrixTypeID, matrixParam}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32TypeID, indexParam}},
	}
	fb.Signature = Instruction{Opcode: OpFunction, Words: []uint32{columnTypeID, functionID, uint32(FunctionControlNone),
		b.getFuncType(columnTypeID, []uint32{matrixTypeID, u32TypeID})}}

	header := NewBlock(b.builder.AllocID())
	merge := b.builder.AllocID()
	header.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{merge, uint32(SelectionControlNone)}})
	cases := make([]uint32, mat.Columns)
	for c := range cases {
		cases[c] = b.builder.AllocID()
	}
	policy := b.options.BoundsCheckPolicies.Index
	var defaultLabel uint32
	switch policy {
	case BoundsCheckRestrict:
		defaultLabel = cases[len(cases)-1]
	case BoundsCheckReadZeroSkipWrite:
		defaultLabel = merge
	default:
		defaultLabel = b.builder.AllocID()
	}
	words := []uint32{indexParam, defaultLabel}
	for c, label := range cases {
		words = append(words, uint32(c), label)
	}
	fb.Consume(header, Instruction{Opcode: OpSwitch, Words: words})

	var incoming []uint32
	for c, label := range cases {
		block := NewBlock(label)
		column := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{columnTypeID, column, matrixParam, uint32(c)}})
		fb.Consume(block, makeBranchInstruction(merge))
		incoming = append(incoming, column, label)
	}
	switch policy {
	case BoundsCheckRestrict:
	case BoundsCheckReadZeroSkipWrite:
		incoming = append(incoming, b.constantNull(columnTypeID), header.LabelID)
	default:
		fb.Consume(NewBlock(defaultLabel), Instruction{Opcode: OpUnreachable})
	}
	mergeBlock := NewBlock(merge)
	result := b.builder.AllocID()
	mergeBlock.Push(Instruction{Opcode: OpPhi, Words: append([]uint32{columnTypeID, result}, incoming...)})
	fb.Consume(mergeBlock, Instruction{Opcode: OpReturnValue, Words: []uint32{result}})
	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	return functionID, nil
}

// uniformPointee returns the type the pointer expression h points to in the
// uniform address space.
func uniformPointee(module *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (ir.TypeHandle, bool) {
	res, err := ir.ResolveExpressionType(module, fn, h)
	if err != nil {
		return 0, false
	}
	p, ok := typeResolutionInner(module, res).(ir.PointerType)
	return p.Base, ok && p.Space == ir.SpaceUniform
}

// isUniformMatCx2Member reports whether h points to a two-row matrix member
// of a struct in a uniform buffer, whose columns are members of the struct's
// std140-compatible type.
func isUniformMatCx2Member(module *ir.Module, fn *ir.Function, h ir.ExpressionHandle) bool {
	ty, ok := uniformPointee(module, fn, h)
	if !ok {
		return false
	}
	if _, ok := twoRowMatrix(module.Types[ty].Inner); !ok {
		return false
	}
	access, ok := fn.Expressions[h].Kind.(ir.ExprAccessIndex)
	if !ok {
		return false
	}
	res, err := ir.ResolveExpressionType(module, fn, access.Base)
	if err != nil {
		return false
	}
	p, ok := typeResolutionInner(module, res).(ir.PointerType)
	if !ok {
		return false
	}
	_, ok = module.Types[p.Base].Inner.(ir.StructType)
	return ok
}

// std140Struct returns the std140-compatible type of the struct base points
// to in a uniform buffer.
func (e *ExpressionEmitter) std140Struct(base ir.ExpressionHandle) *std140Type {
	ty, ok := uniformPointee(e.backend.module, e.function, base)
	if !ok {
		return nil
	}
	return e.backend.std140Types[ty]
}

// loadUniform loads what pointer points to in a uniform buffer when the load
// has to go through a std140-compatible type (Rust naga's write_checked_load):
// a column selected by a dynamic index, or a component of one; a matrix
// member of a struct; a value of a type with a std140-compatible type. ok is
// false for any other pointer.
func (e *ExpressionEmitter) loadUniform(pointer ir.ExpressionHandle) (uint32, bool, error) {
	if id, ok, err := e.loadDynamicMatCx2Column(pointer); ok || err != nil {
		return id, ok, err
	}
	if id, ok, err := e.loadMatCx2Member(pointer); ok || err != nil {
		return id, ok, err
	}
	ty, ok := uniformPointee(e.backend.module, e.function, pointer)
	if !ok {
		return 0, false, nil
	}
	info := e.backend.std140Types[ty]
	if info == nil {
		return 0, false, nil
	}
	typeID, err := e.backend.emitType(ty)
	if err != nil {
		return 0, true, err
	}
	p, err := e.writePointerAccess(pointer)
	if err != nil {
		return 0, true, err
	}
	value, err := e.withAccess(p, info.id, func(ptr uint32) (uint32, error) {
		return e.backend.builder.AddLoad(info.id, ptr), nil
	})
	if err != nil {
		return 0, true, err
	}
	convert, err := e.backend.std140Convert(ty)
	if err != nil {
		return 0, true, err
	}
	return e.emitFunctionCall(typeID, convert, value), true, nil
}

// loadDynamicMatCx2Column loads the column of a two-row matrix in a uniform
// buffer that pointer selects by a dynamic index, or a component of it: the
// columns of the std140-compatible type are struct members, which an access
// chain can't select by a dynamic index, so the matrix is loaded and the
// column taken from it.
func (e *ExpressionEmitter) loadDynamicMatCx2Column(pointer ir.ExpressionHandle) (uint32, bool, error) {
	module := e.backend.module
	var pointerType ir.TypeInner
	if res, err := ir.ResolveExpressionType(module, e.function, pointer); err == nil {
		pointerType = typeResolutionInner(module, res)
	}
	column := pointer
	var component *guardedIndex
	var componentBase ir.ExpressionHandle
	switch p := pointerType.(type) {
	case ir.ValuePointerType:
		if p.Size == nil {
			var index guardedIndex
			switch k := e.function.Expressions[pointer].Kind.(type) {
			case ir.ExprAccess:
				column, index = k.Base, e.guardIndex(k.Index)
			case ir.ExprAccessIndex:
				column, index = k.Base, guardedIndex{known: true, value: k.Index}
			default:
				return 0, false, nil
			}
			component, componentBase = &index, column
		}
	case ir.PointerType:
		if _, ok := module.Types[p.Base].Inner.(ir.VectorType); !ok {
			return 0, false, nil
		}
	default:
		return 0, false, nil
	}
	access, ok := e.function.Expressions[column].Kind.(ir.ExprAccess)
	if !ok {
		return 0, false, nil
	}
	ty, ok := uniformPointee(module, e.function, access.Base)
	if !ok {
		return 0, false, nil
	}
	mat, ok := twoRowMatrix(module.Types[ty].Inner)
	if !ok {
		return 0, false, nil
	}
	columnTypeID, err := e.backend.emitInlineType(ir.VectorType{Size: mat.Rows, Scalar: mat.Scalar})
	if err != nil {
		return 0, true, err
	}
	componentTypeID, err := e.backend.emitScalarType(mat.Scalar)
	if err != nil {
		return 0, true, err
	}
	getColumn, err := e.backend.matCx2GetColumn(ty)
	if err != nil {
		return 0, true, err
	}
	matrix, err := e.emitLoad(ir.ExprLoad{Pointer: access.Base})
	if err != nil {
		return 0, true, err
	}
	index, err := e.emitExpression(access.Index)
	if err != nil {
		return 0, true, err
	}
	indexType, err := ir.ResolveExpressionType(module, e.function, access.Index)
	if err != nil {
		return 0, true, err
	}
	if s, ok := typeResolutionInner(module, indexType).(ir.ScalarType); ok && s.Kind == ir.ScalarSint {
		u32TypeID, err := e.u32Type()
		if err != nil {
			return 0, true, err
		}
		index = e.emitConversionOp(OpBitcast, u32TypeID, index)
	}
	id := e.emitFunctionCall(columnTypeID, getColumn, matrix, index)
	if component == nil {
		return id, true, nil
	}
	var componentIndex uint32
	if !component.known {
		if componentIndex, err = e.emitExpression(component.expr); err != nil {
			return 0, true, err
		}
	}
	value, err := e.vectorElement(componentTypeID, id, componentBase, *component, componentIndex)
	return value, true, err
}

// loadMatCx2Member loads the two-row matrix member of a struct in a uniform
// buffer pointer points to, column by column from the struct's
// std140-compatible type.
func (e *ExpressionEmitter) loadMatCx2Member(pointer ir.ExpressionHandle) (uint32, bool, error) {
	module := e.backend.module
	if !isUniformMatCx2Member(module, e.function, pointer) {
		return 0, false, nil
	}
	ty, _ := uniformPointee(module, e.function, pointer)
	mat, _ := twoRowMatrix(module.Types[ty].Inner)
	access := e.function.Expressions[pointer].Kind.(ir.ExprAccessIndex)
	info := e.std140Struct(access.Base)
	if info == nil {
		return 0, true, fmt.Errorf("uniform struct with a two-row matrix member has no std140-compatible type")
	}
	matrixTypeID, err := e.backend.emitType(ty)
	if err != nil {
		return 0, true, err
	}
	columnTypeID, err := e.backend.emitInlineType(ir.VectorType{Size: mat.Rows, Scalar: mat.Scalar})
	if err != nil {
		return 0, true, err
	}
	columnPointerTypeID := e.backend.emitPointerType(StorageClassUniform, columnTypeID)
	indices := make([]uint32, mat.Columns)
	for c := range indices {
		if indices[c], err = e.u32Constant(info.memberIndices[access.Index] + uint32(c)); err != nil {
			return 0, true, err
		}
	}
	p, err := e.writePointerAccess(access.Base)
	if err != nil {
		return 0, true, err
	}
	value, err := e.withAccess(p, matrixTypeID, func(ptr uint32) (uint32, error) {
		columns := make([]uint32, len(indices))
		for c, index := range indices {
			columns[c] = e.backend.builder.AddLoad(columnTypeID, e.backend.builder.AddAccessChain(columnPointerTypeID, ptr, index))
		}
		return e.backend.builder.AddCompositeConstruct(matrixTypeID, columns...), nil
	})
	return value, true, err
}

// emitFunctionCall calls function with args.
func (e *ExpressionEmitter) emitFunctionCall(resultTypeID, function uint32, args ...uint32) uint32 {
	id := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultTypeID)
	ib.AddWord(id)
	ib.AddWord(function)
	for _, arg := range args {
		ib.AddWord(arg)
	}
	e.backend.builder.funcAppend(ib.Build(OpFunctionCall))
	return id
}
