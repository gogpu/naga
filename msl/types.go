package msl

import (
	"fmt"
	"math"
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
		return w.writeStructDefinition(handle, typ.Name, inner)

	case ir.ArrayType:
		// Arrays need wrapper structs in MSL because arrays can't be passed by value
		return w.writeArrayWrapper(handle, inner)

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

	for memberIdx, member := range st.Members {
		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)})
		memberType := w.writeTypeName(member.Type, StorageAccess(0))

		// Check if this is a vec3 that needs to be packed
		if packed := w.shouldPackMember(st, memberIdx); packed != nil {
			memberType = w.packedVectorTypeName(*packed)
		}

		w.writeLine("%s %s;", memberType, memberName)
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")
	return nil
}

// writeArrayWrapper writes a wrapper struct for array types.
func (w *Writer) writeArrayWrapper(handle ir.TypeHandle, arr ir.ArrayType) error {
	// Only create wrapper for fixed-size arrays
	if arr.Size.Constant == nil {
		return nil // Runtime arrays don't get wrappers
	}

	wrapperName := w.getTypeName(handle)
	elementType := w.writeTypeName(arr.Base, StorageAccess(0))

	w.writeLine("struct %s {", wrapperName)
	w.pushIndent()
	w.writeLine("%s inner[%d];", elementType, *arr.Size.Constant)
	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	w.arrayWrappers[handle] = wrapperName
	return nil
}

// writeTypeName returns the MSL type name for a type handle.
// This generates inline type names (not definitions).
//
//nolint:unparam // access is used by callers for consistency even if always zero here
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

	default:
		return fmt.Sprintf("unknown_type_%T", inner)
	}
}

// scalarTypeName returns the MSL name for a scalar type.
func scalarTypeName(s ir.ScalarType) string {
	switch s.Kind {
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
	case ir.SpacePushConstant:
		return "constant"
	default:
		return ""
	}
}

// imageTypeName returns the MSL texture type name.
//
//nolint:cyclop // Texture type generation requires handling all dimension/class combinations
func (w *Writer) imageTypeName(img ir.ImageType, _ StorageAccess) string {
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
		sampleType = typeFloat // Typically float for sampled textures
	case ir.ImageClassStorage:
		sampleType = typeFloat // Storage textures can have various types
	default:
		sampleType = typeFloat
	}

	// Access mode
	var accessStr string
	switch img.Class {
	case ir.ImageClassDepth:
		accessStr = Namespace + "access::sample"
	case ir.ImageClassStorage:
		accessStr = Namespace + "access::read_write"
	default:
		accessStr = Namespace + "access::sample"
	}

	return fmt.Sprintf("%s<%s, %s>", builder.String(), sampleType, accessStr)
}

// atomicTypeName returns the MSL atomic type name.
func (w *Writer) atomicTypeName(a ir.AtomicType) string {
	switch a.Scalar.Kind {
	case ir.ScalarSint:
		if a.Scalar.Width == 4 {
			return Namespace + "atomic_int"
		}
		return Namespace + "atomic<long>"
	case ir.ScalarUint:
		if a.Scalar.Width == 4 {
			return Namespace + "atomic_uint"
		}
		return Namespace + "atomic<ulong>"
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
// Metal's vec3 types have 16-byte alignment, but sometimes we need 12-byte stride.
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

	// Check if it's a vec3
	vec, ok := memberType.Inner.(ir.VectorType)
	if !ok || vec.Size != ir.Vec3 {
		return nil
	}

	// Check if packing is needed based on stride/offset analysis
	// A vec3 should be packed if it doesn't have 16-byte alignment
	expectedAlignment := uint32(16)
	if member.Offset%expectedAlignment != 0 {
		return &vec.Scalar
	}

	// Also check if next member starts before 16-byte boundary
	if memberIdx+1 < len(st.Members) {
		nextMember := st.Members[memberIdx+1]
		if nextMember.Offset < member.Offset+16 {
			return &vec.Scalar
		}
	}

	return nil
}

// writeConstants writes constant definitions.
func (w *Writer) writeConstants() error {
	if len(w.module.Constants) == 0 {
		return nil
	}

	for handle := range w.module.Constants {
		constant := &w.module.Constants[handle]
		if err := w.writeConstant(ir.ConstantHandle(handle), constant); err != nil {
			return err
		}
	}
	w.writeLine("")
	return nil
}

// writeConstant writes a single constant definition.
func (w *Writer) writeConstant(handle ir.ConstantHandle, constant *ir.Constant) error {
	name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(handle)})
	typeName := w.writeTypeName(constant.Type, StorageAccess(0))

	w.write("constant %s %s = ", typeName, name)
	if err := w.writeConstantValue(constant.Value, constant.Type); err != nil {
		return err
	}
	w.writeLine(";")
	return nil
}

// writeConstantValue writes a constant value.
func (w *Writer) writeConstantValue(value ir.ConstantValue, typeHandle ir.TypeHandle) error {
	switch v := value.(type) {
	case ir.ScalarValue:
		return w.writeScalarValue(v, typeHandle)

	case ir.CompositeValue:
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		w.write("%s(", typeName)
		for i, componentHandle := range v.Components {
			if i > 0 {
				w.write(", ")
			}
			component := &w.module.Constants[componentHandle]
			if err := w.writeConstantValue(component.Value, component.Type); err != nil {
				return err
			}
		}
		w.write(")")
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

		if width == 4 {
			floatVal := math.Float32frombits(uint32(v.Bits))
			if floatVal == float32(int32(floatVal)) {
				w.write("%.1f", floatVal)
			} else {
				w.write("%g", floatVal)
			}
		} else {
			floatVal := math.Float64frombits(v.Bits)
			w.write("%g", floatVal)
		}

	case ir.ScalarSint:
		w.write("%d", int32(v.Bits))

	case ir.ScalarUint:
		w.write("%du", uint32(v.Bits))

	default:
		return fmt.Errorf("unsupported scalar kind: %v", v.Kind)
	}
	return nil
}
