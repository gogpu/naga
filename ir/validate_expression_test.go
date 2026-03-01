package ir

import (
	"strings"
	"testing"
)

// =============================================================================
// Helpers for new validator tests
// =============================================================================

// newValidModule returns a minimal valid module with basic types and one function.
func newValidModule() *Module {
	return &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},
			{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 1}},
			{Name: "mat4f", Inner: MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		Functions: []Function{
			{
				Name: "test_fn",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}},
					{Kind: Literal{Value: LiteralF32(2.0)}},
				},
				Body: nil,
			},
		},
	}
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

// expectValidationErrors validates a module and expects specific error messages.
func expectValidationErrors(t *testing.T, module *Module, expectedSubstrings ...string) {
	t.Helper()
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	for _, expected := range expectedSubstrings {
		found := false
		for _, ve := range errors {
			if strings.Contains(ve.Error(), expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected validation error containing %q, but not found.\nErrors: %v", expected, errors)
		}
	}
}

// expectNoValidationErrors validates a module and expects no errors.
func expectNoValidationErrors(t *testing.T, module *Module) {
	t.Helper()
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  - %s", e.Error())
		}
	}
}

// =============================================================================
// Test: Expression validation - covering gaps not in validate_semantic_test.go
// =============================================================================

func TestValidateNew_FunctionArgOutOfRange(t *testing.T) {
	m := newValidModule()
	// Function has no arguments, but expression references arg index 5
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprFunctionArgument{Index: 5}})
	expectValidationErrors(t, m, "argument index 5 out of range")
}

func TestValidateNew_LocalVariableOutOfRange(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprLocalVariable{Variable: 999}})
	expectValidationErrors(t, m, "local variable index 999 out of range")
}

func TestValidateNew_GlobalVariableInvalid(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprGlobalVariable{Variable: 999}})
	expectValidationErrors(t, m, "global variable 999 does not exist")
}

func TestValidateNew_MathInvalidArg2(t *testing.T) {
	m := newValidModule()
	arg1 := ExpressionHandle(1)
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprMath{Fun: MathClamp, Arg: 0, Arg1: &arg1, Arg2: &invalid}})
	expectValidationErrors(t, m, "arg2 expression 999 does not exist")
}

func TestValidateNew_MathInvalidArg3(t *testing.T) {
	m := newValidModule()
	arg1 := ExpressionHandle(1)
	arg2 := ExpressionHandle(0)
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprMath{Fun: MathClamp, Arg: 0, Arg1: &arg1, Arg2: &arg2, Arg3: &invalid}})
	expectValidationErrors(t, m, "arg3 expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, ArrayIndex: &invalid}})
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidOffset(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, Offset: &invalid}})
	expectValidationErrors(t, m, "offset expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidDepthRef(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, DepthRef: &invalid}})
	expectValidationErrors(t, m, "depth ref expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageLoad{Image: 0, Coordinate: 0, ArrayIndex: &invalid}})
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidSample(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageLoad{Image: 0, Coordinate: 0, Sample: &invalid}})
	expectValidationErrors(t, m, "sample expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidLevel(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		Expression{Kind: ExprImageLoad{Image: 0, Coordinate: 0, Level: &invalid}})
	expectValidationErrors(t, m, "level expression 999 does not exist")
}

// =============================================================================
// Test: Statement validation - covering gaps
// =============================================================================

func TestValidateNew_EmitRangeEndOutOfBounds(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtEmit{Range: Range{Start: 0, End: 100}}},
	}
	expectValidationErrors(t, m, "emit range end 100 out of range")
}

func TestValidateNew_EmitRangeStartGEEnd(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtEmit{Range: Range{Start: 1, End: 1}}},
	}
	expectValidationErrors(t, m, "emit range start 1 >= end 1")
}

func TestValidateNew_ImageStoreInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Body = []Statement{
		{Kind: StmtImageStore{Image: 0, Coordinate: 0, Value: 0, ArrayIndex: &invalid}},
	}
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_AtomicInvalidResult(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Body = []Statement{
		{Kind: StmtAtomic{Pointer: 0, Value: 0, Result: &invalid}},
	}
	expectValidationErrors(t, m, "result expression 999 does not exist")
}

func TestValidateNew_CallInvalidResult(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].Body = []Statement{
		{Kind: StmtCall{Function: 0, Result: &invalid}},
	}
	expectValidationErrors(t, m, "result expression 999 does not exist")
}

func TestValidateNew_CallInvalidArguments(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtCall{Function: 0, Arguments: []ExpressionHandle{999}}},
	}
	expectValidationErrors(t, m, "argument 0 expression 999 does not exist")
}

// =============================================================================
// Test: Type validation - covering gaps
// =============================================================================

func TestValidateNew_ValidScalarWidthByte(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "u8", Inner: ScalarType{Kind: ScalarUint, Width: 1}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_ValidScalarWidthHalf(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "f16", Inner: ScalarType{Kind: ScalarFloat, Width: 2}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_ValidScalarWidthDouble(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_InvalidVectorScalarWidth(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "bad", Inner: VectorType{
		Size:   Vec2,
		Scalar: ScalarType{Kind: ScalarFloat, Width: 3},
	}})
	expectValidationErrors(t, m, "vector scalar width must be 1, 2, 4, or 8")
}

func TestValidateNew_InvalidMatrixColumns(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "bad", Inner: MatrixType{
		Columns: 5, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
	}})
	expectValidationErrors(t, m, "matrix columns must be 2, 3, or 4")
}

func TestValidateNew_InvalidMatrixRows(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "bad", Inner: MatrixType{
		Columns: Vec4, Rows: 1, Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
	}})
	expectValidationErrors(t, m, "matrix rows must be 2, 3, or 4")
}

func TestValidateNew_StructEmptyMemberName(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "bad", Inner: StructType{
		Members: []StructMember{{Name: "", Type: 0}},
	}})
	expectValidationErrors(t, m, "struct member 0 has empty name")
}

func TestValidateNew_StructDuplicateMemberName(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, Type{Name: "bad", Inner: StructType{
		Members: []StructMember{
			{Name: "x", Type: 0},
			{Name: "x", Type: 0},
		},
	}})
	expectValidationErrors(t, m, "duplicate struct member name")
}

func TestValidateNew_StructMemberCircularRef(t *testing.T) {
	m := newValidModule()
	// Add struct at index 5 with member referencing itself
	m.Types = append(m.Types, Type{Name: "bad", Inner: StructType{
		Members: []StructMember{{Name: "self", Type: TypeHandle(5)}},
	}})
	expectValidationErrors(t, m, "struct member \"self\" has circular reference")
}

// =============================================================================
// Test: Entry point validation - covering gaps
// =============================================================================

func TestValidateNew_EntryPointEmptyName(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []EntryPoint{
		{Name: "", Stage: StageVertex, Function: 0},
	}
	expectValidationErrors(t, m, "entry point 0 has empty name")
}

func TestValidateNew_EntryPointDuplicateName(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []EntryPoint{
		{Name: "main", Stage: StageVertex, Function: 0},
		{Name: "main", Stage: StageFragment, Function: 0},
	}
	expectValidationErrors(t, m, "duplicate entry point name")
}

func TestValidateNew_EntryPointInvalidFunction(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []EntryPoint{
		{Name: "main", Stage: StageVertex, Function: 999},
	}
	expectValidationErrors(t, m, "function 999 does not exist")
}

func TestValidateNew_VertexDirectPositionBinding(t *testing.T) {
	m := newValidModule()
	var posBinding Binding = BuiltinBinding{Builtin: BuiltinPosition}
	m.Functions[0].Result = &FunctionResult{
		Type:    1, // vec4f
		Binding: &posBinding,
	}
	m.EntryPoints = []EntryPoint{
		{Name: "vs", Stage: StageVertex, Function: 0},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_VertexStructPositionBinding(t *testing.T) {
	m := newValidModule()
	var posBinding Binding = BuiltinBinding{Builtin: BuiltinPosition}
	m.Types = append(m.Types, Type{
		Name: "VsOut",
		Inner: StructType{
			Members: []StructMember{
				{Name: "pos", Type: 1, Binding: &posBinding},
			},
		},
	})
	m.Functions[0].Result = &FunctionResult{
		Type: TypeHandle(len(m.Types) - 1),
	}
	m.EntryPoints = []EntryPoint{
		{Name: "vs", Stage: StageVertex, Function: 0},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_FragmentValid(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []EntryPoint{
		{Name: "fs", Stage: StageFragment, Function: 0},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_GlobalVarInvalidInit(t *testing.T) {
	m := newValidModule()
	invalid := ConstantHandle(999)
	m.GlobalVariables = []GlobalVariable{
		{Name: "g", Type: 0, Init: &invalid},
	}
	expectValidationErrors(t, m, "init constant 999 does not exist")
}

func TestValidateNew_FunctionInvalidLocalVarInit(t *testing.T) {
	m := newValidModule()
	invalid := ExpressionHandle(999)
	m.Functions[0].LocalVars = []LocalVariable{
		{Name: "x", Type: 0, Init: &invalid},
	}
	expectValidationErrors(t, m, "init expression 999 does not exist")
}

func TestValidateNew_FunctionInvalidResultType(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Result = &FunctionResult{Type: 999}
	expectValidationErrors(t, m, "result type 999 does not exist")
}

// =============================================================================
// Test: ValidationError formatting
// =============================================================================

func TestValidationError_Formatting(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			"plain",
			ValidationError{Message: "something bad", Statement: -1},
			"something bad",
		},
		{
			"in_function",
			ValidationError{Message: "bad type", Function: "main", Statement: -1},
			"in function main: bad type",
		},
		{
			"in_function_expression",
			ValidationError{
				Message:    "invalid",
				Function:   "main",
				Expression: ptrExpr(5),
				Statement:  -1,
			},
			"in function main, expression 5: invalid",
		},
		{
			"in_function_statement",
			ValidationError{
				Message:   "error here",
				Function:  "main",
				Statement: 3,
			},
			"in function main, statement 3: error here",
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

func ptrExpr(h ExpressionHandle) *ExpressionHandle {
	return &h
}

// =============================================================================
// Test: isPositionBuiltin helper
// =============================================================================

func TestIsPositionBuiltin_Cases(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
		want    bool
	}{
		{"position", BuiltinBinding{Builtin: BuiltinPosition}, true},
		{"vertex_index", BuiltinBinding{Builtin: BuiltinVertexIndex}, false},
		{"location", LocationBinding{Location: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPositionBuiltin(tt.binding)
			if got != tt.want {
				t.Errorf("isPositionBuiltin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: structHasPositionBuiltin helper
// =============================================================================

func TestStructHasPositionBuiltin(t *testing.T) {
	var posBinding Binding = BuiltinBinding{Builtin: BuiltinPosition}
	var locBinding Binding = LocationBinding{Location: 0}

	m := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Inner: StructType{Members: []StructMember{
				{Name: "pos", Type: 0, Binding: &posBinding},
			}}},
			{Inner: StructType{Members: []StructMember{
				{Name: "color", Type: 0, Binding: &locBinding},
			}}},
			{Inner: ScalarType{Kind: ScalarSint, Width: 4}}, // not a struct
		},
	}

	v := &Validator{module: m}

	if !v.structHasPositionBuiltin(TypeHandle(1)) {
		t.Error("Expected struct with @builtin(position) to return true")
	}
	if v.structHasPositionBuiltin(TypeHandle(2)) {
		t.Error("Expected struct without position to return false")
	}
	if v.structHasPositionBuiltin(TypeHandle(3)) {
		t.Error("Expected non-struct to return false")
	}
	if v.structHasPositionBuiltin(TypeHandle(999)) {
		t.Error("Expected out-of-range type to return false")
	}
}

// =============================================================================
// Test: Valid module passes all checks
// =============================================================================

func TestValidateNew_CompleteValidModule(t *testing.T) {
	m := newValidModule()

	// Add constants
	m.Constants = []Constant{
		{Name: "PI", Type: 0, Value: ScalarValue{Kind: ScalarFloat, Bits: 0x40490fdb}},
	}

	// Add global variables with unique bindings
	m.GlobalVariables = []GlobalVariable{
		{Name: "g1", Type: 0, Binding: &ResourceBinding{Group: 0, Binding: 0}},
		{Name: "g2", Type: 2, Binding: &ResourceBinding{Group: 0, Binding: 1}},
	}

	// Valid function with arguments and local vars
	m.Functions = append(m.Functions, Function{
		Name: "helper",
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 2},
		},
		Result: &FunctionResult{Type: 0},
		Expressions: []Expression{
			{Kind: ExprFunctionArgument{Index: 0}},
			{Kind: ExprFunctionArgument{Index: 1}},
			{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}},
		},
		LocalVars: []LocalVariable{
			{Name: "tmp", Type: 0},
		},
		Body: []Statement{
			{Kind: StmtEmit{Range: Range{Start: 0, End: 3}}},
		},
	})

	expectNoValidationErrors(t, m)
}

// =============================================================================
// Test: Nested loop break/continue context
// =============================================================================

func TestValidateNew_NestedLoopContext(t *testing.T) {
	m := newValidModule()
	// Nested loops: inner loop continuing block should not allow break/continue
	m.Functions[0].Body = []Statement{
		{Kind: StmtLoop{
			Body: []Statement{
				{Kind: StmtLoop{
					Body: []Statement{
						{Kind: StmtBreak{}}, // Valid: inside inner loop body
					},
					Continuing: []Statement{
						// break in continuing is invalid
						{Kind: StmtBreak{}},
					},
				}},
			},
		}},
	}
	expectValidationErrors(t, m, "break in continuing block")
}

// =============================================================================
// Test: Block statement validation
// =============================================================================

func TestValidateNew_BlockStatement(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtBlock{
			Block: []Statement{
				{Kind: StmtReturn{}},
			},
		}},
	}
	expectNoValidationErrors(t, m)
}

// =============================================================================
// Test: Barrier statement (always valid)
// =============================================================================

func TestValidateNew_BarrierAlwaysValid(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtBarrier{Flags: BarrierWorkGroup | BarrierStorage | BarrierTexture}},
	}
	expectNoValidationErrors(t, m)
}

// =============================================================================
// Test: If/else nested block validation
// =============================================================================

func TestValidateNew_IfWithNestedInvalidStatement(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtIf{
			Condition: 0,
			Accept: []Statement{
				{Kind: nil}, // Invalid in accept block
			},
			Reject: nil,
		}},
	}
	expectValidationErrors(t, m, "statement has nil kind")
}

func TestValidateNew_IfRejectBlockValidation(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []Statement{
		{Kind: StmtIf{
			Condition: 0,
			Accept:    nil,
			Reject: []Statement{
				{Kind: nil}, // Invalid in reject block
			},
		}},
	}
	expectValidationErrors(t, m, "statement has nil kind")
}
