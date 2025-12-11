package wgsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

//nolint:nestif // Test validation requires nested type checking
func TestLowerer_TypeInference(t *testing.T) {
	// Create AST for simple vertex shader with type inference
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name: "vertex_main",
				Params: []*Parameter{
					{
						Name: "index",
						Type: &NamedType{Name: "u32"},
						Attributes: []Attribute{
							{Name: "builtin", Args: []Expr{&Ident{Name: "vertex_index"}}},
						},
					},
				},
				ReturnType: &NamedType{
					Name:       "vec4",
					TypeParams: []Type{&NamedType{Name: "f32"}},
				},
				Attributes: []Attribute{{Name: "vertex"}},
				Body: &BlockStmt{
					Statements: []Stmt{
						&ReturnStmt{
							Value: &CallExpr{
								Func: &Ident{Name: "vec4"},
								Args: []Expr{
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
								},
							},
						},
					},
				},
			},
		},
	}

	// Lower to IR
	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}

	// Verify we have a function
	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]

	// Verify ExpressionTypes has same length as Expressions
	if len(fn.ExpressionTypes) != len(fn.Expressions) {
		t.Errorf("ExpressionTypes length %d != Expressions length %d",
			len(fn.ExpressionTypes), len(fn.Expressions))
	}

	// Verify each expression has a resolved type
	for i, expr := range fn.Expressions {
		exprType := fn.ExpressionTypes[i]

		// Check that we have either a handle or a value
		if exprType.Handle == nil && exprType.Value == nil {
			t.Errorf("Expression %d (%T) has no type resolution", i, expr.Kind)
			continue
		}

		// Log the type for debugging
		t.Logf("Expression %d (%T): handle=%v, value=%v",
			i, expr.Kind, exprType.Handle, exprType.Value)

		// Verify type consistency for specific expression kinds
		switch kind := expr.Kind.(type) {
		case ir.Literal:
			// Literals should have inline types
			if exprType.Value == nil {
				t.Errorf("Literal expression %d should have inline type", i)
			}

		case ir.ExprFunctionArgument:
			// Function arguments should have type handles
			if exprType.Handle == nil {
				t.Errorf("FunctionArgument expression %d should have type handle", i)
			}

		case ir.ExprCompose:
			// Compose should have the target type as handle
			if exprType.Handle == nil {
				t.Errorf("Compose expression %d should have type handle", i)
			}
			// Verify the handle matches the compose type
			if *exprType.Handle != kind.Type {
				t.Errorf("Compose expression %d type handle mismatch: got %d, want %d",
					i, *exprType.Handle, kind.Type)
			}
		}
	}

	// Additional verification: check specific expression types
	foundLiteral := false
	foundCompose := false

	for i, expr := range fn.Expressions {
		switch expr.Kind.(type) {
		case ir.Literal:
			foundLiteral = true
			// Literal should resolve to f32
			exprType := fn.ExpressionTypes[i]
			if scalar, ok := exprType.Value.(ir.ScalarType); ok {
				if scalar.Kind != ir.ScalarFloat {
					t.Errorf("Literal should be float, got %v", scalar.Kind)
				}
			}

		case ir.ExprCompose:
			foundCompose = true
			// Compose should resolve to vec4<f32>
			exprType := fn.ExpressionTypes[i]
			if exprType.Handle != nil {
				typeInner := module.Types[*exprType.Handle].Inner
				if vec, ok := typeInner.(ir.VectorType); ok {
					if vec.Size != ir.Vec4 {
						t.Errorf("Compose should create Vec4, got %v", vec.Size)
					}
					if vec.Scalar.Kind != ir.ScalarFloat {
						t.Errorf("Compose should be float vector, got %v", vec.Scalar.Kind)
					}
				}
			}
		}
	}

	if !foundLiteral {
		t.Error("Expected to find at least one literal expression")
	}
	if !foundCompose {
		t.Error("Expected to find at least one compose expression")
	}
}

//nolint:nestif // Test validation requires nested type checking
func TestLowerer_TypeInference_BinaryOp(t *testing.T) {
	// Create AST for function with binary operation
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "add_test",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						&ReturnStmt{
							Value: &BinaryExpr{
								Op:    TokenPlus,
								Left:  &Literal{Kind: TokenFloatLiteral, Value: "1.0"},
								Right: &Literal{Kind: TokenFloatLiteral, Value: "2.0"},
							},
						},
					},
				},
			},
		},
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]

	// Find binary expression
	foundBinary := false
	for i, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprBinary); ok {
			foundBinary = true
			// Binary add of f32 should return f32
			exprType := fn.ExpressionTypes[i]
			if scalar, ok := exprType.Value.(ir.ScalarType); ok {
				if scalar.Kind != ir.ScalarFloat {
					t.Errorf("Binary add should preserve f32 type, got %v", scalar.Kind)
				}
			} else {
				t.Errorf("Binary add should have scalar type, got %T", exprType.Value)
			}
		}
	}

	if !foundBinary {
		t.Error("Expected to find binary expression")
	}
}

//nolint:nestif // Test validation requires nested type checking
func TestLowerer_TypeInference_Comparison(t *testing.T) {
	// Create AST for function with comparison
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "compare_test",
				ReturnType: &NamedType{Name: "bool"},
				Body: &BlockStmt{
					Statements: []Stmt{
						&ReturnStmt{
							Value: &BinaryExpr{
								Op:    TokenLess,
								Left:  &Literal{Kind: TokenFloatLiteral, Value: "1.0"},
								Right: &Literal{Kind: TokenFloatLiteral, Value: "2.0"},
							},
						},
					},
				},
			},
		},
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]

	// Find binary comparison expression
	foundComparison := false
	for i, expr := range fn.Expressions {
		if binExpr, ok := expr.Kind.(ir.ExprBinary); ok {
			if binExpr.Op == ir.BinaryLess {
				foundComparison = true
				// Comparison should return bool
				exprType := fn.ExpressionTypes[i]
				if scalar, ok := exprType.Value.(ir.ScalarType); ok {
					if scalar.Kind != ir.ScalarBool {
						t.Errorf("Comparison should return bool, got %v", scalar.Kind)
					}
				} else {
					t.Errorf("Comparison should have scalar type, got %T", exprType.Value)
				}
			}
		}
	}

	if !foundComparison {
		t.Error("Expected to find comparison expression")
	}
}
