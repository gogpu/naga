package msl

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Type name constants
const (
	typeFloat = "float"
)

// Namespace is the MSL metal namespace prefix.
const Namespace = "metal::"

// StorageAccess represents storage access flags for images.
type StorageAccess uint8

// needsCustomTypeName returns true for types that need custom identifier names
// in MSL output. Struct and array types need named definitions; all other types
// (scalars, vectors, matrices, pointers, samplers, images, atomics) use built-in
// MSL names and should not consume namer counter slots.
func needsCustomTypeName(inner ir.TypeInner) bool {
	switch inner.(type) {
	case ir.StructType, ir.ArrayType:
		return true
	default:
		return false
	}
}

// writeTypes writes all type definitions.
func (w *Writer) writeTypes() error {
	for handle, typ := range w.module.Types {
		if err := w.writeTypeDefinition(ir.TypeHandle(handle), &typ); err != nil {
			return err
		}
	}
	return nil
}

// writeTypeDefinition writes a type definition if needed.
func (w *Writer) writeTypeDefinition(handle ir.TypeHandle, typ *ir.Type) error {
	switch inner := typ.Inner.(type) {
	case ir.StructType:
		// Skip modf/frexp result structs — they are emitted by the helper function
		// registration (registerModfResult/registerFrexpResult) instead of here.
		// Skip __atomic_compare_exchange_result structs — they are emitted
		// by the atomic compare-exchange helper template registration.
		if strings.HasPrefix(typ.Name, "__modf_result_") || strings.HasPrefix(typ.Name, "__frexp_result_") ||
			strings.HasPrefix(typ.Name, "__atomic_compare_exchange_result") {
			return nil
		}
		return w.writeStructDefinition(handle, typ.Name, inner)

	case ir.ArrayType:
		// Arrays need wrapper structs in MSL because arrays can't be passed by value
		return w.writeArrayWrapper(handle, inner)

	case ir.ImageType:
		// Emit NagaExternalTextureWrapper struct once when we encounter an external image type.
		// Matches Rust naga: writer.rs ~line 4438.
		if inner.Class == ir.ImageClassExternal && !w.externalTextureWrapperEmitted {
			// Get the NagaExternalTextureParams type name.
			if w.module.SpecialTypes.ExternalTextureParams != nil {
				paramsTypeName := w.getTypeName(*w.module.SpecialTypes.ExternalTextureParams)
				w.writeLine("struct NagaExternalTextureWrapper {")
				w.pushIndent()
				w.writeLine("%stexture2d<float, %saccess::sample> plane0;", Namespace, Namespace)
				w.writeLine("%stexture2d<float, %saccess::sample> plane1;", Namespace, Namespace)
				w.writeLine("%stexture2d<float, %saccess::sample> plane2;", Namespace, Namespace)
				w.writeLine("%s params;", paramsTypeName)
				w.popIndent()
				w.writeLine("};")
			}
			w.externalTextureWrapperEmitted = true
		}
		return nil

	case ir.BindingArrayType:
		// Emit NagaArgumentBufferWrapper template struct once.
		// Matches Rust naga: Metal argument buffers need an indirection wrapper
		// because textures/samplers can't be directly in argument buffer arrays.
		if !w.argumentBufferWrapperEmitted {
			w.writeLine("template <typename T>")
			w.writeLine("struct NagaArgumentBufferWrapper {")
			w.pushIndent()
			w.writeLine("T inner;")
			w.popIndent()
			w.writeLine("};")
			w.argumentBufferWrapperEmitted = true
		}
		return nil

	default:
		// Other types don't need definitions
		return nil
	}
}

// writeStructDefinition writes a struct type definition.
func (w *Writer) writeStructDefinition(handle ir.TypeHandle, _ string, st ir.StructType) error {
	structName := w.getTypeName(handle)
	w.writeLine("struct %s {", structName)
	w.pushIndent()

	lastOffset := uint32(0)
	for memberIdx, member := range st.Members {
		// Insert padding bytes if there's a gap between the previous member's end
		// and this member's offset. Matches Rust naga: writer.rs ~line 4519.
		if member.Offset > lastOffset {
			pad := member.Offset - lastOffset
			w.writeLine("char _pad%d[%d];", memberIdx, pad)
			// Track that this member has padding before it, for aggregate init.
			// Matches Rust naga's struct_member_pads set.
			w.structPads[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] = struct{}{}
		}

		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)})
		memberType := w.writeTypeName(member.Type, StorageAccess(0))

		// Check if this is a vec3 that needs to be packed
		packed := w.shouldPackMember(st, memberIdx)
		if packed != nil {
			memberType = w.packedVectorTypeName(*packed)
		}

		w.writeLine("%s %s;", memberType, memberName)

		// Update lastOffset: member.Offset + size of member type.
		lastOffset = member.Offset + w.typeSize(member.Type)

		// For unpacked vec3 types, MSL pads them to 16 bytes (4-component alignment),
		// so add one extra scalar width. Matches Rust naga: writer.rs ~line 4558.
		if packed == nil {
			if int(member.Type) < len(w.module.Types) {
				if vec, ok := w.module.Types[member.Type].Inner.(ir.VectorType); ok && vec.Size == ir.Vec3 {
					lastOffset += uint32(vec.Scalar.Width)
				}
			}
		}
	}

	// Emit trailing padding if the struct's span exceeds the last member's end.
	// This ensures the struct's total size matches the expected layout for uniform
	// and storage buffer types. Matches Rust naga: writer.rs ~line 4568.
	if lastOffset < st.Span {
		pad := st.Span - lastOffset
		w.writeLine("char _pad%d[%d];", len(st.Members), pad)
	}

	w.popIndent()
	w.writeLine("};")
	return nil
}

// writeArrayWrapper writes a wrapper struct for array types.
// For fixed-size arrays, emits a wrapper struct with an inner array member.
// For runtime-sized (dynamic) arrays, emits a typedef with size [1] so that
// the type name is available for buffer parameter declarations.
// Rust naga reference: writer.rs ~line 4508, IndexableLength::Dynamic => typedef T name[1].
func (w *Writer) writeArrayWrapper(handle ir.TypeHandle, arr ir.ArrayType) error {
	wrapperName := w.getTypeName(handle)
	elementType := w.writeTypeName(arr.Base, StorageAccess(0))

	if arr.Size.Constant == nil {
		// Runtime-sized (dynamic) array: emit a typedef with [1] placeholder.
		// MSL does not support unsized arrays directly, so we declare the type
		// with a nominal size of 1; the actual extent is determined by the buffer binding.
		// Note: NOT added to arrayWrappers — dynamic arrays use direct indexing (name[i]),
		// not the struct wrapper pattern (name.inner[i]) used by fixed-size arrays.
		w.writeLine("typedef %s %s[1];", elementType, wrapperName)
		return nil
	}

	w.writeLine("struct %s {", wrapperName)
	w.pushIndent()
	w.writeLine("%s inner[%d];", elementType, *arr.Size.Constant)
	w.popIndent()
	w.writeLine("};")

	w.arrayWrappers[handle] = wrapperName
	return nil
}

// writeTypeName returns the MSL type name for a type handle.
// This generates inline type names (not definitions).
func (w *Writer) writeTypeName(handle ir.TypeHandle, access StorageAccess) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("invalid_type_%d", handle)
	}

	typ := &w.module.Types[handle]
	return w.writeTypeInnerName(handle, typ.Inner, access)
}

// writeTypeInnerName returns the MSL name for a TypeInner.
func (w *Writer) writeTypeInnerName(handle ir.TypeHandle, inner ir.TypeInner, access StorageAccess) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarTypeName(t)

	case ir.VectorType:
		return vectorTypeName(t)

	case ir.MatrixType:
		return matrixTypeName(t)

	case ir.ArrayType:
		// Check if we have a wrapper struct
		if wrapperName, ok := w.arrayWrappers[handle]; ok {
			return wrapperName
		}
		// Otherwise use the registered name or generate inline
		if name, ok := w.typeNames[handle]; ok {
			return name
		}
		return w.inlineArrayTypeName(t)

	case ir.StructType:
		return w.getTypeName(handle)

	case ir.PointerType:
		return w.pointerTypeName(t)

	case ir.SamplerType:
		return Namespace + "sampler"

	case ir.ImageType:
		return w.imageTypeName(t, access)

	case ir.AtomicType:
		return w.atomicTypeName(t)

	case ir.AccelerationStructureType:
		return Namespace + "raytracing::instance_acceleration_structure"

	case ir.RayQueryType:
		return "_RayQuery"

	case ir.BindingArrayType:
		// Metal binding arrays use NagaArgumentBufferWrapper for indirection.
		// Entry point parameters: constant NagaArgumentBufferWrapper<base_type>*
		baseName := w.writeTypeName(t.Base, access)
		return fmt.Sprintf("constant NagaArgumentBufferWrapper<%s>*", baseName)

	default:
		return fmt.Sprintf("unknown_type_%T", inner)
	}
}

// scalarTypeName returns the MSL name for a scalar type.
func scalarTypeName(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarAbstractInt:
		return "int" // Concretize to i32
	case ir.ScalarAbstractFloat:
		return "float" // Concretize to f32
	case ir.ScalarBool:
		return "bool"

	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return "half"
		case 4:
			return "float"
		case 8:
			return "double" // MSL 2.3+ only
		}

	case ir.ScalarSint:
		switch s.Width {
		case 1:
			return "char"
		case 2:
			return "short"
		case 4:
			return "int"
		case 8:
			return "long"
		}

	case ir.ScalarUint:
		switch s.Width {
		case 1:
			return "uchar"
		case 2:
			return "ushort"
		case 4:
			return "uint"
		case 8:
			return "ulong"
		}
	}
	return "unknown_scalar"
}

// vectorTypeName returns the MSL name for a vector type.
func vectorTypeName(v ir.VectorType) string {
	scalar := scalarTypeName(v.Scalar)
	return fmt.Sprintf("%s%s%d", Namespace, scalar, v.Size)
}

// matrixTypeName returns the MSL name for a matrix type.
func matrixTypeName(m ir.MatrixType) string {
	scalar := scalarTypeName(m.Scalar)
	return fmt.Sprintf("%s%s%dx%d", Namespace, scalar, m.Columns, m.Rows)
}

// inlineArrayTypeName generates an inline array type name.
func (w *Writer) inlineArrayTypeName(arr ir.ArrayType) string {
	elementType := w.writeTypeName(arr.Base, StorageAccess(0))
	if arr.Size.Constant != nil {
		return fmt.Sprintf("array<%s, %d>", elementType, *arr.Size.Constant)
	}
	return elementType // Runtime arrays are just the element type in certain contexts
}

// pointerTypeName returns the MSL pointer type name.
func (w *Writer) pointerTypeName(p ir.PointerType) string {
	baseType := w.writeTypeName(p.Base, StorageAccess(0))
	space := addressSpaceName(p.Space)
	if space != "" {
		return fmt.Sprintf("%s %s&", space, baseType)
	}
	return fmt.Sprintf("%s&", baseType)
}

// addressSpaceName returns the MSL address space name.
func addressSpaceName(space ir.AddressSpace) string {
	switch space {
	case ir.SpaceUniform:
		return "constant"
	case ir.SpaceStorage:
		return "device"
	case ir.SpacePrivate, ir.SpaceFunction:
		return "thread"
	case ir.SpaceWorkGroup:
		return "threadgroup"
	case ir.SpaceHandle:
		return "" // Handles don't have address space qualifiers
	case ir.SpacePushConstant, ir.SpaceImmediate:
		return "constant"
	default:
		return ""
	}
}

// imageTypeName returns the MSL texture type name.
func (w *Writer) imageTypeName(img ir.ImageType, _ StorageAccess) string {
	// External textures use the wrapper struct type name.
	if img.Class == ir.ImageClassExternal {
		return "NagaExternalTextureWrapper"
	}

	var builder strings.Builder
	builder.WriteString(Namespace)

	// Dimension and depth
	switch img.Class {
	case ir.ImageClassDepth:
		switch img.Dim {
		case ir.Dim2D:
			if img.Multisampled {
				builder.WriteString("depth2d_ms")
			} else {
				builder.WriteString("depth2d")
			}
		case ir.DimCube:
			builder.WriteString("depthcube")
		default:
			builder.WriteString("depth2d")
		}
	default:
		switch img.Dim {
		case ir.Dim1D:
			builder.WriteString("texture1d")
		case ir.Dim2D:
			if img.Multisampled {
				builder.WriteString("texture2d_ms")
			} else {
				builder.WriteString("texture2d")
			}
		case ir.Dim3D:
			builder.WriteString("texture3d")
		case ir.DimCube:
			builder.WriteString("texturecube")
		}
	}

	// Array suffix
	if img.Arrayed && img.Dim != ir.Dim3D {
		builder.WriteString("_array")
	}

	// Template parameter (sample type)
	var sampleType string
	switch img.Class {
	case ir.ImageClassDepth:
		sampleType = typeFloat
	case ir.ImageClassSampled:
		switch img.SampledKind {
		case ir.ScalarUint:
			sampleType = "uint"
		case ir.ScalarSint:
			sampleType = "int"
		default:
			sampleType = typeFloat
		}
	case ir.ImageClassStorage:
		sampleType = storageFormatToMSLType(img.StorageFormat)
		// R64 formats require Metal 3.1 for int64 atomic textures
		if img.StorageFormat == ir.StorageFormatR64Uint || img.StorageFormat == ir.StorageFormatR64Sint {
			w.requireVersion(Version3_1)
		}
	default:
		sampleType = typeFloat
	}

	// Access mode
	var accessStr string
	switch img.Class {
	case ir.ImageClassDepth:
		if img.Multisampled {
			accessStr = Namespace + "access::read"
		} else {
			accessStr = Namespace + "access::sample"
		}
	case ir.ImageClassStorage:
		// Use the actual storage access mode from the IR.
		// Rust naga: read-only storage textures use access::read,
		// write-only use access::write, read_write use access::read_write.
		switch img.StorageAccess {
		case ir.StorageAccessRead:
			accessStr = Namespace + "access::read"
		case ir.StorageAccessWrite:
			accessStr = Namespace + "access::write"
		default:
			accessStr = Namespace + "access::read_write"
		}
	default:
		// Multisampled sampled textures use access::read.
		// Regular sampled textures use access::sample.
		if img.Multisampled {
			accessStr = Namespace + "access::read"
		} else {
			accessStr = Namespace + "access::sample"
		}
	}

	return fmt.Sprintf("%s<%s, %s>", builder.String(), sampleType, accessStr)
}

// storageFormatToMSLType returns the MSL sample type for a storage format.
// Matches Rust naga: most formats use float, int, or uint based on the format suffix.
// R64Uint/R64Sint use ulong/long (require Metal 3.1).
func storageFormatToMSLType(format ir.StorageFormat) string {
	switch format {
	case ir.StorageFormatR8Uint, ir.StorageFormatR16Uint, ir.StorageFormatR32Uint,
		ir.StorageFormatRg8Uint, ir.StorageFormatRg16Uint, ir.StorageFormatRg32Uint,
		ir.StorageFormatRgba8Uint, ir.StorageFormatRgba16Uint, ir.StorageFormatRgba32Uint,
		ir.StorageFormatRgb10a2Uint:
		return "uint"
	case ir.StorageFormatR8Sint, ir.StorageFormatR16Sint, ir.StorageFormatR32Sint,
		ir.StorageFormatRg8Sint, ir.StorageFormatRg16Sint, ir.StorageFormatRg32Sint,
		ir.StorageFormatRgba8Sint, ir.StorageFormatRgba16Sint, ir.StorageFormatRgba32Sint:
		return "int"
	case ir.StorageFormatR64Uint:
		return "ulong"
	case ir.StorageFormatR64Sint:
		return "long"
	default:
		// Float formats and unknown formats default to float
		return typeFloat
	}
}

// atomicTypeName returns the MSL atomic type name.
func (w *Writer) atomicTypeName(a ir.AtomicType) string {
	switch a.Scalar.Kind {
	case ir.ScalarSint:
		if a.Scalar.Width == 4 {
			return Namespace + "atomic_int"
		}
		// MSL uses atomic_long (not atomic<long>) for 64-bit signed atomics.
		// Matches Rust naga output.
		return Namespace + "atomic_long"
	case ir.ScalarUint:
		if a.Scalar.Width == 4 {
			return Namespace + "atomic_uint"
		}
		// MSL uses atomic_ulong (not atomic<ulong>) for 64-bit unsigned atomics.
		// Matches Rust naga output.
		return Namespace + "atomic_ulong"
	case ir.ScalarFloat:
		// MSL 3.1+ supports atomic_float for f32 atomics.
		// Matches Rust naga output.
		return Namespace + "atomic_float"
	default:
		return Namespace + "atomic_uint"
	}
}

// packedVectorTypeName returns the MSL packed vector type name.
func (w *Writer) packedVectorTypeName(scalar ir.ScalarType) string {
	scalarName := scalarTypeName(scalar)
	return fmt.Sprintf("%spacked_%s3", Namespace, scalarName)
}

// shouldPackMember checks if a struct member should use packed vector type.
// Matches Rust naga's should_pack_struct_member exactly: a vec3 member is packed
// if and only if the layout is "tight" (next member starts immediately after the
// vec3's logical size with no alignment padding). This applies to vec3 with
// scalar width 2 (half) or 4 (float/int/uint).
func (w *Writer) shouldPackMember(st ir.StructType, memberIdx int) *ir.ScalarType {
	if memberIdx >= len(st.Members) {
		return nil
	}
	member := st.Members[memberIdx]

	// Get the member type
	if int(member.Type) >= len(w.module.Types) {
		return nil
	}
	memberType := w.module.Types[member.Type]

	// Check if it's a vec3 with width 4 or 2
	vec, ok := memberType.Inner.(ir.VectorType)
	if !ok || vec.Size != ir.Vec3 {
		return nil
	}
	if vec.Scalar.Width != 4 && vec.Scalar.Width != 2 {
		return nil
	}

	// Compute last_offset = member.offset + type size (no alignment padding)
	lastOffset := member.Offset + w.typeSize(member.Type)

	// Compute next_offset: either next member's offset, or struct span
	var nextOffset uint32
	if memberIdx+1 < len(st.Members) {
		nextOffset = st.Members[memberIdx+1].Offset
	} else {
		nextOffset = st.Span
	}

	// Pack only when layout is tight (no gap between this member and next)
	isTight := nextOffset == lastOffset
	if isTight {
		return &vec.Scalar
	}

	return nil
}

// typeSize returns the size in bytes for a type handle, used for struct padding
// calculation. Matches Rust naga TypeInner::size().
func (w *Writer) typeSize(handle ir.TypeHandle) uint32 {
	if int(handle) >= len(w.module.Types) {
		return 4
	}
	typ := &w.module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return uint32(inner.Width)
	case ir.VectorType:
		return uint32(inner.Size) * uint32(inner.Scalar.Width)
	case ir.MatrixType:
		// Matrix columns are aligned like vectors. Matches Rust naga:
		// Alignment::from(rows) * scalar.width * columns.
		// For vec3 columns: stride is 4 * scalar_width (not 3 * scalar_width).
		scalarWidth := uint32(inner.Scalar.Width)
		if scalarWidth == 0 {
			scalarWidth = 4
		}
		var rowMultiplier uint32
		switch inner.Rows {
		case ir.Vec2:
			rowMultiplier = 2
		case ir.Vec3, ir.Vec4:
			rowMultiplier = 4
		default:
			rowMultiplier = uint32(inner.Rows)
		}
		return uint32(inner.Columns) * rowMultiplier * scalarWidth
	case ir.ArrayType:
		if inner.Size.Constant != nil {
			elemSize := w.typeSize(inner.Base)
			stride := inner.Stride
			if stride == 0 {
				stride = elemSize
			}
			return stride * (*inner.Size.Constant)
		}
		// Dynamic arrays: Rust naga returns 1 * stride (at least one element).
		// This is needed for correct trailing padding calculation in structs.
		stride := inner.Stride
		if stride == 0 {
			stride = w.typeSize(inner.Base)
		}
		return stride
	case ir.StructType:
		return inner.Span
	case ir.AtomicType:
		return uint32(inner.Scalar.Width)
	default:
		return 4
	}
}

// typeInnerToMSL returns the MSL type name for a TypeInner without needing a
// TypeHandle. Works for simple types (scalar, vector, matrix). Returns empty
// string for types that require a handle (struct, array).
func typeInnerToMSL(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarTypeName(t)
	case ir.VectorType:
		return vectorTypeName(t)
	case ir.MatrixType:
		return matrixTypeName(t)
	default:
		return ""
	}
}

// writeConstants writes constant definitions.
func (w *Writer) writeConstants() error {
	for handle := range w.module.Constants {
		constant := &w.module.Constants[handle]
		// Skip unnamed constants (component sub-values of composites).
		// Matches Rust naga which only emits constants with c.name.is_some().
		if constant.Name == "" {
			continue
		}
		// Skip abstract-typed constants (e.g., `const ONE = 1;` without explicit type).
		// In Rust naga, these are removed by the compact pass before reaching the MSL writer.
		if constant.IsAbstract {
			continue
		}
		if err := w.writeConstant(ir.ConstantHandle(handle), constant); err != nil {
			return err
		}
	}

	// Write overrides as constant declarations.
	// In Rust naga, process_overrides converts Override expressions to Constants
	// before the MSL writer runs. We emit them directly as constants here.
	for handle := range w.module.Overrides {
		ov := &w.module.Overrides[handle]
		if err := w.writeOverrideAsConstant(ir.OverrideHandle(handle), ov); err != nil {
			return err
		}
	}

	return nil
}

// writeOverrideAsConstant writes an override as a MSL constant declaration.
// This matches Rust naga's process_overrides which converts overrides to constants.
func (w *Writer) writeOverrideAsConstant(handle ir.OverrideHandle, ov *ir.Override) error {
	name := w.getName(nameKey{kind: nameKeyOverride, handle1: uint32(handle)})
	typeName := w.writeTypeName(ov.Ty, StorageAccess(0))

	w.write("constant %s %s = ", typeName, name)
	if ov.Init != nil {
		if err := w.writeGlobalExpression(*ov.Init); err != nil {
			return err
		}
	} else {
		// No default value — write zero-initialized value.
		w.write("%s {}", typeName)
	}
	w.writeLine(";")
	return nil
}

// writeConstant writes a single constant definition.
func (w *Writer) writeConstant(handle ir.ConstantHandle, constant *ir.Constant) error {
	name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(handle)})
	typeName := w.writeTypeName(constant.Type, StorageAccess(0))

	w.write("constant %s %s = ", typeName, name)
	// If Value is nil, the constant uses Init (GlobalExpressions handle) instead.
	// This matches Rust naga which always uses put_const_expression(constant.init, ...).
	if constant.Value == nil {
		if err := w.writeGlobalExpression(constant.Init); err != nil {
			return err
		}
	} else if err := w.writeConstantValue(constant.Value, constant.Type); err != nil {
		return err
	}
	w.writeLine(";")
	return nil
}

// writeConstantValue writes a constant value.
func (w *Writer) writeConstantValue(value ir.ConstantValue, typeHandle ir.TypeHandle) error {
	switch v := value.(type) {
	case nil:
		// Value is nil — should not normally happen if callers check first.
		// Handle gracefully with zero-value braces.
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		w.write("%s {}", typeName)
		return nil

	case ir.ScalarValue:
		return w.writeScalarValue(v, typeHandle)

	case ir.ZeroConstantValue:
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		w.write("%s {}", typeName)
		return nil

	case ir.CompositeValue:
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		// Arrays (wrapped in structs) and user-defined structs use brace initialization.
		// Vectors and matrices use constructor call parentheses.
		// Matches Rust naga MSL output.
		useBraces := false
		if _, ok := w.arrayWrappers[typeHandle]; ok {
			useBraces = true
		} else if int(typeHandle) < len(w.module.Types) {
			if _, ok := w.module.Types[typeHandle].Inner.(ir.StructType); ok {
				useBraces = true
			}
		}
		if useBraces {
			w.write("%s {", typeName)
		} else {
			w.write("%s(", typeName)
		}
		for i, componentHandle := range v.Components {
			if i > 0 {
				w.write(", ")
			}
			component := &w.module.Constants[componentHandle]
			if err := w.writeConstantValue(component.Value, component.Type); err != nil {
				return err
			}
		}
		if useBraces {
			w.write("}")
		} else {
			w.write(")")
		}
		return nil

	default:
		return fmt.Errorf("unsupported constant value type: %T", value)
	}
}

// writeScalarValue writes a scalar constant value.
func (w *Writer) writeScalarValue(v ir.ScalarValue, typeHandle ir.TypeHandle) error {
	switch v.Kind {
	case ir.ScalarBool:
		if v.Bits != 0 {
			w.write("true")
		} else {
			w.write("false")
		}

	case ir.ScalarFloat:
		// Get width from type
		width := uint8(4) // Default to 32-bit
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				width = scalar.Width
			}
		}

		if width == 2 {
			// f16 (half): bits are stored as uint16 IEEE 754 half-precision.
			// Convert to float64 for formatting, then add 'h' suffix.
			floatVal := float64(halfToFloat32(uint16(v.Bits)))
			if math.IsInf(floatVal, 0) {
				if floatVal < 0 {
					w.write("-INFINITY")
				} else {
					w.write("INFINITY")
				}
			} else if math.IsNaN(floatVal) {
				w.write("NAN")
			} else {
				s := strconv.FormatFloat(floatVal, 'f', -1, 32)
				if !strings.Contains(s, ".") {
					s += ".0"
				}
				w.write("%sh", s)
			}
		} else if width == 4 {
			floatVal := math.Float32frombits(uint32(v.Bits))
			if math.IsInf(float64(floatVal), 0) {
				if floatVal < 0 {
					w.write("-INFINITY")
				} else {
					w.write("INFINITY")
				}
			} else if math.IsNaN(float64(floatVal)) {
				w.write("NAN")
			} else {
				s := strconv.FormatFloat(float64(floatVal), 'f', -1, 32)
				if !strings.Contains(s, ".") {
					s += ".0"
				}
				w.write("%s", s)
			}
		} else {
			floatVal := math.Float64frombits(v.Bits)
			if math.IsInf(floatVal, 0) {
				if floatVal < 0 {
					w.write("-INFINITY")
				} else {
					w.write("INFINITY")
				}
			} else if math.IsNaN(floatVal) {
				w.write("NAN")
			} else {
				s := strconv.FormatFloat(floatVal, 'f', -1, 64)
				if !strings.Contains(s, ".") {
					s += ".0"
				}
				w.write("%s", s)
			}
		}

	case ir.ScalarSint:
		// Get width from type to distinguish i32 vs i64.
		sWidth := uint8(4)
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				sWidth = scalar.Width
			}
		}
		if sWidth == 8 {
			val := int64(v.Bits)
			if val == -9223372036854775808 {
				w.write("(-9223372036854775807L - 1L)")
			} else {
				w.write("%dL", val)
			}
		} else {
			val := int32(v.Bits)
			if val == -2147483648 {
				w.write("(-2147483647 - 1)")
			} else {
				w.write("%d", val)
			}
		}

	case ir.ScalarUint:
		// Get width from type to distinguish u32 vs u64.
		uWidth := uint8(4)
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				uWidth = scalar.Width
			}
		}
		if uWidth == 8 {
			w.write("%duL", uint64(v.Bits))
		} else {
			w.write("%du", uint32(v.Bits))
		}

	default:
		return fmt.Errorf("unsupported scalar kind: %v", v.Kind)
	}
	return nil
}

// halfToFloat32 converts a 16-bit IEEE 754 half-precision float to float32.
func halfToFloat32(bits uint16) float32 {
	sign := uint32(bits>>15) & 1
	exp := uint32(bits>>10) & 0x1f
	frac := uint32(bits) & 0x3ff

	switch {
	case exp == 0:
		if frac == 0 {
			return math.Float32frombits(sign << 31)
		}
		// Subnormal: normalize
		for frac&0x400 == 0 {
			frac <<= 1
			exp--
		}
		exp++
		frac &= 0x3ff
		fallthrough
	case exp < 31:
		exp += 127 - 15
		return math.Float32frombits(sign<<31 | exp<<23 | frac<<13)
	default:
		// Inf or NaN
		return math.Float32frombits(sign<<31 | 0xff<<23 | frac<<13)
	}
}
