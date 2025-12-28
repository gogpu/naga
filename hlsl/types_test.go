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
		// Sampled textures
		{"Texture1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled}, "Texture1D<float4>"},
		{"Texture2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}, "Texture2D<float4>"},
		{"Texture3D", ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled}, "Texture3D<float4>"},
		{"TextureCube", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled}, "TextureCube<float4>"},

		// Array textures
		{"Texture1DArray", ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassSampled}, "Texture1DArray<float4>"},
		{"Texture2DArray", ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled}, "Texture2DArray<float4>"},
		{"TextureCubeArray", ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled}, "TextureCubeArray<float4>"},
		// 3D cannot be arrayed
		{"Texture3D_no_array", ir.ImageType{Dim: ir.Dim3D, Arrayed: true, Class: ir.ImageClassSampled}, "Texture3D<float4>"},

		// Multisampled textures
		{"Texture2DMS", ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Class: ir.ImageClassSampled}, "Texture2DMS<float4>"},
		{"Texture2DMSArray", ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Arrayed: true, Class: ir.ImageClassSampled}, "Texture2DMSArray<float4>"},

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
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		{"nested array", 3, "float", "[10][5]"},
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
	module := &ir.Module{}
	w := &Writer{
		module:           module,
		typeNames:        make(map[ir.TypeHandle]string),
		names:            make(map[nameKey]string),
		registerBindings: make(map[string]string),
	}

	tests := []struct {
		name         string
		bufName      string
		typeName     string
		binding      *BindTarget
		wantContains []string
	}{
		{
			"with binding",
			"uniforms",
			"UniformData",
			&BindTarget{Register: 0, Space: 0},
			[]string{"cbuffer uniforms_cbuffer : register(b0, space0)", "UniformData uniforms;"},
		},
		{
			"without binding",
			"globals",
			"GlobalData",
			nil,
			[]string{"cbuffer globals_cbuffer {", "GlobalData globals;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.writeCBufferDeclaration(tt.bufName, tt.typeName, tt.binding)
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
		{"location 0", ir.LocationBinding{Location: 0}, 0, "TEXCOORD0"},
		{"location 3", ir.LocationBinding{Location: 3}, 0, "TEXCOORD3"},
		{"nil binding", nil, 5, "TEXCOORD5"},
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
