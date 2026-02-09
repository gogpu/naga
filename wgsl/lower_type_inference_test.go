package wgsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

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

func TestLowerer_LetTypeInference_Scalar(t *testing.T) {
	// Test: let x = 1.0; (should infer f32)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_let_scalar",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let x = 1.0;
						&VarDecl{
							Name: "x",
							Type: nil, // No explicit type - should be inferred
							Init: &Literal{Kind: TokenFloatLiteral, Value: "1.0"},
						},
						&ReturnStmt{
							Value: &Ident{Name: "x"},
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

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]
	if localVar.Name != "x" {
		t.Errorf("expected local var name 'x', got '%s'", localVar.Name)
	}

	// Verify the type was inferred as f32
	typeInner := module.Types[localVar.Type].Inner
	if scalar, ok := typeInner.(ir.ScalarType); ok {
		if scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
			t.Errorf("expected f32 type, got kind=%v width=%d", scalar.Kind, scalar.Width)
		}
	} else {
		t.Errorf("expected scalar type, got %T", typeInner)
	}
}

func TestLowerer_LetTypeInference_Vector(t *testing.T) {
	// Test: let v = vec3(1.0, 2.0, 3.0); (should infer vec3<f32>)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name: "test_let_vector",
				ReturnType: &NamedType{
					Name:       "vec3",
					TypeParams: []Type{&NamedType{Name: "f32"}},
				},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let v = vec3(1.0, 2.0, 3.0);
						&VarDecl{
							Name: "v",
							Type: nil, // No explicit type - should be inferred
							Init: &CallExpr{
								Func: &Ident{Name: "vec3"},
								Args: []Expr{
									&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "2.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "3.0"},
								},
							},
						},
						&ReturnStmt{
							Value: &Ident{Name: "v"},
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

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]
	if localVar.Name != "v" {
		t.Errorf("expected local var name 'v', got '%s'", localVar.Name)
	}

	// Verify the type was inferred as vec3<f32>
	typeInner := module.Types[localVar.Type].Inner
	if vec, ok := typeInner.(ir.VectorType); ok {
		if vec.Size != ir.Vec3 {
			t.Errorf("expected vec3 size, got %v", vec.Size)
		}
		if vec.Scalar.Kind != ir.ScalarFloat || vec.Scalar.Width != 4 {
			t.Errorf("expected f32 scalar, got kind=%v width=%d", vec.Scalar.Kind, vec.Scalar.Width)
		}
	} else {
		t.Errorf("expected vector type, got %T", typeInner)
	}
}

func TestLowerer_LetTypeInference_FromBinaryExpr(t *testing.T) {
	// Test: let sum = 1.0 + 2.0; (should infer f32)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_let_binary",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let sum = 1.0 + 2.0;
						&VarDecl{
							Name: "sum",
							Type: nil, // No explicit type - should be inferred
							Init: &BinaryExpr{
								Op:    TokenPlus,
								Left:  &Literal{Kind: TokenFloatLiteral, Value: "1.0"},
								Right: &Literal{Kind: TokenFloatLiteral, Value: "2.0"},
							},
						},
						&ReturnStmt{
							Value: &Ident{Name: "sum"},
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

	fn := module.Functions[0]

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]
	if localVar.Name != "sum" {
		t.Errorf("expected local var name 'sum', got '%s'", localVar.Name)
	}

	// Verify the type was inferred as f32
	typeInner := module.Types[localVar.Type].Inner
	if scalar, ok := typeInner.(ir.ScalarType); ok {
		if scalar.Kind != ir.ScalarFloat {
			t.Errorf("expected float type, got %v", scalar.Kind)
		}
	} else {
		t.Errorf("expected scalar type, got %T", typeInner)
	}
}

func TestLowerer_LetTypeInference_FromMathFunc(t *testing.T) {
	// Test: let n = normalize(vec3(1.0, 0.0, 0.0)); (should infer vec3<f32>)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name: "test_let_math",
				ReturnType: &NamedType{
					Name:       "vec3",
					TypeParams: []Type{&NamedType{Name: "f32"}},
				},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let n = normalize(vec3(1.0, 0.0, 0.0));
						&VarDecl{
							Name: "n",
							Type: nil, // No explicit type - should be inferred
							Init: &CallExpr{
								Func: &Ident{Name: "normalize"},
								Args: []Expr{
									&CallExpr{
										Func: &Ident{Name: "vec3"},
										Args: []Expr{
											&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
											&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
											&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
										},
									},
								},
							},
						},
						&ReturnStmt{
							Value: &Ident{Name: "n"},
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

	fn := module.Functions[0]

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]
	if localVar.Name != "n" {
		t.Errorf("expected local var name 'n', got '%s'", localVar.Name)
	}

	// Verify the type was inferred as vec3<f32>
	typeInner := module.Types[localVar.Type].Inner
	if vec, ok := typeInner.(ir.VectorType); ok {
		if vec.Size != ir.Vec3 {
			t.Errorf("expected vec3 size, got %v", vec.Size)
		}
		if vec.Scalar.Kind != ir.ScalarFloat {
			t.Errorf("expected float scalar, got %v", vec.Scalar.Kind)
		}
	} else {
		t.Errorf("expected vector type, got %T", typeInner)
	}
}

func TestLowerer_LetTypeInference_Integer(t *testing.T) {
	// Test: let i = 42; (should infer i32)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_let_int",
				ReturnType: &NamedType{Name: "i32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let i = 42;
						&VarDecl{
							Name: "i",
							Type: nil, // No explicit type - should be inferred
							Init: &Literal{Kind: TokenIntLiteral, Value: "42"},
						},
						&ReturnStmt{
							Value: &Ident{Name: "i"},
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

	fn := module.Functions[0]

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]

	// Verify the type was inferred as i32
	typeInner := module.Types[localVar.Type].Inner
	if scalar, ok := typeInner.(ir.ScalarType); ok {
		if scalar.Kind != ir.ScalarSint || scalar.Width != 4 {
			t.Errorf("expected i32 type, got kind=%v width=%d", scalar.Kind, scalar.Width)
		}
	} else {
		t.Errorf("expected scalar type, got %T", typeInner)
	}
}

func TestLowerer_LetTypeInference_Bool(t *testing.T) {
	// Test: let b = true; (should infer bool)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_let_bool",
				ReturnType: &NamedType{Name: "bool"},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let b = true;
						&VarDecl{
							Name: "b",
							Type: nil, // No explicit type - should be inferred
							Init: &Literal{Kind: TokenTrue, Value: "true"},
						},
						&ReturnStmt{
							Value: &Ident{Name: "b"},
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

	fn := module.Functions[0]

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]

	// Verify the type was inferred as bool
	typeInner := module.Types[localVar.Type].Inner
	if scalar, ok := typeInner.(ir.ScalarType); ok {
		if scalar.Kind != ir.ScalarBool {
			t.Errorf("expected bool type, got kind=%v", scalar.Kind)
		}
	} else {
		t.Errorf("expected scalar type, got %T", typeInner)
	}
}

func TestLowerer_ArrayInit_Shorthand(t *testing.T) {
	// Test: let arr = array(1.0, 2.0, 3.0); (should infer array<f32, 3>)
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_array_shorthand",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						// let arr = array(1.0, 2.0, 3.0);
						&VarDecl{
							Name: "arr",
							Type: nil, // No explicit type - should be inferred
							Init: &CallExpr{
								Func: &Ident{Name: "array"},
								Args: []Expr{
									&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "2.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "3.0"},
								},
							},
						},
						&ReturnStmt{
							Value: &Literal{Kind: TokenFloatLiteral, Value: "0.0"},
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

	fn := module.Functions[0]

	// Verify local variable was created with inferred type
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]
	if localVar.Name != "arr" {
		t.Errorf("expected local var name 'arr', got '%s'", localVar.Name)
	}

	// Verify the type was inferred as array<f32, 3>
	typeInner := module.Types[localVar.Type].Inner
	if arrType, ok := typeInner.(ir.ArrayType); ok {
		// Check size
		if arrType.Size.Constant == nil || *arrType.Size.Constant != 3 {
			t.Errorf("expected array size 3, got %v", arrType.Size.Constant)
		}
		// Check element type (should be f32)
		elemType := module.Types[arrType.Base].Inner
		if scalar, ok := elemType.(ir.ScalarType); ok {
			if scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
				t.Errorf("expected f32 element type, got kind=%v width=%d", scalar.Kind, scalar.Width)
			}
		} else {
			t.Errorf("expected scalar element type, got %T", elemType)
		}
	} else {
		t.Errorf("expected array type, got %T", typeInner)
	}
}

func TestLowerer_ArrayInit_Vectors(t *testing.T) {
	// Test: let positions = array(vec2(0.0, 0.5), vec2(-0.5, -0.5), vec2(0.5, -0.5));
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_array_vectors",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						&VarDecl{
							Name: "positions",
							Type: nil, // Inferred
							Init: &CallExpr{
								Func: &Ident{Name: "array"},
								Args: []Expr{
									&CallExpr{
										Func: &Ident{Name: "vec2"},
										Args: []Expr{
											&Literal{Kind: TokenFloatLiteral, Value: "0.0"},
											&Literal{Kind: TokenFloatLiteral, Value: "0.5"},
										},
									},
									&CallExpr{
										Func: &Ident{Name: "vec2"},
										Args: []Expr{
											&Literal{Kind: TokenFloatLiteral, Value: "-0.5"},
											&Literal{Kind: TokenFloatLiteral, Value: "-0.5"},
										},
									},
									&CallExpr{
										Func: &Ident{Name: "vec2"},
										Args: []Expr{
											&Literal{Kind: TokenFloatLiteral, Value: "0.5"},
											&Literal{Kind: TokenFloatLiteral, Value: "-0.5"},
										},
									},
								},
							},
						},
						&ReturnStmt{
							Value: &Literal{Kind: TokenFloatLiteral, Value: "0.0"},
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

	fn := module.Functions[0]

	// Verify local variable was created
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]

	// Verify the type is array<vec2<f32>, 3>
	typeInner := module.Types[localVar.Type].Inner
	if arrType, ok := typeInner.(ir.ArrayType); ok {
		// Check size
		if arrType.Size.Constant == nil || *arrType.Size.Constant != 3 {
			t.Errorf("expected array size 3, got %v", arrType.Size.Constant)
		}
		// Check element type (should be vec2<f32>)
		elemType := module.Types[arrType.Base].Inner
		if vecType, ok := elemType.(ir.VectorType); ok {
			if vecType.Size != ir.Vec2 {
				t.Errorf("expected vec2 element type, got size=%v", vecType.Size)
			}
			if vecType.Scalar.Kind != ir.ScalarFloat {
				t.Errorf("expected float scalar, got %v", vecType.Scalar.Kind)
			}
		} else {
			t.Errorf("expected vector element type, got %T", elemType)
		}
	} else {
		t.Errorf("expected array type, got %T", typeInner)
	}
}

func TestLowerer_ArrayInit_ExplicitType(t *testing.T) {
	// Test: var arr: array<f32, 3> = array<f32, 3>(1.0, 2.0, 3.0);
	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name:       "test_array_explicit",
				ReturnType: &NamedType{Name: "f32"},
				Body: &BlockStmt{
					Statements: []Stmt{
						&VarDecl{
							Name: "arr",
							Type: &ArrayType{
								Element: &NamedType{Name: "f32"},
								Size:    &Literal{Kind: TokenIntLiteral, Value: "3"},
							},
							Init: &ConstructExpr{
								Type: &ArrayType{
									Element: &NamedType{Name: "f32"},
									Size:    &Literal{Kind: TokenIntLiteral, Value: "3"},
								},
								Args: []Expr{
									&Literal{Kind: TokenFloatLiteral, Value: "1.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "2.0"},
									&Literal{Kind: TokenFloatLiteral, Value: "3.0"},
								},
							},
						},
						&ReturnStmt{
							Value: &Literal{Kind: TokenFloatLiteral, Value: "0.0"},
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

	fn := module.Functions[0]

	// Verify local variable was created
	if len(fn.LocalVars) != 1 {
		t.Fatalf("expected 1 local var, got %d", len(fn.LocalVars))
	}

	localVar := fn.LocalVars[0]

	// Verify the type is array<f32, 3>
	typeInner := module.Types[localVar.Type].Inner
	if arrType, ok := typeInner.(ir.ArrayType); ok {
		// Check size
		if arrType.Size.Constant == nil || *arrType.Size.Constant != 3 {
			t.Errorf("expected array size 3, got %v", arrType.Size.Constant)
		}
		// Check element type
		elemType := module.Types[arrType.Base].Inner
		if scalar, ok := elemType.(ir.ScalarType); ok {
			if scalar.Kind != ir.ScalarFloat {
				t.Errorf("expected float element type, got %v", scalar.Kind)
			}
		} else {
			t.Errorf("expected scalar element type, got %T", elemType)
		}
	} else {
		t.Errorf("expected array type, got %T", typeInner)
	}
}
