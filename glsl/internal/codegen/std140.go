// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Two-row matrices in uniform buffers.
//
// WGSL lays out the columns of a matCx2<f32> 8 bytes apart, with the
// alignment of a vec2, also in the uniform address space. A uniform block is
// std140, which rounds every column of a matrix up to 16 bytes, so a matCx2
// declared as such is read from the wrong offsets, and so is every member
// after it. Like Rust naga's SPIR-V backend (back/spv/mod.rs), the GLSL
// backend declares each type a uniform buffer holds that contains a two-row
// matrix as a std140-compatible type: a matrix member of a struct becomes one
// vec2 member per column, a matrix elsewhere a struct of its columns, and
// structs and arrays containing them compatible types of their own. Uniform
// buffers are read-only, so only loads convert: a pointer to such a matrix is
// written as the matrix rebuilt from its columns, and a load of a struct or
// array goes through a conversion function.

// std140Type is the std140-compatible declaration of a type.
type std140Type struct {
	name     string // compatible struct name; empty for an array
	convName string // conversion function to the regular type
}

// isStd140Matrix reports whether inner is a two-row f32 matrix, whose
// columns std140 places differently from WGSL.
func isStd140Matrix(inner ir.TypeInner) (ir.MatrixType, bool) {
	m, ok := inner.(ir.MatrixType)
	return m, ok && m.Rows == 2 && m.Scalar.Kind == ir.ScalarFloat && m.Scalar.Width == 4
}

// collectStd140Types finds the types the uniform buffers of the entry point
// hold that need a std140-compatible declaration and names them, operands
// before the types that contain them.
func (w *Writer) collectStd140Types() {
	w.std140Types = make(map[ir.TypeHandle]*std140Type)
	for handle, global := range w.module.GlobalVariables {
		if global.Space != ir.SpaceUniform {
			continue
		}
		if w.reachable != nil && !w.reachable.hasGlobal(ir.GlobalVariableHandle(handle)) {
			continue
		}
		if w.needsStd140(global.Type) {
			w.registerStd140(global.Type)
		}
	}
}

// needsStd140 reports whether ty is or contains a two-row matrix.
func (w *Writer) needsStd140(ty ir.TypeHandle) bool {
	switch t := w.module.Types[ty].Inner.(type) {
	case ir.MatrixType:
		_, needs := isStd140Matrix(t)
		return needs
	case ir.ArrayType:
		return w.needsStd140(t.Base)
	case ir.StructType:
		for _, m := range t.Members {
			if w.needsStd140(m.Type) {
				return true
			}
		}
	}
	return false
}

// registerStd140 names the std140-compatible declaration of ty, which
// needsStd140, after those of its operands. A matrix member of a struct is
// declared as its columns, so it needs no declaration of its own.
func (w *Writer) registerStd140(ty ir.TypeHandle) {
	if _, done := w.std140Types[ty]; done {
		return
	}
	var base string
	switch t := w.module.Types[ty].Inner.(type) {
	case ir.MatrixType:
		base = matrixToGLSL(t)
	case ir.ArrayType:
		w.registerStd140(t.Base)
		base = "array"
	case ir.StructType:
		for _, m := range t.Members {
			if _, isMatrix := isStd140Matrix(w.module.Types[m.Type].Inner); !isMatrix && w.needsStd140(m.Type) {
				w.registerStd140(m.Type)
			}
		}
		base = w.typeNames[ty]
	}
	info := &std140Type{convName: w.namer.call(base + "_from_std140")}
	if _, isArray := w.module.Types[ty].Inner.(ir.ArrayType); !isArray {
		info.name = w.namer.call(base + "_std140")
	}
	w.std140Types[ty] = info
	w.std140Order = append(w.std140Order, ty)
}

// std140TypeName returns the base name and array suffix that declare a value
// of ty in a uniform buffer.
func (w *Writer) std140TypeName(ty ir.TypeHandle) (string, string) {
	info, ok := w.std140Types[ty]
	if !ok {
		return w.getBaseTypeName(ty), w.getArraySuffix(ty)
	}
	if arr, isArray := w.module.Types[ty].Inner.(ir.ArrayType); isArray {
		base, suffix := w.std140TypeName(arr.Base)
		return base, fmt.Sprintf("[%d]", w.arraySize(arr)) + suffix
	}
	return info.name, ""
}

// arraySize returns the element count of a sized array.
func (w *Writer) arraySize(arr ir.ArrayType) uint32 {
	if arr.Size.Constant != nil {
		return *arr.Size.Constant
	}
	return 0
}

// std140MemberName returns the name of column col of the matrix member of a
// std140-compatible struct. A member name ending in a digit has an
// underscore after it, which isn't doubled: GLSL reserves names with two
// underscores in a row, and NVIDIA refuses them.
func (w *Writer) std140MemberName(structType ir.TypeHandle, member uint32, col int) string {
	name := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(structType), handle2: member}]
	return fmt.Sprintf("%s_%d", strings.TrimSuffix(name, "_"), col)
}

// writeStd140Types writes the std140-compatible structs and the functions
// converting them, and arrays of them, to the regular types.
func (w *Writer) writeStd140Types() {
	for _, ty := range w.std140Order {
		info := w.std140Types[ty]
		switch t := w.module.Types[ty].Inner.(type) {
		case ir.MatrixType:
			w.WriteLine("struct %s {", info.name)
			w.PushIndent()
			for c := 0; c < int(t.Columns); c++ {
				w.WriteLine("vec2 col%d;", c)
			}
			w.PopIndent()
			w.WriteLine("};")
		case ir.StructType:
			w.WriteLine("struct %s {", info.name)
			w.PushIndent()
			for i, m := range t.Members {
				if mat, isMatrix := isStd140Matrix(w.module.Types[m.Type].Inner); isMatrix {
					for c := 0; c < int(mat.Columns); c++ {
						w.WriteLine("vec2 %s;", w.std140MemberName(ty, uint32(i), c))
					}
					continue
				}
				base, suffix := w.std140TypeName(m.Type)
				name := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(ty), handle2: uint32(i)}]
				w.WriteLine("%s %s%s;", base, name, suffix)
			}
			w.PopIndent()
			w.WriteLine("};")
		}
	}
	for _, ty := range w.std140Order {
		w.writeStd140Conversion(ty)
	}
}

// writeStd140Conversion writes the function that converts a value of the
// std140-compatible declaration of ty to ty.
func (w *Writer) writeStd140Conversion(ty ir.TypeHandle) {
	info := w.std140Types[ty]
	base, suffix := w.std140TypeName(ty)
	retBase, retSuffix := w.getBaseTypeName(ty), w.getArraySuffix(ty)
	w.WriteLine("%s%s %s(%s v%s) {", retBase, retSuffix, info.convName, base, suffix)
	w.PushIndent()
	switch t := w.module.Types[ty].Inner.(type) {
	case ir.MatrixType:
		cols := make([]string, t.Columns)
		for c := range cols {
			cols[c] = fmt.Sprintf("v.col%d", c)
		}
		w.WriteLine("return %s(%s);", matrixToGLSL(t), joinStrings(cols, ", "))
	case ir.StructType:
		members := make([]string, len(t.Members))
		for i := range t.Members {
			members[i] = w.std140Member(ty, uint32(i), "v")
		}
		w.WriteLine("return %s(%s);", w.typeNames[ty], joinStrings(members, ", "))
	case ir.ArrayType:
		w.WriteLine("%s r%s;", retBase, retSuffix)
		w.WriteLine("for (int i = 0; i < %d; i++) {", w.arraySize(t))
		w.PushIndent()
		w.WriteLine("r[i] = %s;", w.std140Load(t.Base, "v[i]"))
		w.PopIndent()
		w.WriteLine("}")
		w.WriteLine("return r;")
	}
	w.PopIndent()
	w.WriteLine("}")
	w.WriteLine("")
}

// std140Member is the regular value of member index of the std140-compatible
// struct structType that base holds: a matrix member rebuilt from its
// columns, any other member converted when it needs to be.
func (w *Writer) std140Member(structType ir.TypeHandle, index uint32, base string) string {
	member := w.module.Types[structType].Inner.(ir.StructType).Members[index]
	if mat, isMatrix := isStd140Matrix(w.module.Types[member.Type].Inner); isMatrix {
		cols := make([]string, mat.Columns)
		for c := range cols {
			cols[c] = base + "." + w.std140MemberName(structType, index, c)
		}
		return fmt.Sprintf("%s(%s)", matrixToGLSL(mat), joinStrings(cols, ", "))
	}
	name := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(structType), handle2: index}]
	return w.std140Load(member.Type, base+"."+name)
}

// std140Load is the regular value of the std140-compatible lvalue expr of
// type ty: expr itself when ty needs no compatible declaration.
func (w *Writer) std140Load(ty ir.TypeHandle, expr string) string {
	if info, ok := w.std140Types[ty]; ok {
		return fmt.Sprintf("%s(%s)", info.convName, expr)
	}
	return expr
}

// uniformPointee returns the type a pointer expression into a uniform buffer
// points to.
func (w *Writer) uniformPointee(h ir.ExpressionHandle) (ir.TypeHandle, bool) {
	if w.currentFunction == nil || int(h) >= len(w.currentFunction.ExpressionTypes) {
		return 0, false
	}
	res := &w.currentFunction.ExpressionTypes[h]
	var inner ir.TypeInner = res.Value
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	}
	ptr, ok := inner.(ir.PointerType)
	if !ok || ptr.Space != ir.SpaceUniform {
		return 0, false
	}
	return ptr.Base, true
}
