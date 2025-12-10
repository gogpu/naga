package ir

import (
	"testing"
)

func TestValidate_ValidModule(t *testing.T) {
	// Create a minimal valid module
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
			{
				Name:  "vec4f",
				Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
		},
		Functions: []Function{
			{
				Name: "main",
				Result: &FunctionResult{
					Type:    TypeHandle(1), // vec4f
					Binding: bindingPtr(BuiltinBinding{Builtin: BuiltinPosition}),
				},
				Body: []Statement{},
			},
		},
		EntryPoints: []EntryPoint{
			{
				Name:     "main",
				Stage:    StageVertex,
				Function: FunctionHandle(0),
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("Valid module has validation errors:")
		for _, e := range errors {
			t.Errorf("  - %s", e.Error())
		}
	}
}

func TestValidate_NilModule(t *testing.T) {
	_, err := Validate(nil)
	if err == nil {
		t.Error("Expected error for nil module, got nil")
	}
}

func TestValidate_InvalidTypeHandle(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "array",
				Inner: ArrayType{Base: TypeHandle(999), Size: ArraySize{Constant: uint32Ptr(4)}, Stride: 4},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid type handle, got none")
	}
}

func TestValidate_InvalidVectorSize(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "vec5",
				Inner: VectorType{Size: VectorSize(5), Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid vector size, got none")
	}
	if len(errors) > 0 && errors[0].Message == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestValidate_MatrixNonFloat(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name: "mat_int",
				Inner: MatrixType{
					Columns: Vec3,
					Rows:    Vec3,
					Scalar:  ScalarType{Kind: ScalarSint, Width: 4},
				},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for non-float matrix, got none")
	}
}

func TestValidate_DuplicateFunctionName(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		Functions: []Function{
			{Name: "test"},
			{Name: "test"},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for duplicate function names, got none")
	}
}

func TestValidate_BreakOutsideLoop(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		Functions: []Function{
			{
				Name: "test",
				Body: []Statement{
					{Kind: StmtBreak{}},
				},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for break outside loop, got none")
	}
}

func TestValidate_InvalidExpressionHandle(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		Functions: []Function{
			{
				Name:        "test",
				Expressions: []Expression{},
				Body: []Statement{
					{Kind: StmtReturn{Value: exprHandlePtr(999)}},
				},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid expression handle, got none")
	}
}

func TestValidate_VertexEntryPointWithoutPosition(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		Functions: []Function{
			{
				Name: "main",
				Result: &FunctionResult{
					Type:    TypeHandle(0),
					Binding: bindingPtr(LocationBinding{Location: 0}),
				},
			},
		},
		EntryPoints: []EntryPoint{
			{
				Name:     "main",
				Stage:    StageVertex,
				Function: FunctionHandle(0),
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for vertex entry without position, got none")
	}
}

func TestValidate_DuplicateBinding(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		GlobalVariables: []GlobalVariable{
			{
				Name:    "var1",
				Type:    TypeHandle(0),
				Binding: &ResourceBinding{Group: 0, Binding: 0},
			},
			{
				Name:    "var2",
				Type:    TypeHandle(0),
				Binding: &ResourceBinding{Group: 0, Binding: 0},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for duplicate binding, got none")
	}
}

func TestValidate_SwitchWithoutDefault(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "i32",
				Inner: ScalarType{Kind: ScalarSint, Width: 4},
			},
		},
		Functions: []Function{
			{
				Name: "test",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralI32(1)}},
				},
				Body: []Statement{
					{Kind: StmtSwitch{
						Selector: ExpressionHandle(0),
						Cases: []SwitchCase{
							{Value: SwitchValueI32(1), Body: []Statement{}},
						},
					}},
				},
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for switch without default, got none")
	}
}

func TestValidate_ComputeEntryWithoutWorkgroup(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "f32",
				Inner: ScalarType{Kind: ScalarFloat, Width: 4},
			},
		},
		Functions: []Function{
			{Name: "main"},
		},
		EntryPoints: []EntryPoint{
			{
				Name:      "main",
				Stage:     StageCompute,
				Function:  FunctionHandle(0),
				Workgroup: [3]uint32{0, 0, 0}, // Invalid: all zeros
			},
		},
	}

	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Error("Expected validation errors for compute entry without workgroup size, got none")
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "simple message",
			err:  ValidationError{Message: "test error", Statement: -1},
			want: "test error",
		},
		{
			name: "with function",
			err:  ValidationError{Message: "test error", Function: "main", Statement: -1},
			want: "in function main: test error",
		},
		{
			name: "with expression",
			err:  ValidationError{Message: "test error", Function: "main", Expression: exprHandlePtr(5), Statement: -1},
			want: "in function main, expression 5: test error",
		},
		{
			name: "with statement",
			err:  ValidationError{Message: "test error", Function: "main", Statement: 10},
			want: "in function main, statement 10: test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper functions

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func exprHandlePtr(v ExpressionHandle) *ExpressionHandle {
	return &v
}

//nolint:gocritic // ptrToRefParam: helper for tests
func bindingPtr(b Binding) *Binding {
	return &b
}
