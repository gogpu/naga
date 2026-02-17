// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"
	"math"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Type name constants.
const (
	hlslTypeFloat = "float"
	hlslTypeInt   = "int"
	hlslTypeUint  = "uint"
	hlslTypeBool  = "bool"
)

// writeTypes writes all struct type definitions.
// Non-struct types are written inline where needed.
func (w *Writer) writeTypes() error {
	for handle := range w.module.Types {
		typ := &w.module.Types[handle]
		st, ok := typ.Inner.(ir.StructType)
		if !ok {
			continue
		}

		if err := w.writeStructDefinition(ir.TypeHandle(handle), typ.Name, st); err != nil {
			return err
		}
	}
	return nil
}

// writeStructDefinition writes a struct type definition.
func (w *Writer) writeStructDefinition(handle ir.TypeHandle, _ string, st ir.StructType) error {
	structName := w.typeNames[handle]
	if structName == "" {
		structName = fmt.Sprintf("_struct_%d", handle)
	}

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	for memberIdx, member := range st.Members {
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}]
		if memberName == "" {
			memberName = fmt.Sprintf("member_%d", memberIdx)
		}

		// Get the type information for the member
		memberType, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)

		// Write member with type, name, and array suffix
		w.writeLine("%s %s%s;", memberType, memberName, arraySuffix)
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")
	return nil
}

// writeStructMemberWithSemantic writes a struct member with HLSL semantic.
// Used for entry point input/output structs (will be used in Phase 2).
//
//nolint:unused,unparam // Phase 2: error handling will be added for semantic validation
func (w *Writer) writeStructMemberWithSemantic(member ir.FunctionArgument, memberIdx int) error {
	memberType, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)
	memberName := member.Name
	if memberName == "" {
		memberName = fmt.Sprintf("arg_%d", memberIdx)
	}
	memberName = Escape(memberName)

	// Get semantic from binding
	var binding ir.Binding
	if member.Binding != nil {
		binding = *member.Binding
	}
	semantic := w.getSemanticFromBinding(binding, memberIdx)

	// Get interpolation modifiers
	interpMod := w.getInterpolationModifier(binding)
	if interpMod != "" {
		interpMod += " "
	}

	w.writeLine("%s%s %s%s : %s;", interpMod, memberType, memberName, arraySuffix, semantic)
	return nil
}

// getSemanticFromBinding returns the HLSL semantic for a binding.
func (w *Writer) getSemanticFromBinding(binding ir.Binding, idx int) string {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return BuiltInToSemantic(b.Builtin)
	case ir.LocationBinding:
		// Use TEXCOORD semantic with location as index
		return fmt.Sprintf("TEXCOORD%d", b.Location)
	default:
		// Default to TEXCOORD with index
		return fmt.Sprintf("TEXCOORD%d", idx)
	}
}

// getInterpolationModifier returns the HLSL interpolation modifier.
func (w *Writer) getInterpolationModifier(binding ir.Binding) string {
	loc, ok := binding.(ir.LocationBinding)
	if !ok || loc.Interpolation == nil {
		return ""
	}

	var modifiers []string

	// Interpolation kind
	if kindMod := InterpolationToHLSL(loc.Interpolation.Kind); kindMod != "" {
		modifiers = append(modifiers, kindMod)
	}

	// Sampling modifier
	if samplingMod := SamplingToHLSL(loc.Interpolation.Sampling); samplingMod != "" {
		modifiers = append(modifiers, samplingMod)
	}

	return strings.Join(modifiers, " ")
}

// getTypeName returns the HLSL type name for a type handle.
func (w *Writer) getTypeName(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("unknown_type_%d", handle)
	}

	typ := &w.module.Types[handle]
	return w.typeToHLSL(typ)
}

// getTypeNameWithArraySuffix returns the base type name and array suffix separately.
// HLSL arrays are written as `type name[size]`, not `type[size] name`.
func (w *Writer) getTypeNameWithArraySuffix(handle ir.TypeHandle) (typeName, arraySuffix string) {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("unknown_type_%d", handle), ""
	}

	typ := &w.module.Types[handle]
	return w.typeToHLSLWithArraySuffix(typ)
}

// typeToHLSL converts an IR type to HLSL type name.
func (w *Writer) typeToHLSL(typ *ir.Type) string {
	typeName, arraySuffix := w.typeToHLSLWithArraySuffix(typ)
	return typeName + arraySuffix
}

// typeToHLSLWithArraySuffix converts an IR type to HLSL type name,
// returning the base type and array suffix separately.
func (w *Writer) typeToHLSLWithArraySuffix(typ *ir.Type) (typeName, arraySuffix string) {
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return scalarTypeToHLSL(inner), ""

	case ir.VectorType:
		return vectorTypeToHLSL(inner), ""

	case ir.MatrixType:
		return matrixTypeToHLSL(inner), ""

	case ir.ArrayType:
		// HLSL arrays: type name[size]
		baseName, baseSuffix := w.getTypeNameWithArraySuffix(inner.Base)
		if inner.Size.Constant != nil {
			return baseName, baseSuffix + fmt.Sprintf("[%d]", *inner.Size.Constant)
		}
		return baseName, baseSuffix + "[]"

	case ir.StructType:
		// Use registered name or generate one
		if typ.Name != "" {
			return Escape(typ.Name), ""
		}
		// Look up in typeNames
		for h, name := range w.typeNames {
			if int(h) < len(w.module.Types) {
				if _, ok := w.module.Types[h].Inner.(ir.StructType); ok {
					if structsEqual(w.module.Types[h].Inner.(ir.StructType), inner) {
						return name, ""
					}
				}
			}
		}
		return "unknown_struct", ""

	case ir.PointerType:
		// HLSL doesn't have explicit pointers, use the base type
		return w.getTypeName(inner.Base), ""

	case ir.SamplerType:
		return samplerTypeToHLSL(inner.Comparison), ""

	case ir.ImageType:
		return w.imageTypeToHLSL(inner), ""

	case ir.AtomicType:
		// Atomics in HLSL use the underlying scalar type
		return scalarTypeToHLSL(inner.Scalar), ""

	default:
		return fmt.Sprintf("unknown_type_%T", inner), ""
	}
}

// scalarTypeToHLSL returns the HLSL type name for a scalar type.
func scalarTypeToHLSL(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarBool:
		return hlslTypeBool

	case ir.ScalarSint:
		switch s.Width {
		case 1, 2, 4:
			return hlslTypeInt
		case 8:
			return "int64_t"
		default:
			return hlslTypeInt
		}

	case ir.ScalarUint:
		switch s.Width {
		case 1, 2, 4:
			return hlslTypeUint
		case 8:
			return "uint64_t"
		default:
			return hlslTypeUint
		}

	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return "half"
		case 4:
			return hlslTypeFloat
		case 8:
			return "double"
		default:
			return hlslTypeFloat
		}

	default:
		return hlslTypeInt
	}
}

// vectorTypeToHLSL returns the HLSL type name for a vector type.
// HLSL uses TypeN syntax (e.g., float4, int3).
func vectorTypeToHLSL(v ir.VectorType) string {
	size := v.Size
	if size < 2 || size > 4 {
		size = 4 // Clamp to valid range
	}
	return fmt.Sprintf("%s%d", scalarTypeToHLSL(v.Scalar), size)
}

// matrixTypeToHLSL returns the HLSL type name for a matrix type.
// HLSL uses TypeRxC syntax (e.g., float4x4, half3x3).
// Note: HLSL matrices are row-major by default, and we transpose dimensions
// because the IR uses column-major storage (like GLSL).
func matrixTypeToHLSL(m ir.MatrixType) string {
	cols := m.Columns
	rows := m.Rows

	if cols < 2 || cols > 4 {
		cols = 4
	}
	if rows < 2 || rows > 4 {
		rows = 4
	}

	return fmt.Sprintf("%s%dx%d", scalarTypeToHLSL(m.Scalar), cols, rows)
}

// samplerTypeToHLSL returns the HLSL sampler type name.
func samplerTypeToHLSL(comparison bool) string {
	if comparison {
		return "SamplerComparisonState"
	}
	return "SamplerState"
}

// imageTypeToHLSL returns the HLSL texture type name.
func (w *Writer) imageTypeToHLSL(img ir.ImageType) string {
	var builder strings.Builder

	// Determine prefix based on image class (RW for storage, nothing for others)
	if img.Class == ir.ImageClassStorage {
		builder.WriteString("RW")
	}

	builder.WriteString("Texture")

	// Write dimension
	switch img.Dim {
	case ir.Dim1D:
		builder.WriteString("1D")
	case ir.Dim2D:
		builder.WriteString("2D")
	case ir.Dim3D:
		builder.WriteString("3D")
	case ir.DimCube:
		builder.WriteString("Cube")
	default:
		builder.WriteString("2D")
	}

	// Write multisampled suffix (before Array)
	if img.Multisampled {
		builder.WriteString("MS")
	}

	// Write array suffix (3D textures cannot be arrays)
	if img.Arrayed && img.Dim != ir.Dim3D {
		builder.WriteString("Array")
	}

	// Write template parameter based on image class
	switch img.Class {
	case ir.ImageClassDepth:
		// Depth textures use float
		builder.WriteString("<float>")
	case ir.ImageClassSampled:
		// Sampled textures use float4 by default
		builder.WriteString("<float4>")
	case ir.ImageClassStorage:
		// Storage textures also need format, default to float4
		builder.WriteString("<float4>")
	}

	return builder.String()
}

// writeBufferType writes HLSL buffer type declarations.
func (w *Writer) writeBufferType(typeName string, _ *ir.GlobalVariable, readOnly bool) string {
	if readOnly {
		return fmt.Sprintf("StructuredBuffer<%s>", typeName)
	}
	return fmt.Sprintf("RWStructuredBuffer<%s>", typeName)
}

// writeByteAddressBufferType returns the HLSL byte address buffer type.
func (w *Writer) writeByteAddressBufferType(readOnly bool) string {
	if readOnly {
		return "ByteAddressBuffer"
	}
	return "RWByteAddressBuffer"
}

// writeCBufferDeclaration writes a cbuffer declaration.
func (w *Writer) writeCBufferDeclaration(name, typeName string, binding *BindTarget) {
	if binding != nil {
		w.writeLine("cbuffer %s_cbuffer : register(b%d, space%d) {", name, binding.Register, binding.Space)
	} else {
		w.writeLine("cbuffer %s_cbuffer {", name)
	}
	w.pushIndent()
	w.writeLine("%s %s;", typeName, name)
	w.popIndent()
	w.writeLine("};")
}

// writeConstants writes constant definitions.
func (w *Writer) writeConstants() error {
	if len(w.module.Constants) == 0 {
		return nil
	}

	for handle := range w.module.Constants {
		constant := &w.module.Constants[handle]
		name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}]
		if name == "" {
			name = fmt.Sprintf("const_%d", handle)
		}
		typeName := w.getTypeName(constant.Type)
		value := w.writeConstantValue(constant)
		w.writeLine("static const %s %s = %s;", typeName, name, value)
	}
	w.writeLine("")
	return nil
}

// writeConstantValue returns the HLSL representation of a constant value.
func (w *Writer) writeConstantValue(constant *ir.Constant) string {
	switch v := constant.Value.(type) {
	case ir.ScalarValue:
		return w.writeScalarValue(v, constant.Type)
	case ir.CompositeValue:
		return w.writeCompositeValue(v, constant.Type)
	default:
		return "0" // Unknown value type
	}
}

// writeScalarValue returns the HLSL representation of a scalar value.
func (w *Writer) writeScalarValue(v ir.ScalarValue, typeHandle ir.TypeHandle) string {
	switch v.Kind {
	case ir.ScalarBool:
		if v.Bits != 0 {
			return "true"
		}
		return "false"

	case ir.ScalarSint:
		return fmt.Sprintf("%d", int32(v.Bits))

	case ir.ScalarUint:
		return fmt.Sprintf("%du", uint32(v.Bits))

	case ir.ScalarFloat:
		// Get width from type if available
		width := uint8(4) // Default to 32-bit
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				width = scalar.Width
			}
		}

		if width == 4 {
			floatVal := float32FromBits(uint32(v.Bits))
			return formatFloat32(floatVal)
		}
		// 64-bit double
		floatVal := math.Float64frombits(v.Bits)
		return formatFloat64(floatVal)

	default:
		return "0"
	}
}

// writeCompositeValue returns the HLSL representation of a composite value.
func (w *Writer) writeCompositeValue(v ir.CompositeValue, typeHandle ir.TypeHandle) string {
	var components []string
	for _, compHandle := range v.Components {
		if int(compHandle) < len(w.module.Constants) {
			constant := &w.module.Constants[compHandle]
			components = append(components, w.writeConstantValue(constant))
		} else {
			components = append(components, "0")
		}
	}
	// HLSL arrays use initializer list syntax { ... } not constructor syntax type(...)
	if isArrayType(w.module, typeHandle) {
		return fmt.Sprintf("{%s}", strings.Join(components, ", "))
	}
	typeName := w.getTypeName(typeHandle)
	return fmt.Sprintf("%s(%s)", typeName, strings.Join(components, ", "))
}

// writeGlobalVariables writes global resource declarations.
func (w *Writer) writeGlobalVariables() error {
	for handle := range w.module.GlobalVariables {
		global := &w.module.GlobalVariables[handle]
		name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}]
		if name == "" {
			name = fmt.Sprintf("global_%d", handle)
		}

		if err := w.writeGlobalVariable(name, global); err != nil {
			return err
		}
	}
	if len(w.module.GlobalVariables) > 0 {
		w.writeLine("")
	}
	return nil
}

// writeGlobalVariable writes a single global variable declaration.
func (w *Writer) writeGlobalVariable(name string, global *ir.GlobalVariable) error {
	// Get the base type, handling pointers
	typeHandle := global.Type
	if int(typeHandle) < len(w.module.Types) {
		if ptr, ok := w.module.Types[typeHandle].Inner.(ir.PointerType); ok {
			typeHandle = ptr.Base
		}
	}

	typeName := w.getTypeName(typeHandle)

	switch global.Space {
	case ir.SpaceUniform:
		// Constant buffers
		binding := w.getBindTarget(global.Binding)
		w.writeCBufferDeclaration(name, typeName, &binding)
		w.registerBindings[name] = fmt.Sprintf("register(b%d, space%d)", binding.Register, binding.Space)

	case ir.SpaceStorage:
		// Storage buffers (structured or byte address)
		binding := w.getBindTarget(global.Binding)
		// Determine if read-only based on global's storage access
		// For now assume read-write for storage space
		w.writeLine("RWStructuredBuffer<%s> %s : register(u%d, space%d);", typeName, name, binding.Register, binding.Space)
		w.registerBindings[name] = fmt.Sprintf("register(u%d, space%d)", binding.Register, binding.Space)

	case ir.SpaceWorkGroup:
		// Shared memory in compute shaders
		w.writeLine("groupshared %s %s;", typeName, name)

	case ir.SpacePrivate:
		// Module-scope private variable
		w.writeLine("static %s %s;", typeName, name)

	case ir.SpaceHandle:
		// Resource handles (textures, samplers)
		w.writeResourceHandle(name, typeHandle, global)

	default:
		// Default: just write type and name
		w.writeLine("%s %s;", typeName, name)
	}

	return nil
}

// writeResourceHandle writes a texture or sampler declaration.
func (w *Writer) writeResourceHandle(name string, typeHandle ir.TypeHandle, global *ir.GlobalVariable) {
	if int(typeHandle) >= len(w.module.Types) {
		w.writeLine("// Unknown resource type for %s", name)
		return
	}

	typ := &w.module.Types[typeHandle]

	switch inner := typ.Inner.(type) {
	case ir.SamplerType:
		samplerType := samplerTypeToHLSL(inner.Comparison)
		if global.Binding != nil {
			binding := w.getBindTarget(global.Binding)
			w.writeLine("%s %s : register(s%d, space%d);", samplerType, name, binding.Register, binding.Space)
			w.registerBindings[name] = fmt.Sprintf("register(s%d, space%d)", binding.Register, binding.Space)
		} else {
			w.writeLine("%s %s;", samplerType, name)
		}

	case ir.ImageType:
		texType := w.imageTypeToHLSL(inner)
		if global.Binding != nil {
			binding := w.getBindTarget(global.Binding)
			// Use t for textures, u for RW textures
			reg := "t"
			if inner.Class == ir.ImageClassStorage {
				reg = "u"
			}
			w.writeLine("%s %s : register(%s%d, space%d);", texType, name, reg, binding.Register, binding.Space)
			w.registerBindings[name] = fmt.Sprintf("register(%s%d, space%d)", reg, binding.Register, binding.Space)
		} else {
			w.writeLine("%s %s;", texType, name)
		}

	default:
		w.writeLine("// Unsupported resource type for %s: %T", name, inner)
	}
}

// structsEqual compares two struct types for equality.
func structsEqual(a, b ir.StructType) bool {
	if len(a.Members) != len(b.Members) {
		return false
	}
	for i := range a.Members {
		if a.Members[i].Name != b.Members[i].Name {
			return false
		}
		if a.Members[i].Type != b.Members[i].Type {
			return false
		}
	}
	return true
}

// formatFloat32 formats a float32 for HLSL output.
func formatFloat32(f float32) string {
	if math.IsInf(float64(f), 1) {
		return "1.#INF"
	}
	if math.IsInf(float64(f), -1) {
		return "-1.#INF"
	}
	if math.IsNaN(float64(f)) {
		return "0.0/0.0"
	}
	// Use %g for compact representation, ensure decimal point for floats
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
		s += ".0"
	}
	return s
}

// formatFloat64 formats a float64 for HLSL output.
func formatFloat64(f float64) string {
	if math.IsInf(f, 1) {
		return "1.#INF"
	}
	if math.IsInf(f, -1) {
		return "-1.#INF"
	}
	if math.IsNaN(f) {
		return "0.0/0.0"
	}
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
		s += ".0"
	}
	return s
}

// float32FromBits converts uint32 bits to float32.
func float32FromBits(bits uint32) float32 {
	return math.Float32frombits(bits)
}

// isScalarType checks if the type at handle is a scalar type.
func isScalarType(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	_, ok := module.Types[handle].Inner.(ir.ScalarType)
	return ok
}

// isVectorType checks if the type at handle is a vector type.
func isVectorType(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	_, ok := module.Types[handle].Inner.(ir.VectorType)
	return ok
}

// isMatrixType checks if the type at handle is a matrix type.
func isMatrixType(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	_, ok := module.Types[handle].Inner.(ir.MatrixType)
	return ok
}

// isStructType checks if the type at handle is a struct type.
func isStructType(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	_, ok := module.Types[handle].Inner.(ir.StructType)
	return ok
}

// isArrayType checks if the type at handle is an array type.
func isArrayType(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	_, ok := module.Types[handle].Inner.(ir.ArrayType)
	return ok
}

// isRuntimeArray checks if the type at handle is a runtime-sized array.
func isRuntimeArray(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	arr, ok := module.Types[handle].Inner.(ir.ArrayType)
	return ok && arr.Size.Constant == nil
}

// getScalarKind returns the scalar kind for a scalar or vector type.
func getScalarKind(module *ir.Module, handle ir.TypeHandle) (ir.ScalarKind, bool) {
	if int(handle) >= len(module.Types) {
		return 0, false
	}
	switch inner := module.Types[handle].Inner.(type) {
	case ir.ScalarType:
		return inner.Kind, true
	case ir.VectorType:
		return inner.Scalar.Kind, true
	case ir.MatrixType:
		return inner.Scalar.Kind, true
	default:
		return 0, false
	}
}

// getVectorSize returns the size of a vector type.
func getVectorSize(module *ir.Module, handle ir.TypeHandle) (ir.VectorSize, bool) {
	if int(handle) >= len(module.Types) {
		return 0, false
	}
	vec, ok := module.Types[handle].Inner.(ir.VectorType)
	if !ok {
		return 0, false
	}
	return vec.Size, true
}

// getMatrixDimensions returns the columns and rows of a matrix type.
func getMatrixDimensions(module *ir.Module, handle ir.TypeHandle) (cols, rows ir.VectorSize, ok bool) {
	if int(handle) >= len(module.Types) {
		return 0, 0, false
	}
	mat, matOK := module.Types[handle].Inner.(ir.MatrixType)
	if !matOK {
		return 0, 0, false
	}
	return mat.Columns, mat.Rows, true
}

// getArrayElementType returns the base element type of an array.
func getArrayElementType(module *ir.Module, handle ir.TypeHandle) (ir.TypeHandle, bool) {
	if int(handle) >= len(module.Types) {
		return 0, false
	}
	arr, ok := module.Types[handle].Inner.(ir.ArrayType)
	if !ok {
		return 0, false
	}
	return arr.Base, true
}

// getArraySize returns the constant size of an array, or nil for runtime arrays.
func getArraySize(module *ir.Module, handle ir.TypeHandle) (*uint32, bool) {
	if int(handle) >= len(module.Types) {
		return nil, false
	}
	arr, ok := module.Types[handle].Inner.(ir.ArrayType)
	if !ok {
		return nil, false
	}
	return arr.Size.Constant, true
}
