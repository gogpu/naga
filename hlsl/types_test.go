// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestScalarTypeToHLSL tests scalar type conversion.
func TestScalarTypeToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		scalar   ir.ScalarType
		expected string
	}{
		// Bool
		{"bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},

		// Signed integers
		{"int8", ir.ScalarType{Kind: ir.ScalarSint, Width: 1}, "int"},
		{"int16", ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "int"},
		{"int32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"int64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "int64_t"},

		// Unsigned integers
		{"uint8", ir.ScalarType{Kind: ir.ScalarUint, Width: 1}, "uint"},
		{"uint16", ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, "uint"},
		{"uint32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"uint64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "uint64_t"},

		// Floats
		{"half", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{"float", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"double", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarTypeToHLSL(tt.scalar)
			if got != tt.expected {
				t.Errorf("scalarTypeToHLSL(%v) = %q, want %q", tt.scalar, got, tt.expected)
			}
		})
	}
}

// TestVectorTypeToHLSL tests vector type conversion.
func TestVectorTypeToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		vec      ir.VectorType
		expected string
	}{
		// Float vectors
		{"float2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float2"},
		{"float3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3"},
		{"float4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4"},

		// Int vectors
		{"int2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int2"},
		{"int3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int3"},
		{"int4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int4"},

		// Uint vectors
		{"uint2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint2"},
		{"uint3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint3"},
		{"uint4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint4"},

		// Half vectors
		{"half2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half2"},
		{"half4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half4"},

		// Bool vectors
		{"bool2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bool2"},
		{"bool4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bool4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vectorTypeToHLSL(tt.vec)
			if got != tt.expected {
				t.Errorf("vectorTypeToHLSL(%v) = %q, want %q", tt.vec, got, tt.expected)
			}
		})
	}
}

// TestMatrixTypeToHLSL tests matrix type conversion.
func TestMatrixTypeToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		mat      ir.MatrixType
		expected string
	}{
		// Float matrices
		{"float2x2", ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float2x2"},
		{"float3x3", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3x3"},
		{"float4x4", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4x4"},

		// Non-square matrices
		{"float2x3", ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float2x3"},
		{"float3x4", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3x4"},
		{"float4x2", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4x2"},

		// Half matrices
		{"half3x3", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half3x3"},
		{"half4x4", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half4x4"},

		// Double matrices
		{"double4x4", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "double4x4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matrixTypeToHLSL(tt.mat)
			if got != tt.expected {
				t.Errorf("matrixTypeToHLSL(%v) = %q, want %q", tt.mat, got, tt.expected)
			}
		})
	}
}

// TestSamplerTypeToHLSL tests sampler type conversion.
func TestSamplerTypeToHLSL(t *testing.T) {
	tests := []struct {
		name       string
		comparison bool
		expected   string
	}{
		{"regular sampler", false, "SamplerState"},
		{"comparison sampler", true, "SamplerComparisonState"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := samplerTypeToHLSL(tt.comparison)
			if got != tt.expected {
				t.Errorf("samplerTypeToHLSL(%v) = %q, want %q", tt.comparison, got, tt.expected)
			}
		})
	}
}

// TestImageTypeToHLSL tests image/texture type conversion.
func TestImageTypeToHLSL(t *testing.T) {
	// Create a minimal writer for testing
	module := &ir.Module{}
	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
	}

	tests := []struct {
		name     string
		img      ir.ImageType
		expected string
	}{
		// Sampled textures (float)
		{"Texture1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture1D<float4>"},
		{"Texture2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture2D<float4>"},
		{"Texture3D", ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture3D<float4>"},
		{"TextureCube", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "TextureCube<float4>"},

		// Sampled textures (uint/int)
		{"Texture2D_uint", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarUint}, "Texture2D<uint4>"},
		{"Texture2D_sint", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint}, "Texture2D<int4>"},

		// Array textures
		{"Texture1DArray", ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture1DArray<float4>"},
		{"Texture2DArray", ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture2DArray<float4>"},
		{"TextureCubeArray", ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "TextureCubeArray<float4>"},
		// 3D cannot be arrayed
		{"Texture3D_no_array", ir.ImageType{Dim: ir.Dim3D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture3D<float4>"},

		// Multisampled textures
		{"Texture2DMS", ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture2DMS<float4>"},
		{"Texture2DMSArray", ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, "Texture2DMSArray<float4>"},

		// Depth textures
		{"Texture2D_depth", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth}, "Texture2D<float>"},
		{"Texture2DArray_depth", ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassDepth}, "Texture2DArray<float>"},
		{"TextureCube_depth", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassDepth}, "TextureCube<float>"},

		// Storage textures (RW)
		{"RWTexture1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassStorage}, "RWTexture1D<float4>"},
		{"RWTexture2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage}, "RWTexture2D<float4>"},
		{"RWTexture3D", ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassStorage}, "RWTexture3D<float4>"},
		{"RWTexture2DArray", ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassStorage}, "RWTexture2DArray<float4>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.imageTypeToHLSL(tt.img)
			if got != tt.expected {
				t.Errorf("imageTypeToHLSL(%+v) = %q, want %q", tt.img, got, tt.expected)
			}
		})
	}
}

// TestStructsEqual tests struct equality comparison.
func TestStructsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        ir.StructType
		b        ir.StructType
		expected bool
	}{
		{
			"empty structs equal",
			ir.StructType{Members: []ir.StructMember{}},
			ir.StructType{Members: []ir.StructMember{}},
			true,
		},
		{
			"same members",
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 0, Offset: 0},
				{Name: "y", Type: 0, Offset: 4},
			}},
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 0, Offset: 0},
				{Name: "y", Type: 0, Offset: 4},
			}},
			true,
		},
		{
			"different member count",
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 0, Offset: 0},
			}},
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 0, Offset: 0},
				{Name: "y", Type: 0, Offset: 4},
			}},
			false,
		},
		{
			"different member names",
			ir.StructType{Members: []ir.StructMember{
				{Name: "a", Type: 0, Offset: 0},
			}},
			ir.StructType{Members: []ir.StructMember{
				{Name: "b", Type: 0, Offset: 0},
			}},
			false,
		},
		{
			"different member types",
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 0, Offset: 0},
			}},
			ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 1, Offset: 0},
			}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := structsEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("structsEqual() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestFormatFloat32 tests float32 formatting.
func TestFormatFloat32(t *testing.T) {
	tests := []struct {
		name     string
		value    float32
		expected string
	}{
		{"zero", 0.0, "0.0"},
		{"one", 1.0, "1.0"},
		{"negative", -1.0, "-1.0"},
		{"small", 0.5, "0.5"},
		{"large", 1000000.0, "1e+06"},
		{"small_exp", 0.0001, "0.0001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat32(tt.value)
			if got != tt.expected {
				t.Errorf("formatFloat32(%v) = %q, want %q", tt.value, got, tt.expected)
			}
		})
	}
}

// TestFormatFloat64 tests float64 formatting.
func TestFormatFloat64(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"zero", 0.0, "0.0"},
		{"one", 1.0, "1.0"},
		{"negative", -1.0, "-1.0"},
		{"small", 0.5, "0.5"},
		{"large", 1e15, "1e+15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat64(tt.value)
			if got != tt.expected {
				t.Errorf("formatFloat64(%v) = %q, want %q", tt.value, got, tt.expected)
			}
		})
	}
}

// TestWriteBufferType tests buffer type generation.
func TestWriteBufferType(t *testing.T) {
	module := &ir.Module{}
	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
	}

	tests := []struct {
		name     string
		typeName string
		readOnly bool
		expected string
	}{
		{"read-write float4", "float4", false, "RWStructuredBuffer<float4>"},
		{"read-only float4", "float4", true, "StructuredBuffer<float4>"},
		{"read-write MyStruct", "MyStruct", false, "RWStructuredBuffer<MyStruct>"},
		{"read-only MyStruct", "MyStruct", true, "StructuredBuffer<MyStruct>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeBufferType(tt.typeName, nil, tt.readOnly)
			if got != tt.expected {
				t.Errorf("writeBufferType(%q, nil, %v) = %q, want %q", tt.typeName, tt.readOnly, got, tt.expected)
			}
		})
	}
}

// TestWriteByteAddressBufferType tests byte address buffer type generation.
func TestWriteByteAddressBufferType(t *testing.T) {
	module := &ir.Module{}
	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
	}

	tests := []struct {
		name     string
		readOnly bool
		expected string
	}{
		{"read-write", false, "RWByteAddressBuffer"},
		{"read-only", true, "ByteAddressBuffer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeByteAddressBufferType(tt.readOnly)
			if got != tt.expected {
				t.Errorf("writeByteAddressBufferType(%v) = %q, want %q", tt.readOnly, got, tt.expected)
			}
		})
	}
}

// TestTypeClassification tests type classification helper functions.
func TestTypeClassification(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "scalar", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vector", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "matrix", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "struct", Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}}},
			{Name: "array_const", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: ptrUint32(10)}}},
			{Name: "array_runtime", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}}},
		},
	}

	t.Run("isScalarType", func(t *testing.T) {
		if !isScalarType(module, 0) {
			t.Error("expected type 0 to be scalar")
		}
		if isScalarType(module, 1) {
			t.Error("expected type 1 to not be scalar")
		}
	})

	t.Run("isVectorType", func(t *testing.T) {
		if !isVectorType(module, 1) {
			t.Error("expected type 1 to be vector")
		}
		if isVectorType(module, 0) {
			t.Error("expected type 0 to not be vector")
		}
	})

	t.Run("isMatrixType", func(t *testing.T) {
		if !isMatrixType(module, 2) {
			t.Error("expected type 2 to be matrix")
		}
		if isMatrixType(module, 0) {
			t.Error("expected type 0 to not be matrix")
		}
	})

	t.Run("isStructType", func(t *testing.T) {
		if !isStructType(module, 3) {
			t.Error("expected type 3 to be struct")
		}
		if isStructType(module, 0) {
			t.Error("expected type 0 to not be struct")
		}
	})

	t.Run("isArrayType", func(t *testing.T) {
		if !isArrayType(module, 4) {
			t.Error("expected type 4 to be array")
		}
		if isArrayType(module, 0) {
			t.Error("expected type 0 to not be array")
		}
	})

	t.Run("isRuntimeArray", func(t *testing.T) {
		if isRuntimeArray(module, 4) {
			t.Error("expected type 4 to not be runtime array")
		}
		if !isRuntimeArray(module, 5) {
			t.Error("expected type 5 to be runtime array")
		}
	})

	t.Run("getScalarKind", func(t *testing.T) {
		kind, ok := getScalarKind(module, 0)
		if !ok || kind != ir.ScalarFloat {
			t.Errorf("expected ScalarFloat for type 0, got %v, ok=%v", kind, ok)
		}

		kind, ok = getScalarKind(module, 1)
		if !ok || kind != ir.ScalarFloat {
			t.Errorf("expected ScalarFloat for vector type 1, got %v, ok=%v", kind, ok)
		}

		_, ok = getScalarKind(module, 3)
		if ok {
			t.Error("expected no scalar kind for struct type")
		}
	})

	t.Run("getVectorSize", func(t *testing.T) {
		size, ok := getVectorSize(module, 1)
		if !ok || size != ir.Vec4 {
			t.Errorf("expected Vec4 for type 1, got %v, ok=%v", size, ok)
		}

		_, ok = getVectorSize(module, 0)
		if ok {
			t.Error("expected no vector size for scalar type")
		}
	})

	t.Run("getMatrixDimensions", func(t *testing.T) {
		cols, rows, ok := getMatrixDimensions(module, 2)
		if !ok || cols != ir.Vec4 || rows != ir.Vec4 {
			t.Errorf("expected 4x4 for type 2, got %vx%v, ok=%v", cols, rows, ok)
		}

		_, _, ok = getMatrixDimensions(module, 0)
		if ok {
			t.Error("expected no matrix dimensions for scalar type")
		}
	})

	t.Run("getArrayElementType", func(t *testing.T) {
		base, ok := getArrayElementType(module, 4)
		if !ok || base != 0 {
			t.Errorf("expected base type 0 for array, got %v, ok=%v", base, ok)
		}

		_, ok = getArrayElementType(module, 0)
		if ok {
			t.Error("expected no element type for scalar")
		}
	})

	t.Run("getArraySize", func(t *testing.T) {
		size, ok := getArraySize(module, 4)
		if !ok || size == nil || *size != 10 {
			t.Errorf("expected size 10 for const array, got %v, ok=%v", size, ok)
		}

		size, ok = getArraySize(module, 5)
		if !ok || size != nil {
			t.Errorf("expected nil size for runtime array, got %v, ok=%v", size, ok)
		}
	})

	t.Run("out of bounds", func(t *testing.T) {
		if isScalarType(module, 100) {
			t.Error("expected false for out of bounds handle")
		}
		if isVectorType(module, 100) {
			t.Error("expected false for out of bounds handle")
		}
		if isMatrixType(module, 100) {
			t.Error("expected false for out of bounds handle")
		}
		if isStructType(module, 100) {
			t.Error("expected false for out of bounds handle")
		}
		if isArrayType(module, 100) {
			t.Error("expected false for out of bounds handle")
		}
	})
}

// TestGetTypeName tests type name generation for various types.
func TestGetTypeName(t *testing.T) {
	// Create a module with various types
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "MyStruct", Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}}},
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
			{Name: "", Inner: ir.SamplerType{Comparison: true}},
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "", Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
			{Name: "", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
	}

	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
		names:     make(map[nameKey]string),
	}

	tests := []struct {
		name     string
		handle   ir.TypeHandle
		expected string
	}{
		{"scalar float", 0, "float"},
		{"vector float4", 1, "float4"},
		{"matrix float4x4", 2, "float4x4"},
		{"named struct", 3, "MyStruct"},
		{"sampler", 4, "SamplerState"},
		{"comparison sampler", 5, "SamplerComparisonState"},
		{"texture2d", 6, "Texture2D<float4>"},
		{"pointer to float", 7, "float"},
		{"atomic uint", 8, "uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.getTypeName(tt.handle)
			if got != tt.expected {
				t.Errorf("getTypeName(%d) = %q, want %q", tt.handle, got, tt.expected)
			}
		})
	}
}

// TestGetTypeNameWithArraySuffix tests array suffix handling.
func TestGetTypeNameWithArraySuffix(t *testing.T) {
	size10 := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size10}}},
			{Name: "", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}}},
			// Nested array: float[5][10]
			{Name: "", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: ptrUint32(5)}}},
		},
	}

	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
		names:     make(map[nameKey]string),
	}

	tests := []struct {
		name           string
		handle         ir.TypeHandle
		expectedType   string
		expectedSuffix string
	}{
		{"scalar", 0, "float", ""},
		{"const array", 1, "float", "[10]"},
		{"runtime array", 2, "float", "[]"},
		{"nested array", 3, "float", "[5][10]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotSuffix := w.getTypeNameWithArraySuffix(tt.handle)
			if gotType != tt.expectedType {
				t.Errorf("type = %q, want %q", gotType, tt.expectedType)
			}
			if gotSuffix != tt.expectedSuffix {
				t.Errorf("suffix = %q, want %q", gotSuffix, tt.expectedSuffix)
			}
		})
	}
}

// TestWriteCBufferDeclaration tests cbuffer declaration generation.
func TestWriteCBufferDeclaration(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "UniformData", Inner: ir.StructType{Members: nil, Span: 16}}, // 0: struct
		},
	}
	w := &Writer{
		module:           module,
		typeNames:        map[ir.TypeHandle]string{0: "UniformData"},
		names:            make(map[nameKey]string),
		registerBindings: make(map[string]string),
	}

	tests := []struct {
		name         string
		bufName      string
		typeName     string
		typeHandle   ir.TypeHandle
		binding      *BindTarget
		wantContains []string
	}{
		{
			"with binding",
			"uniforms",
			"UniformData",
			ir.TypeHandle(0), // not a matrix → no row_major
			&BindTarget{Register: 0, Space: 0},
			[]string{"cbuffer uniforms : register(b0)", "UniformData uniforms;"},
		},
		{
			"without binding",
			"globals",
			"UniformData", // ignored, type resolved from handle
			ir.TypeHandle(0),
			nil,
			[]string{"cbuffer globals {", "UniformData globals;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.writeCBufferDeclaration(tt.bufName, tt.typeName, tt.typeHandle, tt.binding)
			output := w.out.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q:\n%s", want, output)
				}
			}
		})
	}
}

// TestGetSemanticFromBinding tests semantic generation from bindings.
func TestGetSemanticFromBinding(t *testing.T) {
	module := &ir.Module{}
	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
	}

	tests := []struct {
		name     string
		binding  ir.Binding
		idx      int
		expected string
	}{
		{"position builtin", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, 0, "SV_Position"},
		{"vertex index builtin", ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}, 0, "SV_VertexID"},
		{"instance index builtin", ir.BuiltinBinding{Builtin: ir.BuiltinInstanceIndex}, 0, "SV_InstanceID"},
		{"location 0", ir.LocationBinding{Location: 0}, 0, "LOC0"},
		{"location 3", ir.LocationBinding{Location: 3}, 0, "LOC3"},
		{"nil binding", nil, 5, "LOC5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.getSemanticFromBinding(tt.binding, tt.idx)
			if got != tt.expected {
				t.Errorf("getSemanticFromBinding() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestGetInterpolationModifier tests interpolation modifier generation.
func TestGetInterpolationModifier(t *testing.T) {
	module := &ir.Module{}
	w := &Writer{
		module:    module,
		typeNames: make(map[ir.TypeHandle]string),
	}

	tests := []struct {
		name     string
		binding  ir.Binding
		expected string
	}{
		{"nil binding", nil, ""},
		{"builtin binding", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, ""},
		{"location no interp", ir.LocationBinding{Location: 0, Interpolation: nil}, ""},
		{
			"flat interpolation",
			ir.LocationBinding{
				Location:      0,
				Interpolation: &ir.Interpolation{Kind: ir.InterpolationFlat, Sampling: ir.SamplingCenter},
			},
			"nointerpolation",
		},
		{
			"linear noperspective",
			ir.LocationBinding{
				Location:      0,
				Interpolation: &ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingCenter},
			},
			"noperspective",
		},
		{
			"centroid sampling",
			ir.LocationBinding{
				Location:      0,
				Interpolation: &ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingCentroid},
			},
			"centroid",
		},
		{
			"sample sampling",
			ir.LocationBinding{
				Location:      0,
				Interpolation: &ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingSample},
			},
			"sample",
		},
		{
			"flat with centroid",
			ir.LocationBinding{
				Location:      0,
				Interpolation: &ir.Interpolation{Kind: ir.InterpolationFlat, Sampling: ir.SamplingCentroid},
			},
			"nointerpolation centroid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.getInterpolationModifier(tt.binding)
			if got != tt.expected {
				t.Errorf("getInterpolationModifier() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Helper function to create pointer to uint32
func ptrUint32(v uint32) *uint32 {
	return &v
}

// TestGetInnerMatrixData tests matrix type detection through arrays.
func TestGetInnerMatrixData(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: ptrUint32(2)}, Stride: 32}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	tests := []struct {
		name       string
		handle     ir.TypeHandle
		wantNil    bool
		wantCols   ir.VectorSize
		wantRows   ir.VectorSize
		wantMatCx2 bool
	}{
		{"direct mat3x2", 0, false, ir.Vec3, ir.Vec2, true},
		{"array of mat3x2", 1, false, ir.Vec3, ir.Vec2, true},
		{"scalar (not matrix)", 2, true, 0, 0, false},
		{"mat4x3 (not Cx2)", 3, false, ir.Vec4, ir.Vec3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := getInnerMatrixData(module, tt.handle)
			if tt.wantNil {
				if m != nil {
					t.Errorf("expected nil, got %+v", m)
				}
				return
			}
			if m == nil {
				t.Fatal("expected non-nil")
			}
			if m.columns != tt.wantCols {
				t.Errorf("columns = %d, want %d", m.columns, tt.wantCols)
			}
			if m.rows != tt.wantRows {
				t.Errorf("rows = %d, want %d", m.rows, tt.wantRows)
			}
			if m.isMatCx2() != tt.wantMatCx2 {
				t.Errorf("isMatCx2() = %v, want %v", m.isMatCx2(), tt.wantMatCx2)
			}
		})
	}
}

// TestWriteMatCx2TypedefAndFunctions tests __matCx2 typedef generation.
func TestWriteMatCx2TypedefAndFunctions(t *testing.T) {
	w := &Writer{
		module:        &ir.Module{},
		wrappedMatCx2: make(map[uint8]struct{}),
	}

	w.writeMatCx2TypedefAndFunctions(3)
	output := w.out.String()

	// Check typedef
	if !strings.Contains(output, "typedef struct { float2 _0; float2 _1; float2 _2; } __mat3x2;") {
		t.Error("missing __mat3x2 typedef")
	}
	// Check getter
	if !strings.Contains(output, "float2 __get_col_of_mat3x2(__mat3x2 mat, uint idx)") {
		t.Error("missing __get_col_of_mat3x2")
	}
	// Check column setter
	if !strings.Contains(output, "void __set_col_of_mat3x2(__mat3x2 mat, uint idx, float2 value)") {
		t.Error("missing __set_col_of_mat3x2")
	}
	// Check element setter
	if !strings.Contains(output, "void __set_el_of_mat3x2(__mat3x2 mat, uint idx, uint vec_idx, float value)") {
		t.Error("missing __set_el_of_mat3x2")
	}
}

// TestWriteWrappedStructMatrixAccessFunctions tests per-struct matCx2 Get/Set helper generation.
func TestWriteWrappedStructMatrixAccessFunctions(t *testing.T) {
	matTy := ir.Type{Name: "mat3x2<f32>", Inner: ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}
	bazTy := ir.Type{Name: "Baz", Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "m", Type: 0, Offset: 0},
		},
		Span: 24,
	}}

	module := &ir.Module{
		Types: []ir.Type{matTy, bazTy},
	}

	w := newTestWriter(module, nil, nil)
	w.typeNames[0] = "float3x2"
	w.typeNames[1] = "Baz"
	w.names[nameKey{kind: nameKeyStructMember, handle1: 1, handle2: 0}] = "m"

	w.writeWrappedStructMatrixAccessFunctions(1, 0)
	out := w.out.String()

	if !strings.Contains(out, "float3x2 GetMatmOnBaz(Baz obj)") {
		t.Error("missing GetMatmOnBaz")
	}
	if !strings.Contains(out, "void SetMatmOnBaz(Baz obj, float3x2 mat)") {
		t.Error("missing SetMatmOnBaz")
	}
	if !strings.Contains(out, "void SetMatVecmOnBaz(Baz obj, float2 vec, uint mat_idx)") {
		t.Error("missing SetMatVecmOnBaz")
	}
	if !strings.Contains(out, "void SetMatScalarmOnBaz(Baz obj, float scalar, uint mat_idx, uint vec_idx)") {
		t.Error("missing SetMatScalarmOnBaz")
	}

	// Verify idempotency - calling again should not produce duplicate output
	w.out.Reset()
	w.writeWrappedStructMatrixAccessFunctions(1, 0)
	if w.out.String() != "" {
		t.Error("second call should produce no output (already written)")
	}
}

// TestIsMatCx2Type tests the matCx2 type detection.
func TestIsMatCx2Type(t *testing.T) {
	mat3x2 := ir.Type{Name: "mat3x2<f32>", Inner: ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}
	mat4x3 := ir.Type{Name: "mat4x3<f32>", Inner: ir.MatrixType{Columns: 4, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}
	scalar := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}

	module := &ir.Module{
		Types: []ir.Type{mat3x2, mat4x3, scalar},
	}

	w := newTestWriter(module, nil, nil)

	if !w.isMatCx2Type(0) {
		t.Error("mat3x2 should be matCx2")
	}
	if w.isMatCx2Type(1) {
		t.Error("mat4x3 should NOT be matCx2")
	}
	if w.isMatCx2Type(2) {
		t.Error("scalar should NOT be matCx2")
	}
}

// TestIsSubgroupBuiltinBinding tests subgroup builtin detection.
func TestIsSubgroupBuiltinBinding(t *testing.T) {
	tests := []struct {
		name    string
		binding *ir.Binding
		want    bool
	}{
		{"nil", nil, false},
		{"SubgroupSize", bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupSize}), true},
		{"SubgroupInvocationID", bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupInvocationID}), true},
		{"NumSubgroups", bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinNumSubgroups}), true},
		{"SubgroupID", bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupID}), true},
		{"Position (not subgroup)", bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}), false},
		{"Location (not subgroup)", bindingPtr(ir.LocationBinding{Location: 0}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubgroupBuiltinBinding(tt.binding); got != tt.want {
				t.Errorf("isSubgroupBuiltinBinding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func bindingPtr(b ir.Binding) *ir.Binding {
	return &b
}

// TestImageQueryFunctionName tests wrapper function naming.
func TestImageQueryFunctionName(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
	}

	tests := []struct {
		name string
		key  wrappedImageQueryKey
		want string
	}{
		{"RW Dimensions 2D", wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassStorage, query: imageQuerySize}, "NagaRWDimensions2D"},
		{"Dimensions 2D", wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassSampled, query: imageQuerySize}, "NagaDimensions2D"},
		{"MipDimensions 1D", wrappedImageQueryKey{dim: ir.Dim1D, class: ir.ImageClassSampled, query: imageQuerySizeLevel}, "NagaMipDimensions1D"},
		{"NumLevels Cube", wrappedImageQueryKey{dim: ir.DimCube, class: ir.ImageClassSampled, query: imageQueryNumLevels}, "NagaNumLevelsCube"},
		{"NumLayers 2DArray", wrappedImageQueryKey{dim: ir.Dim2D, arrayed: true, class: ir.ImageClassSampled, query: imageQueryNumLayers}, "NagaNumLayers2DArray"},
		{"MS NumSamples 2D", wrappedImageQueryKey{dim: ir.Dim2D, multi: true, class: ir.ImageClassSampled, query: imageQueryNumSamples}, "NagaMSNumSamples2D"},
		{"Depth Dimensions 2D", wrappedImageQueryKey{dim: ir.Dim2D, class: ir.ImageClassDepth, query: imageQuerySize}, "NagaDepthDimensions2D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.writeImageQueryFunctionNameDirect(tt.key)
			if got := w.out.String(); got != tt.want {
				t.Errorf("writeImageQueryFunctionNameDirect() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWriteSemantic_SkipsSubgroupBuiltins tests that writeSemantic skips subgroup builtins.
func TestWriteSemantic_SkipsSubgroupBuiltins(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
	}

	// Subgroup builtin should produce no output
	binding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupSize})
	w.out.Reset()
	w.writeSemantic(&binding, nil)
	if got := w.out.String(); got != "" {
		t.Errorf("writeSemantic(SubgroupSize) = %q, want empty", got)
	}

	// Normal builtin should produce output
	binding2 := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	w.out.Reset()
	w.writeSemantic(&binding2, nil)
	if got := w.out.String(); got == "" {
		t.Error("writeSemantic(Position) should produce output")
	}
}

// TestIsArrayOfMatCx2Type tests detection of array-of-matCx2 types.
func TestIsArrayOfMatCx2Type(t *testing.T) {
	two := uint32(2)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.MatrixType{Columns: 4, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 0: mat4x2<f32>
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}, Stride: 32}},                     // 1: array<mat4x2<f32>, 2>
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: mat4x4<f32>
			{Inner: ir.ArrayType{Base: 2, Size: ir.ArraySize{Constant: &two}, Stride: 64}},                     // 3: array<mat4x4<f32>, 2>
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                             // 4: f32
		},
	}

	w := newTestWriter(module, nil, nil)

	if !w.isArrayOfMatCx2Type(1) {
		t.Error("array<mat4x2> should be array of matCx2")
	}
	if w.isArrayOfMatCx2Type(0) {
		t.Error("mat4x2 (non-array) should NOT be array of matCx2")
	}
	if w.isArrayOfMatCx2Type(3) {
		t.Error("array<mat4x4> should NOT be array of matCx2")
	}
	if w.isArrayOfMatCx2Type(4) {
		t.Error("f32 should NOT be array of matCx2")
	}
}
