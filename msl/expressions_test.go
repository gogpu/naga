package msl

import (
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
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
		{"modulo", ir.BinaryModulo, "metal::fmod("},
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
	mustContainMSL(t, result, "metal::float4 {}")
}

func TestMSL_ComposeZeroLocalInit(t *testing.T) {
	// Test: local var init with Compose(vec2<i32>, [I32(0), I32(0)])
	// should produce metal::int2(0, 0), NOT metal::int2()
	tVec2i := ir.TypeHandle(0)
	lit0 := ir.ExpressionHandle(0)
	lit1 := ir.ExpressionHandle(1)
	composeH := ir.ExpressionHandle(2)
	localVarH := ir.ExpressionHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},                                         // [0]
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},                                         // [1]
					{Kind: ir.ExprCompose{Type: tVec2i, Components: []ir.ExpressionHandle{lit0, lit1}}}, // [2]
					{Kind: ir.ExprLocalVariable{Variable: 0}},                                           // [3]
				},
				ExpressionTypes: []ir.TypeResolution{
					{}, {}, {Handle: &tVec2i}, {},
				},
				LocalVars: []ir.LocalVariable{
					{Name: "x", Type: tVec2i, Init: &composeH},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
					{Kind: ir.StmtStore{Pointer: localVarH, Value: composeH}},
				},
			},
		},
	}
	result := compileModule(t, module)
	t.Logf("MSL output:\n%s", result)
	mustContainMSL(t, result, "metal::int2(0, 0)")
}

func TestMSL_Vec2EmptyConstructorWithTypeAnnotation(t *testing.T) {
	// Test the real-world case: var x: vec2<i32> = vec2()
	// The lowerer creates Compose<vec2<f32>> then concretizes to vec2<i32>.
	// Regression test for abstract-types-var shader.
	src := `fn test() { var x: vec2<i32> = vec2(); _ = x; }`
	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatal("Lex error:", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("Parse error:", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal("Lower error:", err)
	}

	// Check the IR: function should have a local var with Compose init
	if len(module.Functions) == 0 {
		t.Fatal("no functions")
	}
	fn := &module.Functions[0]
	if len(fn.LocalVars) == 0 {
		t.Fatal("no local vars")
	}
	lv := &fn.LocalVars[0]
	if lv.Init == nil {
		t.Fatal("local var has no init — will produce '= {}' instead of '= type(0, 0)'")
	}
	initH := *lv.Init
	if int(initH) >= len(fn.Expressions) {
		t.Fatalf("init handle %d out of range (exprs: %d)", initH, len(fn.Expressions))
	}
	initExpr := fn.Expressions[initH]
	compose, ok := initExpr.Kind.(ir.ExprCompose)
	if !ok {
		t.Fatalf("init expr is %T, expected ExprCompose", initExpr.Kind)
	}
	if len(compose.Components) == 0 {
		t.Fatal("Compose has 0 components — will produce 'type()' instead of 'type(0, 0)'")
	}
	t.Logf("IR: Compose type=%d, %d components", compose.Type, len(compose.Components))

	// Compile to MSL
	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	opts.BoundsCheckPolicies = BoundsCheckPolicies{}
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatal("MSL compile error:", compileErr)
	}
	t.Logf("MSL output:\n%s", code)

	if !strings.Contains(code, "metal::int2(0, 0)") {
		t.Error("Expected 'metal::int2(0, 0)' in MSL output")
	}
	if strings.Contains(code, "metal::int2()") {
		t.Error("Found 'metal::int2()' — empty constructor instead of explicit zeros")
	}
}

func TestMSL_Vec2EmptyConstructorMultipleVars(t *testing.T) {
	// Test closer to abstract-types-var: multiple typed zero-arg constructors
	src := `
fn test() {
    var a: vec2<i32> = vec2(42, 43);
    var b: vec2<u32> = vec2(44, 45);
    var c: vec2<i32> = vec2();
    var d: vec2<u32> = vec2();
    var e: vec2<f32> = vec2();
    _ = a; _ = b; _ = c; _ = d; _ = e;
}`
	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatal("Lex error:", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("Parse error:", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal("Lower error:", err)
	}

	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	opts.BoundsCheckPolicies = BoundsCheckPolicies{}
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatal("MSL compile error:", compileErr)
	}
	t.Logf("MSL output:\n%s", code)

	// All three zero-arg constructors should have explicit zeros
	if strings.Contains(code, "metal::int2()") {
		t.Error("Found 'metal::int2()' — should be 'metal::int2(0, 0)'")
	}
	if strings.Contains(code, "metal::uint2()") {
		t.Error("Found 'metal::uint2()' — should be 'metal::uint2(0u, 0u)'")
	}
	if strings.Contains(code, "metal::float2()") {
		t.Error("Found 'metal::float2()' — should be 'metal::float2(0.0, 0.0)'")
	}
}

func TestMSL_Vec2EmptyConstructorWithPrivateGlobals(t *testing.T) {
	// Test matching abstract-types-var structure: module-scope private vars
	// passed through to functions, with local zero-arg constructors
	src := `
var<private> gx: vec2<i32> = vec2(42, 43);

fn test() {
    var c: vec2<i32> = vec2();
    var d: vec2<u32> = vec2();
    _ = gx; _ = c; _ = d;
    c = vec2();
    d = vec2();
}`
	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatal("Lex error:", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("Parse error:", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal("Lower error:", err)
	}

	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	opts.BoundsCheckPolicies = BoundsCheckPolicies{}
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatal("MSL compile error:", compileErr)
	}
	t.Logf("MSL output:\n%s", code)

	if strings.Contains(code, "metal::int2()") {
		t.Error("Found 'metal::int2()' — should be 'metal::int2(0, 0)'")
	}
	if strings.Contains(code, "metal::uint2()") {
		t.Error("Found 'metal::uint2()' — should be 'metal::uint2(0u, 0u)'")
	}
}

func TestMSL_AbstractTypesVarFull(t *testing.T) {
	// Use the ACTUAL abstract-types-var shader to reproduce the failure.
	srcBytes, readErr := os.ReadFile("../snapshot/testdata/in/abstract-types-var.wgsl")
	if readErr != nil {
		t.Skip("shader file not found:", readErr)
	}
	src := string(srcBytes)

	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatal("Lex error:", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("Parse error:", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal("Lower error:", err)
	}

	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	opts.BoundsCheckPolicies = BoundsCheckPolicies{}
	opts.FakeMissingBindings = true
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatal("MSL compile error:", compileErr)
	}

	// Dump IR for function "all_constant_arguments" to find the issue
	for fi, fn := range module.Functions {
		if fn.Name != "all_constant_arguments" {
			continue
		}
		for li, lv := range fn.LocalVars {
			if lv.Name != "xvip____" && lv.Name != "xvup____" && lv.Name != "xvfp____" {
				continue
			}
			t.Logf("func[%d] local[%d] %s type=%d init=%v", fi, li, lv.Name, lv.Type, lv.Init)
			if lv.Init != nil {
				initH := *lv.Init
				if int(initH) < len(fn.Expressions) {
					t.Logf("  init expr[%d] = %T %+v", initH, fn.Expressions[initH].Kind, fn.Expressions[initH].Kind)
					if compose, ok := fn.Expressions[initH].Kind.(ir.ExprCompose); ok {
						t.Logf("  Compose components: %v (len=%d)", compose.Components, len(compose.Components))
						for ci, ch := range compose.Components {
							if int(ch) < len(fn.Expressions) {
								t.Logf("    comp[%d] expr[%d] = %T %+v", ci, ch, fn.Expressions[ch].Kind, fn.Expressions[ch].Kind)
							}
						}
					}
				}
			} else {
				t.Logf("  init = nil (will produce '= {}')")
			}
		}
	}

	// Check for the specific failing patterns
	if strings.Contains(code, "metal::int2()") {
		// Find the line for context
		for i, line := range strings.Split(code, "\n") {
			if strings.Contains(line, "metal::int2()") {
				t.Errorf("Line %d: Found 'metal::int2()' — should have explicit zeros: %s", i+1, strings.TrimSpace(line))
			}
		}
	}
	if strings.Contains(code, "metal::uint2()") {
		for i, line := range strings.Split(code, "\n") {
			if strings.Contains(line, "metal::uint2()") {
				t.Errorf("Line %d: Found 'metal::uint2()': %s", i+1, strings.TrimSpace(line))
			}
		}
	}
	if strings.Contains(code, "metal::float2()") {
		for i, line := range strings.Split(code, "\n") {
			if strings.Contains(line, "metal::float2()") {
				t.Errorf("Line %d: Found 'metal::float2()': %s", i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestMSL_AbstractTypesVarPattern(t *testing.T) {
	// Simplified pattern test
	src := `
var<private> xvipaiai: vec2<i32> = vec2(42, 43);
var<private> xvupaiai: vec2<u32> = vec2(44, 45);
var<private> xvfpaiai: vec2<f32> = vec2(46, 47);
var<private> xvip____: vec2<i32> = vec2();
var<private> xvup____: vec2<u32> = vec2();
var<private> xvfp____: vec2<f32> = vec2();
var<private> xmfp____: mat2x2f = mat2x2(vec2(), vec2());

fn all_constant_arguments() {
    var xvipaiai: vec2<i32> = vec2(42, 43);
    var xvupaiai: vec2<u32> = vec2(44, 45);
    var xvfpaiai: vec2<f32> = vec2(46, 47);
    var xvfpafaf: vec2<f32> = vec2(48.0, 49.0);
    var xvfpaiaf: vec2<f32> = vec2(48, 49.0);
    var xvupuai: vec2<u32> = vec2(42u, 43);
    var xvupaiu: vec2<u32> = vec2(42, 43u);
    var xvuuai: vec2<u32> = vec2<u32>(42u, 43);
    var xvuaiu: vec2<u32> = vec2<u32>(42, 43u);
    var xvip: vec2<i32> = vec2();
    var xvup: vec2<u32> = vec2();
    var xvfp: vec2<f32> = vec2();
    var xmfp: mat2x2f = mat2x2(vec2(), vec2());
    _ = xvipaiai; _ = xvupaiai; _ = xvfpaiai; _ = xvfpafaf; _ = xvfpaiaf;
    _ = xvupuai; _ = xvupaiu; _ = xvuuai; _ = xvuaiu;
    _ = xvip; _ = xvup; _ = xvfp; _ = xmfp;
}

@compute @workgroup_size(1)
fn main() {
    all_constant_arguments();
}`
	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatal("Lex error:", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("Parse error:", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal("Lower error:", err)
	}

	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	opts.BoundsCheckPolicies = BoundsCheckPolicies{}
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatal("MSL compile error:", compileErr)
	}
	t.Logf("MSL output:\n%s", code)

	if strings.Contains(code, "metal::int2()") {
		t.Error("Found 'metal::int2()' — should be 'metal::int2(0, 0)'")
	}
	if strings.Contains(code, "metal::uint2()") {
		t.Error("Found 'metal::uint2()' — should be 'metal::uint2(0u, 0u)'")
	}
	if strings.Contains(code, "metal::float2()") {
		t.Error("Found 'metal::float2()' — should be 'metal::float2(0.0, 0.0)'")
	}
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
		mustContainMSL(t, result, "static_cast<int>(")
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
		// Rust naga ignores DerivativeControl — all map to base function name
		{"dfdx_fine", ir.DerivativeX, ir.DerivativeFine, "metal::dfdx("},
		{"dfdx_coarse", ir.DerivativeX, ir.DerivativeCoarse, "metal::dfdx("},
		{"dfdx_none", ir.DerivativeX, ir.DerivativeNone, "metal::dfdx("},
		{"dfdy_fine", ir.DerivativeY, ir.DerivativeFine, "metal::dfdy("},
		{"dfdy_coarse", ir.DerivativeY, ir.DerivativeCoarse, "metal::dfdy("},
		{"dfdy_none", ir.DerivativeY, ir.DerivativeNone, "metal::dfdy("},
		{"fwidth", ir.DerivativeWidth, ir.DerivativeNone, "metal::fwidth("},
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
		{ir.MathRadians, ""}, // handled specially (expanded to multiplication)
		{ir.MathDegrees, ""}, // handled specially (expanded to multiplication)
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
		{ir.MathExtractBits, ""},
		{ir.MathInsertBits, ""},
		{ir.MathFirstTrailingBit, ""},
		{ir.MathFirstLeadingBit, ""},
		{ir.MathPack4x8snorm, "pack_float_to_snorm4x8"},
		{ir.MathPack4x8unorm, "pack_float_to_unorm4x8"},
		{ir.MathPack2x16snorm, "pack_float_to_snorm2x16"},
		{ir.MathPack2x16unorm, "pack_float_to_unorm2x16"},
		{ir.MathPack2x16float, ""},
		{ir.MathUnpack4x8snorm, "unpack_snorm4x8_to_float"},
		{ir.MathUnpack4x8unorm, "unpack_unorm4x8_to_float"},
		{ir.MathUnpack2x16snorm, "unpack_snorm2x16_to_float"},
		{ir.MathUnpack2x16unorm, "unpack_unorm2x16_to_float"},
		{ir.MathUnpack2x16float, ""},
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
		{"modulo", ir.BinaryModulo, "metal::fmod("},
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
