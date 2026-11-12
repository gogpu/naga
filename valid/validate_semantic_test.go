package valid

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// containsError checks if any validation error contains the given substring.
func containsError(errors []Error, substring string) bool {
	for _, e := range errors {
		if strings.Contains(e.Error(), substring) {
			return true
		}
	}
	return false
}

// expectErrors validates the module and asserts at least one error matches.
func expectErrors(t *testing.T, module *ir.Module, expectedSubstrings ...string) {
	t.Helper()
	errors, err := validateErrs(module, CapAll)
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
	module := &ir.Module{
		Types: []ir.Type{
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
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "bad_scalar", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: tt.width}},
				},
			}
			expectErrors(t, module, "scalar width must be 1, 2, 4, or 8")
		})
	}
}

func TestValidateSemantic_VectorInvalidScalarWidth(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name:  "bad_vec",
				Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 3}},
			},
		},
	}
	expectErrors(t, module, "vector scalar width must be 1, 2, 4, or 8")
}

func TestValidateSemantic_MatrixInvalidDimensions(t *testing.T) {
	tests := []struct {
		name   string
		cols   ir.VectorSize
		rows   ir.VectorSize
		errSub string
	}{
		{"invalid columns", ir.VectorSize(5), ir.Vec3, "matrix columns must be 2, 3, or 4"},
		{"invalid rows", ir.Vec4, ir.VectorSize(1), "matrix rows must be 2, 3, or 4"},
		{"both invalid", ir.VectorSize(0), ir.VectorSize(7), "matrix columns must be 2, 3, or 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Types: []ir.Type{
					{
						Name:  "bad_mat",
						Inner: ir.MatrixType{Columns: tt.cols, Rows: tt.rows, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					},
				},
			}
			expectErrors(t, module, tt.errSub)
		})
	}
}

func TestValidateSemantic_ArrayCircularReference(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name:  "self_ref_array",
				Inner: ir.ArrayType{Base: ir.TypeHandle(0), Size: ir.ArraySize{Constant: uint32Ptr(4)}, Stride: 4},
			},
		},
	}
	expectErrors(t, module, "circular reference to itself")
}

func TestValidateSemantic_StructMembers(t *testing.T) {
	t.Run("unnamed member allowed", func(t *testing.T) {
		// An empty name is an unnamed member: the SPIR-V frontend produces
		// them, and each backend names them for itself.
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{
					Name: "unnamed_member_struct",
					Inner: ir.StructType{
						Members: []ir.StructMember{
							{Name: "", Type: ir.TypeHandle(0)},
						},
					},
				},
			},
		}
		if errs, err := validateErrs(module, CapAll); err != nil || len(errs) != 0 {
			t.Errorf("validation errors = %v (%v), want none", errs, err)
		}
	})

	t.Run("duplicate member name", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{
					Name: "dup_struct",
					Inner: ir.StructType{
						Members: []ir.StructMember{
							{Name: "x", Type: ir.TypeHandle(0)},
							{Name: "x", Type: ir.TypeHandle(0)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "duplicate struct member name")
	})

	t.Run("invalid member type", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{
					Name: "bad_member_type",
					Inner: ir.StructType{
						Members: []ir.StructMember{
							{Name: "field", Type: ir.TypeHandle(999)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("circular member reference", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{
					Name: "self_ref_struct",
					Inner: ir.StructType{
						Members: []ir.StructMember{
							{Name: "self", Type: ir.TypeHandle(0)},
						},
					},
				},
			},
		}
		expectErrors(t, module, "circular reference")
	})
}

func TestValidateSemantic_PointerInvalidBase(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{
				Name:  "bad_ptr",
				Inner: ir.PointerType{Base: ir.TypeHandle(100), Space: ir.SpaceFunction},
			},
		},
	}
	expectErrors(t, module, "pointer base type 100 does not exist")
}

// --- Constant validation tests ---

func TestValidateSemantic_ConstantInvalidType(t *testing.T) {
	module := &ir.Module{
		Constants: []ir.Constant{
			{Name: "bad_const", Type: ir.TypeHandle(999)},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

// --- Global variable validation tests ---

func TestValidateSemantic_GlobalVariables(t *testing.T) {
	t.Run("duplicate name", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "gv", Type: ir.TypeHandle(0)},
				{Name: "gv", Type: ir.TypeHandle(0)},
			},
		}
		expectErrors(t, module, "duplicate global variable name")
	})

	t.Run("invalid type", func(t *testing.T) {
		module := &ir.Module{
			GlobalVariables: []ir.GlobalVariable{
				{Name: "bad_gv", Type: ir.TypeHandle(999)},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid init constant", func(t *testing.T) {
		initHandle := ir.ConstantHandle(999)
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "bad_init", Type: ir.TypeHandle(0), Init: &initHandle},
			},
		}
		expectErrors(t, module, "init constant 999 does not exist")
	})
}

// --- Entry point validation tests ---

func TestValidateSemantic_EntryPoints(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "", Stage: ir.StageFragment, Function: ir.Function{Name: "fn"}},
			},
		}
		expectErrors(t, module, "has empty name")
	})

	t.Run("duplicate name", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "main", Stage: ir.StageFragment, Function: ir.Function{Name: "fn0"}},
				{Name: "main", Stage: ir.StageFragment, Function: ir.Function{Name: "fn1"}},
			},
		}
		expectErrors(t, module, "duplicate entry point name")
	})

	t.Run("vertex without result", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "main", Stage: ir.StageVertex, Function: ir.Function{Name: "vs", Result: nil}},
			},
		}
		expectErrors(t, module, "must have a return value")
	})

	t.Run("compute partial zero workgroup", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "main", Stage: ir.StageCompute, Function: ir.Function{Name: "cs"}, Workgroup: [3]uint32{64, 0, 1}},
			},
		}
		expectErrors(t, module, "workgroup size must be non-zero")
	})

	t.Run("vertex with struct position builtin", func(t *testing.T) {
		// This is a POSITIVE test: struct member has @builtin(position), should pass.
		posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				{
					Name: "VertexOutput",
					Inner: ir.StructType{
						Members: []ir.StructMember{
							{Name: "position", Type: ir.TypeHandle(1), Binding: &posBinding},
						},
					},
				},
			},
			EntryPoints: []ir.EntryPoint{
				{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
					Name:   "vs",
					Result: &ir.FunctionResult{Type: ir.TypeHandle(2)},
				}},
			},
		}
		errors, err := validateErrs(module, CapAll)
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Arguments: []ir.FunctionArgument{
					{Name: "arg0", Type: ir.TypeHandle(999)},
				},
			},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

func TestValidateSemantic_FunctionResultInvalidType(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name:   "fn",
				Result: &ir.FunctionResult{Type: ir.TypeHandle(999)},
			},
		},
	}
	expectErrors(t, module, "result type 999 does not exist")
}

func TestValidateSemantic_LocalVariables(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					LocalVars: []ir.LocalVariable{
						{Name: "v", Type: ir.TypeHandle(999)},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid init expression", func(t *testing.T) {
		initExpr := ir.ExpressionHandle(999)
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			Functions: []ir.Function{
				{
					Name: "fn",
					LocalVars: []ir.LocalVariable{
						{Name: "v", Type: ir.TypeHandle(0), Init: &initExpr},
					},
				},
			},
		}
		expectErrors(t, module, "init expression 999 does not exist")
	})
}

// --- Expression validation tests ---

func TestValidateSemantic_ExpressionNilKind(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name:        "fn",
				Expressions: []ir.Expression{{Kind: nil}},
				Body:        []ir.Statement{},
			},
		},
	}
	expectErrors(t, module, "expression has nil kind")
}

func TestValidateSemantic_ExprConstantInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprConstant{Constant: ir.ConstantHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "constant 999 does not exist")
}

func TestValidateSemantic_ExprZeroValueInvalidType(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: ir.TypeHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "type 999 does not exist")
}

func TestValidateSemantic_ExprComposeInvalid(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.ExprCompose{Type: ir.TypeHandle(999), Components: nil}},
					},
				},
			},
		}
		expectErrors(t, module, "type 999 does not exist")
	})

	t.Run("invalid component", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "vec2f", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			},
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprCompose{Type: ir.TypeHandle(0), Components: []ir.ExpressionHandle{0, ir.ExpressionHandle(999)}}},
					},
				},
			},
		}
		expectErrors(t, module, "expression 999 does not exist")
	})
}

func TestValidateSemantic_ExprAccessInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprAccess{Base: ir.ExpressionHandle(999), Index: ir.ExpressionHandle(998)}},
				},
			},
		},
	}
	expectErrors(t, module, "base expression 999 does not exist", "index expression 998 does not exist")
}

func TestValidateSemantic_ExprAccessIndexInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprAccessIndex{Base: ir.ExpressionHandle(999), Index: 0}},
				},
			},
		},
	}
	expectErrors(t, module, "base expression 999 does not exist")
}

func TestValidateSemantic_ExprSplatInvalid(t *testing.T) {
	t.Run("invalid size", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprSplat{Size: ir.VectorSize(5), Value: ir.ExpressionHandle(0)}},
					},
				},
			},
		}
		expectErrors(t, module, "splat size must be 2, 3, or 4")
	})

	t.Run("invalid value expression", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.ExprSplat{Size: ir.Vec3, Value: ir.ExpressionHandle(999)}},
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
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprSwizzle{
							Size:    ir.VectorSize(1),
							Vector:  ir.ExpressionHandle(0),
							Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, 0, 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "swizzle size must be 2, 3, or 4")
	})

	t.Run("invalid vector", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.ExprSwizzle{
							Size:    ir.Vec2,
							Vector:  ir.ExpressionHandle(999),
							Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "vector expression 999 does not exist")
	})

	t.Run("invalid pattern component", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprSwizzle{
							Size:    ir.Vec2,
							Vector:  ir.ExpressionHandle(0),
							Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleComponent(10), 0, 0},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "invalid component")
	})
}

func TestValidateSemantic_ExprFunctionArgOutOfRange(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name:      "fn",
				Arguments: []ir.FunctionArgument{}, // no arguments
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 5}},
				},
			},
		},
	}
	expectErrors(t, module, "argument index 5 out of range")
}

func TestValidateSemantic_ExprGlobalVariableInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "global variable 999 does not exist")
}

func TestValidateSemantic_ExprLocalVariableOutOfRange(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name:      "fn",
				LocalVars: []ir.LocalVariable{}, // no local vars
				Expressions: []ir.Expression{
					{Kind: ir.ExprLocalVariable{Variable: 10}},
				},
			},
		},
	}
	expectErrors(t, module, "local variable index 10 out of range")
}

func TestValidateSemantic_ExprLoadInvalidPointer(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "pointer expression 999 does not exist")
}

func TestValidateSemantic_ExprBinaryInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: ir.ExpressionHandle(999), Right: ir.ExpressionHandle(998)}},
				},
			},
		},
	}
	expectErrors(t, module, "left expression 999 does not exist", "right expression 998 does not exist")
}

func TestValidateSemantic_ExprUnaryInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprUnary{Op: ir.UnaryNegate, Expr: ir.ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "operand expression 999 does not exist")
}

func TestValidateSemantic_ExprSelectInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprSelect{
						Condition: ir.ExpressionHandle(999),
						Accept:    ir.ExpressionHandle(998),
						Reject:    ir.ExpressionHandle(997),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: ir.FunctionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "function 999 does not exist")
}

func TestValidateSemantic_ExprArrayLengthInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprArrayLength{Array: ir.ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "array expression 999 does not exist")
}

func TestValidateSemantic_ExprDerivativeInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine, Expr: ir.ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "expression 999 does not exist")
}

func TestValidateSemantic_ExprRelationalInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprRelational{Fun: ir.RelationalIsNan, Argument: ir.ExpressionHandle(999)}},
				},
			},
		},
	}
	expectErrors(t, module, "argument expression 999 does not exist")
}

func TestValidateSemantic_ExprMathInvalid(t *testing.T) {
	arg1 := ir.ExpressionHandle(998)
	arg2 := ir.ExpressionHandle(997)
	arg3 := ir.ExpressionHandle(996)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprMath{
						Fun:  ir.MathClamp,
						Arg:  ir.ExpressionHandle(999),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprAs{Expr: ir.ExpressionHandle(999), Kind: ir.ScalarFloat}},
				},
			},
		},
	}
	expectErrors(t, module, "expression 999 does not exist")
}

// --- Statement validation tests ---

func TestValidateSemantic_StatementNilKind(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{{Kind: nil}},
			},
		},
	}
	expectErrors(t, module, "statement has nil kind")
}

func TestValidateSemantic_ContinueOutsideLoop(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtContinue{}},
				},
			},
		},
	}
	expectErrors(t, module, "`continue` is used outside of a `loop`")
}

func TestValidateSemantic_BreakInContinuingBlock(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{},
						Continuing: []ir.Statement{
							{Kind: ir.StmtBreak{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "`break` is used outside of a `loop` or `switch`")
}

func TestValidateSemantic_ContinueInContinuingBlock(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{},
						Continuing: []ir.Statement{
							{Kind: ir.StmtContinue{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "`continue` is used outside of a `loop`")
}

func TestValidateSemantic_ReturnInContinuingBlock(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{},
						Continuing: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "`return` is called within a `continuing` block")
}

// TestValidateSemantic_KillInContinuingBlock: a discard may stand in a
// continuing block; Rust naga's validator checks only its stage (valid/
// function.rs, S::Kill), and so do we.
func TestValidateSemantic_KillInContinuingBlock(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{},
						Continuing: []ir.Statement{
							{Kind: ir.StmtKill{}},
						},
					}},
				},
			},
		},
	}
	expectNoValidationErrors(t, module)
}

func TestValidateSemantic_EmitRangeInvalid(t *testing.T) {
	t.Run("start out of range", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name:        "fn",
					Expressions: []ir.Expression{},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 6}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range start 5 out of range")
	})

	t.Run("end out of range", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 100}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range end 100 out of range")
	})

	t.Run("start >= end", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 1}}},
					},
				},
			},
		}
		expectErrors(t, module, "emit range start 2 >= end 1")
	})
}

func TestValidateSemantic_StmtIfInvalidCondition(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtIf{
						Condition: ir.ExpressionHandle(999),
						Accept:    ir.Block{},
						Reject:    ir.Block{},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "condition expression 999 does not exist")
}

func TestValidateSemantic_StmtStoreInvalid(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtStore{
						Pointer: ir.ExpressionHandle(999),
						Value:   ir.ExpressionHandle(998),
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
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "fn",
					Body: []ir.Statement{
						{Kind: ir.StmtCall{Function: ir.FunctionHandle(999), Arguments: nil}},
					},
				},
			},
		}
		expectErrors(t, module, "function 999 does not exist")
	})

	t.Run("invalid argument expression", func(t *testing.T) {
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "callee",
				},
				{
					Name: "fn",
					Body: []ir.Statement{
						{Kind: ir.StmtCall{
							Function:  ir.FunctionHandle(0),
							Arguments: []ir.ExpressionHandle{ir.ExpressionHandle(999)},
						}},
					},
				},
			},
		}
		expectErrors(t, module, "expression 999 does not exist")
	})

	t.Run("invalid result expression", func(t *testing.T) {
		resultExpr := ir.ExpressionHandle(999)
		module := &ir.Module{
			Functions: []ir.Function{
				{
					Name: "callee",
				},
				{
					Name: "fn",
					Body: []ir.Statement{
						{Kind: ir.StmtCall{
							Function: ir.FunctionHandle(0),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: ir.ExpressionHandle(0),
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
							{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "multiple default cases")
}

func TestValidateSemantic_StmtSwitchInvalidSelector(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: ir.ExpressionHandle(999),
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
						},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "selector expression 999 does not exist")
}

func TestValidateSemantic_LoopBreakIfInvalid(t *testing.T) {
	breakIfExpr := ir.ExpressionHandle(999)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body:       ir.Block{},
						Continuing: ir.Block{},
						BreakIf:    &breakIfExpr,
					}},
				},
			},
		},
	}
	expectErrors(t, module, "break-if expression 999 does not exist")
}

func TestValidateSemantic_StmtImageStoreInvalid(t *testing.T) {
	arrayIdx := ir.ExpressionHandle(996)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtImageStore{
						Image:      ir.ExpressionHandle(999),
						Coordinate: ir.ExpressionHandle(998),
						ArrayIndex: &arrayIdx,
						Value:      ir.ExpressionHandle(997),
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
	resultExpr := ir.ExpressionHandle(996)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtAtomic{
						Pointer: ir.ExpressionHandle(999),
						Fun:     ir.AtomicAdd{},
						Value:   ir.ExpressionHandle(998),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtWorkGroupUniformLoad{
						Pointer: ir.ExpressionHandle(999),
						Result:  ir.ExpressionHandle(998),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtRayQuery{
						Query: ir.ExpressionHandle(999),
						Fun:   ir.RayQueryTerminate{},
					}},
				},
			},
		},
	}
	expectErrors(t, module, "query expression 999 does not exist")
}

func TestValidateSemantic_ReturnInvalidValue(t *testing.T) {
	retVal := ir.ExpressionHandle(999)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retVal}},
				},
			},
		},
	}
	expectErrors(t, module, "return value expression 999 does not exist")
}

// --- Image expression validation tests ---

func TestValidateSemantic_ExprImageSampleInvalid(t *testing.T) {
	arrIdx := ir.ExpressionHandle(996)
	offset := ir.ExpressionHandle(995)
	depthRef := ir.ExpressionHandle(994)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprImageSample{
						Image:      ir.ExpressionHandle(999),
						Sampler:    ir.ExpressionHandle(998),
						Coordinate: ir.ExpressionHandle(997),
						ArrayIndex: &arrIdx,
						Offset:     &offset,
						Level:      ir.SampleLevelAuto{},
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
	arrIdx := ir.ExpressionHandle(996)
	sample := ir.ExpressionHandle(995)
	level := ir.ExpressionHandle(994)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprImageLoad{
						Image:      ir.ExpressionHandle(999),
						Coordinate: ir.ExpressionHandle(998),
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
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprImageQuery{
						Image: ir.ExpressionHandle(999),
						Query: ir.ImageQuerySize{},
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
		module := &ir.Module{
			Types: []ir.Type{
				{Name: "s", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: width}},
			},
		}
		errors, err := validateErrs(module, CapAll)
		if err != nil {
			t.Fatalf("width %d: Validate returned error: %v", width, err)
		}
		if len(errors) > 0 {
			t.Errorf("width %d: expected no errors, got: %v", width, errors)
		}
	}
}

func TestValidateSemantic_ValidBreakInsideLoop(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtBreak{}},
						},
						Continuing: ir.Block{},
					}},
				},
			},
		},
	}
	errors, err := validateErrs(module, CapAll)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for break inside loop, got: %v", errors)
	}
}

func TestValidateSemantic_ValidContinueInsideLoop(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtContinue{}},
						},
						Continuing: ir.Block{},
					}},
				},
			},
		},
	}
	errors, err := validateErrs(module, CapAll)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for continue inside loop, got: %v", errors)
	}
}

func TestValidateSemantic_NestedLoopBreakContinue(t *testing.T) {
	// break/continue in inner loop should be valid; outer loop context is irrelevant.
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtLoop{
								Body: []ir.Statement{
									{Kind: ir.StmtBreak{}},
									{Kind: ir.StmtContinue{}},
								},
								Continuing: ir.Block{},
							}},
						},
						Continuing: ir.Block{},
					}},
				},
			},
		},
	}
	errors, err := validateErrs(module, CapAll)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for nested loop break/continue, got: %v", errors)
	}
}

func TestValidateSemantic_ValidComputeWorkgroup(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageCompute, Function: ir.Function{Name: "cs"}, Workgroup: [3]uint32{64, 1, 1}},
		},
	}
	errors, err := validateErrs(module, CapAll)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors for valid compute workgroup, got: %v", errors)
	}
}
