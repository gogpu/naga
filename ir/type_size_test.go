package ir

import "testing"

func TestTypeSize(t *testing.T) {
	u32ptr := func(v uint32) *uint32 { return &v }

	mod := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                                    // 0
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},                                                     // 1
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},                                                     // 2
			{Name: "f16", Inner: ScalarType{Kind: ScalarFloat, Width: 2}},                                                    // 3
			{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 4}},                                                    // 4
			{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                  // 5
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                  // 6
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                  // 7
			{Name: "vec2h", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}},                  // 8
			{Name: "mat2x2f", Inner: MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 9
			{Name: "mat3x3f", Inner: MatrixType{Columns: Vec3, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 10
			{Name: "mat4x4f", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 11
			{Name: "mat2x3f", Inner: MatrixType{Columns: Vec2, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 12
			{Name: "array_f32_4", Inner: ArrayType{Base: 0, Size: ArraySize{Constant: u32ptr(4)}, Stride: 4}},                // 13
			{Name: "array_vec4_10", Inner: ArrayType{Base: 7, Size: ArraySize{Constant: u32ptr(10)}, Stride: 16}},            // 14
			{Name: "array_dyn", Inner: ArrayType{Base: 0, Size: ArraySize{Constant: nil}, Stride: 4}},                        // 15
			{Name: "MyStruct", Inner: StructType{Span: 48, Members: []StructMember{
				{Name: "a", Type: 7, Offset: 0},
				{Name: "b", Type: 7, Offset: 16},
				{Name: "c", Type: 7, Offset: 32},
			}}}, // 16
			{Name: "ptr_f32", Inner: PointerType{Base: 0, Space: SpaceFunction}},                    // 17
			{Name: "atomic_u32", Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}}, // 18
			{Name: "sampler", Inner: SamplerType{Comparison: false}},                                // 19
			{Name: "texture", Inner: ImageType{Dim: Dim2D, Class: ImageClassSampled}},               // 20
			{Name: "NestedStruct", Inner: StructType{Span: 64, Members: []StructMember{
				{Name: "inner", Type: 16, Offset: 0},
				{Name: "extra", Type: 7, Offset: 48},
			}}}, // 21
		},
	}

	tests := []struct {
		name   string
		handle TypeHandle
		want   uint32
	}{
		{"f32", 0, 4},
		{"u32", 1, 4},
		{"i32", 2, 4},
		{"f16", 3, 2},
		{"bool", 4, 4},
		{"vec2<f32>", 5, 8},
		{"vec3<f32>", 6, 12},
		{"vec4<f32>", 7, 16},
		{"vec2<f16>", 8, 4},
		// mat2x2: cols=2, rows=2 → align(vec2)=2 → colStride=2*4=8 → 8*2=16
		{"mat2x2<f32>", 9, 16},
		// mat3x3: cols=3, rows=3 → align(vec3)=4 → colStride=4*4=16 → 16*3=48
		{"mat3x3<f32>", 10, 48},
		// mat4x4: cols=4, rows=4 → align(vec4)=4 → colStride=4*4=16 → 16*4=64
		{"mat4x4<f32>", 11, 64},
		// mat2x3: cols=2, rows=3 → align(vec3)=4 → colStride=4*4=16 → 16*2=32
		{"mat2x3<f32>", 12, 32},
		{"array<f32,4>", 13, 16},
		{"array<vec4,10>", 14, 160},
		{"array<f32> dynamic", 15, 4}, // dynamic = at least 1 element
		{"struct span=48", 16, 48},
		{"pointer", 17, 0},
		{"atomic<u32>", 18, 4},
		{"sampler", 19, 0},
		{"texture_2d", 20, 0},
		{"nested struct span=64", 21, 64},
		{"out of bounds handle", 999, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TypeSize(mod, tt.handle)
			if got != tt.want {
				t.Errorf("TypeSize(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}
