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

	// Entry point functions are inline in EntryPoints, not in Functions[]
	if len(module.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(module.EntryPoints))
	}

	fn := module.EntryPoints[0].Function

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
	// Create AST for function with binary operation on literals.
	// The constant evaluator folds 1.0 + 2.0 to Literal(F32(3.0)),
	// matching Rust naga behavior.
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

	// Constant folding: 1.0 + 2.0 is folded to Literal(F32(3.0)).
	// Verify the folded literal has f32 type.
	foundF32Literal := false
	for i, expr := range fn.Expressions {
		if lit, ok := expr.Kind.(ir.Literal); ok {
			if f32Val, isF32 := lit.Value.(ir.LiteralF32); isF32 && float32(f32Val) == 3.0 {
				foundF32Literal = true
				exprType := fn.ExpressionTypes[i]
				if scalar, ok := exprType.Value.(ir.ScalarType); ok {
					if scalar.Kind != ir.ScalarFloat {
						t.Errorf("Folded literal should have f32 type, got %v", scalar.Kind)
					}
				} else {
					t.Errorf("Folded literal should have scalar type, got %T", exprType.Value)
				}
			}
		}
	}

	if !foundF32Literal {
		t.Error("Expected to find folded F32(3.0) literal")
	}
}

func TestLowerer_TypeInference_Comparison(t *testing.T) {
	// Create AST for function with comparison on literals.
	// The constant evaluator folds 1.0 < 2.0 to Literal(Bool(true)),
	// matching Rust naga behavior.
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

	// Constant folding: 1.0 < 2.0 is folded to Literal(Bool(true)).
	// Verify the folded literal has bool type.
	foundBoolLiteral := false
	for i, expr := range fn.Expressions {
		if lit, ok := expr.Kind.(ir.Literal); ok {
			if boolVal, isBool := lit.Value.(ir.LiteralBool); isBool && bool(boolVal) {
				foundBoolLiteral = true
				exprType := fn.ExpressionTypes[i]
				if scalar, ok := exprType.Value.(ir.ScalarType); ok {
					if scalar.Kind != ir.ScalarBool {
						t.Errorf("Folded comparison should have bool type, got %v", scalar.Kind)
					}
				} else {
					t.Errorf("Folded comparison should have scalar type, got %T", exprType.Value)
				}
			}
		}
	}

	if !foundBoolLiteral {
		t.Error("Expected to find folded Bool(true) literal")
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

// TestAbstractConstantsDontRegisterTypes verifies that abstract module-scope constants
// (e.g., `const g0 = 1;`) do NOT register types in the type arena, matching Rust naga
// where abstract constants are stored in the frontend context and never in the module.
func TestAbstractConstantsDontRegisterTypes(t *testing.T) {
	source := `
const g0 = 1;
const g1 = 1u;
const g2 = 1.0;
const g3 = 1.0f;

@compute @workgroup_size(1)
fn main() {
    var g0x = g0;
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	module, err := LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}

	// Abstract constants (g0, g2) should NOT be in module.Constants
	for _, c := range module.Constants {
		if c.Name == "g0" || c.Name == "g2" {
			t.Errorf("abstract constant %q should not be in module.Constants", c.Name)
		}
	}

	// Concrete constants (g1, g3) SHOULD be in module.Constants
	foundG1, foundG3 := false, false
	for _, c := range module.Constants {
		if c.Name == "g1" {
			foundG1 = true
		}
		if c.Name == "g3" {
			foundG3 = true
		}
	}
	if !foundG1 {
		t.Error("concrete constant g1 missing from module.Constants")
	}
	if !foundG3 {
		t.Error("concrete constant g3 missing from module.Constants")
	}

	// Verify type order: Uint should come before Sint
	// (Uint registered by g1 = 1u, Sint registered later by g0x usage)
	uintIdx, sintIdx := -1, -1
	for i, typ := range module.Types {
		if sc, ok := typ.Inner.(ir.ScalarType); ok {
			if sc.Kind == ir.ScalarUint && sc.Width == 4 && uintIdx < 0 {
				uintIdx = i
			}
			if sc.Kind == ir.ScalarSint && sc.Width == 4 && sintIdx < 0 {
				sintIdx = i
			}
		}
	}
	if uintIdx >= 0 && sintIdx >= 0 && uintIdx >= sintIdx {
		t.Errorf("Uint type (idx=%d) should come before Sint type (idx=%d)", uintIdx, sintIdx)
	}
}

// TestAbstractLocalConstsDeferExpression verifies that abstract local const declarations
// create deferred expressions that are re-lowered at use site, matching Rust naga where
// abstract const expressions become dead after concretization and compact removes them.
func TestAbstractLocalConstsDeferExpression(t *testing.T) {
	source := `
@compute @workgroup_size(1)
fn main() {
    const c0 = 1;
    const c1 = 1u;
    var c0x = c0;
    var c1x = c1;
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	module, err := LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}

	// Find the entry point function
	if len(module.EntryPoints) == 0 {
		t.Fatal("no entry points")
	}
	ep := &module.EntryPoints[0].Function

	// After compact, c0x should have a LATER expression index than c1x,
	// because c0 is abstract (expression created at var site) while c1 is
	// concrete (expression created at const site and reused).
	var c0xInit, c1xInit ir.ExpressionHandle
	for _, lv := range ep.LocalVars {
		if lv.Name == "c0x" && lv.Init != nil {
			c0xInit = *lv.Init
		}
		if lv.Name == "c1x" && lv.Init != nil {
			c1xInit = *lv.Init
		}
	}
	// c1x init should be at a lower index than c0x init
	// (concrete const expression created before abstract const is concretized)
	if c1xInit >= c0xInit {
		t.Errorf("c1x init (%d) should be at lower index than c0x init (%d)", c1xInit, c0xInit)
	}
}
