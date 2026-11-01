package typifier

// The behavior tests of the expression typing the typifier took over from
// ir.ResolveExpressionType, which they tested.

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestResolveLoadType_Dereferences(t *testing.T) {
	// Tests the IR invariant: ExprLoad through a pointer produces the base type.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "atomic_u32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "vec3f", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "x", Space: ir.SpacePrivate, Type: 0},   // f32
			{Name: "a", Space: ir.SpaceWorkGroup, Type: 1}, // atomic<u32>
		},
	}

	t.Run("load_pointer_to_scalar", func(t *testing.T) {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}}, // 0: ptr<private, f32>
				{Kind: ir.ExprLoad{Pointer: 0}},            // 1: load -> f32
			},
		}
		res, err := resolve(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Handle == nil || *res.Handle != 0 {
			t.Errorf("expected Handle=0 (f32), got %v", res)
		}
	})

	t.Run("load_pointer_to_atomic_resolves_scalar", func(t *testing.T) {
		// IR invariant: Load on Atomic(scalar) → scalar (not the atomic wrapper)
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 1}}, // 0: ptr<workgroup, atomic<u32>>
				{Kind: ir.ExprLoad{Pointer: 0}},            // 1: load -> u32 (scalar from atomic)
			},
		}
		res, err := resolve(module, fn, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		scalar, ok := res.Value.(ir.ScalarType)
		if !ok || scalar.Kind != ir.ScalarUint || scalar.Width != 4 {
			t.Errorf("expected ScalarType{Uint, 4}, got %v", res.Value)
		}
	})

	t.Run("load_value_pointer_vector", func(t *testing.T) {
		// ValuePointer{Size: &Vec3, Scalar} → Load → VectorType{Vec3, Scalar}
		f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		vec3 := ir.Vec3
		m := &ir.Module{
			Types: []ir.Type{
				{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: vec3, Scalar: f32}},
				{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
			},
		}
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},     // 0: ptr<function, mat2x3>
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 1: a column, a value pointer to vec3
				{Kind: ir.ExprLoad{Pointer: 1}},               // 2: load
			},
			Arguments: []ir.FunctionArgument{{Name: "p", Type: 1}},
		}
		res, err := resolve(m, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := res.Value; got != (ir.VectorType{Size: vec3, Scalar: f32}) {
			t.Errorf("expected vec3<f32>, got %#v", got)
		}
	})
}

func TestResolveAccessType_Containers(t *testing.T) {
	f32Scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: f32Scalar},                                                            // 0
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar}},                    // 1
			{Name: "mat4x4", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: f32Scalar}}, // 2
			{Name: "arr", Inner: ir.ArrayType{Base: 0, Stride: 4}},                                     // 3
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "vec", Space: ir.SpaceHandle, Type: 1}, // handle space -> direct type
			{Name: "mat", Space: ir.SpaceHandle, Type: 2},
			{Name: "arr", Space: ir.SpaceHandle, Type: 3},
		},
	}

	t.Run("access_vector_returns_scalar", func(t *testing.T) {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},  // 0: vec4f
				{Kind: ir.Literal{Value: ir.LiteralU32(2)}}, // 1: index
				{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // 2: vec[i] -> f32
			},
		}
		res, err := resolve(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		scalar, ok := res.Value.(ir.ScalarType)
		if !ok {
			t.Fatalf("expected ScalarType, got %T", res.Value)
		}
		if scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
			t.Errorf("expected f32, got kind=%v width=%v", scalar.Kind, scalar.Width)
		}
	})

	t.Run("access_matrix_returns_column_vector", func(t *testing.T) {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 1}},  // 0: mat4x4
				{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 1: index
				{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // 2: mat[i] -> vec4f
			},
		}
		res, err := resolve(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		vec, ok := res.Value.(ir.VectorType)
		if !ok {
			t.Fatalf("expected VectorType, got %T", res.Value)
		}
		if vec.Size != ir.Vec4 {
			t.Errorf("expected Vec4 column, got %v", vec.Size)
		}
	})

	t.Run("access_array_returns_element_handle", func(t *testing.T) {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 2}},  // 0: arr
				{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 1: index
				{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // 2: arr[i] -> f32
			},
		}
		res, err := resolve(module, fn, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Handle == nil || *res.Handle != 0 {
			t.Errorf("expected Handle=0 (f32 base), got %v", res)
		}
	})
}

func TestResolveAccessIndexType_Struct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // 1
			{Name: "MyStruct", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0, Offset: 0}, // f32
					{Name: "y", Type: 1, Offset: 4}, // u32
				},
				Span: 8,
			}}, // 2
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "s", Space: ir.SpaceHandle, Type: 2}, // MyStruct
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: MyStruct
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 1: s.y -> u32
		},
	}

	res, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Handle == nil || *res.Handle != 1 {
		t.Errorf("expected Handle=1 (u32), got %v", res)
	}
}

func TestResolveSplatType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 0: f32 literal
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}}, // 1: splat -> vec3f
		},
	}

	res, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := res.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", res.Value)
	}
	if vec.Size != ir.Vec3 || vec.Scalar.Kind != ir.ScalarFloat || vec.Scalar.Width != 4 {
		t.Errorf("expected vec3<f32>, got size=%v kind=%v width=%v",
			vec.Size, vec.Scalar.Kind, vec.Scalar.Width)
	}
}

func TestResolveSwizzleType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "v", Space: ir.SpaceHandle, Type: 0},
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}}, // 0: vec4f
			{Kind: ir.ExprSwizzle{
				Size:    ir.Vec2,
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleZ, 0, 0},
			}}, // 1: v.xz -> vec2f
		},
	}

	res, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := res.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", res.Value)
	}
	if vec.Size != ir.Vec2 || vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected vec2<f32>, got size=%v kind=%v", vec.Size, vec.Scalar.Kind)
	}
}

func TestTypify_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{Name: "test", Expressions: []ir.Expression{}}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range handle")
	}
}

func TestTypify_Override(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Overrides: []ir.Override{
			{Name: "gamma", Ty: 0},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprOverride{Override: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0, got %v", got)
	}
}

func TestTypify_Override_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprOverride{Override: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range override")
	}
}

func TestTypify_ZeroValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestTypify_Compose(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec2f", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprCompose{Type: 0, Components: nil}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestTypify_GlobalVariable_HandleSpace(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "sampler", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "s", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Handle space: returns the type directly (not pointer)
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 for handle-space global")
	}
}

func TestTypify_GlobalVariable_NonHandleSpace(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 0, Space: ir.SpaceUniform},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-handle space: returns pointer type
	ptr, ok := got.Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expected PointerType, got %T", got.Value)
	}
	if ptr.Base != 0 || ptr.Space != ir.SpaceUniform {
		t.Errorf("expected Pointer{base:0, space:Uniform}, got %v", ptr)
	}
}

func TestTypify_GlobalVariable_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range global variable")
	}
}

func TestTypify_LocalVariable(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 0},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ptr, ok := got.Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expected PointerType, got %T", got.Value)
	}
	if ptr.Space != ir.SpaceFunction {
		t.Errorf("expected Function space, got %d", ptr.Space)
	}
}

func TestTypify_LocalVariable_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range local variable")
	}
}

func TestTypify_FunctionArgument_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range function argument")
	}
}

func TestTypify_Load(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.ExprLoad{Pointer: 0}},
		},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 0},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Load dereferences pointer: pointer<Function, f32> -> f32 (handle)
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 (dereferenced pointer)")
	}
}

func TestTypify_ArrayLength(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprArrayLength{Array: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarUint || s.Width != 4 {
		t.Errorf("expected u32, got %v", got.Value)
	}
}

func TestTypify_AtomicResult(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprAtomicResult{Ty: 0, Comparison: false}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestTypify_SubgroupBallotResult(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprSubgroupBallotResult{}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != 4 || vec.Scalar.Kind != ir.ScalarUint || vec.Scalar.Width != 4 {
		t.Errorf("expected vec4<u32>, got %v", vec)
	}
}

func TestTypify_SubgroupOperationResult(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprSubgroupOperationResult{Type: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestTypify_RayQueryProceedResult(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprRayQueryProceedResult{}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarBool || s.Width != 1 {
		t.Errorf("expected bool, got %v", got.Value)
	}
}

func TestTypify_RayQueryGetIntersection(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "RayIntersection", Inner: ir.StructType{Members: nil, Span: 32}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprRayQueryGetIntersection{Query: 0, Committed: true}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (RayIntersection), got %v", got.Handle)
	}
}

func TestTypify_RayQueryGetIntersection_NotFound(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprRayQueryGetIntersection{Query: 0, Committed: true}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error when RayIntersection type not found")
	}
}

func TestTypify_CallResult(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{Name: "helper", Result: &ir.FunctionResult{Type: 0}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprCallResult{Function: 0}},
		},
	}
	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestTypify_CallResult_NoReturn(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "void_func", Result: nil},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprCallResult{Function: 0}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for function with no return type")
	}
}

func TestTypify_CallResult_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprCallResult{Function: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range function")
	}
}

func TestTypify_Constant_OutOfRange(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprConstant{Constant: 99}},
		},
	}
	_, err := resolve(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range constant")
	}
}

func TestTypify_Unary(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unary preserves type
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestTypify_Select(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			{Kind: ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2}},
		},
	}
	got, err := resolve(module, fn, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestTypify_Derivative(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine, Expr: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestTypify_Relational_Vector(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3f", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprRelational{Fun: ir.RelationalIsNan, Argument: 0}},
		},
		Arguments: []ir.FunctionArgument{{Name: "v", Type: 0}},
	}

	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec3 || vec.Scalar.Kind != ir.ScalarBool {
		t.Errorf("expected vec3<bool>, got %v", vec)
	}
}

func TestTypify_Relational_AllAny_CollapseToScalar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3b", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprRelational{Fun: ir.RelationalAll, Argument: 0}},
		},
		Arguments: []ir.FunctionArgument{{Name: "v", Type: 0}},
	}

	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarBool {
		t.Errorf("expected scalar bool, got %v", got.Value)
	}
}

func TestTypify_Relational_Scalar(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprRelational{Fun: ir.RelationalIsNan, Argument: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarBool {
		t.Errorf("expected bool, got %v", got.Value)
	}
}

func TestTypify_As_Convert(t *testing.T) {
	module := &ir.Module{}
	w := uint8(4)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
			{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: &w}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestTypify_As_Bitcast(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
			{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: nil}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32 (bitcast preserves width), got %v", got.Value)
	}
}

func TestTypify_As_VectorConvert(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3i", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
	}
	w := uint8(4)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarFloat, Convert: &w}},
		},
		Arguments: []ir.FunctionArgument{{Name: "v", Type: 0}},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec3 || vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected vec3<f32>, got %v", vec)
	}
}

func TestTypify_Swizzle(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprSwizzle{Size: ir.Vec2, Vector: 0, Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY}}},
		},
		Arguments: []ir.FunctionArgument{{Name: "v", Type: 0}},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec2 || vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected vec2<f32>, got %v", vec)
	}
}

func TestTypify_Access_Array(t *testing.T) {
	size := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "arr", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size}, Stride: 4}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},
		},
		Arguments: []ir.FunctionArgument{{Name: "a", Type: 1}},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 (array element type)")
	}
}

func TestTypify_Access_Matrix(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "mat4x4f", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},
		},
		Arguments: []ir.FunctionArgument{{Name: "m", Type: 0}},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Matrix access returns column vector
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec4 || vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected vec4<f32>, got %v", vec)
	}
}

func TestTypify_AccessIndex_Struct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "MyStruct", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0, Offset: 0},
					{Name: "y", Type: 1, Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // access .y
		},
		Arguments: []ir.FunctionArgument{{Name: "s", Type: 2}},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (u32), got %v", got.Handle)
	}
}

func TestTypify_Binary_Multiply_MatVec(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "mat4x4f", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // mat
			{Kind: ir.ExprFunctionArgument{Index: 1}}, // vec
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}},
		},
		Arguments: []ir.FunctionArgument{
			{Name: "m", Type: 0},
			{Name: "v", Type: 1},
		},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mat4x4 * vec4 -> vec4 (rows of matrix)
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec4 {
		t.Errorf("expected vec4, got vec%d", vec.Size)
	}
}

func TestTypify_Binary_Multiply_MatMat(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "mat4x4f", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "mat2x4f", Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // mat4x4
			{Kind: ir.ExprFunctionArgument{Index: 1}}, // mat2x4
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}},
		},
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mat4x4 * mat2x4 -> mat2x4 (right cols, left rows)
	mat, ok := got.Value.(ir.MatrixType)
	if !ok {
		t.Fatalf("expected MatrixType, got %T", got.Value)
	}
	if mat.Columns != ir.Vec2 || mat.Rows != ir.Vec4 {
		t.Errorf("expected mat2x4, got mat%dx%d", mat.Columns, mat.Rows)
	}
}

func TestTypify_Binary_VectorComparison(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3f", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprBinary{Op: ir.BinaryEqual, Left: 0, Right: 1}},
		},
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 0},
		},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Vector comparison returns vec<bool>
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec3 || vec.Scalar.Kind != ir.ScalarBool {
		t.Errorf("expected vec3<bool>, got %v", vec)
	}
}

func TestTypify_Binary_ScalarVectorBroadcast(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},              // 0: scalar
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}},              // 1: vec3 (from splat)
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}}, // 2: scalar + vec = vec
		},
	}
	got, err := resolve(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType (broadcast), got %T", got.Value)
	}
	if vec.Size != ir.Vec3 {
		t.Errorf("expected vec3, got vec%d", vec.Size)
	}
}

func TestTypify_Math_Dot4Packed(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
			{Kind: ir.ExprMath{Fun: ir.MathDot4I8Packed, Arg: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarSint || s.Width != 4 {
		t.Errorf("expected i32, got %v", got.Value)
	}
}

func TestTypify_Math_PackUnpack(t *testing.T) {
	module := &ir.Module{}

	packFuns := []ir.MathFunction{ir.MathPack4xI8, ir.MathPack4xU8, ir.MathPack4xI8Clamp, ir.MathPack4xU8Clamp}
	for _, fun := range packFuns {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
				{Kind: ir.ExprMath{Fun: fun, Arg: 0}},
			},
		}
		got, err := resolve(module, fn, 1)
		if err != nil {
			t.Fatalf("pack function %d: unexpected error: %v", fun, err)
		}
		s, ok := got.Value.(ir.ScalarType)
		if !ok || s.Kind != ir.ScalarUint {
			t.Errorf("pack function %d: expected u32, got %v", fun, got.Value)
		}
	}

	// unpack4xI8 -> vec4<i32>
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
			{Kind: ir.ExprMath{Fun: ir.MathUnpack4xI8, Arg: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok || vec.Size != ir.Vec4 || vec.Scalar.Kind != ir.ScalarSint {
		t.Errorf("expected vec4<i32>, got %v", got.Value)
	}
}

func TestTypify_ImageLoad_StorageTexture(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "storage_tex", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatRgba8Uint,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Scalar.Kind != ir.ScalarUint {
		t.Errorf("expected uint storage texture load result, got kind %d", vec.Scalar.Kind)
	}
}

func TestTypify_ImageSample_DepthTexture(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "depth_tex", Inner: ir.ImageType{
				Dim:   ir.Dim2D,
				Class: ir.ImageClassDepth,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      ir.SampleLevelZero{},
			}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Depth texture sample returns scalar f32
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32 (depth sample), got %v", got.Value)
	}
}

func TestTypify_ImageSample_Gather(t *testing.T) {
	gather := ir.SwizzleX
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "tex", Inner: ir.ImageType{
				Dim:         ir.Dim2D,
				Class:       ir.ImageClassSampled,
				SampledKind: ir.ScalarFloat,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      ir.SampleLevelZero{},
				Gather:     &gather,
			}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Gather always returns vec4
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType (gather), got %T", got.Value)
	}
	if vec.Size != ir.Vec4 || vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected vec4<f32> (gather), got %v", vec)
	}
}

func TestTypify_ImageQuery_Size(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "tex3d", Inner: ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQuerySize{}}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3D image: size returns vec3<u32>
	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != ir.Vec3 || vec.Scalar.Kind != ir.ScalarUint {
		t.Errorf("expected vec3<u32>, got %v", vec)
	}
}

func TestTypify_ImageQuery_Size_1D(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "tex1d", Inner: ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQuerySize{}}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1D: returns scalar u32
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarUint {
		t.Errorf("expected scalar u32, got %v", got.Value)
	}
}

func TestTypify_ImageQuery_NumLevels(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "tex", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Type: 0, Space: ir.SpaceHandle},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQueryNumLevels{}}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarUint || s.Width != 4 {
		t.Errorf("expected u32, got %v", got.Value)
	}
}

func TestTypify_MathModf(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "__modf_result_f32", Inner: ir.StructType{Members: nil, Span: 8}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.5)}},
			{Kind: ir.ExprMath{Fun: ir.MathModf, Arg: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (__modf_result_f32), got %v", got.Handle)
	}
}

func TestTypify_MathFrexp(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "__frexp_result_f32", Inner: ir.StructType{Members: nil, Span: 8}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.5)}},
			{Kind: ir.ExprMath{Fun: ir.MathFrexp, Arg: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (__frexp_result_f32), got %v", got.Handle)
	}
}

func TestTypify_MathCountBits(t *testing.T) {
	module := &ir.Module{}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0xFF)}},
			{Kind: ir.ExprMath{Fun: ir.MathCountOneBits, Arg: 0}},
		},
	}
	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ir.ScalarType)
	if !ok || s.Kind != ir.ScalarUint {
		t.Errorf("expected u32 (count bits preserves type), got %v", got.Value)
	}
}

func TestResolve_AccessIndex_PointerToMatrix_ProducesValuePointer(t *testing.T) {
	// Pointer<Matrix3x2<f32>>[0] → ValuePointerType{Size: &Vec2, Scalar: f32, Space: Storage}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                            // 0
			{Name: "mat3x2", Inner: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "m", Type: 1, Space: ir.SpaceStorage, Access: ir.StorageAccessLoad | ir.StorageAccessStore}, // 0: var<storage> m: mat3x2<f32>
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &m (Pointer<mat3x2, Storage>)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] (column 0)
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	// Resolve expr 0
	res0, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve expr 0: %v", err)
	}
	types[0] = res0
	// Should be PointerType{Base: mat3x2, Space: Storage}
	ptr, ok := res0.Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", res0.Value, res0)
	}
	if ptr.Space != ir.SpaceStorage {
		t.Errorf("expr 0: expected Storage space, got %v", ptr.Space)
	}

	// Resolve expr 1: AccessIndex on Pointer<Matrix>
	res1, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve expr 1: %v", err)
	}
	types[1] = res1

	vp, ok := res1.Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 1: expected ValuePointerType, got %T (%+v)", res1.Value, res1)
	}
	if vp.Size == nil || *vp.Size != ir.Vec2 {
		t.Errorf("expr 1: expected Size=Vec2 (column vector), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ir.ScalarFloat || vp.Scalar.Width != 4 {
		t.Errorf("expr 1: expected f32 scalar, got %+v", vp.Scalar)
	}
	if vp.Space != ir.SpaceStorage {
		t.Errorf("expr 1: expected Storage space, got %v", vp.Space)
	}
}

func TestResolve_AccessIndex_PointerToVector_ProducesValuePointer(t *testing.T) {
	// Pointer<Vec4<i32>>[2] → ValuePointerType{Size: nil, Scalar: i32, Space: Function}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                         // 0
			{Name: "vec4i", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}, // 1
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // 0: &vec0 (Pointer<vec4i, Function>)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}}, // 1: vec0[2]
		},

		LocalVars: []ir.LocalVariable{
			{Name: "vec0", Type: 1}, // vec4<i32>
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	// Resolve expr 0: LocalVariable → Pointer<vec4i, Function>
	res0, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve expr 0: %v", err)
	}
	types[0] = res0

	// Resolve expr 1: AccessIndex on Pointer<Vector>
	res1, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve expr 1: %v", err)
	}

	vp, ok := res1.Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 1: expected ValuePointerType, got %T (%+v)", res1.Value, res1)
	}
	if vp.Size != nil {
		t.Errorf("expr 1: expected Size=nil (scalar pointer), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ir.ScalarSint || vp.Scalar.Width != 4 {
		t.Errorf("expr 1: expected i32 scalar, got %+v", vp.Scalar)
	}
	if vp.Space != ir.SpaceFunction {
		t.Errorf("expr 1: expected Function space, got %v", vp.Space)
	}
}

func TestResolve_AccessIndex_ValuePointerVector_ProducesValuePointerScalar(t *testing.T) {
	// ValuePointerType{Size: &Vec3}[1] → ValuePointerType{Size: nil}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "mat3x3", Inner: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "m", Type: 1, Space: ir.SpaceUniform},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &m
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] → ValuePointer(vec3)
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 1}}, // 2: m[0][1] → ValuePointer(scalar)
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 2 should be ValuePointerType{Size: nil} — pointer to scalar element
	vp, ok := types[2].Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", types[2].Value, types[2])
	}
	if vp.Size != nil {
		t.Errorf("expr 2: expected Size=nil (scalar), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expr 2: expected float scalar, got %+v", vp.Scalar)
	}
}

func TestResolve_Load_ValuePointerScalar_ProducesScalar(t *testing.T) {
	// Load(ValuePointerType{Size: nil, Scalar: i32}) → ScalarType{Sint, 4}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "vec4i", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // 0: &vec0
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 1: vec0[1] → ValuePointer(scalar)
			{Kind: ir.ExprLoad{Pointer: 1}},               // 2: load → i32
		},

		LocalVars: []ir.LocalVariable{
			{Name: "vec0", Type: 1},
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 2 (Load) should resolve to ScalarType{Sint, 4}
	scalar, ok := types[2].Value.(ir.ScalarType)
	if !ok {
		t.Fatalf("expr 2: expected ScalarType, got %T (%+v)", types[2].Value, types[2])
	}
	if scalar.Kind != ir.ScalarSint || scalar.Width != 4 {
		t.Errorf("expr 2: expected Sint(4), got %+v", scalar)
	}
}

func TestResolve_Load_ValuePointerVector_ProducesVector(t *testing.T) {
	// Load(ValuePointerType{Size: &Vec2, Scalar: f32}) → VectorType{Vec2, f32}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "mat3x2", Inner: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "m", Type: 1, Space: ir.SpaceStorage, Access: ir.StorageAccessLoad | ir.StorageAccessStore},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &m
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] → ValuePointer(vec2)
			{Kind: ir.ExprLoad{Pointer: 1}},               // 2: load → vec2<f32>
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 2 (Load) should resolve to VectorType{Vec2, f32}
	vec, ok := types[2].Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expr 2: expected VectorType, got %T (%+v)", types[2].Value, types[2])
	}
	if vec.Size != ir.Vec2 {
		t.Errorf("expr 2: expected Vec2, got %v", vec.Size)
	}
	if vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expr 2: expected float scalar, got %+v", vec.Scalar)
	}
}

func TestResolve_Access_PointerToMatrix_ProducesValuePointer(t *testing.T) {
	// Dynamic access: Pointer<Matrix>[idx] → ValuePointerType{Size: &rows}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "mat4x3", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "m", Type: 1, Space: ir.SpaceUniform},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}}, // 0: &m
			{Kind: ir.ExprFunctionArgument{Index: 0}},  // 1: idx (i32)
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},   // 2: m[idx] → ValuePointer(vec3)
		},

		Arguments: []ir.FunctionArgument{
			{Name: "idx", Type: 0},
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	vp, ok := types[2].Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", types[2].Value, types[2])
	}
	if vp.Size == nil || *vp.Size != ir.Vec3 {
		t.Errorf("expr 2: expected Size=Vec3 (column rows), got %v", vp.Size)
	}
}

func TestResolve_AccessIndex_PointerToStruct_ProducesPointerToMember(t *testing.T) {
	// AccessIndex on Pointer<Struct>.member → PointerType{Base: memberType, Space}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                            // 0
			{Name: "mat4x3", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1
			{Name: "vec3f", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                    // 2
			{Name: "S", Inner: ir.StructType{Members: []ir.StructMember{{Name: "m", Type: 1}, {Name: "v", Type: 2}}, Span: 64}},            // 3
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 3, Space: ir.SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &g (Pointer<S, Uniform>)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.m (member 0 = mat4x3)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 2: g.v (member 1 = vec3f)
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 0: Pointer<S, Uniform>
	ptr0, ok := types[0].Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", types[0].Value, types[0])
	}
	if ptr0.Base != 3 || ptr0.Space != ir.SpaceUniform {
		t.Errorf("expr 0: expected Pointer{Base:3, Uniform}, got %+v", ptr0)
	}

	// expr 1: Pointer<mat4x3, Uniform> (struct member through pointer)
	ptr1, ok := types[1].Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 1: expected PointerType, got %T (%+v)", types[1].Value, types[1])
	}
	if ptr1.Base != 1 {
		t.Errorf("expr 1: expected Base=1 (mat4x3), got %d", ptr1.Base)
	}
	if ptr1.Space != ir.SpaceUniform {
		t.Errorf("expr 1: expected Uniform space, got %v", ptr1.Space)
	}

	// expr 2: Pointer<vec3f, Uniform> (struct member through pointer)
	ptr2, ok := types[2].Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 2: expected PointerType, got %T (%+v)", types[2].Value, types[2])
	}
	if ptr2.Base != 2 {
		t.Errorf("expr 2: expected Base=2 (vec3f), got %d", ptr2.Base)
	}
	if ptr2.Space != ir.SpaceUniform {
		t.Errorf("expr 2: expected Uniform space, got %v", ptr2.Space)
	}
}

func TestResolve_Load_PointerToMember_ProducesValue(t *testing.T) {
	// Load(Pointer<Vec3<f32>>) → TypeHandle(Vec3<f32>)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                         // 0
			{Name: "vec3f", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1
			{Name: "S", Inner: ir.StructType{Members: []ir.StructMember{{Name: "v", Type: 1}}, Span: 16}},               // 2
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 2, Space: ir.SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &g (Pointer<S, Uniform>)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.v (Pointer<vec3f, Uniform>)
			{Kind: ir.ExprLoad{Pointer: 1}},               // 2: load → vec3<f32>
		},
	}
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 3; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 2 (Load) should produce a Handle to vec3f (type 1)
	// Load through a PointerType resolves as Handle-based (referencing module type).
	expectedHandle := ir.TypeHandle(1)
	if types[2].Handle == nil || *types[2].Handle != expectedHandle {
		t.Fatalf("expr 2: expected Handle=1 (vec3f), got %v", types[2].Handle)
	}
	// Verify the referenced module type is vec3<f32>
	vec, ok := module.Types[*types[2].Handle].Inner.(ir.VectorType)
	if !ok {
		t.Fatalf("expr 2: expected module type 1 to be VectorType, got %T", module.Types[*types[2].Handle].Inner)
	}
	if vec.Size != ir.Vec3 {
		t.Errorf("expr 2: expected Vec3, got %v", vec.Size)
	}
	if vec.Scalar.Kind != ir.ScalarFloat || vec.Scalar.Width != 4 {
		t.Errorf("expr 2: expected f32 scalar, got %+v", vec.Scalar)
	}
}

func TestResolve_FullChain_PointerStructMatrixColumnElement(t *testing.T) {
	// Full chain: GlobalVar(Pointer<Struct>) → AccessIndex(member=mat) → AccessIndex(column)
	//   → AccessIndex(element) → Load
	// Type 0: f32
	// Type 1: mat4x3<f32> (4 columns of vec3)
	// Type 2: struct S { m: mat4x3<f32> }
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                            // 0
			{Name: "mat4x3", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1
			{Name: "S", Inner: ir.StructType{Members: []ir.StructMember{{Name: "m", Type: 1}}, Span: 64}},                                  // 2
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 2, Space: ir.SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: &g
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.m (struct member → matrix)
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 2}}, // 2: g.m[2] (matrix column)
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 1}}, // 3: g.m[2][1] (vector element)
			{Kind: ir.ExprLoad{Pointer: 3}},               // 4: load
		},
	}

	// Resolve all expressions sequentially
	types := make([]ir.TypeResolution, len(fn.Expressions))
	for i := 0; i < 5; i++ {
		res, err := resolve(module, fn, ir.ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		types[i] = res
	}

	// expr 0: PointerType{Base: Struct(2), Space: Uniform}
	ptr0, ok := types[0].Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", types[0].Value, types[0])
	}
	if ptr0.Base != 2 {
		t.Errorf("expr 0: expected Base=2 (struct S), got %d", ptr0.Base)
	}
	if ptr0.Space != ir.SpaceUniform {
		t.Errorf("expr 0: expected Uniform space, got %v", ptr0.Space)
	}

	// expr 1: PointerType{Base: MatrixType(1), Space: Uniform} — struct member through pointer
	ptr1, ok := types[1].Value.(ir.PointerType)
	if !ok {
		t.Fatalf("expr 1: expected PointerType, got %T (%+v)", types[1].Value, types[1])
	}
	if ptr1.Base != 1 {
		t.Errorf("expr 1: expected Base=1 (mat4x3), got %d", ptr1.Base)
	}
	if ptr1.Space != ir.SpaceUniform {
		t.Errorf("expr 1: expected Uniform space, got %v", ptr1.Space)
	}

	// expr 2: ValuePointerType{Size: &Vec3, Scalar: f32, Space: Uniform} — matrix column
	vp2, ok := types[2].Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", types[2].Value, types[2])
	}
	if vp2.Size == nil || *vp2.Size != ir.Vec3 {
		t.Errorf("expr 2: expected Size=Vec3 (column vector), got %v", vp2.Size)
	}
	if vp2.Scalar.Kind != ir.ScalarFloat || vp2.Scalar.Width != 4 {
		t.Errorf("expr 2: expected f32 scalar, got %+v", vp2.Scalar)
	}
	if vp2.Space != ir.SpaceUniform {
		t.Errorf("expr 2: expected Uniform space, got %v", vp2.Space)
	}

	// expr 3: ValuePointerType{Size: nil, Scalar: f32, Space: Uniform} — vector element
	vp3, ok := types[3].Value.(ir.ValuePointerType)
	if !ok {
		t.Fatalf("expr 3: expected ValuePointerType, got %T (%+v)", types[3].Value, types[3])
	}
	if vp3.Size != nil {
		t.Errorf("expr 3: expected Size=nil (scalar pointer), got %v", vp3.Size)
	}
	if vp3.Scalar.Kind != ir.ScalarFloat || vp3.Scalar.Width != 4 {
		t.Errorf("expr 3: expected f32 scalar, got %+v", vp3.Scalar)
	}
	if vp3.Space != ir.SpaceUniform {
		t.Errorf("expr 3: expected Uniform space, got %v", vp3.Space)
	}

	// expr 4: ScalarType{Float, 4} — loaded value
	scalar, ok := types[4].Value.(ir.ScalarType)
	if !ok {
		t.Fatalf("expr 4: expected ScalarType, got %T (%+v)", types[4].Value, types[4])
	}
	if scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
		t.Errorf("expr 4: expected f32, got %+v", scalar)
	}
}

// Result types of the math built-ins whose result is not the argument's
// type (WGSL spec; Rust naga's overload conclusions).
func TestResolveMathResultTypes(t *testing.T) {
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	m := &ir.Module{Types: []ir.Type{
		{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32}}, // 0: mat2x3<f32>
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f16}},                   // 1: vec3<f16>
		{Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},                   // 2: vec2<f32>
		{Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},                   // 3: vec4<f32>
		{Inner: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32}}, // 4: mat3x3<f32>
	}}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprZeroValue{Type: 0}}, // 0: mat2x3<f32>
		{Kind: ir.ExprZeroValue{Type: 1}}, // 1: vec3<f16>
		{Kind: ir.ExprZeroValue{Type: 2}}, // 2: vec2<f32>
		{Kind: ir.ExprZeroValue{Type: 3}}, // 3: vec4<f32>
		{Kind: ir.ExprZeroValue{Type: 4}}, // 4: mat3x3<f32>
	}}
	one, two, three := ir.ExpressionHandle(1), ir.ExpressionHandle(2), ir.ExpressionHandle(3)
	tests := []struct {
		name string
		math ir.ExprMath
		want ir.TypeInner
	}{
		{"determinant", ir.ExprMath{Fun: ir.MathDeterminant, Arg: 4}, f32},
		{"transpose", ir.ExprMath{Fun: ir.MathTranspose, Arg: 0}, ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: f32}},
		{"outer", ir.ExprMath{Fun: ir.MathOuter, Arg: 2, Arg1: &three}, ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec2, Scalar: f32}},
		{"length f16", ir.ExprMath{Fun: ir.MathLength, Arg: one}, f16},
		{"distance f16", ir.ExprMath{Fun: ir.MathDistance, Arg: one, Arg1: &one}, f16},
		{"dot", ir.ExprMath{Fun: ir.MathDot, Arg: two, Arg1: &two}, f32},
	}
	for _, tt := range tests {
		fn.Expressions = append(fn.Expressions[:5], ir.Expression{Kind: tt.math})
		res, err := resolve(m, fn, 5)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got := res.Inner(m.Types); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: result %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

// A sampled image's texel type is its SampledKind (Rust ImageClass::Sampled
// { kind }); i32 is a kind like any other, not "unset".
func TestResolveImageLoadSampledKind(t *testing.T) {
	for _, kind := range []ir.ScalarKind{ir.ScalarSint, ir.ScalarUint, ir.ScalarFloat} {
		m := &ir.Module{Types: []ir.Type{{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: kind}}}}
		fn := &ir.Function{Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
			{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 1}},
		}}
		m.GlobalVariables = []ir.GlobalVariable{{Name: "t", Space: ir.SpaceHandle, Type: 0}}
		res, err := resolve(m, fn, 2)
		if err != nil {
			t.Fatal(err)
		}
		want := ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: kind, Width: 4}}
		if got := res.Inner(m.Types); got != want {
			t.Errorf("kind %v: textureLoad type = %#v, want %#v", kind, got, want)
		}
	}
}

func TestTypify_Constant(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: 0, Init: 0},
		},
		GlobalExpressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF32(0)}}},
	}

	fn := &ir.Function{
		Name:        "test",
		Expressions: []ir.Expression{{Kind: ir.ExprConstant{Constant: 0}}},
	}

	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	if got.Handle == nil {
		t.Errorf("expected type handle, got inline type")
	} else if *got.Handle != 0 {
		t.Errorf("expected type handle 0, got %d", *got.Handle)
	}
}

func TestTypify_FunctionArgument(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4<f32>", Inner: ir.VectorType{
				Size:   ir.Vec4,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: 0},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
	}

	got, err := resolve(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	if got.Handle == nil {
		t.Errorf("expected type handle, got inline type")
	} else if *got.Handle != 0 {
		t.Errorf("expected type handle 0, got %d", *got.Handle)
	}
}

func TestTypify_Splat(t *testing.T) {
	module := &ir.Module{}

	fn := &ir.Function{
		Name: "test",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // Expression 0: scalar
			{Kind: ir.ExprSplat{Size: ir.Vec4, Value: 0}}, // Expression 1: splat
		},
	}

	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	if got.Handle != nil {
		t.Errorf("expected inline type, got handle")
	}

	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}

	if vec.Size != ir.Vec4 {
		t.Errorf("expected size Vec4, got %d", vec.Size)
	}
	if vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", vec.Scalar.Kind)
	}
}

func TestTypify_AccessIndex(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4<f32>", Inner: ir.VectorType{
				Size:   ir.Vec4,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // vec4<f32>
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}}, // vec[2] -> f32
		},
	}
	fn.Arguments = []ir.FunctionArgument{{Name: "v", Type: 1}}

	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	if got.Handle != nil {
		t.Errorf("expected inline type, got handle")
	}

	scalar, ok := got.Value.(ir.ScalarType)
	if !ok {
		t.Fatalf("expected ScalarType, got %T", got.Value)
	}

	if scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", scalar.Kind)
	}
}

func TestTypify_Binary(t *testing.T) {
	module := &ir.Module{}

	tests := []struct {
		name     string
		op       ir.BinaryOperator
		wantType ir.TypeInner
	}{
		{
			name:     "arithmetic preserves type",
			op:       ir.BinaryAdd,
			wantType: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
		{
			name:     "comparison returns bool",
			op:       ir.BinaryEqual,
			wantType: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
		},
		{
			name:     "logical and returns bool",
			op:       ir.BinaryLogicalAnd,
			wantType: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name: "test",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // Left operand
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}}, // Right operand
					{Kind: ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}},
				},
			}

			got, err := resolve(module, fn, 2)
			if err != nil {
				t.Fatalf("resolve() error = %v", err)
			}

			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("got type %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestTypify_MathFunctions(t *testing.T) {
	module := &ir.Module{}

	tests := []struct {
		name     string
		mathFunc ir.MathFunction
		argType  ir.TypeInner
		wantType ir.TypeInner
	}{
		{
			name:     "sin preserves type",
			mathFunc: ir.MathSin,
			argType:  ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			wantType: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
		{
			name:     "dot returns scalar",
			mathFunc: ir.MathDot,
			argType: ir.VectorType{
				Size:   ir.Vec3,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			},
			wantType: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
		{
			name:     "length returns f32",
			mathFunc: ir.MathLength,
			argType: ir.VectorType{
				Size:   ir.Vec3,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			},
			wantType: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &ir.Function{
				Name: "test",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // Placeholder arg
					{Kind: ir.ExprMath{Fun: tt.mathFunc, Arg: 0}},
				},
			}

			got, err := resolve(module, fn, 1)
			if err != nil {
				t.Fatalf("resolve() error = %v", err)
			}

			if !typeInnerEqual(got.Value, tt.wantType) {
				t.Errorf("got type %v, want %v", got.Value, tt.wantType)
			}
		})
	}
}

func TestTypify_ImageSample(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "texture_2d<f32>", Inner: ir.ImageType{
				Dim:         ir.Dim2D,
				Class:       ir.ImageClassSampled,
				SampledKind: ir.ScalarFloat,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}}, // Image
			{Kind: ir.ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      ir.SampleLevelZero{},
			}},
		},
	}

	module.GlobalVariables = []ir.GlobalVariable{
		{Name: "tex", Type: 0, Space: ir.SpaceHandle},
	}

	got, err := resolve(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	vec, ok := got.Value.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}

	if vec.Size != ir.Vec4 {
		t.Errorf("expected Vec4, got %d", vec.Size)
	}
	if vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected ScalarFloat, got %d", vec.Scalar.Kind)
	}
}

// Helper function to compare TypeInner values
func typeInnerEqual(a, b ir.TypeInner) bool {
	switch at := a.(type) {
	case ir.ScalarType:
		bt, ok := b.(ir.ScalarType)
		return ok && at.Kind == bt.Kind && at.Width == bt.Width
	case ir.VectorType:
		bt, ok := b.(ir.VectorType)
		return ok && at.Size == bt.Size && typeInnerEqual(at.Scalar, bt.Scalar)
	case ir.MatrixType:
		bt, ok := b.(ir.MatrixType)
		return ok && at.Columns == bt.Columns && at.Rows == bt.Rows && typeInnerEqual(at.Scalar, bt.Scalar)
	default:
		return false
	}
}
