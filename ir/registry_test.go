package ir

import (
	"testing"
)

func TestTypeRegistry_ScalarDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	// Register f32 twice
	f32_1 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	f32_2 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	if f32_1 != f32_2 {
		t.Errorf("Expected same handle for identical scalar types, got %d and %d", f32_1, f32_2)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 type, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentScalars(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	i32 := registry.GetOrCreate("i32", ScalarType{Kind: ScalarSint, Width: 4})
	u32 := registry.GetOrCreate("u32", ScalarType{Kind: ScalarUint, Width: 4})
	f16 := registry.GetOrCreate("f16", ScalarType{Kind: ScalarFloat, Width: 2})

	// All should be different
	handles := []TypeHandle{f32, i32, u32, f16}
	for i := 0; i < len(handles); i++ {
		for j := i + 1; j < len(handles); j++ {
			if handles[i] == handles[j] {
				t.Errorf("Expected different handles for different types, got %d == %d", handles[i], handles[j])
			}
		}
	}

	if registry.Count() != 4 {
		t.Errorf("Expected 4 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_VectorDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	// Create vec4<f32> twice
	scalar := ScalarType{Kind: ScalarFloat, Width: 4}
	vec4_1 := registry.GetOrCreate("", VectorType{Size: Vec4, Scalar: scalar})
	vec4_2 := registry.GetOrCreate("", VectorType{Size: Vec4, Scalar: scalar})

	if vec4_1 != vec4_2 {
		t.Errorf("Expected same handle for identical vector types, got %d and %d", vec4_1, vec4_2)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 type, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentVectors(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := ScalarType{Kind: ScalarFloat, Width: 4}
	i32 := ScalarType{Kind: ScalarSint, Width: 4}

	vec4f32 := registry.GetOrCreate("", VectorType{Size: Vec4, Scalar: f32})
	vec3f32 := registry.GetOrCreate("", VectorType{Size: Vec3, Scalar: f32})
	vec4i32 := registry.GetOrCreate("", VectorType{Size: Vec4, Scalar: i32})

	// All should be different
	if vec4f32 == vec3f32 {
		t.Error("vec4<f32> should differ from vec3<f32>")
	}
	if vec4f32 == vec4i32 {
		t.Error("vec4<f32> should differ from vec4<i32>")
	}
	if vec3f32 == vec4i32 {
		t.Error("vec3<f32> should differ from vec4<i32>")
	}

	if registry.Count() != 3 {
		t.Errorf("Expected 3 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_MatrixDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	scalar := ScalarType{Kind: ScalarFloat, Width: 4}
	mat4x4_1 := registry.GetOrCreate("", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: scalar})
	mat4x4_2 := registry.GetOrCreate("", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: scalar})

	if mat4x4_1 != mat4x4_2 {
		t.Errorf("Expected same handle for identical matrix types, got %d and %d", mat4x4_1, mat4x4_2)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 type, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentMatrices(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := ScalarType{Kind: ScalarFloat, Width: 4}

	mat4x4 := registry.GetOrCreate("", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: f32})
	mat3x3 := registry.GetOrCreate("", MatrixType{Columns: Vec3, Rows: Vec3, Scalar: f32})
	mat4x3 := registry.GetOrCreate("", MatrixType{Columns: Vec4, Rows: Vec3, Scalar: f32})

	// All should be different
	if mat4x4 == mat3x3 {
		t.Error("mat4x4<f32> should differ from mat3x3<f32>")
	}
	if mat4x4 == mat4x3 {
		t.Error("mat4x4<f32> should differ from mat4x3<f32>")
	}
	if mat3x3 == mat4x3 {
		t.Error("mat3x3<f32> should differ from mat4x3<f32>")
	}

	if registry.Count() != 3 {
		t.Errorf("Expected 3 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_ArrayDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	// Create base type first
	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	size := uint32(10)
	array1 := registry.GetOrCreate("", ArrayType{
		Base:   f32,
		Size:   ArraySize{Constant: &size},
		Stride: 4,
	})
	array2 := registry.GetOrCreate("", ArrayType{
		Base:   f32,
		Size:   ArraySize{Constant: &size},
		Stride: 4,
	})

	if array1 != array2 {
		t.Errorf("Expected same handle for identical array types, got %d and %d", array1, array2)
	}

	// Should have 2 types total: f32 and array<f32, 10>
	if registry.Count() != 2 {
		t.Errorf("Expected 2 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentArrays(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	i32 := registry.GetOrCreate("i32", ScalarType{Kind: ScalarSint, Width: 4})

	size10 := uint32(10)
	size20 := uint32(20)

	arrayF32_10 := registry.GetOrCreate("", ArrayType{
		Base:   f32,
		Size:   ArraySize{Constant: &size10},
		Stride: 4,
	})
	arrayF32_20 := registry.GetOrCreate("", ArrayType{
		Base:   f32,
		Size:   ArraySize{Constant: &size20},
		Stride: 4,
	})
	arrayI32_10 := registry.GetOrCreate("", ArrayType{
		Base:   i32,
		Size:   ArraySize{Constant: &size10},
		Stride: 4,
	})
	arrayRuntime := registry.GetOrCreate("", ArrayType{
		Base:   f32,
		Size:   ArraySize{Constant: nil},
		Stride: 4,
	})

	// All should be different
	if arrayF32_10 == arrayF32_20 {
		t.Error("array<f32, 10> should differ from array<f32, 20>")
	}
	if arrayF32_10 == arrayI32_10 {
		t.Error("array<f32, 10> should differ from array<i32, 10>")
	}
	if arrayF32_10 == arrayRuntime {
		t.Error("array<f32, 10> should differ from array<f32>")
	}

	// Should have: f32, i32, arrayF32_10, arrayF32_20, arrayI32_10, arrayRuntime = 6 types
	if registry.Count() != 6 {
		t.Errorf("Expected 6 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_StructDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	members := []StructMember{
		{Name: "position", Type: f32, Offset: 0},
		{Name: "color", Type: f32, Offset: 16},
	}

	struct1 := registry.GetOrCreate("Vertex", StructType{Members: members, Span: 32})
	struct2 := registry.GetOrCreate("Vertex", StructType{Members: members, Span: 32})

	if struct1 != struct2 {
		t.Errorf("Expected same handle for identical struct types, got %d and %d", struct1, struct2)
	}

	// Should have 2 types: f32 and Vertex struct
	if registry.Count() != 2 {
		t.Errorf("Expected 2 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentStructs(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	i32 := registry.GetOrCreate("i32", ScalarType{Kind: ScalarSint, Width: 4})

	struct1 := registry.GetOrCreate("Struct1", StructType{
		Members: []StructMember{
			{Name: "a", Type: f32, Offset: 0},
		},
		Span: 16,
	})
	struct2 := registry.GetOrCreate("Struct2", StructType{
		Members: []StructMember{
			{Name: "a", Type: i32, Offset: 0},
		},
		Span: 16,
	})
	struct3 := registry.GetOrCreate("Struct3", StructType{
		Members: []StructMember{
			{Name: "b", Type: f32, Offset: 0}, // Different member name
		},
		Span: 16,
	})

	// All should be different
	if struct1 == struct2 {
		t.Error("Structs with different member types should differ")
	}
	if struct1 == struct3 {
		t.Error("Structs with different member names should differ")
	}

	// Should have: f32, i32, struct1, struct2, struct3 = 5 types
	if registry.Count() != 5 {
		t.Errorf("Expected 5 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_PointerDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	ptr1 := registry.GetOrCreate("", PointerType{Base: f32, Space: SpaceFunction})
	ptr2 := registry.GetOrCreate("", PointerType{Base: f32, Space: SpaceFunction})

	if ptr1 != ptr2 {
		t.Errorf("Expected same handle for identical pointer types, got %d and %d", ptr1, ptr2)
	}

	// Should have 2 types: f32 and ptr<f32, function>
	if registry.Count() != 2 {
		t.Errorf("Expected 2 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentPointers(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	ptrFunc := registry.GetOrCreate("", PointerType{Base: f32, Space: SpaceFunction})
	ptrPriv := registry.GetOrCreate("", PointerType{Base: f32, Space: SpacePrivate})
	ptrStorage := registry.GetOrCreate("", PointerType{Base: f32, Space: SpaceStorage})

	// All should be different
	if ptrFunc == ptrPriv {
		t.Error("Pointers with different address spaces should differ")
	}
	if ptrFunc == ptrStorage {
		t.Error("Pointers with different address spaces should differ")
	}

	// Should have: f32, ptrFunc, ptrPriv, ptrStorage = 4 types
	if registry.Count() != 4 {
		t.Errorf("Expected 4 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_ImageDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	img1 := registry.GetOrCreate("", ImageType{
		Dim:          Dim2D,
		Arrayed:      false,
		Class:        ImageClassSampled,
		Multisampled: false,
	})
	img2 := registry.GetOrCreate("", ImageType{
		Dim:          Dim2D,
		Arrayed:      false,
		Class:        ImageClassSampled,
		Multisampled: false,
	})

	if img1 != img2 {
		t.Errorf("Expected same handle for identical image types, got %d and %d", img1, img2)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 type, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentImages(t *testing.T) {
	registry := NewTypeRegistry()

	img2D := registry.GetOrCreate("", ImageType{
		Dim:          Dim2D,
		Arrayed:      false,
		Class:        ImageClassSampled,
		Multisampled: false,
	})
	img3D := registry.GetOrCreate("", ImageType{
		Dim:          Dim3D,
		Arrayed:      false,
		Class:        ImageClassSampled,
		Multisampled: false,
	})
	img2DArray := registry.GetOrCreate("", ImageType{
		Dim:          Dim2D,
		Arrayed:      true,
		Class:        ImageClassSampled,
		Multisampled: false,
	})
	img2DMS := registry.GetOrCreate("", ImageType{
		Dim:          Dim2D,
		Arrayed:      false,
		Class:        ImageClassSampled,
		Multisampled: true,
	})

	// All should be different
	if img2D == img3D {
		t.Error("Images with different dimensions should differ")
	}
	if img2D == img2DArray {
		t.Error("Images with different array status should differ")
	}
	if img2D == img2DMS {
		t.Error("Images with different multisampling should differ")
	}

	if registry.Count() != 4 {
		t.Errorf("Expected 4 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_SamplerDeduplication(t *testing.T) {
	registry := NewTypeRegistry()

	sampler1 := registry.GetOrCreate("", SamplerType{Comparison: false})
	sampler2 := registry.GetOrCreate("", SamplerType{Comparison: false})

	if sampler1 != sampler2 {
		t.Errorf("Expected same handle for identical sampler types, got %d and %d", sampler1, sampler2)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 type, got %d", registry.Count())
	}
}

func TestTypeRegistry_DifferentSamplers(t *testing.T) {
	registry := NewTypeRegistry()

	samplerReg := registry.GetOrCreate("", SamplerType{Comparison: false})
	samplerComp := registry.GetOrCreate("", SamplerType{Comparison: true})

	if samplerReg == samplerComp {
		t.Error("Regular sampler should differ from comparison sampler")
	}

	if registry.Count() != 2 {
		t.Errorf("Expected 2 types, got %d", registry.Count())
	}
}

func TestTypeRegistry_Lookup(t *testing.T) {
	registry := NewTypeRegistry()

	f32 := registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})

	typ, ok := registry.Lookup(f32)
	if !ok {
		t.Error("Expected to find registered type")
	}
	if typ.Name != "f32" {
		t.Errorf("Expected name 'f32', got '%s'", typ.Name)
	}

	// Try invalid handle
	_, ok = registry.Lookup(TypeHandle(999))
	if ok {
		t.Error("Expected not to find invalid handle")
	}
}

func TestTypeRegistry_ComplexNestedTypes(t *testing.T) {
	registry := NewTypeRegistry()

	// Build: struct { position: vec4<f32>, color: vec4<f32> }
	_ = registry.GetOrCreate("f32", ScalarType{Kind: ScalarFloat, Width: 4})
	vec4f32 := registry.GetOrCreate("", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}})

	vertex := registry.GetOrCreate("Vertex", StructType{
		Members: []StructMember{
			{Name: "position", Type: vec4f32, Offset: 0},
			{Name: "color", Type: vec4f32, Offset: 16},
		},
		Span: 32,
	})

	// Create same struct again - should deduplicate
	vertex2 := registry.GetOrCreate("Vertex", StructType{
		Members: []StructMember{
			{Name: "position", Type: vec4f32, Offset: 0},
			{Name: "color", Type: vec4f32, Offset: 16},
		},
		Span: 32,
	})

	if vertex != vertex2 {
		t.Errorf("Expected same handle for identical complex structs, got %d and %d", vertex, vertex2)
	}

	// Should have: f32, vec4<f32>, Vertex = 3 types
	if registry.Count() != 3 {
		t.Errorf("Expected 3 types, got %d", registry.Count())
	}
}
