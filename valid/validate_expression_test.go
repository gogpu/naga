package valid

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helpers for new validator tests
// =============================================================================

// newValidModule returns a minimal valid module with basic types and one function.
func newValidModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "mat4f", Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
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
func expectValidationErrors(t *testing.T, module *ir.Module, expectedSubstrings ...string) {
	t.Helper()
	errors, err := validateErrs(module, CapAll)
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
func expectNoValidationErrors(t *testing.T, module *ir.Module) {
	t.Helper()
	errors, err := validateErrs(module, CapAll)
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
		ir.Expression{Kind: ir.ExprFunctionArgument{Index: 5}})
	expectValidationErrors(t, m, "argument index 5 out of range")
}

func TestValidateNew_LocalVariableOutOfRange(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprLocalVariable{Variable: 999}})
	expectValidationErrors(t, m, "local variable index 999 out of range")
}

func TestValidateNew_GlobalVariableInvalid(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprGlobalVariable{Variable: 999}})
	expectValidationErrors(t, m, "global variable 999 does not exist")
}

func TestValidateNew_MathInvalidArg2(t *testing.T) {
	m := newValidModule()
	arg1 := ir.ExpressionHandle(1)
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprMath{Fun: ir.MathClamp, Arg: 0, Arg1: &arg1, Arg2: &invalid}})
	expectValidationErrors(t, m, "arg2 expression 999 does not exist")
}

func TestValidateNew_MathInvalidArg3(t *testing.T) {
	m := newValidModule()
	arg1 := ir.ExpressionHandle(1)
	arg2 := ir.ExpressionHandle(0)
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprMath{Fun: ir.MathClamp, Arg: 0, Arg1: &arg1, Arg2: &arg2, Arg3: &invalid}})
	expectValidationErrors(t, m, "arg3 expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, ArrayIndex: &invalid}})
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidOffset(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, Offset: &invalid}})
	expectValidationErrors(t, m, "offset expression 999 does not exist")
}

func TestValidateNew_ImageSampleInvalidDepthRef(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageSample{Image: 0, Sampler: 0, Coordinate: 0, DepthRef: &invalid}})
	expectValidationErrors(t, m, "depth ref expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 0, ArrayIndex: &invalid}})
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidSample(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 0, Sample: &invalid}})
	expectValidationErrors(t, m, "sample expression 999 does not exist")
}

func TestValidateNew_ImageLoadInvalidLevel(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Expressions = append(m.Functions[0].Expressions,
		ir.Expression{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 0, Level: &invalid}})
	expectValidationErrors(t, m, "level expression 999 does not exist")
}

// =============================================================================
// Test: Statement validation - covering gaps
// =============================================================================

func TestValidateNew_EmitRangeEndOutOfBounds(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 100}}},
	}
	expectValidationErrors(t, m, "emit range end 100 out of range")
}

func TestValidateNew_EmitRangeStartGEEnd(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 1}}},
	}
	expectValidationErrors(t, m, "emit range start 1 >= end 1")
}

func TestValidateNew_ImageStoreInvalidArrayIndex(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtImageStore{Image: 0, Coordinate: 0, Value: 0, ArrayIndex: &invalid}},
	}
	expectValidationErrors(t, m, "array index expression 999 does not exist")
}

func TestValidateNew_AtomicInvalidResult(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtAtomic{Pointer: 0, Value: 0, Result: &invalid}},
	}
	expectValidationErrors(t, m, "result expression 999 does not exist")
}

func TestValidateNew_CallInvalidResult(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtCall{Function: 0, Result: &invalid}},
	}
	expectValidationErrors(t, m, "result expression 999 does not exist")
}

func TestValidateNew_CallInvalidArguments(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{999}}},
	}
	expectValidationErrors(t, m, "argument 0 expression 999 does not exist")
}

// =============================================================================
// Test: Type validation - covering gaps
// =============================================================================

func TestValidateNew_ValidScalarWidthByte(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "u8", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 1}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_ValidScalarWidthHalf(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "f16", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_ValidScalarWidthDouble(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "f64", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}})
	expectNoValidationErrors(t, m)
}

func TestValidateNew_InvalidVectorScalarWidth(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "bad", Inner: ir.VectorType{
		Size:   ir.Vec2,
		Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 3},
	}})
	expectValidationErrors(t, m, "vector scalar width must be 1, 2, 4, or 8")
}

func TestValidateNew_InvalidMatrixColumns(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "bad", Inner: ir.MatrixType{
		Columns: 5, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
	}})
	expectValidationErrors(t, m, "matrix columns must be 2, 3, or 4")
}

func TestValidateNew_InvalidMatrixRows(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "bad", Inner: ir.MatrixType{
		Columns: ir.Vec4, Rows: 1, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
	}})
	expectValidationErrors(t, m, "matrix rows must be 2, 3, or 4")
}

// An empty member name is an unnamed member: the SPIR-V frontend produces
// them, and each backend names them for itself.
func TestValidateNew_StructUnnamedMemberAllowed(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "unnamed-member", Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "", Type: 0},
			{Name: "", Type: 0},
		},
		Span: 4,
	}})
	if errs, err := validateErrs(m, CapAll); err != nil || len(errs) != 0 {
		t.Errorf("validation errors = %v (%v), want none", errs, err)
	}
}

func TestValidateNew_StructDuplicateMemberName(t *testing.T) {
	m := newValidModule()
	m.Types = append(m.Types, ir.Type{Name: "bad", Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 0},
			{Name: "x", Type: 0},
		},
	}})
	expectValidationErrors(t, m, "duplicate struct member name")
}

func TestValidateNew_StructMemberCircularRef(t *testing.T) {
	m := newValidModule()
	// Add struct at index 5 with member referencing itself
	m.Types = append(m.Types, ir.Type{Name: "bad", Inner: ir.StructType{
		Members: []ir.StructMember{{Name: "self", Type: ir.TypeHandle(5)}},
	}})
	expectValidationErrors(t, m, "struct member \"self\" has circular reference")
}

// =============================================================================
// Test: Entry point validation - covering gaps
// =============================================================================

func TestValidateNew_EntryPointEmptyName(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []ir.EntryPoint{
		{Name: "", Stage: ir.StageVertex, Function: ir.Function{Name: "vs"}},
	}
	expectValidationErrors(t, m, "entry point 0 has empty name")
}

func TestValidateNew_EntryPointDuplicateName(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: ir.Function{Name: "main"}},
		{Name: "main", Stage: ir.StageFragment, Function: ir.Function{Name: "main"}},
	}
	expectValidationErrors(t, m, "duplicate entry point name")
}

func TestValidateNew_VertexDirectPositionBinding(t *testing.T) {
	m := newValidModule()
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	m.EntryPoints = []ir.EntryPoint{
		{Name: "vs", Stage: ir.StageVertex, Function: ir.Function{
			Name: "vs",
			Result: &ir.FunctionResult{
				Type:    1, // vec4f
				Binding: &posBinding,
			},
		}},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_VertexStructPositionBinding(t *testing.T) {
	m := newValidModule()
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	m.Types = append(m.Types, ir.Type{
		Name: "VsOut",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "pos", Type: 1, Binding: &posBinding},
			},
		},
	})
	m.EntryPoints = []ir.EntryPoint{
		{Name: "vs", Stage: ir.StageVertex, Function: ir.Function{
			Name: "vs",
			Result: &ir.FunctionResult{
				Type: ir.TypeHandle(len(m.Types) - 1),
			},
		}},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_FragmentValid(t *testing.T) {
	m := newValidModule()
	m.EntryPoints = []ir.EntryPoint{
		{Name: "fs", Stage: ir.StageFragment, Function: ir.Function{Name: "fs"}},
	}
	expectNoValidationErrors(t, m)
}

func TestValidateNew_GlobalVarInvalidInit(t *testing.T) {
	m := newValidModule()
	invalid := ir.ConstantHandle(999)
	m.GlobalVariables = []ir.GlobalVariable{
		{Name: "g", Type: 0, Init: &invalid},
	}
	expectValidationErrors(t, m, "init constant 999 does not exist")
}

// TestValidateNew_ConstantInvalidInit: a constant's value is its Init, so an
// Init outside Module.GlobalExpressions is a constant without a value.
func TestValidateNew_ConstantInvalidInit(t *testing.T) {
	m := newValidModule()
	m.Constants = []ir.Constant{{Name: "C", Type: 0, Init: ir.ExpressionHandle(len(m.GlobalExpressions) + 7)}}
	expectValidationErrors(t, m, fmt.Sprintf("constant 0 (C): init expression %d does not exist", len(m.GlobalExpressions)+7))
}

func TestValidateNew_FunctionInvalidLocalVarInit(t *testing.T) {
	m := newValidModule()
	invalid := ir.ExpressionHandle(999)
	m.Functions[0].LocalVars = []ir.LocalVariable{
		{Name: "x", Type: 0, Init: &invalid},
	}
	expectValidationErrors(t, m, "init expression 999 does not exist")
}

func TestValidateNew_FunctionInvalidResultType(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Result = &ir.FunctionResult{Type: 999}
	expectValidationErrors(t, m, "result type 999 does not exist")
}

// =============================================================================
// Test: ValidationError formatting
// =============================================================================

func TestValidationError_Formatting(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		want string
	}{
		{
			"plain",
			Error{Message: "something bad", Statement: -1},
			"something bad",
		},
		{
			"in_function",
			Error{Message: "bad type", Function: "main", Statement: -1},
			"in function main: bad type",
		},
		{
			"in_function_expression",
			Error{
				Message:    "invalid",
				Function:   "main",
				Expression: ptrExpr(5),
				Statement:  -1,
			},
			"in function main, expression 5: invalid",
		},
		{
			"in_function_statement",
			Error{
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

func ptrExpr(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

// =============================================================================
// Test: isPositionBuiltin helper
// =============================================================================

func TestIsPositionBuiltin_Cases(t *testing.T) {
	tests := []struct {
		name    string
		binding ir.Binding
		want    bool
	}{
		{"position", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, true},
		{"vertex_index", ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}, false},
		{"location", ir.LocationBinding{Location: 0}, false},
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
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var locBinding ir.Binding = ir.LocationBinding{Location: 0}

	m := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "pos", Type: 0, Binding: &posBinding},
			}}},
			{Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "color", Type: 0, Binding: &locBinding},
			}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, // not a struct
		},
	}

	v := &validator{module: m}

	if !v.structHasPositionBuiltin(ir.TypeHandle(1)) {
		t.Error("Expected struct with @builtin(position) to return true")
	}
	if v.structHasPositionBuiltin(ir.TypeHandle(2)) {
		t.Error("Expected struct without position to return false")
	}
	if v.structHasPositionBuiltin(ir.TypeHandle(3)) {
		t.Error("Expected non-struct to return false")
	}
	if v.structHasPositionBuiltin(ir.TypeHandle(999)) {
		t.Error("Expected out-of-range type to return false")
	}
}

// =============================================================================
// Test: Valid module passes all checks
// =============================================================================

func TestValidateNew_CompleteValidModule(t *testing.T) {
	m := newValidModule()

	// Add constants
	m.Constants = []ir.Constant{
		{Name: "PI", Type: 0, Init: ir.ExpressionHandle(len(m.GlobalExpressions))},
	}
	m.GlobalExpressions = append(m.GlobalExpressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(3.1415927)}})

	// Add global variables with unique bindings
	m.GlobalVariables = []ir.GlobalVariable{
		{Name: "g1", Type: 0, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
		{Name: "g2", Type: 2, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},
	}

	// Valid function with arguments and local vars
	m.Functions = append(m.Functions, ir.Function{
		Name: "helper",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 0}, // one type: Add takes equal operands (P1-IR-029)
		},
		Result: &ir.FunctionResult{Type: 0},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},
		},
		LocalVars: []ir.LocalVariable{
			{Name: "tmp", Type: 0},
		},
		Body: []ir.Statement{
			// The arguments need no Emit (Rust: an Emit of one is
			// ExpressionAlreadyInScope).
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
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
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtLoop{
			Body: []ir.Statement{
				{Kind: ir.StmtLoop{
					Body: []ir.Statement{
						{Kind: ir.StmtBreak{}}, // Valid: inside inner loop body
					},
					Continuing: []ir.Statement{
						// break in continuing is invalid
						{Kind: ir.StmtBreak{}},
					},
				}},
			},
		}},
	}
	expectValidationErrors(t, m, "`break` is used outside of a `loop` or `switch`")
}

// =============================================================================
// Test: Block statement validation
// =============================================================================

func TestValidateNew_BlockStatement(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtBlock{
			Block: []ir.Statement{
				{Kind: ir.StmtReturn{}},
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
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtControlBarrier{Flags: ir.BarrierWorkGroup | ir.BarrierStorage | ir.BarrierTexture}},
	}
	expectNoValidationErrors(t, m)
}

// =============================================================================
// Test: If/else nested block validation
// =============================================================================

func TestValidateNew_IfWithNestedInvalidStatement(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtIf{
			Condition: 0,
			Accept: []ir.Statement{
				{Kind: nil}, // Invalid in accept block
			},
			Reject: nil,
		}},
	}
	expectValidationErrors(t, m, "statement has nil kind")
}

func TestValidateNew_IfRejectBlockValidation(t *testing.T) {
	m := newValidModule()
	m.Functions[0].Body = []ir.Statement{
		{Kind: ir.StmtIf{
			Condition: 0,
			Accept:    nil,
			Reject: []ir.Statement{
				{Kind: nil}, // Invalid in reject block
			},
		}},
	}
	expectValidationErrors(t, m, "statement has nil kind")
}
