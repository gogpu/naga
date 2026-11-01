package valid

import (
	"runtime"
	"testing"

	"github.com/gogpu/naga/ir"
)

func benchBindingPtr(b ir.Binding) *ir.Binding {
	return &b
}

// BenchmarkValidateModule benchmarks IR validation for a synthetically
// constructed module with representative complexity.
func BenchmarkValidateModule(b *testing.B) {
	// Build a minimal valid module
	m := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "vs_main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "vs_main",
					Arguments: []ir.FunctionArgument{
						{Name: "idx", Type: 1, Binding: benchBindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})},
					},
					Result: &ir.FunctionResult{
						Type:    2,
						Binding: benchBindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}),
					},
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					},
				},
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		errs, err := validateErrs(m, CapAll)
		if err != nil {
			b.Fatalf("validate failed: %v", err)
		}
		runtime.KeepAlive(errs)
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		want string
	}{
		{
			name: "message_only",
			err:  Error{Message: "test error"},
			want: "test error",
		},
		{
			name: "with_function",
			err:  Error{Message: "bad type", Function: "myFunc", Statement: -1},
			want: "in function myFunc: bad type",
		},
		{
			name: "with_function_and_expression",
			err: Error{
				Message:    "invalid",
				Function:   "foo",
				Expression: func() *ir.ExpressionHandle { h := ir.ExpressionHandle(5); return &h }(),
			},
			want: "in function foo, expression 5: invalid",
		},
		{
			name: "with_function_and_statement",
			err: Error{
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

func TestValidate_ErrorCases(t *testing.T) {
	t.Run("nil_module", func(t *testing.T) {
		_, err := Validate(nil, Options{})
		if err == nil {
			t.Error("expected error for nil module")
		}
	})

	t.Run("invalid_type_handle_in_constant", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			Constants: []ir.Constant{
				{Name: "bad", Type: 999}, // out of range
			},
		}
		errs, err := validateErrs(module, CapAll)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) == 0 {
			t.Error("expected validation errors for invalid type handle")
		}
	})

	t.Run("invalid_expression_handle_in_function", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			Functions: []ir.Function{
				{
					Name: "broken",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 999}}, // out of range
					},
					Body: ir.Block{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					},
				},
			},
		}
		errs, err := validateErrs(module, CapAll)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) == 0 {
			t.Error("expected validation errors for out-of-range global variable handle")
		}
	})

	t.Run("valid_minimal_module", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
		}
		errs, err := validateErrs(module, CapAll)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errs) != 0 {
			t.Errorf("expected 0 validation errors, got %d: %v", len(errs), errs)
		}
	})
}
