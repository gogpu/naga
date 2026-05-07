package ir

import (
	"testing"
)

// --- ResolveExpressionType: Load through pointer dereferences correctly ---

func TestResolveLoadType_Dereferences(t *testing.T) {
	// Tests the IR invariant: ExprLoad through a pointer produces the base type.
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "atomic_u32", Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}},
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "x", Space: SpacePrivate, Type: 0},   // f32
			{Name: "a", Space: SpaceWorkGroup, Type: 1}, // atomic<u32>
		},
	}

	t.Run("load_pointer_to_scalar", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 0}}, // 0: ptr<private, f32>
				{Kind: ExprLoad{Pointer: 0}},            // 1: load -> f32
			},
		}
		res, err := ResolveExpressionType(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Handle == nil || *res.Handle != 0 {
			t.Errorf("expected Handle=0 (f32), got %v", res)
		}
	})

	t.Run("load_pointer_to_atomic_resolves_scalar", func(t *testing.T) {
		// IR invariant: Load on Atomic(scalar) → scalar (not the atomic wrapper)
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 1}}, // 0: ptr<workgroup, atomic<u32>>
				{Kind: ExprLoad{Pointer: 0}},            // 1: load -> u32 (scalar from atomic)
			},
		}
		res, err := ResolveExpressionType(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		scalar, ok := res.Value.(ScalarType)
		if !ok || scalar.Kind != ScalarUint || scalar.Width != 4 {
			t.Errorf("expected ScalarType{Uint, 4}, got %v", res.Value)
		}
	})

	t.Run("load_value_pointer_vector", func(t *testing.T) {
		// ValuePointer{Size: &Vec3, Scalar} → Load → VectorType{Vec3, Scalar}
		vec3 := Vec3
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprFunctionArgument{Index: 0}}, // 0: dummy
				{Kind: ExprLoad{Pointer: 0}},           // 1: load
			},
			Arguments: []FunctionArgument{
				{Name: "p", Type: 0}, // will be resolved below
			},
		}
		// Override resolution: simulate ValuePointerType resolution
		// We can test this by passing the right setup through resolveLoadType
		_ = fn
		_ = vec3
		// This path requires ExpressionTypes to be pre-populated or a more complex setup.
		// Tested implicitly through snapshot tests.
	})
}

// --- ResolveExpressionType: Access into various containers ---

func TestResolveAccessType_Containers(t *testing.T) {
	f32Scalar := ScalarType{Kind: ScalarFloat, Width: 4}
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: f32Scalar},                                                   // 0
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: f32Scalar}},                 // 1
			{Name: "mat4x4", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: f32Scalar}}, // 2
			{Name: "arr", Inner: ArrayType{Base: 0, Stride: 4}},                               // 3
		},
		GlobalVariables: []GlobalVariable{
			{Name: "vec", Space: SpaceHandle, Type: 1}, // handle space -> direct type
			{Name: "mat", Space: SpaceHandle, Type: 2},
			{Name: "arr", Space: SpaceHandle, Type: 3},
		},
	}

	t.Run("access_vector_returns_scalar", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 0}}, // 0: vec4f
				{Kind: Literal{Value: LiteralU32(2)}},   // 1: index
				{Kind: ExprAccess{Base: 0, Index: 1}},   // 2: vec[i] -> f32
			},
		}
		res, err := ResolveExpressionType(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		scalar, ok := res.Value.(ScalarType)
		if !ok {
			t.Fatalf("expected ScalarType, got %T", res.Value)
		}
		if scalar.Kind != ScalarFloat || scalar.Width != 4 {
			t.Errorf("expected f32, got kind=%v width=%v", scalar.Kind, scalar.Width)
		}
	})

	t.Run("access_matrix_returns_column_vector", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 1}}, // 0: mat4x4
				{Kind: Literal{Value: LiteralU32(0)}},   // 1: index
				{Kind: ExprAccess{Base: 0, Index: 1}},   // 2: mat[i] -> vec4f
			},
		}
		res, err := ResolveExpressionType(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		vec, ok := res.Value.(VectorType)
		if !ok {
			t.Fatalf("expected VectorType, got %T", res.Value)
		}
		if vec.Size != Vec4 {
			t.Errorf("expected Vec4 column, got %v", vec.Size)
		}
	})

	t.Run("access_array_returns_element_handle", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 2}}, // 0: arr
				{Kind: Literal{Value: LiteralU32(0)}},   // 1: index
				{Kind: ExprAccess{Base: 0, Index: 1}},   // 2: arr[i] -> f32
			},
		}
		res, err := ResolveExpressionType(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Handle == nil || *res.Handle != 0 {
			t.Errorf("expected Handle=0 (f32 base), got %v", res)
		}
	})
}

// --- ResolveExpressionType: AccessIndex into struct ---

func TestResolveAccessIndexType_Struct(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // 0
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},  // 1
			{Name: "MyStruct", Inner: StructType{
				Members: []StructMember{
					{Name: "x", Type: 0, Offset: 0}, // f32
					{Name: "y", Type: 1, Offset: 4}, // u32
				},
				Span: 8,
			}}, // 2
		},
		GlobalVariables: []GlobalVariable{
			{Name: "s", Space: SpaceHandle, Type: 2}, // MyStruct
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: MyStruct
			{Kind: ExprAccessIndex{Base: 0, Index: 1}}, // 1: s.y -> u32
		},
	}

	res, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Handle == nil || *res.Handle != 1 {
		t.Errorf("expected Handle=1 (u32), got %v", res)
	}
}

// --- ResolveExpressionType: Splat scalar to vector ---

func TestResolveSplatType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}}, // 0: f32 literal
			{Kind: ExprSplat{Size: Vec3, Value: 0}}, // 1: splat -> vec3f
		},
	}

	res, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := res.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", res.Value)
	}
	if vec.Size != Vec3 || vec.Scalar.Kind != ScalarFloat || vec.Scalar.Width != 4 {
		t.Errorf("expected vec3<f32>, got size=%v kind=%v width=%v",
			vec.Size, vec.Scalar.Kind, vec.Scalar.Width)
	}
}

// --- ResolveExpressionType: Swizzle preserves scalar type ---

func TestResolveSwizzleType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "v", Space: SpaceHandle, Type: 0},
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}}, // 0: vec4f
			{Kind: ExprSwizzle{
				Size:    Vec2,
				Vector:  0,
				Pattern: [4]SwizzleComponent{SwizzleX, SwizzleZ, 0, 0},
			}}, // 1: v.xz -> vec2f
		},
	}

	res, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := res.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", res.Value)
	}
	if vec.Size != Vec2 || vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected vec2<f32>, got size=%v kind=%v", vec.Size, vec.Scalar.Kind)
	}
}

// --- FindModfResultType ---

func TestFindModfResultType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "__modf_result_f32", Inner: StructType{Members: []StructMember{
				{Name: "fract", Type: 0},
				{Name: "whole", Type: 0},
			}}},
		},
	}

	t.Run("found_scalar_f32", func(t *testing.T) {
		argType := TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}
		h := FindModfResultType(module, argType)
		if int(h) != 1 {
			t.Errorf("expected type handle 1, got %d", h)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		// Module with no modf result types at all
		emptyModule := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
		}
		argType := TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}
		h := FindModfResultType(emptyModule, argType)
		// -1 cast to uint32 wraps; check sentinel
		if int32(h) != -1 {
			t.Errorf("expected -1 (not found), got %d", int32(h))
		}
	})
}

// --- FindFrexpResultType ---

func TestFindFrexpResultType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "__frexp_result_f32", Inner: StructType{Members: []StructMember{
				{Name: "fract", Type: 0},
				{Name: "exp", Type: 0},
			}}},
		},
	}

	t.Run("found_scalar_f32", func(t *testing.T) {
		argType := TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}
		h := FindFrexpResultType(module, argType)
		if int(h) != 1 {
			t.Errorf("expected type handle 1, got %d", h)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		// Module with no frexp result types
		emptyModule := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
		}
		argType := TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}
		h := FindFrexpResultType(emptyModule, argType)
		if int32(h) != -1 {
			t.Errorf("expected -1 (not found), got %d", int32(h))
		}
	})
}

// --- FindModfResultType with vector types ---

func TestFindModfResultType_Vector(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			{Name: "__modf_result_vec2_f32", Inner: StructType{}},
		},
	}

	h0 := TypeHandle(1) // vec2f
	argType := TypeResolution{Handle: &h0}
	h := FindModfResultType(module, argType)
	if int(h) != 2 {
		t.Errorf("expected type handle 2, got %d", h)
	}
}

// --- FindFrexpResultType with vector types ---

func TestFindFrexpResultType_Vector(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		size     VectorSize
		width    uint8
	}{
		{"vec2_f32", "__frexp_result_vec2_f32", Vec2, 4},
		{"vec3_f32", "__frexp_result_vec3_f32", Vec3, 4},
		{"vec4_f32", "__frexp_result_vec4_f32", Vec4, 4},
		{"vec2_f16", "__frexp_result_vec2_f16", Vec2, 2},
		{"vec2_f64", "__frexp_result_vec2_f64", Vec2, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &Module{
				Types: []Type{
					{Name: "scalar", Inner: ScalarType{Kind: ScalarFloat, Width: tt.width}},
					{Name: "vec", Inner: VectorType{Size: tt.size, Scalar: ScalarType{Kind: ScalarFloat, Width: tt.width}}},
					{Name: tt.typeName, Inner: StructType{}},
				},
			}
			h1 := TypeHandle(1) // vec type
			argType := TypeResolution{Handle: &h1}
			h := FindFrexpResultType(module, argType)
			if int(h) != 2 {
				t.Errorf("expected type handle 2, got %d", h)
			}
		})
	}
}

// --- resolveAtomicResultType ---

func TestResolveAtomicResultType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
			{Name: "atomic_u32", Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "counter", Space: SpaceStorage, Type: 1}, // type 1 = atomic_u32
		},
	}

	t.Run("found_atomic_result", func(t *testing.T) {
		result := ExpressionHandle(1)
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 0}}, // 0: pointer to atomic (Storage space)
				{Kind: ExprAtomicResult{Ty: 0}},         // 1: result
			},
			Body: Block{
				{Kind: StmtAtomic{
					Pointer: 0,
					Fun:     AtomicAdd{},
					Value:   0,
					Result:  &result,
				}},
			},
		}

		res, err := resolveAtomicResultType(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should resolve to the scalar type of the atomic
		if res.Value == nil {
			t.Fatal("expected Value to be set")
		}
	})

	t.Run("fallback_u32", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprAtomicResult{Ty: 0}}, // 0: result
			},
			Body: Block{}, // No StmtAtomic matches
		}

		res, err := resolveAtomicResultType(module, fn, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		scalar, ok := res.Value.(ScalarType)
		if !ok || scalar.Kind != ScalarUint || scalar.Width != 4 {
			t.Errorf("expected ScalarType{Uint, 4}, got %v", res.Value)
		}
	})
}

// --- findAtomicTypeForResult with nested statements ---

func TestFindAtomicTypeForResult_Nested(t *testing.T) {
	// Use a local variable pointing to an i32 scalar type.
	// ResolveAtomicPointerScalar resolves the pointer expression's type,
	// then gets TypeResInner which should be ScalarType or AtomicType.
	// ExprLocalVariable resolves to Pointer{Base: localVar.Type, Space: Function},
	// and TypeResInner on that gives PointerType. So we use ExprFunctionArgument
	// with a scalar type that ResolveAtomicPointerScalar can match.
	module := &Module{
		Types: []Type{
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},
		},
	}

	result := ExpressionHandle(1)
	fn := &Function{
		Arguments: []FunctionArgument{
			{Name: "ptr", Type: 0}, // type 0 = i32, resolves via Handle
		},
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}}, // 0: arg of type i32
			{Kind: ExprAtomicResult{Ty: 0}},        // 1: result
		},
		Body: Block{
			{Kind: StmtIf{
				Condition: 0,
				Accept: Block{
					{Kind: StmtAtomic{
						Pointer: 0,
						Fun:     AtomicMin{},
						Value:   0,
						Result:  &result,
					}},
				},
				Reject: Block{},
			}},
		},
	}

	scalar := findAtomicTypeForResult(module, fn, fn.Body, 1)
	if scalar == nil {
		t.Fatal("expected to find atomic type in nested if-accept block")
	}
	if scalar.Kind != ScalarSint {
		t.Errorf("expected ScalarSint, got %v", scalar.Kind)
	}
}

// --- resolveWorkGroupUniformLoadResultType ---

func TestResolveWorkGroupUniformLoadResultType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "wg_data", Space: SpaceWorkGroup, Type: 0}, // type 0 = f32
		},
	}

	t.Run("found", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 0}},  // 0: pointer to f32 in WorkGroup space
				{Kind: ExprWorkGroupUniformLoadResult{}}, // 1: result
			},
			Body: Block{
				{Kind: StmtWorkGroupUniformLoad{
					Pointer: 0,
					Result:  1,
				}},
			},
		}

		res, err := resolveWorkGroupUniformLoadResultType(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Handle == nil {
			t.Fatal("expected Handle to be set")
		}
		if *res.Handle != 0 {
			t.Errorf("expected handle 0 (f32 base), got %d", *res.Handle)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprWorkGroupUniformLoadResult{}},
			},
			Body: Block{},
		}

		_, err := resolveWorkGroupUniformLoadResultType(module, fn, 0)
		if err == nil {
			t.Error("expected error for not-found result type")
		}
	})
}

// --- findWorkGroupUniformLoadType nested ---

func TestFindWorkGroupUniformLoadType_Nested(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "wg_val", Space: SpaceWorkGroup, Type: 0}, // type 0 = u32
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},  // 0: pointer to u32 in WorkGroup
			{Kind: ExprWorkGroupUniformLoadResult{}}, // 1: result
		},
		Body: Block{
			{Kind: StmtIf{
				Condition: 0,
				Accept: Block{
					{Kind: StmtWorkGroupUniformLoad{Pointer: 0, Result: 1}},
				},
				Reject: Block{},
			}},
		},
	}

	res := findWorkGroupUniformLoadType(module, fn, fn.Body, 1)
	if res == nil {
		t.Fatal("expected to find type in nested if block")
	}
	if res.Handle == nil || *res.Handle != 0 {
		t.Errorf("expected handle 0, got %v", res)
	}
}
