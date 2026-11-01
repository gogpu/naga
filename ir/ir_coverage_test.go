package ir

import (
	"testing"
)

// --- StorageFormat.Scalar ---

func TestStorageFormatScalar(t *testing.T) {
	tests := []struct {
		name      string
		format    StorageFormat
		wantKind  ScalarKind
		wantWidth uint8
	}{
		// Standard 4-byte formats
		{"R8Uint", StorageFormatR8Uint, ScalarUint, 4},
		{"R32Float", StorageFormatR32Float, ScalarFloat, 4},
		{"R32Sint", StorageFormatR32Sint, ScalarSint, 4},
		{"Rgba8Unorm", StorageFormatRgba8Unorm, ScalarFloat, 4},

		// 64-bit formats — width 8
		{"R64Uint", StorageFormatR64Uint, ScalarUint, 8},
		{"R64Sint", StorageFormatR64Sint, ScalarSint, 8},

		// Other standard 4-byte
		{"Rgba32Uint", StorageFormatRgba32Uint, ScalarUint, 4},
		{"Rgba32Sint", StorageFormatRgba32Sint, ScalarSint, 4},
		{"Rgba32Float", StorageFormatRgba32Float, ScalarFloat, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.format.Scalar()
			if got.Kind != tt.wantKind {
				t.Errorf("Scalar().Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if got.Width != tt.wantWidth {
				t.Errorf("Scalar().Width = %v, want %v", got.Width, tt.wantWidth)
			}
		})
	}
}

// --- ValuePointerType ---

func TestValuePointerTypeInterface(t *testing.T) {
	// Verify ValuePointerType implements TypeInner
	vec2 := Vec2
	tests := []struct {
		name string
		vpt  ValuePointerType
	}{
		{"scalar_pointer", ValuePointerType{Size: nil, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}, Space: SpaceFunction}},
		{"vector_pointer", ValuePointerType{Size: &vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}, Space: SpacePrivate}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify it satisfies TypeInner
			var inner TypeInner = tt.vpt
			inner.typeInner()
		})
	}
}

// --- Interpolation and Binding types ---

func TestBindingTypes(t *testing.T) {
	t.Run("builtin_binding", func(t *testing.T) {
		b := BuiltinBinding{Builtin: BuiltinPosition, Invariant: true}
		var binding Binding = b
		binding.binding()
		if b.Builtin != BuiltinPosition {
			t.Error("unexpected builtin value")
		}
		if !b.Invariant {
			t.Error("expected invariant=true")
		}
	})

	t.Run("location_binding", func(t *testing.T) {
		interp, sampling := InterpolationFlat, SamplingCenter
		blendSrc := uint32(1)
		b := LocationBinding{Location: 3, Interpolation: &interp, Sampling: &sampling, BlendSrc: &blendSrc}
		var binding Binding = b
		binding.binding()
		if b.Location != 3 {
			t.Error("unexpected location")
		}
		if *b.Interpolation != InterpolationFlat || *b.Sampling != SamplingCenter {
			t.Error("unexpected interpolation")
		}
		if *b.BlendSrc != 1 {
			t.Error("unexpected blend src")
		}
	})
}

// --- Layouter sizes: verify WGSL alignment rules ---

func TestLayouterSizeEdgeCases(t *testing.T) {
	sz4 := uint32(4)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                                   // 0
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},                                                    // 1
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                 // 2
			{Name: "mat2x2", Inner: MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 3
			{Name: "arr_4", Inner: ArrayType{Base: 0, Stride: 4, Size: ArraySize{Constant: &sz4}}},                          // 4
			{Name: "MyStruct", Inner: StructType{Span: 24}},                                                                 // 5
			{Name: "atomic_u32", Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}},                         // 6
			{Name: "ptr", Inner: PointerType{Base: 0, Space: SpaceFunction}},                                                // 7
			{Name: "sampler", Inner: SamplerType{Comparison: false}},                                                        // 8
			{Name: "image", Inner: ImageType{Dim: Dim2D}},                                                                   // 9
			{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                 // 10
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                 // 11
			{Name: "mat4x3", Inner: MatrixType{Columns: Vec4, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 12
		},
	}

	tests := []struct {
		name   string
		handle TypeHandle
		want   uint32
	}{
		{"scalar_f32", 0, 4},
		{"scalar_u32", 1, 4},
		{"vec4f_16bytes", 2, 16},
		{"mat2x2_vec2_align2", 3, 16}, // 2 columns * (align2 * 4bytes) = 2 * 8 = 16
		{"array_4_elements", 4, 16},   // 4 * stride(4) = 16
		{"struct_span", 5, 24},
		{"atomic_u32", 6, 4},
		{"pointer_opaque", 7, 0},
		{"sampler_opaque", 8, 0},
		{"image_opaque", 9, 0},
		{"out_of_range", 999, 0},
		{"vec2f_8bytes", 10, 8},
		{"vec3f_12bytes", 11, 12},
		{"mat4x3_vec3_align4", 12, 64}, // 4 columns * (align4 * 4bytes) = 4 * 16 = 64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layoutOf(t, module, tt.handle).Size
			if got != tt.want {
				t.Errorf("Layout(%d).Size = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// --- Validate: exercises error paths for invalid modules ---

// --- MeshOutputTopology ---

func TestMeshOutputTopology(t *testing.T) {
	tests := []struct {
		name     string
		topology MeshOutputTopology
		want     MeshOutputTopology
	}{
		{"Points", MeshTopologyPoints, 0},
		{"Lines", MeshTopologyLines, 1},
		{"Triangles", MeshTopologyTriangles, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.topology != tt.want {
				t.Errorf("got %d, want %d", tt.topology, tt.want)
			}
		})
	}
}

// --- ConservativeDepth ---

func TestConservativeDepth(t *testing.T) {
	tests := []struct {
		name  string
		depth ConservativeDepth
	}{
		{"Unchanged", ConservativeDepthUnchanged},
		{"GreaterEqual", ConservativeDepthGreaterEqual},
		{"LessEqual", ConservativeDepthLessEqual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var edt EarlyDepthTest = EarlyDepthAllow{Conservative: tt.depth}
			if edt.(EarlyDepthAllow).Conservative != tt.depth {
				t.Error("unexpected conservative depth value")
			}
		})
	}
}

// --- ShaderStage ---

func TestShaderStage(t *testing.T) {
	tests := []struct {
		name  string
		stage ShaderStage
	}{
		{"Vertex", StageVertex},
		{"Task", StageTask},
		{"Mesh", StageMesh},
		{"Fragment", StageFragment},
		{"Compute", StageCompute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := EntryPoint{Stage: tt.stage}
			if ep.Stage != tt.stage {
				t.Error("unexpected stage")
			}
		})
	}
}

// --- AddressSpace ---

func TestAddressSpace(t *testing.T) {
	tests := []struct {
		name  string
		space AddressSpace
	}{
		{"Function", SpaceFunction},
		{"Private", SpacePrivate},
		{"WorkGroup", SpaceWorkGroup},
		{"Uniform", SpaceUniform},
		{"Storage", SpaceStorage},
		{"Handle", SpaceHandle},
		{"Immediate", SpaceImmediate},
		{"TaskPayload", SpaceTaskPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gv := GlobalVariable{Space: tt.space}
			if gv.Space != tt.space {
				t.Error("unexpected address space")
			}
		})
	}
}

// --- BarrierFlags ---

func TestBarrierFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags BarrierFlags
		want  BarrierFlags
	}{
		{"Storage", BarrierStorage, 1},
		{"WorkGroup", BarrierWorkGroup, 2},
		{"SubGroup", BarrierSubGroup, 4},
		{"Texture", BarrierTexture, 8},
		{"combined", BarrierStorage | BarrierWorkGroup, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.flags != tt.want {
				t.Errorf("got %d, want %d", tt.flags, tt.want)
			}
		})
	}
}
