package ir

import (
	"testing"
)

// --- ResolveExpressionType extended tests ---

func TestResolveExpressionType_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{Name: "test", Expressions: []Expression{}}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range handle")
	}
}

func TestResolveExpressionType_Override(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "gamma", Ty: 0},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprOverride{Override: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0, got %v", got)
	}
}

func TestResolveExpressionType_Override_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprOverride{Override: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range override")
	}
}

func TestResolveExpressionType_ZeroValue(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprZeroValue{Type: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestResolveExpressionType_Compose(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprCompose{Type: 0, Components: nil}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestResolveExpressionType_GlobalVariable_HandleSpace(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "sampler", Inner: SamplerType{Comparison: false}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "s", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Handle space: returns the type directly (not pointer)
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 for handle-space global")
	}
}

func TestResolveExpressionType_GlobalVariable_NonHandleSpace(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 0, Space: SpaceUniform},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-handle space: returns pointer type
	ptr, ok := got.Value.(PointerType)
	if !ok {
		t.Fatalf("expected PointerType, got %T", got.Value)
	}
	if ptr.Base != 0 || ptr.Space != SpaceUniform {
		t.Errorf("expected Pointer{base:0, space:Uniform}, got %v", ptr)
	}
}

func TestResolveExpressionType_GlobalVariable_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range global variable")
	}
}

func TestResolveExpressionType_LocalVariable(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprLocalVariable{Variable: 0}},
		},
		LocalVars: []LocalVariable{
			{Name: "x", Type: 0},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ptr, ok := got.Value.(PointerType)
	if !ok {
		t.Fatalf("expected PointerType, got %T", got.Value)
	}
	if ptr.Space != SpaceFunction {
		t.Errorf("expected Function space, got %d", ptr.Space)
	}
}

func TestResolveExpressionType_LocalVariable_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprLocalVariable{Variable: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range local variable")
	}
}

func TestResolveExpressionType_FunctionArgument_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range function argument")
	}
}

func TestResolveExpressionType_Load(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprLocalVariable{Variable: 0}},
			{Kind: ExprLoad{Pointer: 0}},
		},
		LocalVars: []LocalVariable{
			{Name: "x", Type: 0},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Load dereferences pointer: pointer<Function, f32> -> f32 (handle)
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 (dereferenced pointer)")
	}
}

func TestResolveExpressionType_ArrayLength(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprArrayLength{Array: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarUint || s.Width != 4 {
		t.Errorf("expected u32, got %v", got.Value)
	}
}

func TestResolveExpressionType_AtomicResult(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprAtomicResult{Ty: 0, Comparison: false}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestResolveExpressionType_SubgroupBallotResult(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprSubgroupBallotResult{}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != 4 || vec.Scalar.Kind != ScalarUint || vec.Scalar.Width != 4 {
		t.Errorf("expected vec4<u32>, got %v", vec)
	}
}

func TestResolveExpressionType_SubgroupOperationResult(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprSubgroupOperationResult{Type: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestResolveExpressionType_RayQueryProceedResult(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprRayQueryProceedResult{}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarBool || s.Width != 1 {
		t.Errorf("expected bool, got %v", got.Value)
	}
}

func TestResolveExpressionType_RayQueryGetIntersection(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "RayIntersection", Inner: StructType{Members: nil, Span: 32}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprRayQueryGetIntersection{Query: 0, Committed: true}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (RayIntersection), got %v", got.Handle)
	}
}

func TestResolveExpressionType_RayQueryGetIntersection_NotFound(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprRayQueryGetIntersection{Query: 0, Committed: true}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error when RayIntersection type not found")
	}
}

func TestResolveExpressionType_CallResult(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Functions: []Function{
			{Name: "helper", Result: &FunctionResult{Type: 0}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprCallResult{Function: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0")
	}
}

func TestResolveExpressionType_CallResult_NoReturn(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "void_func", Result: nil},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprCallResult{Function: 0}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for function with no return type")
	}
}

func TestResolveExpressionType_CallResult_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprCallResult{Function: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range function")
	}
}

func TestResolveExpressionType_Constant_OutOfRange(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprConstant{Constant: 99}},
		},
	}
	_, err := ResolveExpressionType(module, fn, 0)
	if err == nil {
		t.Error("expected error for out-of-range constant")
	}
}

func TestResolveExpressionType_Unary(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}},
			{Kind: ExprUnary{Op: UnaryNegate, Expr: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unary preserves type
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestResolveExpressionType_Select(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralBool(true)}},
			{Kind: Literal{Value: LiteralF32(1.0)}},
			{Kind: Literal{Value: LiteralF32(2.0)}},
			{Kind: ExprSelect{Condition: 0, Accept: 1, Reject: 2}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestResolveExpressionType_Derivative(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}},
			{Kind: ExprDerivative{Axis: DerivativeX, Control: DerivativeFine, Expr: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestResolveExpressionType_Relational_Vector(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprRelational{Fun: RelationalIsNan, Argument: 0}},
		},
		Arguments:       []FunctionArgument{{Name: "v", Type: 0}},
		ExpressionTypes: []TypeResolution{{}, {}},
	}

	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec3 || vec.Scalar.Kind != ScalarBool {
		t.Errorf("expected vec3<bool>, got %v", vec)
	}
}

func TestResolveExpressionType_Relational_AllAny_CollapseToScalar(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec3b", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarBool, Width: 1}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprRelational{Fun: RelationalAll, Argument: 0}},
		},
		Arguments:       []FunctionArgument{{Name: "v", Type: 0}},
		ExpressionTypes: []TypeResolution{{}, {}},
	}

	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarBool {
		t.Errorf("expected scalar bool, got %v", got.Value)
	}
}

func TestResolveExpressionType_Relational_Scalar(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}},
			{Kind: ExprRelational{Fun: RelationalIsNan, Argument: 0}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarBool {
		t.Errorf("expected bool, got %v", got.Value)
	}
}

func TestResolveExpressionType_As_Convert(t *testing.T) {
	module := &Module{}
	w := uint8(4)
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(42)}},
			{Kind: ExprAs{Expr: 0, Kind: ScalarFloat, Convert: &w}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32, got %v", got.Value)
	}
}

func TestResolveExpressionType_As_Bitcast(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralI32(42)}},
			{Kind: ExprAs{Expr: 0, Kind: ScalarFloat, Convert: nil}},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32 (bitcast preserves width), got %v", got.Value)
	}
}

func TestResolveExpressionType_As_VectorConvert(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec3i", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarSint, Width: 4}}},
		},
	}
	w := uint8(4)
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprAs{Expr: 0, Kind: ScalarFloat, Convert: &w}},
		},
		Arguments:       []FunctionArgument{{Name: "v", Type: 0}},
		ExpressionTypes: []TypeResolution{{}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec3 || vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected vec3<f32>, got %v", vec)
	}
}

func TestResolveExpressionType_Swizzle(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprSwizzle{Size: Vec2, Vector: 0, Pattern: [4]SwizzleComponent{SwizzleX, SwizzleY}}},
		},
		Arguments:       []FunctionArgument{{Name: "v", Type: 0}},
		ExpressionTypes: []TypeResolution{{}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec2 || vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected vec2<f32>, got %v", vec)
	}
}

func TestResolveExpressionType_Access_Array(t *testing.T) {
	size := uint32(4)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "arr", Inner: ArrayType{Base: 0, Size: ArraySize{Constant: &size}, Stride: 4}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: Literal{Value: LiteralU32(0)}},
			{Kind: ExprAccess{Base: 0, Index: 1}},
		},
		Arguments:       []FunctionArgument{{Name: "a", Type: 1}},
		ExpressionTypes: []TypeResolution{{}, {}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 0 {
		t.Errorf("expected handle 0 (array element type)")
	}
}

func TestResolveExpressionType_Access_Matrix(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "mat4x4f", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: Literal{Value: LiteralU32(0)}},
			{Kind: ExprAccess{Base: 0, Index: 1}},
		},
		Arguments:       []FunctionArgument{{Name: "m", Type: 0}},
		ExpressionTypes: []TypeResolution{{}, {}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Matrix access returns column vector
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec4 || vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected vec4<f32>, got %v", vec)
	}
}

func TestResolveExpressionType_AccessIndex_Struct(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
			{Name: "MyStruct", Inner: StructType{
				Members: []StructMember{
					{Name: "x", Type: 0, Offset: 0},
					{Name: "y", Type: 1, Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprAccessIndex{Base: 0, Index: 1}}, // access .y
		},
		Arguments:       []FunctionArgument{{Name: "s", Type: 2}},
		ExpressionTypes: []TypeResolution{{}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (u32), got %v", got.Handle)
	}
}

func TestResolveExpressionType_Binary_Multiply_MatVec(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "mat4x4f", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}}, // mat
			{Kind: ExprFunctionArgument{Index: 1}}, // vec
			{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}},
		},
		Arguments: []FunctionArgument{
			{Name: "m", Type: 0},
			{Name: "v", Type: 1},
		},
		ExpressionTypes: []TypeResolution{{}, {}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mat4x4 * vec4 -> vec4 (rows of matrix)
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec4 {
		t.Errorf("expected vec4, got vec%d", vec.Size)
	}
}

func TestResolveExpressionType_Binary_Multiply_MatMat(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "mat4x4f", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			{Name: "mat2x4f", Inner: MatrixType{Columns: Vec2, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}}, // mat4x4
			{Kind: ExprFunctionArgument{Index: 1}}, // mat2x4
			{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}},
		},
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
		ExpressionTypes: []TypeResolution{{}, {}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mat4x4 * mat2x4 -> mat2x4 (right cols, left rows)
	mat, ok := got.Value.(MatrixType)
	if !ok {
		t.Fatalf("expected MatrixType, got %T", got.Value)
	}
	if mat.Columns != Vec2 || mat.Rows != Vec4 {
		t.Errorf("expected mat2x4, got mat%dx%d", mat.Columns, mat.Rows)
	}
}

func TestResolveExpressionType_Binary_VectorComparison(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprFunctionArgument{Index: 1}},
			{Kind: ExprBinary{Op: BinaryEqual, Left: 0, Right: 1}},
		},
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 0},
		},
		ExpressionTypes: []TypeResolution{{}, {}, {}},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Vector comparison returns vec<bool>
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec3 || vec.Scalar.Kind != ScalarBool {
		t.Errorf("expected vec3<bool>, got %v", vec)
	}
}

func TestResolveExpressionType_Binary_ScalarVectorBroadcast(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(2.0)}},              // 0: scalar
			{Kind: ExprSplat{Size: Vec3, Value: 0}},              // 1: vec3 (from splat)
			{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}}, // 2: scalar + vec = vec
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Value: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType (broadcast), got %T", got.Value)
	}
	if vec.Size != Vec3 {
		t.Errorf("expected vec3, got vec%d", vec.Size)
	}
}

func TestResolveExpressionType_Math_Dot4Packed(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralU32(0)}},
			{Kind: ExprMath{Fun: MathDot4I8Packed, Arg: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarUint, Width: 4}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarSint || s.Width != 4 {
		t.Errorf("expected i32, got %v", got.Value)
	}
}

func TestResolveExpressionType_Math_PackUnpack(t *testing.T) {
	module := &Module{}

	packFuns := []MathFunction{MathPack4xI8, MathPack4xU8, MathPack4xI8Clamp, MathPack4xU8Clamp}
	for _, fun := range packFuns {
		fn := &Function{
			Expressions: []Expression{
				{Kind: Literal{Value: LiteralU32(0)}},
				{Kind: ExprMath{Fun: fun, Arg: 0}},
			},
			ExpressionTypes: []TypeResolution{
				{Value: ScalarType{Kind: ScalarUint, Width: 4}},
				{},
			},
		}
		got, err := ResolveExpressionType(module, fn, 1)
		if err != nil {
			t.Fatalf("pack function %d: unexpected error: %v", fun, err)
		}
		s, ok := got.Value.(ScalarType)
		if !ok || s.Kind != ScalarUint {
			t.Errorf("pack function %d: expected u32, got %v", fun, got.Value)
		}
	}

	// unpack4xI8 -> vec4<i32>
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralU32(0)}},
			{Kind: ExprMath{Fun: MathUnpack4xI8, Arg: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarUint, Width: 4}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok || vec.Size != Vec4 || vec.Scalar.Kind != ScalarSint {
		t.Errorf("expected vec4<i32>, got %v", got.Value)
	}
}

func TestResolveExpressionType_ImageLoad_StorageTexture(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "storage_tex", Inner: ImageType{
				Dim:           Dim2D,
				Class:         ImageClassStorage,
				StorageFormat: StorageFormatRgba8Uint,
			}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageLoad{Image: 0, Coordinate: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Scalar.Kind != ScalarUint {
		t.Errorf("expected uint storage texture load result, got kind %d", vec.Scalar.Kind)
	}
}

func TestResolveExpressionType_ImageSample_DepthTexture(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "depth_tex", Inner: ImageType{
				Dim:   Dim2D,
				Class: ImageClassDepth,
			}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      SampleLevelZero{},
			}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Depth texture sample returns scalar f32
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarFloat || s.Width != 4 {
		t.Errorf("expected f32 (depth sample), got %v", got.Value)
	}
}

func TestResolveExpressionType_ImageSample_Gather(t *testing.T) {
	gather := SwizzleX
	module := &Module{
		Types: []Type{
			{Name: "tex", Inner: ImageType{
				Dim:         Dim2D,
				Class:       ImageClassSampled,
				SampledKind: ScalarFloat,
			}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageSample{
				Image:      0,
				Sampler:    0,
				Coordinate: 0,
				Level:      SampleLevelZero{},
				Gather:     &gather,
			}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Gather always returns vec4
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType (gather), got %T", got.Value)
	}
	if vec.Size != Vec4 || vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expected vec4<f32> (gather), got %v", vec)
	}
}

func TestResolveExpressionType_ImageQuery_Size(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "tex3d", Inner: ImageType{Dim: Dim3D, Class: ImageClassSampled}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageQuery{Image: 0, Query: ImageQuerySize{}}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3D image: size returns vec3<u32>
	vec, ok := got.Value.(VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", got.Value)
	}
	if vec.Size != Vec3 || vec.Scalar.Kind != ScalarUint {
		t.Errorf("expected vec3<u32>, got %v", vec)
	}
}

func TestResolveExpressionType_ImageQuery_Size_1D(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "tex1d", Inner: ImageType{Dim: Dim1D, Class: ImageClassSampled}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageQuery{Image: 0, Query: ImageQuerySize{}}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1D: returns scalar u32
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarUint {
		t.Errorf("expected scalar u32, got %v", got.Value)
	}
}

func TestResolveExpressionType_ImageQuery_NumLevels(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "tex", Inner: ImageType{Dim: Dim2D, Class: ImageClassSampled}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "tex", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: ExprImageQuery{Image: 0, Query: ImageQueryNumLevels{}}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarUint || s.Width != 4 {
		t.Errorf("expected u32, got %v", got.Value)
	}
}

// --- resolveInner / findNamedType tests ---

func TestFindNamedType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "MyStruct", Inner: StructType{}},
		},
	}

	if idx := findNamedType(module, "MyStruct"); idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if idx := findNamedType(module, "NotFound"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestResolveInner(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}

	t.Run("with handle", func(t *testing.T) {
		h := TypeHandle(0)
		inner := resolveInner(module, TypeResolution{Handle: &h})
		if _, ok := inner.(ScalarType); !ok {
			t.Errorf("expected ScalarType, got %T", inner)
		}
	})

	t.Run("with value", func(t *testing.T) {
		inner := resolveInner(module, TypeResolution{Value: VectorType{Size: Vec3}})
		if _, ok := inner.(VectorType); !ok {
			t.Errorf("expected VectorType, got %T", inner)
		}
	})

	t.Run("with out-of-range handle uses value", func(t *testing.T) {
		h := TypeHandle(99)
		inner := resolveInner(module, TypeResolution{Handle: &h, Value: ScalarType{Kind: ScalarBool}})
		if s, ok := inner.(ScalarType); !ok || s.Kind != ScalarBool {
			t.Errorf("expected ScalarBool, got %v", inner)
		}
	})
}

// --- ResolveLiteralType (exported wrapper) ---

func TestResolveLiteralType_Exported(t *testing.T) {
	tests := []struct {
		name      string
		literal   Literal
		wantKind  ScalarKind
		wantWidth uint8
	}{
		{"f64", Literal{Value: LiteralF64(1.0)}, ScalarFloat, 8},
		{"f16", Literal{Value: LiteralF16(1.0)}, ScalarFloat, 2},
		{"u64", Literal{Value: LiteralU64(1)}, ScalarUint, 8},
		{"i64", Literal{Value: LiteralI64(1)}, ScalarSint, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveLiteralType(tt.literal)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s, ok := got.Value.(ScalarType)
			if !ok || s.Kind != tt.wantKind || s.Width != tt.wantWidth {
				t.Errorf("expected %d/%d, got %v", tt.wantKind, tt.wantWidth, got.Value)
			}
		})
	}
}

// --- findOrInferScalarHandle / findOrInferVectorHandle tests ---

func TestFindOrInferScalarHandle(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
	}

	h := findOrInferScalarHandle(module, ScalarType{Kind: ScalarUint, Width: 4})
	if h != 1 {
		t.Errorf("expected handle 1, got %d", h)
	}

	// Not found returns 0
	h = findOrInferScalarHandle(module, ScalarType{Kind: ScalarBool, Width: 1})
	if h != 0 {
		t.Errorf("expected handle 0 (not found fallback), got %d", h)
	}
}

// --- modfResultStructName / frexpResultStructName tests ---

func TestModfResultStructName(t *testing.T) {
	module := &Module{}

	tests := []struct {
		name    string
		argType TypeResolution
		want    string
	}{
		{
			"scalar f32",
			TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			"__modf_result_f32",
		},
		{
			"vec2 f32",
			TypeResolution{Value: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			"__modf_result_vec2_f32",
		},
		{
			"vec3 f64",
			TypeResolution{Value: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}},
			"__modf_result_vec3_f64",
		},
		{
			"vec4 f16",
			TypeResolution{Value: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}},
			"__modf_result_vec4_f16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modfResultStructName(module, tt.argType)
			if got != tt.want {
				t.Errorf("modfResultStructName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFrexpResultStructName(t *testing.T) {
	module := &Module{}

	tests := []struct {
		name    string
		argType TypeResolution
		want    string
	}{
		{
			"scalar f32",
			TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			"__frexp_result_f32",
		},
		{
			"vec2 f32",
			TypeResolution{Value: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			"__frexp_result_vec2_f32",
		},
		{
			"vec4 f64",
			TypeResolution{Value: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}},
			"__frexp_result_vec4_f64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := frexpResultStructName(module, tt.argType)
			if got != tt.want {
				t.Errorf("frexpResultStructName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- ResolveAtomicPointerScalar tests ---

func TestResolveAtomicPointerScalar(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "atomic<u32>", Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}},
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "counter", Type: 0, Space: SpaceHandle},
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
		},
	}

	// Atomic type pointer
	scalar := ResolveAtomicPointerScalar(module, fn, 0)
	if scalar == nil {
		t.Fatal("expected non-nil scalar")
	}
	if scalar.Kind != ScalarUint || scalar.Width != 4 {
		t.Errorf("expected u32, got %v", scalar)
	}
}

func TestResolveAtomicPointerScalar_ScalarType(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "v", Type: 0, Space: SpaceHandle},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
		},
	}
	scalar := ResolveAtomicPointerScalar(module, fn, 0)
	if scalar == nil {
		t.Fatal("expected non-nil scalar")
	}
	if scalar.Kind != ScalarUint {
		t.Errorf("expected uint, got %v", scalar.Kind)
	}
}

func TestResolveAtomicPointerScalar_Error(t *testing.T) {
	module := &Module{}
	fn := &Function{Expressions: []Expression{}}
	scalar := ResolveAtomicPointerScalar(module, fn, 99) // out of range
	if scalar != nil {
		t.Error("expected nil for out-of-range")
	}
}

func TestResolveExpressionType_MathModf(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "__modf_result_f32", Inner: StructType{Members: nil, Span: 8}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.5)}},
			{Kind: ExprMath{Fun: MathModf, Arg: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (__modf_result_f32), got %v", got.Handle)
	}
}

func TestResolveExpressionType_MathFrexp(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "__frexp_result_f32", Inner: StructType{Members: nil, Span: 8}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.5)}},
			{Kind: ExprMath{Fun: MathFrexp, Arg: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Handle == nil || *got.Handle != 1 {
		t.Errorf("expected handle 1 (__frexp_result_f32), got %v", got.Handle)
	}
}

func TestResolveExpressionType_MathCountBits(t *testing.T) {
	module := &Module{}
	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralU32(0xFF)}},
			{Kind: ExprMath{Fun: MathCountOneBits, Arg: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Value: ScalarType{Kind: ScalarUint, Width: 4}},
			{},
		},
	}
	got, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := got.Value.(ScalarType)
	if !ok || s.Kind != ScalarUint {
		t.Errorf("expected u32 (count bits preserves type), got %v", got.Value)
	}
}

func TestFindOrInferVectorHandle(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}

	h := findOrInferVectorHandle(module, VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}})
	if h != 1 {
		t.Errorf("expected handle 1, got %d", h)
	}

	// Not found
	h = findOrInferVectorHandle(module, VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}})
	if h != 0 {
		t.Errorf("expected handle 0 (not found), got %d", h)
	}
}

// --- ValuePointerType tests ---
// These test that AccessIndex/Access through Pointer<Matrix/Vector> produces
// ValuePointerType, and that Load on ValuePointerType dereferences correctly.

func TestResolve_AccessIndex_PointerToMatrix_ProducesValuePointer(t *testing.T) {
	// Pointer<Matrix3x2<f32>>[0] → ValuePointerType{Size: &Vec2, Scalar: f32, Space: Storage}
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                                   // 0
			{Name: "mat3x2", Inner: MatrixType{Columns: Vec3, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 1
		},
		GlobalVariables: []GlobalVariable{
			{Name: "m", Type: 1, Space: SpaceStorage}, // 0: var<storage> m: mat3x2<f32>
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &m (Pointer<mat3x2, Storage>)
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] (column 0)
		},
		ExpressionTypes: make([]TypeResolution, 2),
	}
	// Resolve expr 0
	res0, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve expr 0: %v", err)
	}
	fn.ExpressionTypes[0] = res0
	// Should be PointerType{Base: mat3x2, Space: Storage}
	ptr, ok := res0.Value.(PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", res0.Value, res0)
	}
	if ptr.Space != SpaceStorage {
		t.Errorf("expr 0: expected Storage space, got %v", ptr.Space)
	}

	// Resolve expr 1: AccessIndex on Pointer<Matrix>
	res1, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve expr 1: %v", err)
	}
	fn.ExpressionTypes[1] = res1

	vp, ok := res1.Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 1: expected ValuePointerType, got %T (%+v)", res1.Value, res1)
	}
	if vp.Size == nil || *vp.Size != Vec2 {
		t.Errorf("expr 1: expected Size=Vec2 (column vector), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ScalarFloat || vp.Scalar.Width != 4 {
		t.Errorf("expr 1: expected f32 scalar, got %+v", vp.Scalar)
	}
	if vp.Space != SpaceStorage {
		t.Errorf("expr 1: expected Storage space, got %v", vp.Space)
	}
}

func TestResolve_AccessIndex_PointerToVector_ProducesValuePointer(t *testing.T) {
	// Pointer<Vec4<i32>>[2] → ValuePointerType{Size: nil, Scalar: i32, Space: Function}
	module := &Module{
		Types: []Type{
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},                                   // 0
			{Name: "vec4i", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarSint, Width: 4}}}, // 1
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprLocalVariable{Variable: 0}},     // 0: &vec0 (Pointer<vec4i, Function>)
			{Kind: ExprAccessIndex{Base: 0, Index: 2}}, // 1: vec0[2]
		},
		ExpressionTypes: make([]TypeResolution, 2),
		LocalVars: []LocalVariable{
			{Name: "vec0", Type: 1}, // vec4<i32>
		},
	}
	// Resolve expr 0: LocalVariable → Pointer<vec4i, Function>
	res0, err := ResolveExpressionType(module, fn, 0)
	if err != nil {
		t.Fatalf("resolve expr 0: %v", err)
	}
	fn.ExpressionTypes[0] = res0

	// Resolve expr 1: AccessIndex on Pointer<Vector>
	res1, err := ResolveExpressionType(module, fn, 1)
	if err != nil {
		t.Fatalf("resolve expr 1: %v", err)
	}

	vp, ok := res1.Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 1: expected ValuePointerType, got %T (%+v)", res1.Value, res1)
	}
	if vp.Size != nil {
		t.Errorf("expr 1: expected Size=nil (scalar pointer), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ScalarSint || vp.Scalar.Width != 4 {
		t.Errorf("expr 1: expected i32 scalar, got %+v", vp.Scalar)
	}
	if vp.Space != SpaceFunction {
		t.Errorf("expr 1: expected Function space, got %v", vp.Space)
	}
}

func TestResolve_AccessIndex_ValuePointerVector_ProducesValuePointerScalar(t *testing.T) {
	// ValuePointerType{Size: &Vec3}[1] → ValuePointerType{Size: nil}
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "mat3x3", Inner: MatrixType{Columns: Vec3, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "m", Type: 1, Space: SpaceUniform},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &m
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] → ValuePointer(vec3)
			{Kind: ExprAccessIndex{Base: 1, Index: 1}}, // 2: m[0][1] → ValuePointer(scalar)
		},
		ExpressionTypes: make([]TypeResolution, 3),
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 2 should be ValuePointerType{Size: nil} — pointer to scalar element
	vp, ok := fn.ExpressionTypes[2].Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if vp.Size != nil {
		t.Errorf("expr 2: expected Size=nil (scalar), got %v", vp.Size)
	}
	if vp.Scalar.Kind != ScalarFloat {
		t.Errorf("expr 2: expected float scalar, got %+v", vp.Scalar)
	}
}

func TestResolve_Load_ValuePointerScalar_ProducesScalar(t *testing.T) {
	// Load(ValuePointerType{Size: nil, Scalar: i32}) → ScalarType{Sint, 4}
	module := &Module{
		Types: []Type{
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},
			{Name: "vec4i", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarSint, Width: 4}}},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprLocalVariable{Variable: 0}},     // 0: &vec0
			{Kind: ExprAccessIndex{Base: 0, Index: 1}}, // 1: vec0[1] → ValuePointer(scalar)
			{Kind: ExprLoad{Pointer: 1}},               // 2: load → i32
		},
		ExpressionTypes: make([]TypeResolution, 3),
		LocalVars: []LocalVariable{
			{Name: "vec0", Type: 1},
		},
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 2 (Load) should resolve to ScalarType{Sint, 4}
	scalar, ok := fn.ExpressionTypes[2].Value.(ScalarType)
	if !ok {
		t.Fatalf("expr 2: expected ScalarType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if scalar.Kind != ScalarSint || scalar.Width != 4 {
		t.Errorf("expr 2: expected Sint(4), got %+v", scalar)
	}
}

func TestResolve_Load_ValuePointerVector_ProducesVector(t *testing.T) {
	// Load(ValuePointerType{Size: &Vec2, Scalar: f32}) → VectorType{Vec2, f32}
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "mat3x2", Inner: MatrixType{Columns: Vec3, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "m", Type: 1, Space: SpaceStorage},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &m
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: m[0] → ValuePointer(vec2)
			{Kind: ExprLoad{Pointer: 1}},               // 2: load → vec2<f32>
		},
		ExpressionTypes: make([]TypeResolution, 3),
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 2 (Load) should resolve to VectorType{Vec2, f32}
	vec, ok := fn.ExpressionTypes[2].Value.(VectorType)
	if !ok {
		t.Fatalf("expr 2: expected VectorType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if vec.Size != Vec2 {
		t.Errorf("expr 2: expected Vec2, got %v", vec.Size)
	}
	if vec.Scalar.Kind != ScalarFloat {
		t.Errorf("expr 2: expected float scalar, got %+v", vec.Scalar)
	}
}

func TestResolve_Access_PointerToMatrix_ProducesValuePointer(t *testing.T) {
	// Dynamic access: Pointer<Matrix>[idx] → ValuePointerType{Size: &rows}
	module := &Module{
		Types: []Type{
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},
			{Name: "mat4x3", Inner: MatrixType{Columns: Vec4, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "m", Type: 1, Space: SpaceUniform},
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}}, // 0: &m
			{Kind: ExprFunctionArgument{Index: 0}},  // 1: idx (i32)
			{Kind: ExprAccess{Base: 0, Index: 1}},   // 2: m[idx] → ValuePointer(vec3)
		},
		ExpressionTypes: make([]TypeResolution, 3),
		Arguments: []FunctionArgument{
			{Name: "idx", Type: 0},
		},
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	vp, ok := fn.ExpressionTypes[2].Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if vp.Size == nil || *vp.Size != Vec3 {
		t.Errorf("expr 2: expected Size=Vec3 (column rows), got %v", vp.Size)
	}
}

// --- Struct member pointer resolution tests ---

func TestResolve_AccessIndex_PointerToStruct_ProducesPointerToMember(t *testing.T) {
	// AccessIndex on Pointer<Struct>.member → PointerType{Base: memberType, Space}
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                                   // 0
			{Name: "mat4x3", Inner: MatrixType{Columns: Vec4, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 1
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},                 // 2
			{Name: "S", Inner: StructType{Members: []StructMember{{Name: "m", Type: 1}, {Name: "v", Type: 2}}, Span: 64}},   // 3
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 3, Space: SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &g (Pointer<S, Uniform>)
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.m (member 0 = mat4x3)
			{Kind: ExprAccessIndex{Base: 0, Index: 1}}, // 2: g.v (member 1 = vec3f)
		},
		ExpressionTypes: make([]TypeResolution, 3),
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 0: Pointer<S, Uniform>
	ptr0, ok := fn.ExpressionTypes[0].Value.(PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", fn.ExpressionTypes[0].Value, fn.ExpressionTypes[0])
	}
	if ptr0.Base != 3 || ptr0.Space != SpaceUniform {
		t.Errorf("expr 0: expected Pointer{Base:3, Uniform}, got %+v", ptr0)
	}

	// expr 1: Pointer<mat4x3, Uniform> (struct member through pointer)
	ptr1, ok := fn.ExpressionTypes[1].Value.(PointerType)
	if !ok {
		t.Fatalf("expr 1: expected PointerType, got %T (%+v)", fn.ExpressionTypes[1].Value, fn.ExpressionTypes[1])
	}
	if ptr1.Base != 1 {
		t.Errorf("expr 1: expected Base=1 (mat4x3), got %d", ptr1.Base)
	}
	if ptr1.Space != SpaceUniform {
		t.Errorf("expr 1: expected Uniform space, got %v", ptr1.Space)
	}

	// expr 2: Pointer<vec3f, Uniform> (struct member through pointer)
	ptr2, ok := fn.ExpressionTypes[2].Value.(PointerType)
	if !ok {
		t.Fatalf("expr 2: expected PointerType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if ptr2.Base != 2 {
		t.Errorf("expr 2: expected Base=2 (vec3f), got %d", ptr2.Base)
	}
	if ptr2.Space != SpaceUniform {
		t.Errorf("expr 2: expected Uniform space, got %v", ptr2.Space)
	}
}

func TestResolve_Load_PointerToMember_ProducesValue(t *testing.T) {
	// Load(Pointer<Vec3<f32>>) → TypeHandle(Vec3<f32>)
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                   // 0
			{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 1
			{Name: "S", Inner: StructType{Members: []StructMember{{Name: "v", Type: 1}}, Span: 16}},         // 2
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 2, Space: SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &g (Pointer<S, Uniform>)
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.v (Pointer<vec3f, Uniform>)
			{Kind: ExprLoad{Pointer: 1}},               // 2: load → vec3<f32>
		},
		ExpressionTypes: make([]TypeResolution, 3),
	}
	for i := 0; i < 3; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 2 (Load) should produce a Handle to vec3f (type 1)
	// Load through a PointerType resolves as Handle-based (referencing module type).
	expectedHandle := TypeHandle(1)
	if fn.ExpressionTypes[2].Handle == nil || *fn.ExpressionTypes[2].Handle != expectedHandle {
		t.Fatalf("expr 2: expected Handle=1 (vec3f), got %v", fn.ExpressionTypes[2].Handle)
	}
	// Verify the referenced module type is vec3<f32>
	vec, ok := module.Types[*fn.ExpressionTypes[2].Handle].Inner.(VectorType)
	if !ok {
		t.Fatalf("expr 2: expected module type 1 to be VectorType, got %T", module.Types[*fn.ExpressionTypes[2].Handle].Inner)
	}
	if vec.Size != Vec3 {
		t.Errorf("expr 2: expected Vec3, got %v", vec.Size)
	}
	if vec.Scalar.Kind != ScalarFloat || vec.Scalar.Width != 4 {
		t.Errorf("expr 2: expected f32 scalar, got %+v", vec.Scalar)
	}
}

func TestResolve_FullChain_PointerStructMatrixColumnElement(t *testing.T) {
	// Full chain: GlobalVar(Pointer<Struct>) → AccessIndex(member=mat) → AccessIndex(column)
	//   → AccessIndex(element) → Load
	// Type 0: f32
	// Type 1: mat4x3<f32> (4 columns of vec3)
	// Type 2: struct S { m: mat4x3<f32> }
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                                   // 0
			{Name: "mat4x3", Inner: MatrixType{Columns: Vec4, Rows: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 1
			{Name: "S", Inner: StructType{Members: []StructMember{{Name: "m", Type: 1}}, Span: 64}},                         // 2
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 2, Space: SpaceUniform}, // 0: var<uniform> g: S
		},
	}
	fn := &Function{
		Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},    // 0: &g
			{Kind: ExprAccessIndex{Base: 0, Index: 0}}, // 1: g.m (struct member → matrix)
			{Kind: ExprAccessIndex{Base: 1, Index: 2}}, // 2: g.m[2] (matrix column)
			{Kind: ExprAccessIndex{Base: 2, Index: 1}}, // 3: g.m[2][1] (vector element)
			{Kind: ExprLoad{Pointer: 3}},               // 4: load
		},
		ExpressionTypes: make([]TypeResolution, 5),
	}

	// Resolve all expressions sequentially
	for i := 0; i < 5; i++ {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err != nil {
			t.Fatalf("resolve expr %d: %v", i, err)
		}
		fn.ExpressionTypes[i] = res
	}

	// expr 0: PointerType{Base: Struct(2), Space: Uniform}
	ptr0, ok := fn.ExpressionTypes[0].Value.(PointerType)
	if !ok {
		t.Fatalf("expr 0: expected PointerType, got %T (%+v)", fn.ExpressionTypes[0].Value, fn.ExpressionTypes[0])
	}
	if ptr0.Base != 2 {
		t.Errorf("expr 0: expected Base=2 (struct S), got %d", ptr0.Base)
	}
	if ptr0.Space != SpaceUniform {
		t.Errorf("expr 0: expected Uniform space, got %v", ptr0.Space)
	}

	// expr 1: PointerType{Base: MatrixType(1), Space: Uniform} — struct member through pointer
	ptr1, ok := fn.ExpressionTypes[1].Value.(PointerType)
	if !ok {
		t.Fatalf("expr 1: expected PointerType, got %T (%+v)", fn.ExpressionTypes[1].Value, fn.ExpressionTypes[1])
	}
	if ptr1.Base != 1 {
		t.Errorf("expr 1: expected Base=1 (mat4x3), got %d", ptr1.Base)
	}
	if ptr1.Space != SpaceUniform {
		t.Errorf("expr 1: expected Uniform space, got %v", ptr1.Space)
	}

	// expr 2: ValuePointerType{Size: &Vec3, Scalar: f32, Space: Uniform} — matrix column
	vp2, ok := fn.ExpressionTypes[2].Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 2: expected ValuePointerType, got %T (%+v)", fn.ExpressionTypes[2].Value, fn.ExpressionTypes[2])
	}
	if vp2.Size == nil || *vp2.Size != Vec3 {
		t.Errorf("expr 2: expected Size=Vec3 (column vector), got %v", vp2.Size)
	}
	if vp2.Scalar.Kind != ScalarFloat || vp2.Scalar.Width != 4 {
		t.Errorf("expr 2: expected f32 scalar, got %+v", vp2.Scalar)
	}
	if vp2.Space != SpaceUniform {
		t.Errorf("expr 2: expected Uniform space, got %v", vp2.Space)
	}

	// expr 3: ValuePointerType{Size: nil, Scalar: f32, Space: Uniform} — vector element
	vp3, ok := fn.ExpressionTypes[3].Value.(ValuePointerType)
	if !ok {
		t.Fatalf("expr 3: expected ValuePointerType, got %T (%+v)", fn.ExpressionTypes[3].Value, fn.ExpressionTypes[3])
	}
	if vp3.Size != nil {
		t.Errorf("expr 3: expected Size=nil (scalar pointer), got %v", vp3.Size)
	}
	if vp3.Scalar.Kind != ScalarFloat || vp3.Scalar.Width != 4 {
		t.Errorf("expr 3: expected f32 scalar, got %+v", vp3.Scalar)
	}
	if vp3.Space != SpaceUniform {
		t.Errorf("expr 3: expected Uniform space, got %v", vp3.Space)
	}

	// expr 4: ScalarType{Float, 4} — loaded value
	scalar, ok := fn.ExpressionTypes[4].Value.(ScalarType)
	if !ok {
		t.Fatalf("expr 4: expected ScalarType, got %T (%+v)", fn.ExpressionTypes[4].Value, fn.ExpressionTypes[4])
	}
	if scalar.Kind != ScalarFloat || scalar.Width != 4 {
		t.Errorf("expr 4: expected f32, got %+v", scalar)
	}
}
