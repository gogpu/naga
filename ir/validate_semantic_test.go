package ir

import (
	"strings"
	"testing"
)

// containsError checks if any validation error contains the given substring.
func containsError(errors []ValidationError, substring string) bool {
	for _, e := range errors {
		if strings.Contains(e.Error(), substring) {
			return true
		}
	}
	return false
}

// expectErrors validates the module and asserts at least one error matches.
func expectErrors(t *testing.T, module *Module, expectedSubstrings ...string) {
	t.Helper()
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Fatal("expected validation errors, got none")
	}
	for _, sub := range expectedSubstrings {
		if !containsError(errors, sub) {
			t.Errorf("expected error containing %q, got errors: %v", sub, errors)
		}
	}
}

// --- Type validation tests ---

func TestValidateSemantic_TypeNilInner(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "broken", Inner: nil},
		},
	}
	expectErrors(t, module, "has nil inner type")
}

func TestValidateSemantic_ScalarInvalidWidth(t *testing.T) {
	tests := []struct {
		name  string
		width uint8
	}{
		{"width 0", 0},
		{"width 3", 3},
		{"width 5", 5},
		{"width 7", 7},
		{"width 16", 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &Module{
				Types: []Type{
					{Name: "bad_scalar", Inner: ScalarType{Kind: ScalarFloat, Width: tt.width}},
				},
			}
			expectErrors(t, module, "scalar width must be 1, 2, 4, or 8")
		})
	}
}

func TestValidateSemantic_VectorInvalidScalarWidth(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "bad_vec",
				Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 3}},
			},
		},
	}
	expectErrors(t, module, "vector scalar width must be 1, 2, 4, or 8")
}

func TestValidateSemantic_MatrixInvalidDimensions(t *testing.T) {
	tests := []struct {
		name   string
		cols   VectorSize
		rows   VectorSize
		errSub string
	}{
		{"invalid columns", VectorSize(5), Vec3, "matrix columns must be 2, 3, or 4"},
		{"invalid rows", Vec4, VectorSize(1), "matrix rows must be 2, 3, or 4"},
		{"both invalid", VectorSize(0), VectorSize(7), "matrix columns must be 2, 3, or 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &Module{
				Types: []Type{
					{
						Name:  "bad_mat",
						Inner: MatrixType{Columns: tt.cols, Rows: tt.rows, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
					},
				},
			}
			expectErrors(t, module, tt.errSub)
		})
	}
}

func TestValidateSemantic_ArrayCircularReference(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "self_ref_array",
				Inner: ArrayType{Base: TypeHandle(0), Size: ArraySize{Constant: uint32Ptr(4)}, Stride: 4},
			},
		},
	}
	expectErrors(t, module, "circular reference to itself")
}

func TestValidateSemantic_StructMembers(t *testing.T) {
	t.Run("empty member name", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
				{
					Name: "bad_struct",
					Inner: StructType{
						Members: []StructMember{
							{Name: "", Type: TypeHandle(0)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "has empty name")
	})

	t.Run("duplicate member name", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
				{
					Name: "dup_struct",
					Inner: StructType{
						Members: []StructMember{
							{Name: "x", Type: TypeHandle(0)},
							{Name: "x", Type: TypeHandle(0)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "duplicate struct member name")
	})

	t.Run("invalid member type", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{
					Name: "bad_member_type",
					Inner: StructType{
						Members: []StructMember{
							{Name: "field", Type: TypeHandle(999)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("circular member reference", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{
					Name: "self_ref_struct",
					Inner: StructType{
						Members: []StructMember{
							{Name: "self", Type: TypeHandle(0)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "circular reference")
	})
}

func TestValidateSemantic_PointerInvalidBase(t *testing.T) {
	module := &Module{
		Types: []Type{
			{
				Name:  "bad_ptr",
				Inner: PointerType{Base: TypeHandle(100), Space: SpaceFunction},
			},
		},
	}
	expectErrors(t, module, "pointer base type 100 does not exist")
}

// --- Constant validation tests ---

func TestValidateSemantic_ConstantInvalidType(t *testing.T) {
	module := &Module{
		Constants: []Constant{
			{Name: "bad_const", Type: TypeHandle(999)},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

// --- Global variable validation tests ---

func TestValidateSemantic_GlobalVariables(t *testing.T) {
	t.Run("duplicate name", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
			GlobalVariables: []GlobalVariable{
				{Name: "gv", Type: TypeHandle(0)},
				{Name: "gv", Type: TypeHandle(0)},
			},
		}
		expectErrors(t, module, "duplicate global variable name")
	})

	t.Run("invalid type", func(t *testing.T) {
		module := &Module{
			GlobalVariables: []GlobalVariable{
				{Name: "bad_gv", Type: TypeHandle(999)},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid init constant", func(t *testing.T) {
		initHandle := ConstantHandle(999)
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
			GlobalVariables: []GlobalVariable{
				{Name: "bad_init", Type: TypeHandle(0), Init: &initHandle},
			},
		}
		expectErrors(t, module, "init constant 999 does not exist")
	})
}

// --- Entry point validation tests ---

func TestValidateSemantic_EntryPoints(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{Name: "fn"},
			},
			EntryPoints: []EntryPoint{
				{Name: "", Stage: StageFragment, Function: FunctionHandle(0)},
			},
		}
		expectErrors(t, module, "has empty name")
	})

	t.Run("duplicate name", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{Name: "fn0"},
				{Name: "fn1"},
			},
			EntryPoints: []EntryPoint{
				{Name: "main", Stage: StageFragment, Function: FunctionHandle(0)},
				{Name: "main", Stage: StageFragment, Function: FunctionHandle(1)},
			},
		}
		expectErrors(t, module, "duplicate entry point name")
	})

	t.Run("invalid function handle", func(t *testing.T) {
		module := &Module{
			EntryPoints: []EntryPoint{
				{Name: "main", Stage: StageFragment, Function: FunctionHandle(999)},
			},
		}
		expectErrors(t, module, "function 999 does not exist")
	})

	t.Run("vertex without result", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{Name: "vs", Result: nil},
			},
			EntryPoints: []EntryPoint{
				{Name: "main", Stage: StageVertex, Function: FunctionHandle(0)},
			},
		}
		expectErrors(t, module, "must have a return value")
	})

	t.Run("compute partial zero workgroup", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{Name: "cs"},
			},
			EntryPoints: []EntryPoint{
				{Name: "main", Stage: StageCompute, Function: FunctionHandle(0), Workgroup: [3]uint32{64, 0, 1}},
			},
		}
		expectErrors(t, module, "workgroup size must be non-zero")
	})

	t.Run("vertex with struct position builtin", func(t *testing.T) {
		// This is a POSITIVE test: struct member has @builtin(position), should pass.
		posBinding := Binding(BuiltinBinding{Builtin: BuiltinPosition})
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
				{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
				{
					Name: "VertexOutput",
					Inner: StructType{
						Members: []StructMember{
							{Name: "position", Type: TypeHandle(1), Binding: &posBinding},
						},
					},
				},
			},
			Functions: []Function{
				{
					Name:   "vs",
					Result: &FunctionResult{Type: TypeHandle(2)},
				},
			},
			EntryPoints: []EntryPoint{
				{Name: "main", Stage: StageVertex, Function: FunctionHandle(0)},
			},
		}
		errors, err := Validate(module)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if len(errors) > 0 {
			t.Errorf("expected no errors for vertex with struct position builtin, got: %v", errors)
		}
	})
}

// --- Function validation tests ---

func TestValidateSemantic_FunctionArgInvalidType(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Arguments: []FunctionArgument{
					{Name: "arg0", Type: TypeHandle(999)},
				},
			},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

func TestValidateSemantic_FunctionResultInvalidType(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name:   "fn",
				Result: &FunctionResult{Type: TypeHandle(999)},
			},
		},
	}
	expectErrors(t, module, "result type 999 does not exist")
}

func TestValidateSemantic_LocalVariables(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					LocalVars: []LocalVariable{
						{Name: "v", Type: TypeHandle(999)},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid init expression", func(t *testing.T) {
		initExpr := ExpressionHandle(999)
		module := &Module{
			Types: []Type{
				{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			},
			Functions: []Function{
				{
					Name: "fn",
					LocalVars: []LocalVariable{
						{Name: "v", Type: TypeHandle(0), Init: &initExpr},
					},
				},
			},
		}
		expectErrors(t, module, "init expression 999 does not exist")
	})
}

// --- Expression validation tests ---

func TestValidateSemantic_ExpressionNilKind(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name:        "fn",
				Expressions: []Expression{{Kind: nil}},
				Body:        []Statement{},
			},
		},
	}
	expectErrors(t, module, "expression has nil kind")
}

func TestValidateSemantic_ExprConstantInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprConstant{Constant: ConstantHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "constant 999 does not exist")
}

func TestValidateSemantic_ExprZeroValueInvalidType(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprZeroValue{Type: TypeHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

func TestValidateSemantic_ExprComposeInvalid(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: ExprCompose{Type: TypeHandle(999), Components: nil}},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid component", func(t *testing.T) {
		module := &Module{
			Types: []Type{
				{Name: "vec2f", Inner: VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
			},
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
						{Kind: ExprCompose{Type: TypeHandle(0), Components: []ExpressionHandle{0, ExpressionHandle(999)}}},
					},
				},
			},
		}
		expectErrors(t, module, "expression 999 does not exist")
	})
}

func TestValidateSemantic_ExprAccessInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprAccess{Base: ExpressionHandle(999), Index: ExpressionHandle(998)}},
				},
			},
		},
	}
	expectErrors(t, module, "base expression 999 does not exist", "index expression 998 does not exist")
}

func TestValidateSemantic_ExprAccessIndexInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprAccessIndex{Base: ExpressionHandle(999), Index: 0}},
				},
			},
		},
	}
	expectErrors(t, module, "base expression 999 does not exist")
}

func TestValidateSemantic_ExprSplatInvalid(t *testing.T) {
	t.Run("invalid size", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
						{Kind: ExprSplat{Size: VectorSize(5), Value: ExpressionHandle(0)}},
					},
				},
			},
		}
		expectErrors(t, module, "splat size must be 2, 3, or 4")
	})

	t.Run("invalid value expression", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: ExprSplat{Size: Vec3, Value: ExpressionHandle(999)}},
					},
				},
			},
		}
		expectErrors(t, module, "value expression 999 does not exist")
	})
}

func TestValidateSemantic_ExprSwizzleInvalid(t *testing.T) {
	t.Run("invalid size", func(t *testing.T) {
		// Use size 1 (invalid but <= 4) to avoid index-out-of-bounds in Pattern[4] array.
		// Size > 4 would panic in the validator's pattern loop before returning the error.
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
						{Kind: ExprSwizzle{
							Size:    VectorSize(1),
							Vector:  ExpressionHandle(0),
							Pattern: [4]SwizzleComponent{SwizzleX, 0, 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "swizzle size must be 2, 3, or 4")
	})

	t.Run("invalid vector", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: ExprSwizzle{
							Size:    Vec2,
							Vector:  ExpressionHandle(999),
							Pattern: [4]SwizzleComponent{SwizzleX, SwizzleY, 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "vector expression 999 does not exist")
	})

	t.Run("invalid pattern component", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
						{Kind: ExprSwizzle{
							Size:    Vec2,
							Vector:  ExpressionHandle(0),
							Pattern: [4]SwizzleComponent{SwizzleX, SwizzleComponent(10), 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "invalid component")
	})
}

func TestValidateSemantic_ExprFunctionArgOutOfRange(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name:      "fn",
				Arguments: []FunctionArgument{}, // no arguments
				Expressions: []Expression{
					{Kind: ExprFunctionArgument{Index: 5}},
				},
			},
		},
	}
	expectErrors(t, module, "argument index 5 out of range")
}

func TestValidateSemantic_ExprGlobalVariableInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprGlobalVariable{Variable: GlobalVariableHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "global variable 999 does not exist")
}

func TestValidateSemantic_ExprLocalVariableOutOfRange(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name:      "fn",
				LocalVars: []LocalVariable{}, // no local vars
				Expressions: []Expression{
					{Kind: ExprLocalVariable{Variable: 10}},
				},
			},
		},
	}
	expectErrors(t, module, "local variable index 10 out of range")
}

func TestValidateSemantic_ExprLoadInvalidPointer(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprLoad{Pointer: ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "pointer expression 999 does not exist")
}

func TestValidateSemantic_ExprBinaryInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprBinary{Op: BinaryAdd, Left: ExpressionHandle(999), Right: ExpressionHandle(998)}},
				},
			},
		},
	}
	expectErrors(t, module, "left expression 999 does not exist", "right expression 998 does not exist")
}

func TestValidateSemantic_ExprUnaryInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprUnary{Op: UnaryNegate, Expr: ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "operand expression 999 does not exist")
}

func TestValidateSemantic_ExprSelectInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprSelect{
						Condition: ExpressionHandle(999),
						Accept:    ExpressionHandle(998),
						Reject:    ExpressionHandle(997),
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"condition expression 999 does not exist",
		"accept expression 998 does not exist",
		"reject expression 997 does not exist",
	)
}

func TestValidateSemantic_ExprCallResultInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprCallResult{Function: FunctionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "function 999 does not exist")
}

func TestValidateSemantic_ExprArrayLengthInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprArrayLength{Array: ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "array expression 999 does not exist")
}

func TestValidateSemantic_ExprDerivativeInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprDerivative{Axis: DerivativeX, Control: DerivativeFine, Expr: ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "expression 999 does not exist")
}

func TestValidateSemantic_ExprRelationalInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprRelational{Fun: RelationalIsNan, Argument: ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "argument expression 999 does not exist")
}

func TestValidateSemantic_ExprMathInvalid(t *testing.T) {
	arg1 := ExpressionHandle(998)
	arg2 := ExpressionHandle(997)
	arg3 := ExpressionHandle(996)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprMath{
						Fun:  MathClamp,
						Arg:  ExpressionHandle(999),
						Arg1: &arg1,
						Arg2: &arg2,
						Arg3: &arg3,
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"arg expression 999 does not exist",
		"arg1 expression 998 does not exist",
		"arg2 expression 997 does not exist",
		"arg3 expression 996 does not exist",
	)
}

func TestValidateSemantic_ExprAsInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprAs{Expr: ExpressionHandle(999), Kind: ScalarFloat}},
				},
			},
		},
	}
	expectErrors(t, module, "expression 999 does not exist")
}

// --- Statement validation tests ---

func TestValidateSemantic_StatementNilKind(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{{Kind: nil}},
			},
		},
	}
	expectErrors(t, module, "statement has nil kind")
}

func TestValidateSemantic_ContinueOutsideLoop(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtContinue{}},
				},
			},
		},
	}
	expectErrors(t, module, "continue outside of loop")
}

func TestValidateSemantic_BreakInContinuingBlock(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 1}},
		},
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{},
						Continuing: []Statement{
							{Kind: StmtBreak{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "break in continuing block")
}

func TestValidateSemantic_ContinueInContinuingBlock(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{},
						Continuing: []Statement{
							{Kind: StmtContinue{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "continue in continuing block")
}

func TestValidateSemantic_ReturnInContinuingBlock(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{},
						Continuing: []Statement{
							{Kind: StmtReturn{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "return in continuing block")
}

func TestValidateSemantic_KillInContinuingBlock(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{},
						Continuing: []Statement{
							{Kind: StmtKill{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "kill in continuing block")
}

func TestValidateSemantic_EmitRangeInvalid(t *testing.T) {
	t.Run("start out of range", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name:        "fn",
					Expressions: []Expression{},
					Body: []Statement{
						{Kind: StmtEmit{Range: Range{Start: 5, End: 6}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range start 5 out of range")
	})

	t.Run("end out of range", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
					},
					Body: []Statement{
						{Kind: StmtEmit{Range: Range{Start: 0, End: 100}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range end 100 out of range")
	})

	t.Run("start >= end", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
						{Kind: Literal{Value: LiteralF32(2.0)}},
						{Kind: Literal{Value: LiteralF32(3.0)}},
					},
					Body: []Statement{
						{Kind: StmtEmit{Range: Range{Start: 2, End: 1}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range start 2 >= end 1")
	})
}

func TestValidateSemantic_StmtIfInvalidCondition(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtIf{
						Condition: ExpressionHandle(999),
						Accept:    Block{},
						Reject:    Block{},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "condition expression 999 does not exist")
}

func TestValidateSemantic_StmtStoreInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtStore{
						Pointer: ExpressionHandle(999),
						Value:   ExpressionHandle(998),
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"pointer expression 999 does not exist",
		"value expression 998 does not exist",
	)
}

func TestValidateSemantic_StmtCallInvalid(t *testing.T) {
	t.Run("invalid function", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "fn",
					Body: []Statement{
						{Kind: StmtCall{Function: FunctionHandle(999), Arguments: nil}},
					},
				},
			},
		}
		expectErrors(t, module, "function 999 does not exist")
	})

	t.Run("invalid argument expression", func(t *testing.T) {
		module := &Module{
			Functions: []Function{
				{
					Name: "callee",
				},
				{
					Name: "fn",
					Body: []Statement{
						{Kind: StmtCall{
							Function:  FunctionHandle(0),
							Arguments: []ExpressionHandle{ExpressionHandle(999)},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "expression 999 does not exist")
	})

	t.Run("invalid result expression", func(t *testing.T) {
		resultExpr := ExpressionHandle(999)
		module := &Module{
			Functions: []Function{
				{
					Name: "callee",
				},
				{
					Name: "fn",
					Body: []Statement{
						{Kind: StmtCall{
							Function: FunctionHandle(0),
							Result:   &resultExpr,
						}},
					},
				},
			},
		}
		expectErrors(t, module, "result expression 999 does not exist")
	})
}

func TestValidateSemantic_SwitchMultipleDefaults(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralI32(1)}},
				},
				Body: []Statement{
					{Kind: StmtSwitch{
						Selector: ExpressionHandle(0),
						Cases: []SwitchCase{
							{Value: SwitchValueDefault{}, Body: Block{}},
							{Value: SwitchValueDefault{}, Body: Block{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "multiple default cases")
}

func TestValidateSemantic_StmtSwitchInvalidSelector(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtSwitch{
						Selector: ExpressionHandle(999),
						Cases: []SwitchCase{
							{Value: SwitchValueDefault{}, Body: Block{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "selector expression 999 does not exist")
}

func TestValidateSemantic_LoopBreakIfInvalid(t *testing.T) {
	breakIfExpr := ExpressionHandle(999)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body:       Block{},
						Continuing: Block{},
						BreakIf:    &breakIfExpr,
					}},
				},
			},
		},
	}
	expectErrors(t, module, "break-if expression 999 does not exist")
}

func TestValidateSemantic_StmtImageStoreInvalid(t *testing.T) {
	arrayIdx := ExpressionHandle(996)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtImageStore{
						Image:      ExpressionHandle(999),
						Coordinate: ExpressionHandle(998),
						ArrayIndex: &arrayIdx,
						Value:      ExpressionHandle(997),
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"image expression 999 does not exist",
		"coordinate expression 998 does not exist",
		"array index expression 996 does not exist",
		"value expression 997 does not exist",
	)
}

func TestValidateSemantic_StmtAtomicInvalid(t *testing.T) {
	resultExpr := ExpressionHandle(996)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtAtomic{
						Pointer: ExpressionHandle(999),
						Fun:     AtomicAdd{},
						Value:   ExpressionHandle(998),
						Result:  &resultExpr,
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"pointer expression 999 does not exist",
		"value expression 998 does not exist",
		"result expression 996 does not exist",
	)
}

func TestValidateSemantic_StmtWorkGroupUniformLoadInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtWorkGroupUniformLoad{
						Pointer: ExpressionHandle(999),
						Result:  ExpressionHandle(998),
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"pointer expression 999 does not exist",
		"result expression 998 does not exist",
	)
}

func TestValidateSemantic_StmtRayQueryInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtRayQuery{
						Query: ExpressionHandle(999),
						Fun:   RayQueryTerminate{},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "query expression 999 does not exist")
}

func TestValidateSemantic_ReturnInvalidValue(t *testing.T) {
	retVal := ExpressionHandle(999)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtReturn{Value: &retVal}},
				},
			},
		},
	}
	expectErrors(t, module, "return value expression 999 does not exist")
}

// --- Image expression validation tests ---

func TestValidateSemantic_ExprImageSampleInvalid(t *testing.T) {
	arrIdx := ExpressionHandle(996)
	offset := ExpressionHandle(995)
	depthRef := ExpressionHandle(994)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprImageSample{
						Image:      ExpressionHandle(999),
						Sampler:    ExpressionHandle(998),
						Coordinate: ExpressionHandle(997),
						ArrayIndex: &arrIdx,
						Offset:     &offset,
						Level:      SampleLevelAuto{},
						DepthRef:   &depthRef,
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"image expression 999 does not exist",
		"sampler expression 998 does not exist",
		"coordinate expression 997 does not exist",
		"array index expression 996 does not exist",
		"offset expression 995 does not exist",
		"depth ref expression 994 does not exist",
	)
}

func TestValidateSemantic_ExprImageLoadInvalid(t *testing.T) {
	arrIdx := ExpressionHandle(996)
	sample := ExpressionHandle(995)
	level := ExpressionHandle(994)
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprImageLoad{
						Image:      ExpressionHandle(999),
						Coordinate: ExpressionHandle(998),
						ArrayIndex: &arrIdx,
						Sample:     &sample,
						Level:      &level,
					}},
				},
			},
		},
	}
	expectErrors(t, module,
		"image expression 999 does not exist",
		"coordinate expression 998 does not exist",
		"array index expression 996 does not exist",
		"sample expression 995 does not exist",
		"level expression 994 does not exist",
	)
}

func TestValidateSemantic_ExprImageQueryInvalid(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Expressions: []Expression{
					{Kind: ExprImageQuery{
						Image: ExpressionHandle(999),
						Query: ImageQuerySize{},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "image expression 999 does not exist")
}

// --- Positive tests for edge cases ---

func TestValidateSemantic_ValidScalarWidths(t *testing.T) {
	for _, width := range []uint8{1, 2, 4, 8} {
		module := &Module{
			Types: []Type{
				{Name: "s", Inner: ScalarType{Kind: ScalarFloat, Width: width}},
			},
		}
		errors, err := Validate(module)
		if err != nil {
			t.Fatalf("width %d: Validate returned error: %v", width, err)
		}
		if len(errors) > 0 {
			t.Errorf("width %d: expected no errors, got: %v", width, errors)
		}
	}
}

func TestValidateSemantic_ValidBreakInsideLoop(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{
							{Kind: StmtBreak{}},
						},
						Continuing: Block{},
					}},
				},
			},
		},
	}
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for break inside loop, got: %v", errors)
	}
}

func TestValidateSemantic_ValidContinueInsideLoop(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{
							{Kind: StmtContinue{}},
						},
						Continuing: Block{},
					}},
				},
			},
		},
	}
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for continue inside loop, got: %v", errors)
	}
}

func TestValidateSemantic_NestedLoopBreakContinue(t *testing.T) {
	// break/continue in inner loop should be valid; outer loop context is irrelevant.
	module := &Module{
		Functions: []Function{
			{
				Name: "fn",
				Body: []Statement{
					{Kind: StmtLoop{
						Body: []Statement{
							{Kind: StmtLoop{
								Body: []Statement{
									{Kind: StmtBreak{}},
									{Kind: StmtContinue{}},
								},
								Continuing: Block{},
							}},
						},
						Continuing: Block{},
					}},
				},
			},
		},
	}
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for nested loop break/continue, got: %v", errors)
	}
}

func TestValidateSemantic_ValidComputeWorkgroup(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "cs"},
		},
		EntryPoints: []EntryPoint{
			{Name: "main", Stage: StageCompute, Function: FunctionHandle(0), Workgroup: [3]uint32{64, 1, 1}},
		},
	}
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for valid compute workgroup, got: %v", errors)
	}
}
