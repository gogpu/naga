package msl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helpers
// =============================================================================

// compileModule compiles a module and returns the output or fails the test.
func compileModule(t *testing.T, module *ir.Module) string {
	t.Helper()
	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	return result
}

// mustContainMSL asserts the output contains the expected substring.
func mustContainMSL(t *testing.T, source, expected string) {
	t.Helper()
	if !strings.Contains(source, expected) {
		t.Errorf("Expected output to contain %q, but it was not found.\nOutput:\n%s", expected, source)
	}
}

// mustNotContainMSL asserts the output does NOT contain the substring.
func mustNotContainMSL(t *testing.T, source, forbidden string) {
	t.Helper()
	if strings.Contains(source, forbidden) {
		t.Errorf("Output should NOT contain %q, but it was found.\nOutput:\n%s", forbidden, source)
	}
}

// =============================================================================
// Test: Literal expression generation
// =============================================================================

func TestMSL_Literals(t *testing.T) {
	tests := []struct {
		name    string
		literal ir.LiteralValue
		want    string
	}{
		{"bool_true", ir.LiteralBool(true), "true"},
		{"bool_false", ir.LiteralBool(false), "false"},
		{"i32_positive", ir.LiteralI32(42), "42"},
		{"i32_negative", ir.LiteralI32(-7), "-7"},
		{"u32", ir.LiteralU32(100), "100u"},
		{"f32_integer", ir.LiteralF32(1.0), "1.0"},
		{"f32_fraction", ir.LiteralF32(0.5), "0.5"},
		{"f64", ir.LiteralF64(3.14), "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			tVec4 := ir.TypeHandle(1)
			retExpr := ir.ExpressionHandle(0)
			var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Result: &ir.FunctionResult{
							Type:    tVec4,
							Binding: &posBinding,
						},
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: tt.literal}},
							{Kind: ir.ExprZeroValue{Type: tVec4}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tVec4},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: Unary expression generation
// =============================================================================

func TestMSL_UnaryOperators(t *testing.T) {
	tests := []struct {
		name string
		op   ir.UnaryOperator
		want string
	}{
		{"negate", ir.UnaryNegate, "-("},
		{"logical_not", ir.UnaryLogicalNot, "!("},
		{"bitwise_not", ir.UnaryBitwiseNot, "~("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.ExprUnary{Op: tt.op, Expr: 0}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: Binary expression generation
// =============================================================================

func TestMSL_BinaryOperators(t *testing.T) {
	tests := []struct {
		name string
		op   ir.BinaryOperator
		want string
	}{
		{"add", ir.BinaryAdd, "+"},
		{"subtract", ir.BinarySubtract, "-"},
		{"multiply", ir.BinaryMultiply, "*"},
		{"divide", ir.BinaryDivide, "/"},
		{"modulo", ir.BinaryModulo, "%"},
		{"equal", ir.BinaryEqual, "=="},
		{"not_equal", ir.BinaryNotEqual, "!="},
		{"less", ir.BinaryLess, "<"},
		{"less_equal", ir.BinaryLessEqual, "<="},
		{"greater", ir.BinaryGreater, ">"},
		{"greater_equal", ir.BinaryGreaterEqual, ">="},
		{"and", ir.BinaryAnd, "&"},
		{"xor", ir.BinaryExclusiveOr, "^"},
		{"or", ir.BinaryInclusiveOr, "|"},
		{"logical_and", ir.BinaryLogicalAnd, "&&"},
		{"logical_or", ir.BinaryLogicalOr, "||"},
		{"shift_left", ir.BinaryShiftLeft, "<<"},
		{"shift_right", ir.BinaryShiftRight, ">>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(2)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
							{Kind: ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: Swizzle expression
// =============================================================================

func TestMSL_Swizzle(t *testing.T) {
	tVec4 := ir.TypeHandle(1)
	tVec2 := ir.TypeHandle(2)

	retExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},                                                // [0]
					{Kind: ir.ExprSwizzle{Vector: 0, Size: 2, Pattern: [4]ir.SwizzleComponent{0, 1}}},    // [1] .xy
					{Kind: ir.ExprSwizzle{Vector: 0, Size: 3, Pattern: [4]ir.SwizzleComponent{2, 1, 0}}}, // [2] .zyx
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
					{Handle: &tVec2},
					{Handle: &tVec2},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	// Only expression 2 (the return value) is emitted inline; expression 1 is dead code
	mustContainMSL(t, result, ".zyx")
}

// =============================================================================
// Test: Select (ternary) expression
// =============================================================================

func TestMSL_Select(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tBool := ir.TypeHandle(1)

	retExpr := ir.ExpressionHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},           // [0]
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},             // [1]
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},             // [2]
					{Kind: ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2}}, // [3]
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tBool},
					{Handle: &tF32},
					{Handle: &tF32},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "?")
	mustContainMSL(t, result, ":")
}

// =============================================================================
// Test: Compose expression (vector construction)
// =============================================================================

func TestMSL_Compose(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)

	retExpr := ir.ExpressionHandle(4)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(4.0)}},
					{Kind: ir.ExprCompose{Type: tVec4, Components: []ir.ExpressionHandle{0, 1, 2, 3}}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tF32},
					{Handle: &tF32},
					{Handle: &tF32},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "metal::float4(")
}

// =============================================================================
// Test: Math function expressions
// =============================================================================

func TestMSL_MathFunctions(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.MathFunction
		want string
		args int // 1, 2, or 3
	}{
		{"abs", ir.MathAbs, "metal::abs(", 1},
		{"min", ir.MathMin, "metal::min(", 2},
		{"max", ir.MathMax, "metal::max(", 2},
		{"clamp", ir.MathClamp, "metal::clamp(", 3},
		{"saturate", ir.MathSaturate, "metal::saturate(", 1},
		{"cos", ir.MathCos, "metal::cos(", 1},
		{"sin", ir.MathSin, "metal::sin(", 1},
		{"tan", ir.MathTan, "metal::tan(", 1},
		{"floor", ir.MathFloor, "metal::floor(", 1},
		{"ceil", ir.MathCeil, "metal::ceil(", 1},
		{"round", ir.MathRound, "metal::round(", 1},
		{"sqrt", ir.MathSqrt, "metal::sqrt(", 1},
		{"rsqrt", ir.MathInverseSqrt, "metal::rsqrt(", 1},
		{"exp", ir.MathExp, "metal::exp(", 1},
		{"exp2", ir.MathExp2, "metal::exp2(", 1},
		{"log", ir.MathLog, "metal::log(", 1},
		{"log2", ir.MathLog2, "metal::log2(", 1},
		{"pow", ir.MathPow, "metal::pow(", 2},
		{"fract", ir.MathFract, "metal::fract(", 1},
		{"sign", ir.MathSign, "metal::sign(", 1},
		{"step", ir.MathStep, "metal::step(", 2},
		{"smoothstep", ir.MathSmoothStep, "metal::smoothstep(", 3},
		{"mix", ir.MathMix, "metal::mix(", 3},
		{"length", ir.MathLength, "metal::length(", 1},
		{"normalize", ir.MathNormalize, "metal::normalize(", 1},
		{"distance", ir.MathDistance, "metal::distance(", 2},
		{"reflect", ir.MathReflect, "metal::reflect(", 2},
		{"fma", ir.MathFma, "metal::fma(", 3},
		{"dot", ir.MathDot, "metal::dot(", 2},
		{"cross", ir.MathCross, "metal::cross(", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)

			exprs := make([]ir.Expression, 0, 4)
			exprs = append(exprs,
				ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
				ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
			)
			exprTypes := make([]ir.TypeResolution, 0, 4)
			exprTypes = append(exprTypes,
				ir.TypeResolution{Handle: &tF32},
				ir.TypeResolution{Handle: &tF32},
				ir.TypeResolution{Handle: &tF32},
			)

			mathExpr := ir.ExprMath{Fun: tt.fun, Arg: 0}
			if tt.args >= 2 {
				arg1 := ir.ExpressionHandle(1)
				mathExpr.Arg1 = &arg1
			}
			if tt.args >= 3 {
				arg2 := ir.ExpressionHandle(2)
				mathExpr.Arg2 = &arg2
			}

			exprs = append(exprs, ir.Expression{Kind: mathExpr})
			exprTypes = append(exprTypes, ir.TypeResolution{Handle: &tF32})

			retExpr := ir.ExpressionHandle(3)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name:            "test_fn",
						Expressions:     exprs,
						ExpressionTypes: exprTypes,
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: ZeroValue expression
// =============================================================================

func TestMSL_ZeroValue(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "metal::float4()")
}

// =============================================================================
// Test: Type cast (As) expression
// =============================================================================

func TestMSL_TypeCast(t *testing.T) {
	t.Run("conversion", func(t *testing.T) {
		tF32 := ir.TypeHandle(0)
		tI32 := ir.TypeHandle(1)
		width := uint8(4)
		retExpr := ir.ExpressionHandle(1)

		module := &ir.Module{
			Types: []ir.Type{
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			},
			Functions: []ir.Function{
				{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(3.14)}},
						{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarSint, Convert: &width}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tI32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				},
			},
		}
		result := compileModule(t, module)
		mustContainMSL(t, result, "int(")
	})

	t.Run("bitcast", func(t *testing.T) {
		tF32 := ir.TypeHandle(0)
		tU32 := ir.TypeHandle(1)
		retExpr := ir.ExpressionHandle(1)

		module := &ir.Module{
			Types: []ir.Type{
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			},
			Functions: []ir.Function{
				{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarUint, Convert: nil}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tU32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				},
			},
		}
		result := compileModule(t, module)
		mustContainMSL(t, result, "as_type<uint>(")
	})
}

// =============================================================================
// Test: ScalarCastTypeName
// =============================================================================

func TestMSL_ScalarCastTypeName(t *testing.T) {
	w := &Writer{}

	tests := []struct {
		kind    ir.ScalarKind
		convert *uint8
		want    string
	}{
		{ir.ScalarFloat, nil, "float"},
		{ir.ScalarFloat, ptrUint8(4), "float"},
		{ir.ScalarFloat, ptrUint8(2), "half"},
		{ir.ScalarSint, nil, "int"},
		{ir.ScalarUint, nil, "uint"},
		{ir.ScalarBool, nil, "bool"},
	}

	for _, tt := range tests {
		got := w.scalarCastTypeName(tt.kind, tt.convert)
		if got != tt.want {
			t.Errorf("scalarCastTypeName(%v, %v) = %q, want %q", tt.kind, tt.convert, got, tt.want)
		}
	}
}

func ptrUint8(v uint8) *uint8 {
	return &v
}

// =============================================================================
// Test: Derivative expressions
// =============================================================================

func TestMSL_Derivative(t *testing.T) {
	tests := []struct {
		name    string
		axis    ir.DerivativeAxis
		control ir.DerivativeControl
		want    string
	}{
		{"dfdx_fine", ir.DerivativeX, ir.DerivativeFine, "metal::dfdx_fine("},
		{"dfdx_coarse", ir.DerivativeX, ir.DerivativeCoarse, "metal::dfdx_coarse("},
		{"dfdy_fine", ir.DerivativeY, ir.DerivativeFine, "metal::dfdy_fine("},
		{"dfdy_coarse", ir.DerivativeY, ir.DerivativeCoarse, "metal::dfdy_coarse("},
		{"fwidth", ir.DerivativeWidth, 0, "metal::fwidth("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.ExprDerivative{Axis: tt.axis, Control: tt.control, Expr: 0}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: Relational expressions
// =============================================================================

func TestMSL_Relational(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.RelationalFunction
		want string
	}{
		{"all", ir.RelationalAll, "metal::all("},
		{"any", ir.RelationalAny, "metal::any("},
		{"isnan", ir.RelationalIsNan, "metal::isnan("},
		{"isinf", ir.RelationalIsInf, "metal::isinf("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.ExprRelational{Fun: tt.fun, Argument: 0}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: mathFunctionName coverage for additional functions
// =============================================================================

func TestMSL_MathFunctionName(t *testing.T) {
	tests := []struct {
		fun  ir.MathFunction
		want string
	}{
		{ir.MathAbs, "abs"},
		{ir.MathMin, "min"},
		{ir.MathMax, "max"},
		{ir.MathClamp, "clamp"},
		{ir.MathSaturate, "saturate"},
		{ir.MathCos, "cos"},
		{ir.MathSin, "sin"},
		{ir.MathTan, "tan"},
		{ir.MathAcos, "acos"},
		{ir.MathAsin, "asin"},
		{ir.MathAtan, "atan"},
		{ir.MathAtan2, "atan2"},
		{ir.MathCosh, "cosh"},
		{ir.MathSinh, "sinh"},
		{ir.MathTanh, "tanh"},
		{ir.MathAsinh, "asinh"},
		{ir.MathAcosh, "acosh"},
		{ir.MathAtanh, "atanh"},
		{ir.MathRadians, "radians"},
		{ir.MathDegrees, "degrees"},
		{ir.MathCeil, "ceil"},
		{ir.MathFloor, "floor"},
		{ir.MathRound, "round"},
		{ir.MathFract, "fract"},
		{ir.MathTrunc, "trunc"},
		{ir.MathExp, "exp"},
		{ir.MathExp2, "exp2"},
		{ir.MathLog, "log"},
		{ir.MathLog2, "log2"},
		{ir.MathPow, "pow"},
		{ir.MathDot, "dot"},
		{ir.MathCross, "cross"},
		{ir.MathDistance, "distance"},
		{ir.MathLength, "length"},
		{ir.MathNormalize, "normalize"},
		{ir.MathFaceForward, "faceforward"},
		{ir.MathReflect, "reflect"},
		{ir.MathRefract, "refract"},
		{ir.MathSign, "sign"},
		{ir.MathFma, "fma"},
		{ir.MathMix, "mix"},
		{ir.MathStep, "step"},
		{ir.MathSmoothStep, "smoothstep"},
		{ir.MathSqrt, "sqrt"},
		{ir.MathInverseSqrt, "rsqrt"},
		{ir.MathTranspose, "transpose"},
		{ir.MathDeterminant, "determinant"},
		{ir.MathCountTrailingZeros, "ctz"},
		{ir.MathCountLeadingZeros, "clz"},
		{ir.MathCountOneBits, "popcount"},
		{ir.MathReverseBits, "reverse_bits"},
		{ir.MathExtractBits, "extract_bits"},
		{ir.MathInsertBits, "insert_bits"},
		{ir.MathFunction(255), "unknown_math_255"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := mathFunctionName(tt.fun)
			if got != tt.want {
				t.Errorf("mathFunctionName(%d) = %q, want %q", tt.fun, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: MSL access (dynamic index)
// =============================================================================

func TestMSL_AccessExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tI32 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				{Kind: ir.ExprAccess{Base: 0, Index: 1}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tI32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "[")
}

// =============================================================================
// Test: MSL access index (struct member / constant index)
// =============================================================================

func TestMSL_AccessIndexExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprCompose{
					Type:       tVec4,
					Components: []ir.ExpressionHandle{},
				}},
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tVec4},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileModule(t, module)
	// Should access component by index
	mustContainMSL(t, result, "return")
}

// =============================================================================
// Test: MSL more binary operators
// =============================================================================

func TestMSL_BinaryOperatorsExtended(t *testing.T) {
	tests := []struct {
		name string
		op   ir.BinaryOperator
		want string
	}{
		{"modulo", ir.BinaryModulo, "%"},
		{"logical_and", ir.BinaryLogicalAnd, "&&"},
		{"logical_or", ir.BinaryLogicalOr, "||"},
		{"shift_left", ir.BinaryShiftLeft, "<<"},
		{"shift_right", ir.BinaryShiftRight, ">>"},
		{"bitwise_and", ir.BinaryAnd, "&"},
		{"bitwise_xor", ir.BinaryExclusiveOr, "^"},
		{"bitwise_or", ir.BinaryInclusiveOr, "|"},
		{"equal", ir.BinaryEqual, "=="},
		{"not_equal", ir.BinaryNotEqual, "!="},
		{"less", ir.BinaryLess, "<"},
		{"less_equal", ir.BinaryLessEqual, "<="},
		{"greater", ir.BinaryGreater, ">"},
		{"greater_equal", ir.BinaryGreaterEqual, ">="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(2)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
						{Kind: ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tF32},
						{Handle: &tF32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				}},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: MSL load expression
// =============================================================================

func TestMSL_LoadExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.ExprLoad{Pointer: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "return")
}

// =============================================================================
// Test: MSL splat expression
// =============================================================================

func TestMSL_SplatExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.ExprSplat{Value: 0, Size: 4}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tVec4},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileModule(t, module)
	// Splat should create a vector from a scalar
	mustContainMSL(t, result, "1.0")
}

// =============================================================================
// Test: MSL function argument expression
// =============================================================================

func TestMSL_FunctionWithArgument(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Arguments: []ir.FunctionArgument{
				{Name: "x", Type: tF32},
			},
			Result: &ir.FunctionResult{Type: tF32},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "float x")
	mustContainMSL(t, result, "return")
}

// =============================================================================
// Test: MSL local variable expression
// =============================================================================

func TestMSL_LocalVariableExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			LocalVars: []ir.LocalVariable{
				{Name: "myVar", Type: tF32},
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},
				{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
			},
		}},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "myVar")
}

// =============================================================================
// Test: MSL if with else
// =============================================================================

func TestMSL_IfElseStatement(t *testing.T) {
	tBool := ir.TypeHandle(0)
	tF32 := ir.TypeHandle(1)
	expr1 := ir.ExpressionHandle(1)
	expr2 := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    []ir.Statement{{Kind: ir.StmtReturn{Value: &expr1}}},
					Reject:    []ir.Statement{{Kind: ir.StmtReturn{Value: &expr2}}},
				}},
			},
		}},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "if (")
	mustContainMSL(t, result, "} else {")
}
