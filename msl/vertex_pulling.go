package msl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// doVertexPulling returns true if VPT should be applied for the given entry point.
func (w *Writer) doVertexPulling(ep *ir.EntryPoint) bool {
	return ep.Stage == ir.StageVertex &&
		w.options.VertexPullingTransform &&
		len(w.options.VertexBufferMappings) > 0
}

// initVertexPulling initializes VPT state: resolves buffer mapping names,
// generates unpacking functions, and determines vertex/instance ID needs.
// Must be called early in writeModule, before any entry points or functions.
// Matches Rust naga writer.rs ~6430-6496.
func (w *Writer) initVertexPulling() {
	if !w.options.VertexPullingTransform || len(w.options.VertexBufferMappings) == 0 {
		return
	}

	w.vptUnpackingFunctions = make(map[VertexFormat]vptUnpackFunc)
	w.vptNeedsVertexID = false
	w.vptNeedsInstanceID = false

	// Generate v_id and i_id names through the namer (matches Rust namer.call order).
	w.vptVertexIDName = w.namer.call("v_id")
	w.vptInstanceIDName = w.namer.call("i_id")

	for _, vbm := range w.options.VertexBufferMappings {
		switch vbm.StepMode {
		case VertexStepModeByVertex:
			w.vptNeedsVertexID = true
		case VertexStepModeByInstance:
			w.vptNeedsInstanceID = true
		}

		tyName := w.namer.call(fmt.Sprintf("vb_%d_type", vbm.ID))
		paramName := w.namer.call(fmt.Sprintf("vb_%d_in", vbm.ID))
		elemName := w.namer.call(fmt.Sprintf("vb_%d_elem", vbm.ID))

		w.vptBufferMappings = append(w.vptBufferMappings, vptBufferMappingResolved{
			id:         vbm.ID,
			stride:     vbm.Stride,
			stepMode:   vbm.StepMode,
			tyName:     tyName,
			paramName:  paramName,
			elemName:   elemName,
			attributes: vbm.Attributes,
		})

		// Generate unpacking functions for each unique format.
		for _, attr := range vbm.Attributes {
			if _, exists := w.vptUnpackingFunctions[attr.Format]; exists {
				continue
			}
			name, byteCount, dimension := w.writeUnpackingFunction(attr.Format)
			if name != "" {
				w.vptUnpackingFunctions[attr.Format] = vptUnpackFunc{
					name:      name,
					byteCount: byteCount,
					dimension: dimension,
				}
			}
		}
	}
}

// writeUnpackingFunction writes a single unpacking function and returns (name, byteCount, dimension).
// Matches Rust naga writer.rs write_unpacking_function exactly.
func (w *Writer) writeUnpackingFunction(format VertexFormat) (string, uint32, uint32) {
	switch format {
	case VertexFormatUint8:
		name := w.namer.call("unpackUint8")
		w.out.WriteString(fmt.Sprintf("uint %s(metal::uchar b0) {\n", name))
		w.out.WriteString("    return uint(b0);\n}\n")
		return name, 1, 1
	case VertexFormatUint8x2:
		name := w.namer.call("unpackUint8x2")
		w.out.WriteString(fmt.Sprintf("metal::uint2 %s(metal::uchar b0, metal::uchar b1) {\n", name))
		w.out.WriteString("    return metal::uint2(b0, b1);\n}\n")
		return name, 2, 2
	case VertexFormatUint8x4:
		name := w.namer.call("unpackUint8x4")
		w.out.WriteString(fmt.Sprintf("metal::uint4 %s(metal::uchar b0, metal::uchar b1, metal::uchar b2, metal::uchar b3) {\n", name))
		w.out.WriteString("    return metal::uint4(b0, b1, b2, b3);\n}\n")
		return name, 4, 4
	case VertexFormatSint8:
		name := w.namer.call("unpackSint8")
		w.out.WriteString(fmt.Sprintf("int %s(metal::uchar b0) {\n", name))
		w.out.WriteString("    return int(as_type<char>(b0));\n}\n")
		return name, 1, 1
	case VertexFormatSint8x2:
		name := w.namer.call("unpackSint8x2")
		w.out.WriteString(fmt.Sprintf("metal::int2 %s(metal::uchar b0, metal::uchar b1) {\n", name))
		w.out.WriteString("    return metal::int2(as_type<char>(b0), as_type<char>(b1));\n}\n")
		return name, 2, 2
	case VertexFormatSint8x4:
		name := w.namer.call("unpackSint8x4")
		w.out.WriteString(fmt.Sprintf("metal::int4 %s(metal::uchar b0, metal::uchar b1, metal::uchar b2, metal::uchar b3) {\n", name))
		w.out.WriteString("    return metal::int4(as_type<char>(b0), as_type<char>(b1), as_type<char>(b2), as_type<char>(b3));\n}\n")
		return name, 4, 4
	case VertexFormatUnorm8:
		name := w.namer.call("unpackUnorm8")
		w.out.WriteString(fmt.Sprintf("float %s(metal::uchar b0) {\n", name))
		w.out.WriteString("    return float(float(b0) / 255.0f);\n}\n")
		return name, 1, 1
	case VertexFormatUnorm8x2:
		name := w.namer.call("unpackUnorm8x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(metal::uchar b0, metal::uchar b1) {\n", name))
		w.out.WriteString("    return metal::float2(float(b0) / 255.0f, float(b1) / 255.0f);\n}\n")
		return name, 2, 2
	case VertexFormatUnorm8x4:
		name := w.namer.call("unpackUnorm8x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(metal::uchar b0, metal::uchar b1, metal::uchar b2, metal::uchar b3) {\n", name))
		w.out.WriteString("    return metal::float4(float(b0) / 255.0f, float(b1) / 255.0f, float(b2) / 255.0f, float(b3) / 255.0f);\n}\n")
		return name, 4, 4
	case VertexFormatSnorm8:
		name := w.namer.call("unpackSnorm8")
		w.out.WriteString(fmt.Sprintf("float %s(metal::uchar b0) {\n", name))
		w.out.WriteString("    return float(metal::max(-1.0f, as_type<char>(b0) / 127.0f));\n}\n")
		return name, 1, 1
	case VertexFormatSnorm8x2:
		name := w.namer.call("unpackSnorm8x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(metal::uchar b0, metal::uchar b1) {\n", name))
		w.out.WriteString("    return metal::float2(metal::max(-1.0f, as_type<char>(b0) / 127.0f), metal::max(-1.0f, as_type<char>(b1) / 127.0f));\n}\n")
		return name, 2, 2
	case VertexFormatSnorm8x4:
		name := w.namer.call("unpackSnorm8x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(metal::uchar b0, metal::uchar b1, metal::uchar b2, metal::uchar b3) {\n", name))
		w.out.WriteString("    return metal::float4(metal::max(-1.0f, as_type<char>(b0) / 127.0f), metal::max(-1.0f, as_type<char>(b1) / 127.0f), metal::max(-1.0f, as_type<char>(b2) / 127.0f), metal::max(-1.0f, as_type<char>(b3) / 127.0f));\n}\n")
		return name, 4, 4
	case VertexFormatUint16:
		name := w.namer.call("unpackUint16")
		w.out.WriteString(fmt.Sprintf("metal::uint %s(metal::uint b0, metal::uint b1) {\n", name))
		w.out.WriteString("    return metal::uint(b1 << 8 | b0);\n}\n")
		return name, 2, 1
	case VertexFormatUint16x2:
		name := w.namer.call("unpackUint16x2")
		w.out.WriteString(fmt.Sprintf("metal::uint2 %s(metal::uint b0, metal::uint b1, metal::uint b2, metal::uint b3) {\n", name))
		w.out.WriteString("    return metal::uint2(b1 << 8 | b0, b3 << 8 | b2);\n}\n")
		return name, 4, 2
	case VertexFormatUint16x4:
		name := w.namer.call("unpackUint16x4")
		w.out.WriteString(fmt.Sprintf("metal::uint4 %s(metal::uint b0, metal::uint b1, metal::uint b2, metal::uint b3, metal::uint b4, metal::uint b5, metal::uint b6, metal::uint b7) {\n", name))
		w.out.WriteString("    return metal::uint4(b1 << 8 | b0, b3 << 8 | b2, b5 << 8 | b4, b7 << 8 | b6);\n}\n")
		return name, 8, 4
	case VertexFormatSint16:
		name := w.namer.call("unpackSint16")
		w.out.WriteString(fmt.Sprintf("int %s(metal::ushort b0, metal::ushort b1) {\n", name))
		w.out.WriteString("    return int(as_type<short>(metal::ushort(b1 << 8 | b0)));\n}\n")
		return name, 2, 1
	case VertexFormatSint16x2:
		name := w.namer.call("unpackSint16x2")
		w.out.WriteString(fmt.Sprintf("metal::int2 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3) {\n", name))
		w.out.WriteString("    return metal::int2(as_type<short>(metal::ushort(b1 << 8 | b0)), as_type<short>(metal::ushort(b3 << 8 | b2)));\n}\n")
		return name, 4, 2
	case VertexFormatSint16x4:
		name := w.namer.call("unpackSint16x4")
		w.out.WriteString(fmt.Sprintf("metal::int4 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3, metal::ushort b4, metal::ushort b5, metal::ushort b6, metal::ushort b7) {\n", name))
		w.out.WriteString("    return metal::int4(as_type<short>(metal::ushort(b1 << 8 | b0)), as_type<short>(metal::ushort(b3 << 8 | b2)), as_type<short>(metal::ushort(b5 << 8 | b4)), as_type<short>(metal::ushort(b7 << 8 | b6)));\n}\n")
		return name, 8, 4
	case VertexFormatUnorm16:
		name := w.namer.call("unpackUnorm16")
		w.out.WriteString(fmt.Sprintf("float %s(metal::ushort b0, metal::ushort b1) {\n", name))
		w.out.WriteString("    return float(float(b1 << 8 | b0) / 65535.0f);\n}\n")
		return name, 2, 1
	case VertexFormatUnorm16x2:
		name := w.namer.call("unpackUnorm16x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3) {\n", name))
		w.out.WriteString("    return metal::float2(float(b1 << 8 | b0) / 65535.0f, float(b3 << 8 | b2) / 65535.0f);\n}\n")
		return name, 4, 2
	case VertexFormatUnorm16x4:
		name := w.namer.call("unpackUnorm16x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3, metal::ushort b4, metal::ushort b5, metal::ushort b6, metal::ushort b7) {\n", name))
		w.out.WriteString("    return metal::float4(float(b1 << 8 | b0) / 65535.0f, float(b3 << 8 | b2) / 65535.0f, float(b5 << 8 | b4) / 65535.0f, float(b7 << 8 | b6) / 65535.0f);\n}\n")
		return name, 8, 4
	case VertexFormatSnorm16:
		name := w.namer.call("unpackSnorm16")
		w.out.WriteString(fmt.Sprintf("float %s(metal::ushort b0, metal::ushort b1) {\n", name))
		w.out.WriteString("    return metal::unpack_snorm2x16_to_float(b1 << 8 | b0).x;\n}\n")
		return name, 2, 1
	case VertexFormatSnorm16x2:
		name := w.namer.call("unpackSnorm16x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(uint b0, uint b1, uint b2, uint b3) {\n", name))
		w.out.WriteString("    return metal::unpack_snorm2x16_to_float(b3 << 24 | b2 << 16 | b1 << 8 | b0);\n}\n")
		return name, 4, 2
	case VertexFormatSnorm16x4:
		name := w.namer.call("unpackSnorm16x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7) {\n", name))
		w.out.WriteString("    return metal::float4(metal::unpack_snorm2x16_to_float(b3 << 24 | b2 << 16 | b1 << 8 | b0), metal::unpack_snorm2x16_to_float(b7 << 24 | b6 << 16 | b5 << 8 | b4));\n}\n")
		return name, 8, 4
	case VertexFormatFloat16:
		name := w.namer.call("unpackFloat16")
		w.out.WriteString(fmt.Sprintf("float %s(metal::ushort b0, metal::ushort b1) {\n", name))
		w.out.WriteString("    return float(as_type<half>(metal::ushort(b1 << 8 | b0)));\n}\n")
		return name, 2, 1
	case VertexFormatFloat16x2:
		name := w.namer.call("unpackFloat16x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3) {\n", name))
		w.out.WriteString("    return metal::float2(as_type<half>(metal::ushort(b1 << 8 | b0)), as_type<half>(metal::ushort(b3 << 8 | b2)));\n}\n")
		return name, 4, 2
	case VertexFormatFloat16x4:
		name := w.namer.call("unpackFloat16x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(metal::ushort b0, metal::ushort b1, metal::ushort b2, metal::ushort b3, metal::ushort b4, metal::ushort b5, metal::ushort b6, metal::ushort b7) {\n", name))
		w.out.WriteString("    return metal::float4(as_type<half>(metal::ushort(b1 << 8 | b0)), as_type<half>(metal::ushort(b3 << 8 | b2)), as_type<half>(metal::ushort(b5 << 8 | b4)), as_type<half>(metal::ushort(b7 << 8 | b6)));\n}\n")
		return name, 8, 4
	case VertexFormatFloat32:
		name := w.namer.call("unpackFloat32")
		w.out.WriteString(fmt.Sprintf("float %s(uint b0, uint b1, uint b2, uint b3) {\n", name))
		w.out.WriteString("    return as_type<float>(b3 << 24 | b2 << 16 | b1 << 8 | b0);\n}\n")
		return name, 4, 1
	case VertexFormatFloat32x2:
		name := w.namer.call("unpackFloat32x2")
		w.out.WriteString(fmt.Sprintf("metal::float2 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7) {\n", name))
		w.out.WriteString("    return metal::float2(as_type<float>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<float>(b7 << 24 | b6 << 16 | b5 << 8 | b4));\n}\n")
		return name, 8, 2
	case VertexFormatFloat32x3:
		name := w.namer.call("unpackFloat32x3")
		w.out.WriteString(fmt.Sprintf("metal::float3 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11) {\n", name))
		w.out.WriteString("    return metal::float3(as_type<float>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<float>(b7 << 24 | b6 << 16 | b5 << 8 | b4), as_type<float>(b11 << 24 | b10 << 16 | b9 << 8 | b8));\n}\n")
		return name, 12, 3
	case VertexFormatFloat32x4:
		name := w.namer.call("unpackFloat32x4")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11, uint b12, uint b13, uint b14, uint b15) {\n", name))
		w.out.WriteString("    return metal::float4(as_type<float>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<float>(b7 << 24 | b6 << 16 | b5 << 8 | b4), as_type<float>(b11 << 24 | b10 << 16 | b9 << 8 | b8), as_type<float>(b15 << 24 | b14 << 16 | b13 << 8 | b12));\n}\n")
		return name, 16, 4
	case VertexFormatUint32:
		name := w.namer.call("unpackUint32")
		w.out.WriteString(fmt.Sprintf("uint %s(uint b0, uint b1, uint b2, uint b3) {\n", name))
		w.out.WriteString("    return (b3 << 24 | b2 << 16 | b1 << 8 | b0);\n}\n")
		return name, 4, 1
	case VertexFormatUint32x2:
		name := w.namer.call("unpackUint32x2")
		w.out.WriteString(fmt.Sprintf("uint2 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7) {\n", name))
		w.out.WriteString("    return uint2((b3 << 24 | b2 << 16 | b1 << 8 | b0), (b7 << 24 | b6 << 16 | b5 << 8 | b4));\n}\n")
		return name, 8, 2
	case VertexFormatUint32x3:
		name := w.namer.call("unpackUint32x3")
		w.out.WriteString(fmt.Sprintf("uint3 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11) {\n", name))
		w.out.WriteString("    return uint3((b3 << 24 | b2 << 16 | b1 << 8 | b0), (b7 << 24 | b6 << 16 | b5 << 8 | b4), (b11 << 24 | b10 << 16 | b9 << 8 | b8));\n}\n")
		return name, 12, 3
	case VertexFormatUint32x4:
		name := w.namer.call("unpackUint32x4")
		w.out.WriteString(fmt.Sprintf("metal::uint4 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11, uint b12, uint b13, uint b14, uint b15) {\n", name))
		w.out.WriteString("    return metal::uint4((b3 << 24 | b2 << 16 | b1 << 8 | b0), (b7 << 24 | b6 << 16 | b5 << 8 | b4), (b11 << 24 | b10 << 16 | b9 << 8 | b8), (b15 << 24 | b14 << 16 | b13 << 8 | b12));\n}\n")
		return name, 16, 4
	case VertexFormatSint32:
		name := w.namer.call("unpackSint32")
		w.out.WriteString(fmt.Sprintf("int %s(uint b0, uint b1, uint b2, uint b3) {\n", name))
		w.out.WriteString("    return as_type<int>(b3 << 24 | b2 << 16 | b1 << 8 | b0);\n}\n")
		return name, 4, 1
	case VertexFormatSint32x2:
		name := w.namer.call("unpackSint32x2")
		w.out.WriteString(fmt.Sprintf("metal::int2 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7) {\n", name))
		w.out.WriteString("    return metal::int2(as_type<int>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<int>(b7 << 24 | b6 << 16 | b5 << 8 | b4));\n}\n")
		return name, 8, 2
	case VertexFormatSint32x3:
		name := w.namer.call("unpackSint32x3")
		w.out.WriteString(fmt.Sprintf("metal::int3 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11) {\n", name))
		w.out.WriteString("    return metal::int3(as_type<int>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<int>(b7 << 24 | b6 << 16 | b5 << 8 | b4), as_type<int>(b11 << 24 | b10 << 16 | b9 << 8 | b8));\n}\n")
		return name, 12, 3
	case VertexFormatSint32x4:
		name := w.namer.call("unpackSint32x4")
		w.out.WriteString(fmt.Sprintf("metal::int4 %s(uint b0, uint b1, uint b2, uint b3, uint b4, uint b5, uint b6, uint b7, uint b8, uint b9, uint b10, uint b11, uint b12, uint b13, uint b14, uint b15) {\n", name))
		w.out.WriteString("    return metal::int4(as_type<int>(b3 << 24 | b2 << 16 | b1 << 8 | b0), as_type<int>(b7 << 24 | b6 << 16 | b5 << 8 | b4), as_type<int>(b11 << 24 | b10 << 16 | b9 << 8 | b8), as_type<int>(b15 << 24 | b14 << 16 | b13 << 8 | b12));\n}\n")
		return name, 16, 4
	case VertexFormatUnorm10_10_10_2:
		name := w.namer.call("unpackUnorm10_10_10_2")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(uint b0, uint b1, uint b2, uint b3) {\n", name))
		w.out.WriteString("    return metal::unpack_unorm10a2_to_float(b3 << 24 | b2 << 16 | b1 << 8 | b0);\n}\n")
		return name, 4, 4
	case VertexFormatUnorm8x4Bgra:
		name := w.namer.call("unpackUnorm8x4Bgra")
		w.out.WriteString(fmt.Sprintf("metal::float4 %s(metal::uchar b0, metal::uchar b1, metal::uchar b2, metal::uchar b3) {\n", name))
		w.out.WriteString("    return metal::float4(float(b2) / 255.0f, float(b1) / 255.0f, float(b0) / 255.0f, float(b3) / 255.0f);\n}\n")
		return name, 4, 4
	}
	return "", 0, 0
}

// vptVertexInputDimension returns the number of components for a type used as a vertex input.
// Matches Rust naga TypeContext::vertex_input_dimension.
func (w *Writer) vptVertexInputDimension(tyHandle ir.TypeHandle) uint32 {
	if int(tyHandle) >= len(w.module.Types) {
		return 1
	}
	switch inner := w.module.Types[tyHandle].Inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return uint32(inner.Size)
	}
	return 1
}

// vptTypeIsInt returns true if the scalar type of tyHandle is integer (signed, unsigned, bool).
// Matches Rust naga's scalar_is_int.
func (w *Writer) vptTypeIsInt(tyHandle ir.TypeHandle) bool {
	if int(tyHandle) >= len(w.module.Types) {
		return false
	}
	var kind ir.ScalarKind
	found := false
	switch inner := w.module.Types[tyHandle].Inner.(type) {
	case ir.ScalarType:
		kind = inner.Kind
		found = true
	case ir.VectorType:
		kind = inner.Scalar.Kind
		found = true
	}
	if !found {
		return false
	}
	switch kind {
	case ir.ScalarSint, ir.ScalarUint, ir.ScalarAbstractInt, ir.ScalarBool:
		return true
	}
	return false
}

// vptFormatDebugName returns the debug name for a VertexFormat (used in comments).
func vptFormatDebugName(format VertexFormat) string {
	switch format {
	case VertexFormatUint8:
		return "Uint8"
	case VertexFormatUint8x2:
		return "Uint8x2"
	case VertexFormatUint8x4:
		return "Uint8x4"
	case VertexFormatSint8:
		return "Sint8"
	case VertexFormatSint8x2:
		return "Sint8x2"
	case VertexFormatSint8x4:
		return "Sint8x4"
	case VertexFormatUnorm8:
		return "Unorm8"
	case VertexFormatUnorm8x2:
		return "Unorm8x2"
	case VertexFormatUnorm8x4:
		return "Unorm8x4"
	case VertexFormatSnorm8:
		return "Snorm8"
	case VertexFormatSnorm8x2:
		return "Snorm8x2"
	case VertexFormatSnorm8x4:
		return "Snorm8x4"
	case VertexFormatUint16:
		return "Uint16"
	case VertexFormatUint16x2:
		return "Uint16x2"
	case VertexFormatUint16x4:
		return "Uint16x4"
	case VertexFormatSint16:
		return "Sint16"
	case VertexFormatSint16x2:
		return "Sint16x2"
	case VertexFormatSint16x4:
		return "Sint16x4"
	case VertexFormatUnorm16:
		return "Unorm16"
	case VertexFormatUnorm16x2:
		return "Unorm16x2"
	case VertexFormatUnorm16x4:
		return "Unorm16x4"
	case VertexFormatSnorm16:
		return "Snorm16"
	case VertexFormatSnorm16x2:
		return "Snorm16x2"
	case VertexFormatSnorm16x4:
		return "Snorm16x4"
	case VertexFormatFloat16:
		return "Float16"
	case VertexFormatFloat16x2:
		return "Float16x2"
	case VertexFormatFloat16x4:
		return "Float16x4"
	case VertexFormatFloat32:
		return "Float32"
	case VertexFormatFloat32x2:
		return "Float32x2"
	case VertexFormatFloat32x3:
		return "Float32x3"
	case VertexFormatFloat32x4:
		return "Float32x4"
	case VertexFormatUint32:
		return "Uint32"
	case VertexFormatUint32x2:
		return "Uint32x2"
	case VertexFormatUint32x3:
		return "Uint32x3"
	case VertexFormatUint32x4:
		return "Uint32x4"
	case VertexFormatSint32:
		return "Sint32"
	case VertexFormatSint32x2:
		return "Sint32x2"
	case VertexFormatSint32x3:
		return "Sint32x3"
	case VertexFormatSint32x4:
		return "Sint32x4"
	case VertexFormatUnorm10_10_10_2:
		return "Unorm10_10_10_2"
	case VertexFormatUnorm8x4Bgra:
		return "Unorm8x4Bgra"
	}
	return "Unknown"
}

// writeVPTBufferSizesStruct writes the _mslBufferSizes struct with members for VPT buffers.
// When VPT is active, each vertex buffer gets a buffer_sizeN member.
// This replaces the normal buffer sizes struct when VPT is the only reason for it.
func (w *Writer) vptBufferSizeMembers() []uint32 {
	if len(w.vptBufferMappings) == 0 {
		return nil
	}
	var ids []uint32
	for _, vbm := range w.vptBufferMappings {
		ids = append(ids, vbm.id)
	}
	return ids
}

// writeVPTEntryPointInputStruct handles the input struct for a VPT-enabled entry point.
// Instead of emitting a stage_in struct, it builds the attribute mapping (am_resolved)
// that will be used later to emit zero-init + bounds-check + unpacking code.
// Returns the am_resolved map.
// Matches Rust naga writer.rs ~6818-6862.
func (w *Writer) writeVPTEntryPointInputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) map[uint32]vptAttributeResolved {
	amResolved := make(map[uint32]vptAttributeResolved)

	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})
	// Must call namer.call for Input name even though we won't emit the struct.
	_ = w.namer.call(epName + "Input")

	// Flatten arguments and find location-bound members.
	for _, arg := range fn.Arguments {
		if arg.Binding != nil {
			// Direct argument — skip non-location bindings
			if loc, ok := (*arg.Binding).(ir.LocationBinding); ok {
				// This shouldn't happen for VPT (struct args are typical), but handle it
				tyName := w.writeTypeName(arg.Type, StorageAccess(0))
				dim := w.vptVertexInputDimension(arg.Type)
				isInt := w.vptTypeIsInt(arg.Type)
				// For direct args, we need to generate a name via namer
				argKey := nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(0)}
				name := w.getName(argKey)
				freshName := w.namer.call(name)
				amResolved[loc.Location] = vptAttributeResolved{
					tyName:    tyName,
					dimension: dim,
					tyIsInt:   isInt,
					name:      freshName,
				}
			}
			continue
		}
		// Struct argument — flatten members
		if int(arg.Type) >= len(w.module.Types) {
			continue
		}
		st, ok := w.module.Types[arg.Type].Inner.(ir.StructType)
		if !ok {
			continue
		}
		for memberIdx, member := range st.Members {
			key := nameKey{kind: nameKeyStructMember, handle1: uint32(arg.Type), handle2: uint32(memberIdx)}
			baseName := w.getName(key)

			if member.Binding != nil {
				if loc, ok := (*member.Binding).(ir.LocationBinding); ok {
					// Location member — use namer.call (not varyings namer) for VPT
					freshName := w.namer.call(baseName)
					w.flattenedMemberNames[key] = freshName
					tyName := w.writeTypeName(member.Type, StorageAccess(0))
					dim := w.vptVertexInputDimension(member.Type)
					isInt := w.vptTypeIsInt(member.Type)
					amResolved[loc.Location] = vptAttributeResolved{
						tyName:    tyName,
						dimension: dim,
						tyIsInt:   isInt,
						name:      freshName,
					}
				} else {
					// Builtin member
					freshName := w.namer.call(baseName)
					w.flattenedMemberNames[key] = freshName
				}
			}
		}
	}

	return amResolved
}

// writeVPTBufferTypeStructs emits the vb_N_type struct definitions for VPT.
// Matches Rust naga writer.rs ~6945-6957.
func (w *Writer) writeVPTBufferTypeStructs() {
	for _, vbm := range w.vptBufferMappings {
		w.write("struct %s { %suchar data[%d]; };\n", vbm.tyName, Namespace, vbm.stride)
	}
}

// writeVPTFunctionParams writes VPT-specific entry point parameters:
// [[vertex_id]], [[instance_id]], buffer pointers, and _mslBufferSizes.
// Matches Rust naga writer.rs ~7250-7286.
func (w *Writer) writeVPTFunctionParams(ep *ir.EntryPoint, fn *ir.Function, paramCount *int,
	vExistingID, iExistingID string) {
	if w.vptNeedsVertexID && vExistingID == "" {
		w.writeEntryPointParam(*paramCount, fmt.Sprintf("uint %s [[vertex_id]]", w.vptVertexIDName))
		*paramCount++
	}
	if w.vptNeedsInstanceID && iExistingID == "" {
		w.writeEntryPointParam(*paramCount, fmt.Sprintf("uint %s [[instance_id]]", w.vptInstanceIDName))
		*paramCount++
	}

	for _, vbm := range w.vptBufferMappings {
		w.writeEntryPointParam(*paramCount, fmt.Sprintf("const device %s* %s [[buffer(%d)]]",
			vbm.tyName, vbm.paramName, vbm.id))
		*paramCount++
	}
}

// writeVPTBodyPrologue writes the VPT body prologue: zero-init attributes,
// bounds check, buffer element read, and attribute unpacking.
// Matches Rust naga writer.rs ~7292-7438.
func (w *Writer) writeVPTBodyPrologue(amResolved map[uint32]vptAttributeResolved,
	vExistingID, iExistingID string) {
	for _, vbm := range w.vptBufferMappings {
		// Zero-initialize all used attributes for this buffer.
		for _, attr := range vbm.attributes {
			am, ok := amResolved[attr.ShaderLocation]
			if !ok {
				continue
			}
			w.writeLine("%s %s = {};", am.tyName, am.name)
		}

		// Bounds check block.
		var indexName string
		switch vbm.stepMode {
		case VertexStepModeConstant:
			indexName = "0"
		case VertexStepModeByVertex:
			if vExistingID != "" {
				indexName = vExistingID
			} else {
				indexName = w.vptVertexIDName
			}
		case VertexStepModeByInstance:
			if iExistingID != "" {
				indexName = iExistingID
			} else {
				indexName = w.vptInstanceIDName
			}
		}

		w.writeLine("if (%s < (_buffer_sizes.buffer_size%d / %d)) {", indexName, vbm.id, vbm.stride)
		w.pushIndent()

		// Read buffer element.
		w.writeLine("const %s %s = %s[%s];", vbm.tyName, vbm.elemName, vbm.paramName, indexName)

		// Unpack each attribute.
		for _, attr := range vbm.attributes {
			am, ok := amResolved[attr.ShaderLocation]
			if !ok {
				continue
			}

			unpackFunc, ok := w.vptUnpackingFunctions[attr.Format]
			if !ok {
				continue
			}

			// Check dimensionality: attribute dimension vs unpack dimension.
			needsPaddingOrTruncation := 0 // 0=equal, 1=padding (attr>unpack), -1=truncation (attr<unpack)
			if am.dimension > unpackFunc.dimension {
				needsPaddingOrTruncation = 1
			} else if am.dimension < unpackFunc.dimension {
				needsPaddingOrTruncation = -1
			}

			if needsPaddingOrTruncation != 0 {
				w.writeLine("// %s <- %s", am.tyName, vptFormatDebugName(attr.Format))
			}

			// Build the unpack call.
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%s = ", am.name))

			if needsPaddingOrTruncation > 0 {
				sb.WriteString(fmt.Sprintf("%s(", am.tyName))
			}

			sb.WriteString(fmt.Sprintf("%s(%s.data[%d]", unpackFunc.name, vbm.elemName, attr.Offset))
			for i := attr.Offset + 1; i < attr.Offset+unpackFunc.byteCount; i++ {
				sb.WriteString(fmt.Sprintf(", %s.data[%d]", vbm.elemName, i))
			}
			sb.WriteString(")")

			switch {
			case needsPaddingOrTruncation > 0:
				// Padding: fill remaining components.
				zeroVal := "0.0"
				oneVal := "1.0"
				if am.tyIsInt {
					zeroVal = "0"
					oneVal = "1"
				}
				for i := unpackFunc.dimension; i < am.dimension; i++ {
					if i == 3 {
						sb.WriteString(fmt.Sprintf(", %s", oneVal))
					} else {
						sb.WriteString(fmt.Sprintf(", %s", zeroVal))
					}
				}
				sb.WriteString(")")
			case needsPaddingOrTruncation < 0:
				// Truncate
				swizzle := "xyzw"[:am.dimension]
				sb.WriteString(fmt.Sprintf(".%s", swizzle))
			}

			sb.WriteString(";")
			w.writeLine("%s", sb.String())
		}

		w.popIndent()
		w.writeLine("}")
	}
}
