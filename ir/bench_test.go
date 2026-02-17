package ir

import (
	"runtime"
	"testing"
)

// benchBindingPtr converts a Binding interface value to *Binding for use
// in struct literals that require pointer-to-interface fields.
func benchBindingPtr(b Binding) *Binding {
	return &b
}

// ---------------------------------------------------------------------------
// Module creation benchmarks
// ---------------------------------------------------------------------------

// BenchmarkModuleCreation benchmarks allocating an empty ir.Module
// and basic field initialization.
func BenchmarkModuleCreation(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := &Module{
			Types:           make([]Type, 0, 16),
			Constants:       make([]Constant, 0, 8),
			GlobalVariables: make([]GlobalVariable, 0, 4),
			Functions:       make([]Function, 0, 4),
			EntryPoints:     make([]EntryPoint, 0, 2),
		}
		runtime.KeepAlive(m)
	}
}

// BenchmarkAddType benchmarks adding types to a module's type arena.
func BenchmarkAddType(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := &Module{
			Types: make([]Type, 0, 32),
		}

		// Add a representative set of types (scalar, vector, matrix, struct)
		m.Types = append(m.Types, Type{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}})
		m.Types = append(m.Types, Type{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}})
		m.Types = append(m.Types, Type{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}})
		m.Types = append(m.Types, Type{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 1}})
		m.Types = append(m.Types, Type{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}})
		m.Types = append(m.Types, Type{Name: "vec3f", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}})
		m.Types = append(m.Types, Type{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}})
		m.Types = append(m.Types, Type{Name: "mat4x4f", Inner: MatrixType{
			Columns: Vec4, Rows: Vec4,
			Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
		}})
		m.Types = append(m.Types, Type{Name: "VertexOutput", Inner: StructType{
			Members: []StructMember{
				{Name: "position", Type: 6, Binding: benchBindingPtr(BuiltinBinding{Builtin: BuiltinPosition})},
				{Name: "color", Type: 6, Binding: benchBindingPtr(LocationBinding{Location: 0})},
			},
			Span: 32,
		}})

		runtime.KeepAlive(m)
	}
}

// BenchmarkAddFunction benchmarks adding a function with arguments,
// local variables, expressions, and statements to a module.
func BenchmarkAddFunction(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := &Module{
			Functions: make([]Function, 0, 4),
		}

		fn := Function{
			Name: "vs_main",
			Arguments: []FunctionArgument{
				{Name: "idx", Type: 2, Binding: benchBindingPtr(BuiltinBinding{Builtin: BuiltinVertexIndex})},
			},
			Result: &FunctionResult{
				Type:    6,
				Binding: benchBindingPtr(BuiltinBinding{Builtin: BuiltinPosition}),
			},
			LocalVars: []LocalVariable{
				{Name: "out", Type: 8},
				{Name: "pos", Type: 7},
			},
			Expressions: make([]Expression, 0, 16),
			Body:        make([]Statement, 0, 8),
		}

		// Simulate adding expressions
		for j := 0; j < 10; j++ {
			fn.Expressions = append(fn.Expressions, Expression{
				Kind: Literal{Value: LiteralF32(float32(j) * 0.1)},
			})
		}

		// Simulate adding statements
		fn.Body = append(fn.Body, Statement{
			Kind: StmtEmit{Range: Range{Start: 0, End: 10}},
		})
		fn.Body = append(fn.Body, Statement{
			Kind: StmtReturn{Value: newExprHandle(9)},
		})

		m.Functions = append(m.Functions, fn)
		runtime.KeepAlive(m)
	}
}

// BenchmarkExpressionAlloc benchmarks expression allocation and handle creation.
func BenchmarkExpressionAlloc(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		exprs := make([]Expression, 0, 64)

		// Simulate building a chain of expressions for a complex computation
		for j := 0; j < 50; j++ {
			exprs = append(exprs, Expression{
				Kind: Literal{Value: LiteralF32(float32(j) * 0.5)},
			})
		}

		for j := 0; j < 10; j++ {
			exprs = append(exprs, Expression{
				Kind: ExprBinary{
					Op:    BinaryAdd,
					Left:  ExpressionHandle(j * 2),
					Right: ExpressionHandle(j*2 + 1),
				},
			})
		}

		runtime.KeepAlive(exprs)
	}
}

// BenchmarkValidateModule benchmarks IR validation for a synthetically
// constructed module with representative complexity.
func BenchmarkValidateModule(b *testing.B) {
	// Build a minimal valid module
	m := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
		Functions: []Function{
			{
				Name: "vs_main",
				Arguments: []FunctionArgument{
					{Name: "idx", Type: 1, Binding: benchBindingPtr(BuiltinBinding{Builtin: BuiltinVertexIndex})},
				},
				Result: &FunctionResult{
					Type:    2,
					Binding: benchBindingPtr(BuiltinBinding{Builtin: BuiltinPosition}),
				},
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(0.0)}},
					{Kind: Literal{Value: LiteralF32(0.0)}},
					{Kind: Literal{Value: LiteralF32(0.0)}},
					{Kind: Literal{Value: LiteralF32(1.0)}},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 4}}},
				},
			},
		},
		EntryPoints: []EntryPoint{
			{Name: "vs_main", Stage: StageVertex, Function: 0},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		errs, err := Validate(m)
		if err != nil {
			b.Fatalf("validate failed: %v", err)
		}
		runtime.KeepAlive(errs)
	}
}

// newExprHandle creates a pointer to an ExpressionHandle (helper for benchmarks).
func newExprHandle(h ExpressionHandle) *ExpressionHandle {
	return &h
}
