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

// --- ValidationError.Error ---

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "message_only",
			err:  ValidationError{Message: "test error"},
			want: "test error",
		},
		{
			name: "with_function",
			err:  ValidationError{Message: "bad type", Function: "myFunc", Statement: -1},
			want: "in function myFunc: bad type",
		},
		{
			name: "with_function_and_expression",
			err: ValidationError{
				Message:    "invalid",
				Function:   "foo",
				Expression: func() *ExpressionHandle { h := ExpressionHandle(5); return &h }(),
			},
			want: "in function foo, expression 5: invalid",
		},
		{
			name: "with_function_and_statement",
			err: ValidationError{
				Message:   "bad stmt",
				Function:  "bar",
				Statement: 3,
			},
			want: "in function bar, statement 3: bad stmt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- ExprAlias and ExprPhi expressionKind ---

func TestDxilOnlyExpressionKinds(t *testing.T) {
	// Verify DXIL-only expression kinds satisfy ExpressionKind interface
	tests := []struct {
		name string
		kind ExpressionKind
	}{
		{"ExprAlias", ExprAlias{Source: 0}},
		{"ExprPhi", ExprPhi{Incoming: []PhiIncoming{
			{PredKey: PhiPredIfAccept, Value: 0},
			{PredKey: PhiPredIfReject, Value: 1},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.kind.expressionKind()
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
		interp := &Interpolation{Kind: InterpolationFlat, Sampling: SamplingCenter}
		blendSrc := uint32(1)
		b := LocationBinding{Location: 3, Interpolation: interp, BlendSrc: &blendSrc}
		var binding Binding = b
		binding.binding()
		if b.Location != 3 {
			t.Error("unexpected location")
		}
		if b.Interpolation.Kind != InterpolationFlat {
			t.Error("unexpected interpolation kind")
		}
		if *b.BlendSrc != 1 {
			t.Error("unexpected blend src")
		}
	})
}

// --- TypeSize: verify WGSL alignment rules ---

func TestTypeSizeEdgeCases(t *testing.T) {
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
			got := TypeSize(module, tt.handle)
			if got != tt.want {
				t.Errorf("TypeSize(%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// --- Validate: exercises error paths for invalid modules ---

func TestValidate_ErrorCases(t *testing.T) {
	t.Run("nil_module", func(t *testing.T) {
		_, err := Validate(nil)
		if err == nil {
			t.Error("expected error for nil module")
		}
	})

	t.Run("invalid_type_handle_in_constant", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
			Constants: []Constant{
				{Name: "bad", Type: 999}, // out of range
			},
		}
		errs, err := Validate(module)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) == 0 {
			t.Error("expected validation errors for invalid type handle")
		}
	})

	t.Run("invalid_expression_handle_in_function", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
			Functions: []Function{
				{
					Name: "broken",
					Expressions: []Expression{
						{Kind: ExprGlobalVariable{Variable: 999}}, // out of range
					},
					Body: Block{
						{Kind: StmtEmit{Range: Range{Start: 0, End: 1}}},
					},
				},
			},
		}
		errs, err := Validate(module)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) == 0 {
			t.Error("expected validation errors for out-of-range global variable handle")
		}
	})

	t.Run("valid_minimal_module", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
		}
		errs, err := Validate(module)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) != 0 {
			t.Errorf("expected 0 validation errors, got %d: %v", len(errs), errs)
		}
	})
}

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
			edt := &EarlyDepthTest{Conservative: tt.depth}
			if edt.Conservative != tt.depth {
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
		{"PushConstant", SpacePushConstant},
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
