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

// formatRegister formats a register binding string.
// Matches Rust naga: space is only written when non-zero.
func formatRegister(regType string, register uint32, space uint8) string {
	if space != 0 {
		return fmt.Sprintf("register(%s%d, space%d)", regType, register, space)
	}
	return fmt.Sprintf("register(%s%d)", regType, register)
}

// writeTypes writes all struct type definitions.
// Non-struct types are written inline where needed.
func (w *Writer) writeTypes() error {
	for handle := range w.module.Types {
		typ := &w.module.Types[handle]
		st, ok := typ.Inner.(ir.StructType)
		if !ok {
			continue
		}

		// Skip structs whose last member is dynamically sized.
		// These can only be in storage buffers, for which we use ByteAddressBuffer.
		// Matches Rust naga behavior.
		if len(st.Members) > 0 {
			lastMember := st.Members[len(st.Members)-1]
			if w.isDynamicallySized(lastMember.Type) {
				continue
			}
		}

		if err := w.writeStructDefinition(ir.TypeHandle(handle), typ.Name, st); err != nil {
			return err
		}
	}
	return nil
}

// isDynamicallySized checks if a type is dynamically sized (contains a runtime-sized array).
// Matches Rust naga's TypeInner::is_dynamically_sized.
func (w *Writer) isDynamicallySized(handle ir.TypeHandle) bool {
	if int(handle) >= len(w.module.Types) {
		return false
	}
	inner := w.module.Types[handle].Inner
	switch t := inner.(type) {
	case ir.ArrayType:
		return t.Size.Constant == nil // Dynamic = no constant size
	case ir.StructType:
		if len(t.Members) > 0 {
			return w.isDynamicallySized(t.Members[len(t.Members)-1].Type)
		}
	}
	return false
}

// writeStructDefinition writes a struct type definition.
// If the struct is used as an entry point result, semantics are written on members.
func (w *Writer) writeStructDefinition(handle ir.TypeHandle, _ string, st ir.StructType) error {
	structName := w.typeNames[handle]
	if structName == "" {
		structName = fmt.Sprintf("_struct_%d", handle)
	}

	// Check if this struct is an EP result type — determines semantic behavior.
	// Rust naga passes shader_stage to write_struct for EP result types only.
	// But write_semantic writes semantics for ALL members with bindings.
	var stageIO *shaderStageIO
	if info, ok := w.epResultTypes[handle]; ok {
		stageIO = &shaderStageIO{stage: info.stage, io: IoOutput}
	}

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	var lastOffset uint32
	for memberIdx, member := range st.Members {
		// Add padding between members if needed (matches Rust naga)
		if member.Binding == nil && member.Offset > lastOffset {
			padding := (member.Offset - lastOffset) / 4
			for i := uint32(0); i < padding; i++ {
				w.writeLine("int _pad%d_%d;", memberIdx, i)
			}
		}

		// Track last offset for padding calculation
		memberSize := w.hlslTypeSize(member.Type)
		lastOffset = member.Offset + memberSize

		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}]
		if memberName == "" {
			memberName = fmt.Sprintf("member_%d", memberIdx)
		}

		// Get the type information for the member
		memberType, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)

		// Write modifier if present.
		// - BuiltIn(Position{invariant: true}) gets "precise" prefix
		// - Location bindings get interpolation modifiers
		if member.Binding != nil {
			if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok {
				if bb.Builtin == ir.BuiltinPosition && bb.Invariant {
					memberType = "precise " + memberType
				}
			} else {
				interpMod := w.getInterpolationModifierForType(*member.Binding, member.Type)
				if interpMod != "" {
					memberType = interpMod + " " + memberType
				}
			}
		}

		// Handle matCx2 decomposition for struct members.
		// Rust naga decomposes matCx2 (rows == 2) members without bindings into
		// individual column vectors: float2 m_0; float2 m_1; float2 m_2;
		// For arrays of matCx2, use __matNx2 typedef.
		// Other matrices get row_major prefix.
		w.writeIndent()
		if member.Binding == nil && w.isMatCx2Type(member.Type) {
			// Decompose matCx2 into column vectors on a single line
			mat := w.module.Types[member.Type].Inner.(ir.MatrixType)
			vecTypeName := w.vectorTypeName(mat.Scalar, uint8(mat.Rows))
			for i := uint8(0); i < uint8(mat.Columns); i++ {
				if i != 0 {
					w.out.WriteString("; ")
				}
				fmt.Fprintf(&w.out, "%s %s_%d", vecTypeName, memberName, i)
			}
		} else if member.Binding == nil && w.isArrayOfMatCx2Type(member.Type) {
			// Array of matCx2: use __matNx2 typedef with array suffix.
			// Matches Rust naga: write_global_type + write_array_size for Array{base: matCx2}.
			m := getInnerMatrixData(w.module, member.Type)
			fmt.Fprintf(&w.out, "__mat%dx2 %s", m.columns, memberName)
			w.writeArraySizes(member.Type)
		} else {
			if member.Binding == nil && (isMatrixType(w.module, member.Type) || containsMatrix(w.module, member.Type)) {
				fmt.Fprintf(&w.out, "row_major ")
			}
			fmt.Fprintf(&w.out, "%s %s%s", memberType, memberName, arraySuffix)
		}

		// Write semantic for members with bindings (builtins always, locations use LOC{N}
		// except Fragment Output which uses SV_Target{N})
		if member.Binding != nil {
			w.writeSemantic(member.Binding, stageIO)
		}
		w.out.WriteString(";\n")
	}

	// Add end padding if needed (matches Rust naga)
	if len(st.Members) > 0 && st.Members[len(st.Members)-1].Binding == nil && st.Span > lastOffset {
		padding := (st.Span - lastOffset) / 4
		for i := uint32(0); i < padding; i++ {
			w.writeLine("int _end_pad_%d;", i)
		}
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")
	return nil
}

// locationSemantic is the prefix for user-defined location semantics (matches Rust naga).
const locationSemantic = "LOC"

// getSemanticFromBinding returns the HLSL semantic for a binding.
// For location bindings, returns LOC{N} (not TEXCOORD{N}), matching Rust naga.
func (w *Writer) getSemanticFromBinding(binding ir.Binding, idx int) string {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return BuiltInToSemantic(b.Builtin)
	case ir.LocationBinding:
		return fmt.Sprintf("%s%d", locationSemantic, b.Location)
	default:
		return fmt.Sprintf("%s%d", locationSemantic, idx)
	}
}

// writeSemantic writes the HLSL semantic annotation for a binding, considering the
// shader stage and I/O direction. Fragment outputs use SV_Target{N} for locations.
// This matches Rust naga's write_semantic function.
func (w *Writer) writeSemantic(binding *ir.Binding, stage *shaderStageIO) {
	if binding == nil {
		return
	}
	// Skip subgroup builtins — they don't have HLSL semantics
	if isSubgroupBuiltinBinding(binding) {
		return
	}
	switch b := (*binding).(type) {
	case ir.BuiltinBinding:
		fmt.Fprintf(&w.out, " : %s", BuiltInToSemantic(b.Builtin))
	case ir.LocationBinding:
		if b.BlendSrc != nil && *b.BlendSrc == 1 {
			fmt.Fprintf(&w.out, " : SV_Target1")
		} else if stage != nil && stage.stage == ir.StageFragment && stage.io == IoOutput {
			fmt.Fprintf(&w.out, " : SV_Target%d", b.Location)
		} else {
			fmt.Fprintf(&w.out, " : %s%d", locationSemantic, b.Location)
		}
	}
}

// shaderStageIO combines a shader stage with I/O direction.
type shaderStageIO struct {
	stage ir.ShaderStage
	io    Io
}

// getInterpolationModifier returns the HLSL interpolation modifier.
func (w *Writer) getInterpolationModifier(binding ir.Binding) string {
	loc, ok := binding.(ir.LocationBinding)
	if !ok {
		return ""
	}

	if loc.Interpolation == nil {
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

// getInterpolationModifierForType returns the HLSL interpolation modifier,
// applying default interpolation if not explicitly set (matches Rust naga's
// apply_default_interpolation). Integer types default to "nointerpolation" (Flat).
func (w *Writer) getInterpolationModifierForType(binding ir.Binding, memberType ir.TypeHandle) string {
	loc, ok := binding.(ir.LocationBinding)
	if !ok {
		return ""
	}

	if loc.Interpolation != nil {
		return w.getInterpolationModifier(binding)
	}

	// Apply default interpolation based on scalar kind (matches Rust naga)
	if int(memberType) < len(w.module.Types) {
		kind, hasKind := getScalarKind(w.module, memberType)
		if hasKind {
			switch kind {
			case ir.ScalarSint, ir.ScalarUint:
				return "nointerpolation"
			}
		}
	}

	return ""
}

// getTypeName returns the HLSL type name for a type handle.
// For struct types, uses the registered name from typeNames (which includes
// namer-generated suffixes for disambiguation), matching Rust naga's behavior.
func (w *Writer) getTypeName(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("unknown_type_%d", handle)
	}

	// For struct types, always use the registered namer name
	if _, ok := w.module.Types[handle].Inner.(ir.StructType); ok {
		if name, exists := w.typeNames[handle]; exists && name != "" {
			return name
		}
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

	// For struct types, always use the registered namer name (no array suffix for structs)
	if _, ok := w.module.Types[handle].Inner.(ir.StructType); ok {
		if name, exists := w.typeNames[handle]; exists && name != "" {
			return name, ""
		}
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
		// HLSL arrays: type name[outerSize][innerSize]
		// Outer array size comes FIRST in the suffix
		baseName, baseSuffix := w.getTypeNameWithArraySuffix(inner.Base)
		if inner.Size.Constant != nil {
			return baseName, fmt.Sprintf("[%d]", *inner.Size.Constant) + baseSuffix
		}
		return baseName, "[]" + baseSuffix

	case ir.StructType:
		// Use registered name with namer suffix (includes trailing underscore for digit-ending names)
		if typ.Name != "" {
			// First check if any registered type has this name
			for h, name := range w.typeNames {
				if int(h) < len(w.module.Types) && w.module.Types[h].Name == typ.Name {
					return name, ""
				}
			}
			return Escape(typ.Name), ""
		}
		// Look up in typeNames by structure equality
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

	case ir.AccelerationStructureType:
		return "RaytracingAccelerationStructure", ""

	case ir.RayQueryType:
		return "RayQuery<RAY_FLAG_NONE>", ""

	default:
		return fmt.Sprintf("unknown_type_%T", inner), ""
	}
}

// scalarTypeToHLSL returns the HLSL type name for a scalar type.
// scalarKindToHLSLBase returns the base HLSL scalar name for a given ScalarKind,
// assuming width=4 (32-bit). Used for texture template parameters.
// Matches Rust naga: Scalar { kind, width: 4 }.to_hlsl_str().
func scalarKindToHLSLBase(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarFloat:
		return "float"
	case ir.ScalarSint:
		return "int"
	case ir.ScalarUint:
		return "uint"
	case ir.ScalarBool:
		return "bool"
	default:
		return "float"
	}
}

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

// writeSamplerHeaps writes the SamplerState and SamplerComparisonState heap arrays.
// Only written once per module. Matches Rust naga's write_sampler_heaps.
func (w *Writer) writeSamplerHeaps() {
	if w.samplerHeapsWritten {
		return
	}
	w.samplerHeapsWritten = true

	stdTarget := w.options.SamplerHeapTargets.StandardSamplers
	cmpTarget := w.options.SamplerHeapTargets.ComparisonSamplers

	w.writeLine("SamplerState nagaSamplerHeap[2048]: register(s%d, space%d);", stdTarget.Register, stdTarget.Space)
	w.writeLine("SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s%d, space%d);", cmpTarget.Register, cmpTarget.Space)
}

// writeSamplerIndexBuffer writes the StructuredBuffer<uint> for a given group's sampler indices.
// Only written once per group. Matches Rust naga's write_wrapped_sampler_buffer.
func (w *Writer) writeSamplerIndexBuffer(group uint32) {
	if w.samplerIndexBuffers == nil {
		w.samplerIndexBuffers = make(map[uint32]string)
	}
	if _, ok := w.samplerIndexBuffers[group]; ok {
		return
	}

	bufName := w.namer.call(fmt.Sprintf("nagaGroup%dSamplerIndexArray", group))

	// Look up the bind target for this group's index buffer
	var bt BindTarget
	if w.options.SamplerBufferBindingMap != nil {
		if target, ok := w.options.SamplerBufferBindingMap[group]; ok {
			bt = target
		} else if w.options.FakeMissingBindings {
			bt = BindTarget{Space: 255, Register: group}
		}
	} else if w.options.FakeMissingBindings {
		bt = BindTarget{Space: 255, Register: group}
	}

	w.writeLine("StructuredBuffer<uint> %s : register(t%d, space%d);", bufName, bt.Register, bt.Space)
	w.samplerIndexBuffers[group] = bufName
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
		// Sampled textures use the scalar kind from the image type
		// Matches Rust naga: Scalar { kind, width: 4 }.to_hlsl_str()
		kindStr := scalarKindToHLSLBase(img.SampledKind)
		builder.WriteString("<")
		builder.WriteString(kindStr)
		builder.WriteString("4>")
	case ir.ImageClassStorage:
		// Storage textures use format-specific type (matches Rust naga)
		builder.WriteString("<")
		builder.WriteString(storageFormatToHLSL(img.StorageFormat))
		builder.WriteString(">")
	}

	return builder.String()
}

// storageFormatToHLSL returns the HLSL template type for a storage format.
// Matches Rust naga's StorageFormat::to_hlsl_str().
func storageFormatToHLSL(format ir.StorageFormat) string {
	switch format {
	// Single-channel float formats -> scalar "float"
	case ir.StorageFormatR16Float, ir.StorageFormatR32Float:
		return "float"
	case ir.StorageFormatR8Unorm, ir.StorageFormatR16Unorm:
		return "unorm float"
	case ir.StorageFormatR8Snorm, ir.StorageFormatR16Snorm:
		return "snorm float"
	case ir.StorageFormatR8Uint, ir.StorageFormatR16Uint, ir.StorageFormatR32Uint:
		return "uint"
	case ir.StorageFormatR8Sint, ir.StorageFormatR16Sint, ir.StorageFormatR32Sint:
		return "int"
	case ir.StorageFormatR64Uint:
		return "uint64_t"

	// Two-channel formats -> float4 (HLSL requires vec4 for RG formats)
	case ir.StorageFormatRg16Float, ir.StorageFormatRg32Float:
		return "float4"
	case ir.StorageFormatRg8Unorm, ir.StorageFormatRg16Unorm:
		return "unorm float4"
	case ir.StorageFormatRg8Snorm, ir.StorageFormatRg16Snorm:
		return "snorm float4"
	case ir.StorageFormatRg8Sint, ir.StorageFormatRg16Sint, ir.StorageFormatRg32Uint:
		return "int4"
	case ir.StorageFormatRg8Uint, ir.StorageFormatRg16Uint, ir.StorageFormatRg32Sint:
		return "uint4"

	// Packed formats
	case ir.StorageFormatRg11b10Ufloat:
		return "float4"

	// Four-channel float formats -> float4
	case ir.StorageFormatRgba16Float, ir.StorageFormatRgba32Float:
		return "float4"
	case ir.StorageFormatRgba8Unorm, ir.StorageFormatBgra8Unorm, ir.StorageFormatRgba16Unorm, ir.StorageFormatRgb10a2Unorm:
		return "unorm float4"
	case ir.StorageFormatRgba8Snorm, ir.StorageFormatRgba16Snorm:
		return "snorm float4"
	case ir.StorageFormatRgba8Uint, ir.StorageFormatRgba16Uint, ir.StorageFormatRgba32Uint, ir.StorageFormatRgb10a2Uint:
		return "uint4"
	case ir.StorageFormatRgba8Sint, ir.StorageFormatRgba16Sint, ir.StorageFormatRgba32Sint:
		return "int4"

	default:
		return "float4" // fallback
	}
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
// Matches Rust naga: `cbuffer name : register(bN) { type name; }`
// - No `_cbuffer` suffix on the cbuffer name
// - space is only written when non-zero
// - No trailing `;` after closing `}`
// - All on one logical line with `{ ... }` inline
func (w *Writer) writeCBufferDeclaration(name, _ string, typeHandle ir.TypeHandle, binding *BindTarget) {
	w.writeIndent()
	w.out.WriteString("cbuffer")

	// Write the global variable name
	fmt.Fprintf(&w.out, " %s", name)

	// Write register binding
	if binding != nil {
		fmt.Fprintf(&w.out, " : register(b%d", binding.Register)
		if binding.Space != 0 {
			fmt.Fprintf(&w.out, ", space%d", binding.Space)
		}
		w.out.WriteString(")")
	}

	// Write inline struct body: { type name[size]; }
	w.out.WriteString(" { ")

	// Check for matCx2 decomposition: matrices with rows=2 in uniform buffers
	// use __matCx2 struct instead of row_major floatCxR.
	// Matches Rust naga write_global_type in writer.rs.
	matData := getInnerMatrixData(w.module, typeHandle)
	if matData.isMatCx2() {
		// Use __matCx2 type, with possible array suffix
		_, arraySuffix := w.getTypeNameWithArraySuffix(typeHandle)
		fmt.Fprintf(&w.out, "__mat%dx2 %s%s", uint8(matData.columns), name, arraySuffix)
	} else {
		// For arrays, use the base type name and add array suffix separately
		// to avoid duplicate array sizes (e.g., "Light[10] u_lights[10]")
		baseTypeName, arraySuffix := w.getTypeNameWithArraySuffix(typeHandle)
		// Add row_major prefix for direct matrix types in cbuffer
		if isMatrixType(w.module, typeHandle) || containsMatrix(w.module, typeHandle) {
			fmt.Fprintf(&w.out, "row_major %s %s%s", baseTypeName, name, arraySuffix)
		} else {
			fmt.Fprintf(&w.out, "%s %s%s", baseTypeName, name, arraySuffix)
		}
	}

	w.out.WriteString("; }\n")
}

// writeConstants writes constant definitions.
func (w *Writer) writeConstants() error {
	if len(w.module.Constants) == 0 {
		return nil
	}

	for handle := range w.module.Constants {
		constant := &w.module.Constants[handle]
		// Skip unnamed constants (only named constants are written, matching Rust naga)
		if constant.Name == "" {
			continue
		}
		name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}]
		if name == "" {
			name = fmt.Sprintf("const_%d", handle)
		}
		typeName, arraySuffix := w.getTypeNameWithArraySuffix(constant.Type)
		w.writeIndent()
		fmt.Fprintf(&w.out, "static const %s %s%s = ", typeName, name, arraySuffix)
		// Use GlobalExpressions init if available
		if int(constant.Init) < len(w.module.GlobalExpressions) {
			if err := w.writeGlobalConstExpression(constant.Init); err != nil {
				// Fallback to old method
				w.out.WriteString(w.writeConstantValue(constant))
			}
		} else {
			w.out.WriteString(w.writeConstantValue(constant))
		}
		w.out.WriteString(";\n")
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
		// Get width from type
		swidth := uint8(4)
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				swidth = scalar.Width
			}
		}
		if swidth == 8 {
			i := int64(v.Bits)
			if i == math.MinInt64 {
				return fmt.Sprintf("(%dL - 1L)", i+1)
			}
			return fmt.Sprintf("%dL", i)
		}
		// I32: wrap in int() constructor (matches Rust naga)
		i := int32(v.Bits)
		if i == math.MinInt32 {
			return fmt.Sprintf("int(%d - 1)", i+1)
		}
		return fmt.Sprintf("int(%d)", i)

	case ir.ScalarUint:
		// Get width from type
		uwidth := uint8(4)
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				uwidth = scalar.Width
			}
		}
		if uwidth == 8 {
			return fmt.Sprintf("%duL", uint64(v.Bits))
		}
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
		// 64-bit double: add L suffix
		floatVal := math.Float64frombits(v.Bits)
		return formatFloat64(floatVal) + "L"

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
		gvHandle := ir.GlobalVariableHandle(handle)

		// External textures are decomposed into 3 planes + params cbuffer
		if w.isExternalTexture(global.Type) {
			w.writeGlobalExternalTexture(gvHandle, global)
			continue
		}

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
		w.writeCBufferDeclaration(name, typeName, typeHandle, &binding)
		w.registerBindings[name] = formatRegister("b", binding.Register, binding.Space)

	case ir.SpaceStorage:
		// Storage buffers use ByteAddressBuffer / RWByteAddressBuffer (raw byte access).
		// Matches Rust naga: storage globals always use byte address buffers.
		binding := w.getBindTarget(global.Binding)
		readOnly := global.Access == ir.StorageRead
		prefix := "RW"
		regType := "u"
		if readOnly {
			prefix = ""
			regType = "t"
		}
		regStr := formatRegister(regType, binding.Register, binding.Space)
		w.writeLine("%sByteAddressBuffer %s : %s;", prefix, name, regStr)
		w.registerBindings[name] = regStr

	case ir.SpaceWorkGroup:
		// Shared memory in compute shaders — use array suffix for correct declaration
		baseTypeName, arraySuffix := w.getTypeNameWithArraySuffix(global.Type)
		w.writeLine("groupshared %s %s%s;", baseTypeName, name, arraySuffix)

	case ir.SpacePrivate:
		// Module-scope private variable (Rust naga always initializes)
		w.writeIndent()
		fmt.Fprintf(&w.out, "static %s %s = ", typeName, name)
		if global.InitExpr != nil {
			if err := w.writeGlobalConstExpression(*global.InitExpr); err != nil {
				return err
			}
		} else if global.Init != nil {
			// Fall back to constant reference
			constName := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(*global.Init)}]
			if constName != "" {
				w.out.WriteString(constName)
			} else {
				fmt.Fprintf(&w.out, "(%s)0", typeName)
			}
		} else {
			// Zero initialize: (Type)0
			fmt.Fprintf(&w.out, "(%s)0", typeName)
		}
		w.out.WriteString(";\n")

	case ir.SpaceImmediate:
		// Immediate data (push constants) — wrapped in ConstantBuffer<T>
		// Matches Rust naga: `ConstantBuffer<Type> name: register(bN, spaceN);`
		binding := w.getBindTarget(global.Binding)
		regStr := formatRegister("b", binding.Register, binding.Space)
		w.writeLine("ConstantBuffer<%s> %s: %s;", typeName, name, regStr)
		w.registerBindings[name] = regStr

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

			// Get the group from the original resource binding
			group := uint32(0)
			if global.Binding != nil {
				group = global.Binding.Group
			}

			// Always use sampler heap indirection. The DX12 HAL must provide
			// SamplerBufferBindingMap so that naga generates the correct
			// nagaSamplerHeap[indexBuffer[N]] pattern. This matches Rust wgpu-hal.
			w.writeSamplerHeaps()
			w.writeSamplerIndexBuffer(group)

			// Write static const sampler variable that indexes into the heap
			heapVar := "nagaSamplerHeap"
			if inner.Comparison {
				heapVar = "nagaComparisonSamplerHeap"
			}
			indexBufName := w.samplerIndexBuffers[group]
			w.writeLine("static const %s %s = %s[%s[%d]];", samplerType, name, heapVar, indexBufName, binding.Register)
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
			regStr := formatRegister(reg, binding.Register, binding.Space)
			w.writeLine("%s %s : %s;", texType, name, regStr)
			w.registerBindings[name] = regStr
		} else {
			w.writeLine("%s %s;", texType, name)
		}

	case ir.AccelerationStructureType:
		if global.Binding != nil {
			binding := w.getBindTarget(global.Binding)
			regStr := formatRegister("t", binding.Register, binding.Space)
			w.writeLine("RaytracingAccelerationStructure %s : %s;", name, regStr)
			w.registerBindings[name] = regStr
		} else {
			w.writeLine("RaytracingAccelerationStructure %s;", name)
		}

	case ir.BindingArrayType:
		// Write binding array as an array of the base type
		w.writeBindingArrayDeclaration(name, inner, global)

	default:
		w.writeLine("// Unsupported resource type for %s: %T", name, inner)
	}
}

// writeBindingArrayDeclaration writes a binding array (array of textures/samplers) declaration.
// Matches Rust naga's handling of BindingArray types.
// For sampler binding arrays: writes sampler heap + index buffer + static const uint base index.
// For texture binding arrays: writes standard array declaration with optional overridden size.
func (w *Writer) writeBindingArrayDeclaration(name string, ba ir.BindingArrayType, global *ir.GlobalVariable) {
	if int(ba.Base) >= len(w.module.Types) {
		w.writeLine("// Unknown binding array base type for %s", name)
		return
	}

	baseType := &w.module.Types[ba.Base]

	// Sampler binding arrays use the sampler heap pattern.
	// Matches Rust naga write_global_sampler for TypeInner::BindingArray case.
	if _, isSampler := baseType.Inner.(ir.SamplerType); isSampler {
		if global.Binding != nil {
			binding := w.getBindTarget(global.Binding)
			group := global.Binding.Group
			w.writeSamplerHeaps()
			w.writeSamplerIndexBuffer(group)
			w.writeLine("static const uint %s = %d;", name, binding.Register)
		}
		return
	}

	// Determine the HLSL base type name
	var baseTypeName string
	switch inner := baseType.Inner.(type) {
	case ir.ImageType:
		baseTypeName = w.imageTypeToHLSL(inner)
	default:
		baseTypeName = w.getTypeName(ba.Base)
	}

	// Determine array size
	sizeStr := ""
	if ba.Size != nil {
		sizeStr = fmt.Sprintf("%d", *ba.Size)
	} else {
		// Unbounded arrays: use 10 as placeholder (matches Rust naga HLSL default)
		sizeStr = "10"
	}

	if global.Binding != nil {
		binding := w.getBindTarget(global.Binding)

		// If there's an overridden binding_array_size, use it
		if binding.BindingArraySize != nil {
			sizeStr = fmt.Sprintf("%d", *binding.BindingArraySize)
		}

		// Determine register type
		reg := "t"
		if img, ok := baseType.Inner.(ir.ImageType); ok {
			if img.Class == ir.ImageClassStorage {
				reg = "u"
			}
		}
		regStr := formatRegister(reg, binding.Register, binding.Space)
		w.writeLine("%s %s[%s] : %s;", baseTypeName, name, sizeStr, regStr)
	} else {
		w.writeLine("%s %s[%s];", baseTypeName, name, sizeStr)
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
// isMatCx2Type returns true if the type at handle is a matrix with rows == 2 (matCx2).
// These matrices need special decomposition in struct members for HLSL.
func (w *Writer) isMatCx2Type(handle ir.TypeHandle) bool {
	if int(handle) >= len(w.module.Types) {
		return false
	}
	mat, ok := w.module.Types[handle].Inner.(ir.MatrixType)
	return ok && mat.Rows == 2
}

// isArrayOfMatCx2Type returns true if the type is an Array whose inner matrix is matCx2.
// This is used for struct members that are arrays of matCx2 (not single matCx2).
func (w *Writer) isArrayOfMatCx2Type(handle ir.TypeHandle) bool {
	if int(handle) >= len(w.module.Types) {
		return false
	}
	if _, ok := w.module.Types[handle].Inner.(ir.ArrayType); !ok {
		return false
	}
	m := getInnerMatrixData(w.module, handle)
	return m != nil && m.isMatCx2()
}

// writeArraySizes writes [size] suffixes for an array type (possibly nested).
// Matches Rust naga write_array_size for struct member array declarations.
func (w *Writer) writeArraySizes(handle ir.TypeHandle) {
	if int(handle) >= len(w.module.Types) {
		return
	}
	arr, ok := w.module.Types[handle].Inner.(ir.ArrayType)
	if !ok {
		return
	}
	if arr.Size.Constant != nil {
		fmt.Fprintf(&w.out, "[%d]", *arr.Size.Constant)
	} else {
		w.out.WriteString("[1]") // runtime-sized fallback
	}
	// Handle nested arrays
	if int(arr.Base) < len(w.module.Types) {
		if _, ok := w.module.Types[arr.Base].Inner.(ir.ArrayType); ok {
			w.writeArraySizes(arr.Base)
		}
	}
}

// vectorTypeName returns the HLSL type name for a vector with the given scalar and size.
func (w *Writer) vectorTypeName(scalar ir.ScalarType, size uint8) string {
	return fmt.Sprintf("%s%d", scalarTypeHLSL(scalar), size)
}

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

// containsMatrix checks if the type at handle contains a matrix type
// (e.g., array of matrices). Does not match plain matrix types — use isMatrixType for that.
func containsMatrix(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	arr, ok := module.Types[handle].Inner.(ir.ArrayType)
	if !ok {
		return false
	}
	return isMatrixType(module, arr.Base) || containsMatrix(module, arr.Base)
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

// hlslTypeSize returns the HLSL size in bytes of a type.
// Matches Rust naga's TypeInner::size_hlsl for padding calculations.
func (w *Writer) hlslTypeSize(handle ir.TypeHandle) uint32 {
	if int(handle) >= len(w.module.Types) {
		return 0
	}
	typ := &w.module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return uint32(inner.Width)
	case ir.VectorType:
		return uint32(inner.Size) * uint32(inner.Scalar.Width)
	case ir.MatrixType:
		// HLSL matrix size: (columns-1) * stride + lastRowSize
		// stride = alignment(rows) * width
		// lastRowSize = rows * width
		rows := uint32(inner.Rows)
		cols := uint32(inner.Columns)
		width := uint32(inner.Scalar.Width)
		// Alignment for vectors: round up to power of 2
		alignment := rows * width
		if alignment == 12 { // vec3 alignment = 16
			alignment = 16
		}
		lastRowSize := rows * width
		return (cols-1)*alignment + lastRowSize
	case ir.ArrayType:
		if inner.Size.Constant != nil {
			count := *inner.Size.Constant
			if count == 0 {
				return 0
			}
			stride := inner.Stride
			if stride == 0 {
				stride = w.hlslTypeSize(inner.Base)
			}
			lastElSize := w.hlslTypeSize(inner.Base)
			return (count-1)*stride + lastElSize
		}
		return w.hlslTypeSize(inner.Base)
	case ir.StructType:
		return inner.Span
	case ir.AtomicType:
		return uint32(inner.Scalar.Width)
	default:
		return 4 // fallback
	}
}

// matrixTypeInfo holds the column/row/width info for a matrix type,
// matching Rust naga's MatrixType in back/hlsl/writer.rs.
type matrixTypeInfo struct {
	columns ir.VectorSize
	rows    ir.VectorSize
	width   uint8
}

// getInnerMatrixData returns the matrix type info for a type handle,
// recursing through arrays. Matches Rust naga get_inner_matrix_data.
func getInnerMatrixData(module *ir.Module, handle ir.TypeHandle) *matrixTypeInfo {
	if int(handle) >= len(module.Types) {
		return nil
	}
	switch inner := module.Types[handle].Inner.(type) {
	case ir.MatrixType:
		return &matrixTypeInfo{
			columns: inner.Columns,
			rows:    inner.Rows,
			width:   inner.Scalar.Width,
		}
	case ir.ArrayType:
		return getInnerMatrixData(module, inner.Base)
	default:
		return nil
	}
}

// isMatCx2 returns true if the matrix has rows=2 and width=4 (float).
// These matrices need decomposition in HLSL uniform buffers.
func (m *matrixTypeInfo) isMatCx2() bool {
	return m != nil && m.rows == ir.Vec2 && m.width == 4
}

// writeMatCx2TypedefAndFunctions writes the __matCx2 typedef and helper functions.
// Matches Rust naga help.rs write_mat_cx2_typedef_and_functions.
func (w *Writer) writeMatCx2TypedefAndFunctions(columns uint8) {
	// typedef struct { float2 _0; float2 _1; ... } __matCx2;
	w.out.WriteString("typedef struct { ")
	for i := uint8(0); i < columns; i++ {
		fmt.Fprintf(&w.out, "float2 _%d; ", i)
	}
	fmt.Fprintf(&w.out, "} __mat%dx2;\n", columns)

	// __get_col_of_matCx2
	fmt.Fprintf(&w.out, "float2 __get_col_of_mat%dx2(__mat%dx2 mat, uint idx) {\n", columns, columns)
	w.out.WriteString("    switch(idx) {\n")
	for i := uint8(0); i < columns; i++ {
		fmt.Fprintf(&w.out, "    case %d: { return mat._%d; }\n", i, i)
	}
	w.out.WriteString("    default: { return (float2)0; }\n")
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n")

	// __set_col_of_matCx2
	fmt.Fprintf(&w.out, "void __set_col_of_mat%dx2(__mat%dx2 mat, uint idx, float2 value) {\n", columns, columns)
	w.out.WriteString("    switch(idx) {\n")
	for i := uint8(0); i < columns; i++ {
		fmt.Fprintf(&w.out, "    case %d: { mat._%d = value; break; }\n", i, i)
	}
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n")

	// __set_el_of_matCx2
	fmt.Fprintf(&w.out, "void __set_el_of_mat%dx2(__mat%dx2 mat, uint idx, uint vec_idx, float value) {\n", columns, columns)
	w.out.WriteString("    switch(idx) {\n")
	for i := uint8(0); i < columns; i++ {
		fmt.Fprintf(&w.out, "    case %d: { mat._%d[vec_idx] = value; break; }\n", i, i)
	}
	w.out.WriteString("    }\n")
	w.out.WriteString("}\n")

	w.out.WriteString("\n")
}

// writeAllMatCx2TypedefsAndFunctions scans global variables and struct members
// for matCx2 types in uniform space and writes all needed typedefs.
// Matches Rust naga help.rs write_all_mat_cx2_typedefs_and_functions.
func (w *Writer) writeAllMatCx2TypedefsAndFunctions() {
	// Scan global variables in Uniform address space
	for handle := range w.module.GlobalVariables {
		global := &w.module.GlobalVariables[handle]
		if global.Space == ir.SpaceUniform {
			if m := getInnerMatrixData(w.module, global.Type); m.isMatCx2() {
				cols := uint8(m.columns)
				if _, written := w.wrappedMatCx2[cols]; !written {
					w.wrappedMatCx2[cols] = struct{}{}
					w.writeMatCx2TypedefAndFunctions(cols)
				}
			}
		}
	}

	// Scan struct members for arrays of matCx2
	for handle := range w.module.Types {
		if st, ok := w.module.Types[handle].Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if _, ok := w.module.Types[member.Type].Inner.(ir.ArrayType); ok {
					if m := getInnerMatrixData(w.module, member.Type); m.isMatCx2() {
						cols := uint8(m.columns)
						if _, written := w.wrappedMatCx2[cols]; !written {
							w.wrappedMatCx2[cols] = struct{}{}
							w.writeMatCx2TypedefAndFunctions(cols)
						}
					}
				}
			}
		}
	}
}

// writeWrappedStructMatrixAccessFunctions writes GetMat/SetMat/SetMatVec/SetMatScalar
// helper functions for a matCx2 member in a struct, if not already written.
// Matches Rust naga help.rs write_wrapped_struct_matrix_* functions.
func (w *Writer) writeWrappedStructMatrixAccessFunctions(tyHandle ir.TypeHandle, memberIndex uint32) {
	key := wrappedStructMatrixAccessKey{ty: tyHandle, index: memberIndex}
	if _, done := w.wrappedStructMatrixAccess[key]; done {
		return
	}
	w.wrappedStructMatrixAccess[key] = struct{}{}

	st, ok := w.module.Types[tyHandle].Inner.(ir.StructType)
	if !ok || int(memberIndex) >= len(st.Members) {
		return
	}
	member := st.Members[memberIndex]
	mat, ok := w.module.Types[member.Type].Inner.(ir.MatrixType)
	if !ok || mat.Rows != 2 {
		return
	}

	structName := w.typeNames[tyHandle]
	fieldName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(tyHandle), handle2: memberIndex}]
	matTypeName := w.getTypeName(member.Type)
	vecTypeName := w.vectorTypeName(mat.Scalar, uint8(mat.Rows))
	scalarTypeName := scalarTypeHLSL(mat.Scalar)

	// GetMat{field}On{struct}
	fmt.Fprintf(&w.out, "%s GetMat%sOn%s(%s obj) {\n", matTypeName, fieldName, structName, structName)
	fmt.Fprintf(&w.out, "    return %s(", matTypeName)
	for i := uint8(0); i < uint8(mat.Columns); i++ {
		if i != 0 {
			w.out.WriteString(", ")
		}
		fmt.Fprintf(&w.out, "obj.%s_%d", fieldName, i)
	}
	w.out.WriteString(");\n}\n\n")

	// SetMat{field}On{struct}
	fmt.Fprintf(&w.out, "void SetMat%sOn%s(%s obj, %s mat) {\n", fieldName, structName, structName, matTypeName)
	for i := uint8(0); i < uint8(mat.Columns); i++ {
		fmt.Fprintf(&w.out, "    obj.%s_%d = mat[%d];\n", fieldName, i, i)
	}
	w.out.WriteString("}\n\n")

	// SetMatVec{field}On{struct}
	fmt.Fprintf(&w.out, "void SetMatVec%sOn%s(%s obj, %s vec, uint mat_idx) {\n", fieldName, structName, structName, vecTypeName)
	w.out.WriteString("    switch(mat_idx) {\n")
	for i := uint8(0); i < uint8(mat.Columns); i++ {
		fmt.Fprintf(&w.out, "    case %d: { obj.%s_%d = vec; break; }\n", i, fieldName, i)
	}
	w.out.WriteString("    }\n}\n\n")

	// SetMatScalar{field}On{struct}
	fmt.Fprintf(&w.out, "void SetMatScalar%sOn%s(%s obj, %s scalar, uint mat_idx, uint vec_idx) {\n", fieldName, structName, structName, scalarTypeName)
	w.out.WriteString("    switch(mat_idx) {\n")
	for i := uint8(0); i < uint8(mat.Columns); i++ {
		fmt.Fprintf(&w.out, "    case %d: { obj.%s_%d[vec_idx] = scalar; break; }\n", i, fieldName, i)
	}
	w.out.WriteString("    }\n}\n\n")
}

// scalarTypeHLSL returns the HLSL name for a scalar type.
func scalarTypeHLSL(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarFloat:
		if s.Width == 8 {
			return "double"
		}
		if s.Width == 2 {
			return "half"
		}
		return "float"
	case ir.ScalarSint:
		return "int"
	case ir.ScalarUint:
		return "uint"
	case ir.ScalarBool:
		return "bool"
	default:
		return "float"
	}
}
