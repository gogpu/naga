package wgsl

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestLowerSimpleVertexShader(t *testing.T) {
	// Simple vertex shader:
	// @vertex
	// fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
	//     return vec4<f32>(0.0, 0.0, 0.0, 1.0);
	// }

	ast := &Module{
		Functions: []*FunctionDecl{
			{
				Name: "main",
				Params: []*Parameter{
					{
						Name: "idx",
						Type: &NamedType{Name: "u32"},
						Attributes: []Attribute{
							{
								Name: "builtin",
								Args: []Expr{&Ident{Name: "vertex_index"}},
							},
						},
					},
				},
				ReturnType: &NamedType{
					Name:       "vec4",
					TypeParams: []Type{&NamedType{Name: "f32"}},
				},
				ReturnAttrs: []Attribute{
					{
						Name: "builtin",
						Args: []Expr{&Ident{Name: "position"}},
					},
				},
				Attributes: []Attribute{
					{Name: "vertex"},
				},
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

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Verify module structure: entry point functions are NOT in Module.Functions[]
	if len(module.Functions) != 0 {
		t.Errorf("expected 0 functions (entry points are inline), got %d", len(module.Functions))
	}

	// Verify entry point
	if len(module.EntryPoints) != 1 {
		t.Errorf("expected 1 entry point, got %d", len(module.EntryPoints))
	}

	ep := module.EntryPoints[0]
	if ep.Name != "main" {
		t.Errorf("expected entry point name 'main', got '%s'", ep.Name)
	}
	if ep.Stage != ir.StageVertex {
		t.Errorf("expected vertex stage, got %v", ep.Stage)
	}

	// Verify function (inline in entry point)
	fn := ep.Function
	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got '%s'", fn.Name)
	}

	if len(fn.Arguments) != 1 {
		t.Errorf("expected 1 argument, got %d", len(fn.Arguments))
	}

	// Verify argument binding
	arg := fn.Arguments[0]
	if arg.Name != "idx" {
		t.Errorf("expected argument name 'idx', got '%s'", arg.Name)
	}
	if arg.Binding == nil {
		t.Fatal("expected argument binding, got nil")
	}

	if binding, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
		if binding.Builtin != ir.BuiltinVertexIndex {
			t.Errorf("expected BuiltinVertexIndex, got %v", binding.Builtin)
		}
	} else {
		t.Errorf("expected BuiltinBinding, got %T", *arg.Binding)
	}

	// Verify return type
	if fn.Result == nil {
		t.Fatal("expected function result, got nil")
	}
	if fn.Result.Binding == nil {
		t.Fatal("expected result binding, got nil")
	}

	if binding, ok := (*fn.Result.Binding).(ir.BuiltinBinding); ok {
		if binding.Builtin != ir.BuiltinPosition {
			t.Errorf("expected BuiltinPosition, got %v", binding.Builtin)
		}
	} else {
		t.Errorf("expected BuiltinBinding, got %T", *fn.Result.Binding)
	}

	// Verify function body ends with a return statement
	if len(fn.Body) == 0 {
		t.Fatal("expected non-empty body")
	}
	// Find return statement (may be preceded by StmtEmit for named expressions)
	lastStmt := fn.Body[len(fn.Body)-1]
	if stmt, ok := lastStmt.Kind.(ir.StmtReturn); ok {
		if stmt.Value == nil {
			t.Error("expected return value, got nil")
		}
	} else {
		t.Errorf("expected last statement to be StmtReturn, got %T", lastStmt.Kind)
	}

	t.Logf("Successfully lowered vertex shader with %d expressions", len(fn.Expressions))
}

func TestLowerTypes(t *testing.T) {
	tests := []struct {
		name     string
		wgslType Type
		wantKind string
	}{
		{
			name:     "f32 scalar",
			wgslType: &NamedType{Name: "f32"},
			wantKind: "ScalarType",
		},
		{
			name:     "vec3<f32>",
			wgslType: &NamedType{Name: "vec3", TypeParams: []Type{&NamedType{Name: "f32"}}},
			wantKind: "VectorType",
		},
		{
			name:     "mat4x4<f32>",
			wgslType: &NamedType{Name: "mat4x4", TypeParams: []Type{&NamedType{Name: "f32"}}},
			wantKind: "MatrixType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lowerer{
				module:   &ir.Module{},
				registry: ir.NewTypeRegistry(),
				types:    make(map[string]ir.TypeHandle),
			}
			l.registerBuiltinTypes()

			_, err := l.resolveType(tt.wgslType)
			if err != nil {
				t.Fatalf("resolveType failed: %v", err)
			}

			// Type was added to registry
			if l.registry.Count() == 0 {
				t.Error("expected types in registry")
			}
		})
	}
}

func TestLowerExpressions(t *testing.T) {
	l := &Lowerer{
		module:   &ir.Module{},
		registry: ir.NewTypeRegistry(),
		types:    make(map[string]ir.TypeHandle),
		locals:   make(map[string]ir.ExpressionHandle),
	}
	l.registerBuiltinTypes()
	l.currentFunc = &ir.Function{
		Expressions: []ir.Expression{},
	}

	tests := []struct {
		name     string
		expr     Expr
		wantKind string
	}{
		{
			name:     "integer literal",
			expr:     &Literal{Kind: TokenIntLiteral, Value: "42"},
			wantKind: "Literal",
		},
		{
			name:     "float literal",
			expr:     &Literal{Kind: TokenFloatLiteral, Value: "3.14"},
			wantKind: "Literal",
		},
		{
			name:     "bool literal",
			expr:     &Literal{Kind: TokenTrue},
			wantKind: "Literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target []ir.Statement
			_, err := l.lowerExpression(tt.expr, &target)
			if err != nil {
				t.Fatalf("lowerExpression failed: %v", err)
			}
		})
	}
}

func TestUnusedVariableWarnings(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    var unused: f32 = 1.0;
    var used: f32 = 2.0;
    return vec4<f32>(used, used, used, 1.0);
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, err := LowerWithWarnings(ast, source)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Should have exactly one warning for 'unused'
	if len(result.Warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(result.Warnings))
	}

	if result.Warnings[0].Message != "unused variable 'unused' in function 'main'" {
		t.Errorf("Unexpected warning message: %s", result.Warnings[0].Message)
	}
}

func TestUnusedVariableWarnings_UnderscoreIgnored(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    var _ignored: f32 = 1.0;
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, err := LowerWithWarnings(ast, source)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Variables starting with _ should not produce warnings
	if len(result.Warnings) != 0 {
		t.Errorf("Expected no warnings for _ignored variable, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

// TestMathFunctions verifies that all WGSL built-in math functions are recognized
func TestMathFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function string
		args     string // WGSL function call arguments
		wantMath ir.MathFunction
	}{
		// Comparison functions
		{"abs", "abs", "x", ir.MathAbs},
		{"min", "min", "x, y", ir.MathMin},
		{"max", "max", "x, y", ir.MathMax},
		{"clamp", "clamp", "x, 0.0, 1.0", ir.MathClamp},
		{"saturate", "saturate", "x", ir.MathSaturate},

		// Trigonometric functions
		{"sin", "sin", "x", ir.MathSin},
		{"cos", "cos", "x", ir.MathCos},
		{"tan", "tan", "x", ir.MathTan},
		{"sinh", "sinh", "x", ir.MathSinh},
		{"cosh", "cosh", "x", ir.MathCosh},
		{"tanh", "tanh", "x", ir.MathTanh},
		{"asin", "asin", "x", ir.MathAsin},
		{"acos", "acos", "x", ir.MathAcos},
		{"atan", "atan", "x", ir.MathAtan},
		{"atan2", "atan2", "x, y", ir.MathAtan2},
		{"asinh", "asinh", "x", ir.MathAsinh},
		{"acosh", "acosh", "x", ir.MathAcosh},
		{"atanh", "atanh", "x", ir.MathAtanh},

		// Angle conversion
		{"radians", "radians", "x", ir.MathRadians},
		{"degrees", "degrees", "x", ir.MathDegrees},

		// Decomposition functions
		{"ceil", "ceil", "x", ir.MathCeil},
		{"floor", "floor", "x", ir.MathFloor},
		{"round", "round", "x", ir.MathRound},
		{"fract", "fract", "x", ir.MathFract},
		{"trunc", "trunc", "x", ir.MathTrunc},

		// Exponential functions
		{"exp", "exp", "x", ir.MathExp},
		{"exp2", "exp2", "x", ir.MathExp2},
		{"log", "log", "x", ir.MathLog},
		{"log2", "log2", "x", ir.MathLog2},
		{"pow", "pow", "x, y", ir.MathPow},

		// Geometric functions
		{"sqrt", "sqrt", "x", ir.MathSqrt},
		{"inverseSqrt", "inverseSqrt", "x", ir.MathInverseSqrt},
		{"length", "length", "v", ir.MathLength},
		{"normalize", "normalize", "v", ir.MathNormalize},
		{"dot", "dot", "v, v", ir.MathDot},
		{"cross", "cross", "v3, v3", ir.MathCross},
		{"distance", "distance", "x, y", ir.MathDistance},
		{"faceForward", "faceForward", "v3, v3, v3", ir.MathFaceForward},
		{"reflect", "reflect", "v3, v3", ir.MathReflect},
		{"refract", "refract", "v3, v3, x", ir.MathRefract},

		// Computational functions
		{"sign", "sign", "x", ir.MathSign},
		{"fma", "fma", "x, y, x", ir.MathFma},
		{"mix", "mix", "x, y, x", ir.MathMix},
		{"step", "step", "x, y", ir.MathStep},
		{"smoothstep", "smoothstep", "x, y, x", ir.MathSmoothStep},
	}

	// Functions that return vec3 and take vec3 args
	vec3Funcs := map[string]bool{"cross": true, "faceForward": true, "reflect": true}
	// Functions that return vec4 and take vec4 args
	vec4Funcs := map[string]bool{"normalize": true}
	// Functions that take vec3 args and also a scalar, returning vec3
	refractFunc := map[string]bool{"refract": true}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var source string
			switch {
			case vec3Funcs[tt.function]:
				source = `
fn test() -> vec3<f32> {
    var v3: vec3<f32> = vec3<f32>(1.0, 0.0, 0.0);
    return ` + tt.function + `(` + tt.args + `);
}
`
			case vec4Funcs[tt.function]:
				source = `
fn test() -> vec4<f32> {
    var v: vec4<f32> = vec4<f32>(1.0, 0.0, 0.0, 0.0);
    return ` + tt.function + `(` + tt.args + `);
}
`
			case refractFunc[tt.function]:
				source = `
fn test() -> vec3<f32> {
    var v3: vec3<f32> = vec3<f32>(1.0, 0.0, 0.0);
    var x: f32 = 1.0;
    return refract(v3, v3, x);
}
`
			default:
				source = `
fn test() -> f32 {
    var x: f32 = 1.0;
    var y: f32 = 2.0;
    var v: vec4<f32> = vec4<f32>(1.0, 0.0, 0.0, 0.0);
    var v3: vec3<f32> = vec3<f32>(1.0, 0.0, 0.0);
    return ` + tt.function + `(` + tt.args + `);
}
`
			}

			lexer := NewLexer(source)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("Tokenize failed: %v", err)
			}

			parser := NewParser(tokens)
			ast, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			module, err := Lower(ast)
			if err != nil {
				t.Fatalf("Lower failed for %s: %v", tt.function, err)
			}

			// Verify the function compiled successfully
			if len(module.Functions) != 1 {
				t.Fatalf("expected 1 function, got %d", len(module.Functions))
			}

			fn := module.Functions[0]

			// Verify the math function is in the function's expressions
			found := false
			for _, expr := range fn.Expressions {
				if math, ok := expr.Kind.(ir.ExprMath); ok {
					if math.Fun == tt.wantMath {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("math function %s (%v) not found in function expressions", tt.function, tt.wantMath)
			}
		})
	}
}

// TestSelectBuiltin verifies the WGSL select() built-in function is lowered to ExprSelect.
func TestSelectBuiltin(t *testing.T) {
	source := `
fn test() -> f32 {
    var a: f32 = 1.0;
    var b: f32 = 2.0;
    var c: bool = true;
    return select(a, b, c);
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]

	// Verify ExprSelect is in the function's expressions
	found := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprSelect); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("select() not found as ExprSelect in function expressions")
	}
}

// TestIfElseWithReturn verifies if/else with return in both branches generates correct IR.
func TestIfElseWithReturn(t *testing.T) {
	source := `
fn test(x: f32) -> f32 {
    if (x > 0.5) {
        return 1.0;
    } else {
        return 0.0;
    }
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]

	// Verify the function body contains an if statement
	foundIf := false
	for _, stmt := range fn.Body {
		ifStmt, ok := stmt.Kind.(ir.StmtIf)
		if !ok {
			continue
		}
		foundIf = true
		// Verify both branches are non-empty
		if len(ifStmt.Accept) == 0 {
			t.Error("if accept block is empty")
		}
		if len(ifStmt.Reject) == 0 {
			t.Error("if reject block is empty")
		}
		// Check last statement in accept is return
		lastAccept := ifStmt.Accept[len(ifStmt.Accept)-1]
		if _, ok := lastAccept.Kind.(ir.StmtReturn); !ok {
			t.Errorf("last accept statement is %T, expected StmtReturn", lastAccept.Kind)
		}
		// Reject may contain a StmtBlock wrapping the return (parser places else body in a block)
		// Find the return statement by traversing into blocks
		if !containsReturn(ifStmt.Reject) {
			t.Error("if reject block does not contain a return statement")
		}
	}
	if !foundIf {
		t.Error("if statement not found in function body")
	}
}

// containsReturn checks if a block contains a return statement, traversing into nested blocks.
func containsReturn(block ir.Block) bool {
	for _, stmt := range block {
		switch k := stmt.Kind.(type) {
		case ir.StmtReturn:
			return true
		case ir.StmtBlock:
			if containsReturn(k.Block) {
				return true
			}
		}
	}
	return false
}

// TestLowerVec3ZeroValBinop verifies that vec3(vec2i(), 0) + vec3(1) produces
// correct IR with Vec3<Sint> type (not Vec2<Sint>).
// Regression test for const-exprs shader compose_vector_zero_val_binop.
func TestLowerVec3ZeroValBinop(t *testing.T) {
	src := `fn test() {
    var a = vec3(vec2i(), 0) + vec3(1);
    _ = a;
}`
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal("tokenize:", err)
	}
	parser := NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal("parse:", parseErr)
	}
	module, lowerErr := Lower(ast)
	if lowerErr != nil {
		t.Fatal("lower:", lowerErr)
	}

	// Dump pre-compact types
	t.Logf("Types: %d", len(module.Types))
	for i, typ := range module.Types {
		t.Logf("  type[%d] = %T %+v", i, typ.Inner, typ.Inner)
	}

	// Find the test function
	var fn *ir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "test" {
			fn = &module.Functions[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("function 'test' not found")
	}

	// Check local var 'a' — should be vec3<i32>
	if len(fn.LocalVars) == 0 {
		t.Fatal("no local vars")
	}
	lv := &fn.LocalVars[0]
	if int(lv.Type) >= len(module.Types) {
		t.Fatalf("local var type %d out of range", lv.Type)
	}
	typeInner := module.Types[lv.Type].Inner
	vecType, ok := typeInner.(ir.VectorType)
	if !ok {
		t.Fatalf("local var type is %T, expected VectorType", typeInner)
	}
	if vecType.Size != ir.Vec3 {
		t.Errorf("local var vector size = %d, want Vec3 (3)", vecType.Size)
	}
	if vecType.Scalar.Kind != ir.ScalarSint {
		t.Errorf("local var scalar kind = %v, want Sint", vecType.Scalar.Kind)
	}

	// If init is set, verify Compose has correct type
	if lv.Init != nil {
		initH := *lv.Init
		if int(initH) < len(fn.Expressions) {
			if compose, ok := fn.Expressions[initH].Kind.(ir.ExprCompose); ok {
				composeTypeInner := module.Types[compose.Type].Inner
				if cvt, ok := composeTypeInner.(ir.VectorType); ok {
					if cvt.Size != ir.Vec3 {
						t.Errorf("Compose type vector size = %d, want Vec3 (3). Components count = %d", cvt.Size, len(compose.Components))
					}
				}
			}
		}
	}
	t.Logf("Local var 'a': type=%d (%+v), init=%v", lv.Type, typeInner, lv.Init)
	// Dump all expressions and their types
	for i, e := range fn.Expressions {
		typeStr := "?"
		if i < len(fn.ExpressionTypes) {
			et := fn.ExpressionTypes[i]
			if et.Handle != nil {
				typeStr = fmt.Sprintf("Handle(%d)", *et.Handle)
			} else if et.Value != nil {
				typeStr = fmt.Sprintf("Value(%T)", et.Value)
			} else {
				typeStr = "nil"
			}
		}
		t.Logf("  expr[%d] %T typeRes=%s", i, e.Kind, typeStr)
	}
}

// TestLowerComposeThreeDeep verifies that vec4(vec3(vec2(6,7), 8), 9) folds
// nested Compose, making intermediate Vec3<Sint>/Vec2<Sint> types dead.
func TestLowerComposeThreeDeep(t *testing.T) {
	src := `fn test() {
    var out = vec4(vec3(vec2(6, 7), 8), 9)[0];
    _ = out;
}`
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	parser := NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	module, lowerErr := Lower(ast)
	if lowerErr != nil {
		t.Fatal(lowerErr)
	}

	// Check: Vec3<Sint> should NOT be in the type arena
	// (it should be dead after Compose folding + compact)
	for i, typ := range module.Types {
		if vt, ok := typ.Inner.(ir.VectorType); ok {
			if vt.Size == ir.Vec3 && vt.Scalar.Kind == ir.ScalarSint {
				t.Errorf("type[%d] = Vec3<Sint> should be dead (folded away), but survived compact", i)
			}
			if vt.Size == ir.Vec2 && vt.Scalar.Kind == ir.ScalarSint {
				t.Errorf("type[%d] = Vec2<Sint> should be dead (folded away), but survived compact", i)
			}
		}
		t.Logf("type[%d] %T %+v", i, typ.Inner, typ.Inner)
	}
}

// TestLowerSwizzleOfCompose verifies that vec4(vec2(1,2), vec2(3,4)).wzyx
// is const-folded to vec4(4,3,2,1) — matching Rust naga's const evaluator.
func TestLowerSwizzleOfCompose(t *testing.T) {
	src := `fn swizzle_of_compose() {
    var out = vec4(vec2(1, 2), vec2(3, 4)).wzyx;
    _ = out;
}`
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	parser := NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	module, lowerErr := Lower(ast)
	if lowerErr != nil {
		t.Fatal(lowerErr)
	}

	// Find the swizzle_of_compose function
	var fn *ir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "swizzle_of_compose" {
			fn = &module.Functions[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("function swizzle_of_compose not found")
	}

	// The swizzle should be const-folded: the Compose references components
	// in swizzled order (wzyx = indices 3,2,1,0). The var init should point
	// to the Compose (const-init, no Store needed).
	// Find the Compose expression
	composeIdx := -1
	for i, expr := range fn.Expressions {
		if comp, ok := expr.Kind.(ir.ExprCompose); ok {
			composeIdx = i
			// Verify components are in swizzled order
			if len(comp.Components) != 4 {
				t.Errorf("Compose should have 4 components, got %d", len(comp.Components))
			} else {
				// Components should reference literals 4,3,2,1 (in some handle order)
				for j, ch := range comp.Components {
					lit, ok := fn.Expressions[ch].Kind.(ir.Literal)
					if !ok {
						t.Errorf("Compose component[%d] should be Literal, got %T", j, fn.Expressions[ch].Kind)
						continue
					}
					v, ok := lit.Value.(ir.LiteralI32)
					if !ok {
						t.Errorf("Compose component[%d] should be LiteralI32, got %T", j, lit.Value)
						continue
					}
					expected := []int32{4, 3, 2, 1}
					if int32(v) != expected[j] {
						t.Errorf("Compose component[%d]: expected I32(%d), got I32(%d)", j, expected[j], int32(v))
					}
				}
			}
			break
		}
	}
	if composeIdx == -1 {
		t.Error("no Compose expression found — swizzle was not const-folded")
	}
	// Var should have const init pointing to the Compose
	if len(fn.LocalVars) > 0 && fn.LocalVars[0].Init == nil {
		t.Error("var 'out' should have const init (swizzle folded)")
	}
	t.Logf("expressions: %d", len(fn.Expressions))
	for i, expr := range fn.Expressions {
		t.Logf("  [%d] %T %+v", i, expr.Kind, expr.Kind)
	}
}

// TestLowerSwizzleOfSplat verifies that swizzle of Splat-derived vectors
// reuses handles (not copies), matching Rust naga which produces
// [Lit(1.0), Lit(2.0), Compose] — only 3 expressions for vec4f(vec3f(1.0), 2.0).wzyx.
func TestLowerSwizzleOfSplat(t *testing.T) {
	src := `fn compose_of_splat() {
    var x = vec4f(vec3f(1.0), 2.0).wzyx;
    _ = x;
}`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	var fn *ir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "compose_of_splat" {
			fn = &module.Functions[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("function compose_of_splat not found")
	}

	// Count Literal and Compose expressions.
	// Rust produces exactly 3: Lit(1.0), Lit(2.0), Compose(vec4, [2,0,0,0])
	// The Splat-derived components share one handle — NOT copied.
	litCount := 0
	composeCount := 0
	for _, expr := range fn.Expressions {
		switch expr.Kind.(type) {
		case ir.Literal:
			litCount++
		case ir.ExprCompose:
			composeCount++
		}
	}

	// After compact, dead intermediates (Splat, inner Compose) are removed.
	// We should have at most 3 non-infrastructure expressions (2 Lit + 1 Compose).
	// With LocalVariable + Load we may have 5 total, but no more.
	if litCount > 2 {
		t.Errorf("expected at most 2 Literals (Splat shares handles), got %d", litCount)
		for i, expr := range fn.Expressions {
			t.Logf("  [%d] %T %+v", i, expr.Kind, expr.Kind)
		}
	}
}

// TestLowerPackedDotProduct verifies that dot4I8Packed and dot4U8Packed
// are const-evaluated to scalar literals matching Rust naga.
// E.g., dot4I8Packed(2u, 2u) → I32(4), dot4U8Packed(2u, 2u) → U32(4).
func TestLowerPackedDotProduct(t *testing.T) {
	src := `const TWO: u32 = 2u;
fn packed_dot_product() {
    var signed_four = dot4I8Packed(TWO, TWO);
    var unsigned_four = dot4U8Packed(TWO, TWO);
    var signed_seventy = dot4I8Packed(0x01020304u, 0x05060708u);
    var unsigned_seventy = dot4U8Packed(0x01020304u, 0x05060708u);
    var minus_four = dot4I8Packed(0xff02fd04u, 0x0506f9f8u);
}`
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	parser := NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	module, lowerErr := Lower(ast)
	if lowerErr != nil {
		t.Fatal(lowerErr)
	}

	var fn *ir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "packed_dot_product" {
			fn = &module.Functions[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("function packed_dot_product not found")
	}

	// Rust naga produces 7 Literal expressions (all const-evaluated):
	// I32(4), U32(4), I32(12), U32(12), I32(70), U32(70), I32(-4)
	// All local vars have init=Some(N), body=[Return]
	type litCheck struct {
		tag   string
		value interface{}
	}
	expected := []litCheck{
		{"I32", int32(4)},
		{"U32", uint32(4)},
		{"I32", int32(70)},
		{"U32", uint32(70)},
		{"I32", int32(-4)},
	}

	litCount := 0
	for _, expr := range fn.Expressions {
		if lit, ok := expr.Kind.(ir.Literal); ok {
			if litCount < len(expected) {
				exp := expected[litCount]
				switch v := lit.Value.(type) {
				case ir.LiteralI32:
					if exp.tag != "I32" || int32(v) != exp.value.(int32) {
						t.Errorf("literal[%d]: expected %s(%v), got I32(%d)", litCount, exp.tag, exp.value, int32(v))
					}
				case ir.LiteralU32:
					if exp.tag != "U32" || uint32(v) != exp.value.(uint32) {
						t.Errorf("literal[%d]: expected %s(%v), got U32(%d)", litCount, exp.tag, exp.value, uint32(v))
					}
				default:
					t.Errorf("literal[%d]: unexpected literal type %T", litCount, lit.Value)
				}
			}
			litCount++
		}
	}

	// Check that dot4 results are const-folded (vars have init set, not runtime)
	constInitCount := 0
	for _, lv := range fn.LocalVars {
		if lv.Init != nil {
			constInitCount++
		}
	}
	if constInitCount < 5 {
		t.Errorf("expected at least 5 vars with const init (dot4 folded), got %d", constInitCount)
	}

	t.Logf("expressions: %d, literals: %d, const-init vars: %d", len(fn.Expressions), litCount, constInitCount)
	for i, expr := range fn.Expressions {
		t.Logf("  [%d] %T %+v", i, expr.Kind, expr.Kind)
	}
}

// TestLowerAbstractAccessInline verifies that abstract constants (ABSTRACT_ARRAY,
// ABSTRACT_VECTOR) are inlined as expression trees in function bodies, matching
// Rust naga's process_overrides behavior.
func TestLowerAbstractAccessInline(t *testing.T) {
	src := `const ABSTRACT_ARRAY = array(1, 2, 3, 4, 5, 6, 7, 8, 9);
const ABSTRACT_VECTOR = vec4(1, 2, 3, 4);

fn abstract_access(i: u32) {
    var a: f32 = ABSTRACT_ARRAY[0];
    var b: u32 = ABSTRACT_VECTOR.x;
    var c: i32 = ABSTRACT_ARRAY[i];
    var d: i32 = ABSTRACT_VECTOR[i];
}`
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	parser := NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	module, lowerErr := Lower(ast)
	if lowerErr != nil {
		t.Fatal(lowerErr)
	}

	var fn *ir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "abstract_access" {
			fn = &module.Functions[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("function abstract_access not found")
	}

	// Rust naga produces 22 expressions for this function.
	// The abstract constants are fully inlined as Literal+Compose trees.
	// Check that we DON'T have ExprConstant references (they should be inlined).
	constCount := 0
	composeCount := 0
	literalCount := 0
	for _, expr := range fn.Expressions {
		switch expr.Kind.(type) {
		case ir.ExprConstant:
			constCount++
		case ir.ExprCompose:
			composeCount++
		case ir.Literal:
			literalCount++
		}
	}

	if constCount > 0 {
		t.Errorf("expected 0 ExprConstant (abstract constants should be inlined), got %d", constCount)
	}
	if composeCount < 2 {
		t.Errorf("expected at least 2 Compose (array + vector inlined), got %d", composeCount)
	}
	if literalCount < 13 {
		t.Errorf("expected at least 13 Literals (9 array + 4 vector elements), got %d", literalCount)
	}

	// Rust has exactly 22 expressions
	if len(fn.Expressions) != 22 {
		t.Errorf("expected 22 expressions (matching Rust naga), got %d", len(fn.Expressions))
	}

	t.Logf("expressions: %d (literals=%d, compose=%d, constant=%d)",
		len(fn.Expressions), literalCount, composeCount, constCount)
	for i, expr := range fn.Expressions {
		t.Logf("  [%d] %T %+v", i, expr.Kind, expr.Kind)
	}
}

// TestLowerVoidCallEmitterRestart verifies that void function calls properly
// restart the emitter, preventing overlapping Emit statements.
// Regression test for: lowerCall left emitter stopped for void calls,
// causing ExprStmt's emitFinish to re-emit pre-call expressions.
func TestLowerVoidCallEmitterRestart(t *testing.T) {
	src := `fn takes_ptr(p: ptr<function, array<i32, 4>>) {}
fn local_var_from_arg(a: array<i32, 4>, i: u32) {
    var b = a;
    takes_ptr(&b[i]);
}
fn let_binding(a: ptr<function, array<i32, 4>>, i: u32) {
    let p0 = &a[i];
    takes_ptr(p0);
}
@compute @workgroup_size(1)
fn main() {
    var arr1d: array<i32, 4>;
    local_var_from_arg(array(1, 2, 3, 4), 5);
    let_binding(&arr1d, 1);
}`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	// Find main entry point
	if len(module.EntryPoints) == 0 {
		t.Fatal("no entry points")
	}
	fn := &module.EntryPoints[0].Function

	// Count non-empty Emit statements
	nonEmptyEmits := 0
	for _, s := range fn.Body {
		if emit, ok := s.Kind.(ir.StmtEmit); ok {
			if emit.Range.Start != emit.Range.End {
				nonEmptyEmits++
			}
		}
	}

	// Rust naga produces exactly 1 non-empty Emit for the array Compose.
	// Before the fix, we produced 2+ due to overlapping Emit from void call
	// not restarting the emitter.
	if nonEmptyEmits > 1 {
		t.Errorf("expected at most 1 non-empty Emit (array Compose), got %d", nonEmptyEmits)
		for i, s := range fn.Body {
			t.Logf("  [%d] %T %+v", i, s.Kind, s.Kind)
		}
	}
}

// TestLowerNestedMatrixColumnGrouping verifies that matrix constructors nested
// inside array constructors produce column-grouped Compose GEs.
// array<mat2x2<f32>, 1>(mat2x2<f32>(0, 1, 2, 3)) should create:
//
//	Compose(vec2, [0, 1]), Compose(vec2, [2, 3]), Compose(mat2x2, [col0, col1]), Compose(array, [mat])
//
// NOT: Compose(mat2x2, [0, 1, 2, 3]) with flat scalar args.
func TestLowerNestedMatrixColumnGrouping(t *testing.T) {
	src := `const c = array<mat2x2<f32>, 1>(mat2x2<f32>(0.0, 1.0, 2.0, 3.0));
@compute @workgroup_size(1)
fn main() { _ = c; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	// Count Compose GEs — should have column vec2 Composes + mat + array
	composeCount := 0
	for _, ge := range module.GlobalExpressions {
		if _, ok := ge.Kind.(ir.ExprCompose); ok {
			composeCount++
		}
	}

	// Rust creates 4 Compose: vec2(col0), vec2(col1), mat2x2, array
	// Without the fix, we'd get 2: mat2x2(flat scalars), array
	if composeCount < 4 {
		t.Errorf("expected at least 4 Compose GEs (2 col vecs + mat + array), got %d", composeCount)
		for i, ge := range module.GlobalExpressions {
			t.Logf("  GE[%d] %T %+v", i, ge.Kind, ge.Kind)
		}
	}
}

// TestNeedsPreEmitInterrupt verifies that Literal expressions auto-interrupt
// the emitter, producing Emit statements BEFORE Store (not after).
// Regression test: without needsPreEmit in addExpression, var init lowering
// produces Store→Emit instead of Emit→Store.
func TestNeedsPreEmitInterrupt(t *testing.T) {
	src := `fn test() {
    var x: i32 = 42;
    var y: i32 = x + 1;
}`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	fn := &module.Functions[0]
	// Find Store statements — each should be preceded by Emit (not followed)
	for i, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtStore); ok {
			// Check that preceding statement is Emit or start of block
			if i > 0 {
				if _, ok := fn.Body[i-1].Kind.(ir.StmtEmit); !ok {
					t.Errorf("stmt[%d] Store should be preceded by Emit, got %T", i, fn.Body[i-1].Kind)
				}
			}
		}
	}
}

// TestSplatInGlobalExpressions verifies that single-arg vector constructors
// for var<private> produce ExprSplat GE (not ExprCompose with one component).
func TestSplatInGlobalExpressions(t *testing.T) {
	src := `var<private> v: vec3<f32> = vec3(1.0);
@compute @workgroup_size(1)
fn main() { _ = v; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	splatCount := 0
	for _, ge := range module.GlobalExpressions {
		if _, ok := ge.Kind.(ir.ExprSplat); ok {
			splatCount++
		}
	}
	if splatCount == 0 {
		t.Error("expected ExprSplat in GlobalExpressions for vec3(1.0), got none")
		for i, ge := range module.GlobalExpressions {
			t.Logf("  GE[%d] %T", i, ge.Kind)
		}
	}
}

// TestZeroValueInGlobalExpressions verifies that zero-arg constructors
// for module constants produce the correct IR depending on the type source.
// - Constructor with explicit type params (vec2<u32>()) → ExprZeroValue
// - Partial constructor with type annotation (: vec2<u32> = vec2()) → Compose with zeros
// This matches Rust naga where abstract constructors get concretized via annotations.
func TestZeroValueInGlobalExpressions(t *testing.T) {
	// Case 1: Constructor has explicit type params → ExprZeroValue
	src := `const z = vec2<u32>();
@compute @workgroup_size(1)
fn main() { _ = z; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	zeroCount := 0
	for _, ge := range module.GlobalExpressions {
		if _, ok := ge.Kind.(ir.ExprZeroValue); ok {
			zeroCount++
		}
	}
	if zeroCount == 0 {
		t.Error("expected ExprZeroValue in GlobalExpressions for vec2<u32>(), got none")
	}

	// Case 2: Partial constructor with type annotation → Compose (not ZeroValue)
	src2 := `const z2: vec2<u32> = vec2();
@compute @workgroup_size(1)
fn main() { _ = z2; }`
	module2, err := compileWGSL(t, src2)
	if err != nil {
		t.Fatal(err)
	}

	composeCount := 0
	for _, ge := range module2.GlobalExpressions {
		if _, ok := ge.Kind.(ir.ExprCompose); ok {
			composeCount++
		}
	}
	if composeCount == 0 {
		t.Error("expected ExprCompose in GlobalExpressions for typed vec2(), got none")
	}
}

// TestZeroVarExpandsToLiterals verifies that var<private> with zero-arg
// constructor expands to explicit Literal+Compose (not ZeroValue) matching Rust.
func TestZeroVarExpandsToLiterals(t *testing.T) {
	src := `var<private> v: vec2<i32> = vec2();
@compute @workgroup_size(1)
fn main() { _ = v; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	litCount := 0
	composeCount := 0
	zeroCount := 0
	for _, ge := range module.GlobalExpressions {
		switch ge.Kind.(type) {
		case ir.Literal:
			litCount++
		case ir.ExprCompose:
			composeCount++
		case ir.ExprZeroValue:
			zeroCount++
		}
	}
	// Rust expands vec2() to Literal(0)+Literal(0)+Compose, NOT ZeroValue
	if litCount < 2 || composeCount < 1 {
		t.Errorf("expected 2+ Literals + 1+ Compose for var<private> vec2(), got lit=%d compose=%d zero=%d",
			litCount, composeCount, zeroCount)
	}
}

// TestConstantAliasSharesGE verifies that constant aliases reuse the source
// constant's GlobalExpression handle instead of creating a duplicate.
func TestConstantAliasSharesGE(t *testing.T) {
	src := `const FOUR: i32 = 4;
const FOUR_ALIAS = FOUR;
@compute @workgroup_size(1)
fn main() { _ = FOUR_ALIAS; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	// Both FOUR and FOUR_ALIAS should point to the same GE
	if len(module.Constants) < 2 {
		t.Fatalf("expected at least 2 constants, got %d", len(module.Constants))
	}
	fourInit := module.Constants[0].Init
	aliasInit := module.Constants[1].Init
	if fourInit != aliasInit {
		t.Errorf("FOUR_ALIAS.Init (%d) should equal FOUR.Init (%d) — GE should be shared", aliasInit, fourInit)
	}
}

// TestBinaryConstEvalProducesGE verifies that vector binary constant expressions
// produce GlobalExpressions directly (not sub-Constants).
func TestBinaryConstEvalProducesGE(t *testing.T) {
	src := `const result = vec2(1.0f) + vec2(3.0f, 4.0f);
@compute @workgroup_size(1)
fn main() { _ = result; }`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	// result should have Init set (GE-based), not Value (sub-constant-based)
	var resultConst *ir.Constant
	for i := range module.Constants {
		if module.Constants[i].Name == "result" {
			resultConst = &module.Constants[i]
			break
		}
	}
	if resultConst == nil {
		t.Fatal("constant 'result' not found")
	}
	if resultConst.Value != nil {
		t.Error("result.Value should be nil (GE-based), got non-nil — sub-constants not converted to GE")
	}

	// Check that GE contains the evaluated result: F32(4.0), F32(5.0), Compose
	litCount := 0
	composeCount := 0
	for _, ge := range module.GlobalExpressions {
		switch ge.Kind.(type) {
		case ir.Literal:
			litCount++
		case ir.ExprCompose:
			composeCount++
		}
	}
	if composeCount < 1 {
		t.Error("expected at least 1 Compose GE for binary const result")
	}
}

// TestAbstractLiteralConcretization verifies that AbstractInt and AbstractFloat
// scalar values are concretized to I32/F32 when converted to literals.
func TestAbstractLiteralConcretization(t *testing.T) {
	src := `const ABSTRACT_ARRAY = array(10, 20, 30);
fn test(i: u32) {
    var c: i32 = ABSTRACT_ARRAY[i];
}`
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatal(err)
	}

	fn := &module.Functions[0]
	// Abstract array should be inlined — no ExprConstant references
	for i, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprConstant); ok {
			t.Errorf("expr[%d] is ExprConstant — abstract constant should be inlined", i)
		}
	}
	// Should have Compose with I32 literals (concretized from AbstractInt)
	hasCompose := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprCompose); ok {
			hasCompose = true
		}
	}
	if !hasCompose {
		t.Error("expected inlined Compose expression for ABSTRACT_ARRAY")
	}
}

// compileWGSL is a test helper that compiles WGSL source to an IR module.
func compileWGSL(t *testing.T, src string) (*ir.Module, error) {
	t.Helper()
	lexer := NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	return Lower(ast)
}

// TestBreakIfEmitsSubExpressions verifies that the break-if condition's
// sub-expressions are emitted in the continuing block, matching Rust naga IR.
// Without this emit, backends cannot properly bake Load expressions referenced
// by the break-if condition.
func TestBreakIfEmitsSubExpressions(t *testing.T) {
	source := `
fn test(a: bool) {
    var b: bool;
    loop {
        b = a;
        continuing {
            break if a == b;
        }
    }
}
`
	module, err := compileWGSL(t, source)
	if err != nil {
		t.Fatalf("parse+lower failed: %v", err)
	}

	if len(module.Functions) < 1 {
		t.Fatal("expected at least 1 function")
	}
	fn := &module.Functions[0]

	// Find the Loop statement
	var loop *ir.StmtLoop
	for _, stmt := range fn.Body {
		if l, ok := stmt.Kind.(ir.StmtLoop); ok {
			loop = &l
			break
		}
	}
	if loop == nil {
		t.Fatal("no loop statement found")
	}
	if loop.BreakIf == nil {
		t.Fatal("loop has no break-if condition")
	}

	// The continuing block should contain an Emit that covers the break-if
	// condition's sub-expressions (Load + Binary).
	breakIfHandle := *loop.BreakIf
	foundEmitCovering := false
	for _, stmt := range loop.Continuing {
		if emit, ok := stmt.Kind.(ir.StmtEmit); ok {
			if emit.Range.Start <= breakIfHandle && breakIfHandle < emit.Range.End {
				foundEmitCovering = true
			}
		}
	}
	if !foundEmitCovering {
		t.Errorf("continuing block has no Emit covering break-if handle %d; "+
			"break-if sub-expressions must be emitted in the continuing block "+
			"(matching Rust naga IR)", breakIfHandle)
	}
}

// TestAtomicTypeAlignment verifies that AtomicType has correct alignment/size.
func TestAtomicTypeAlignment(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage,read_write> a: array<atomic<u32>, 4>;
@compute @workgroup_size(1)
fn main() {
    atomicStore(&a[1], 42u);
}
`
	module, err := compileWGSL(t, source)
	if err != nil {
		t.Fatalf("parse+lower failed: %v", err)
	}

	// Find the array type
	for _, typ := range module.Types {
		if arr, ok := typ.Inner.(ir.ArrayType); ok {
			if arr.Stride != 4 {
				t.Errorf("atomic<u32> array stride = %d, want 4", arr.Stride)
			}
		}
	}
}

// TestBindingBufferArrayStorageSpace verifies that binding arrays of buffer types
// (structs) use the declared address space (Storage), not Handle space.
// Binding arrays of opaque resources (textures, samplers) should use Handle space.
func TestBindingBufferArrayStorageSpace(t *testing.T) {
	source := `
struct Foo { x: u32 }
@group(0) @binding(0) var<storage, read> buf_array: binding_array<Foo, 2>;
@group(0) @binding(1) var tex_array: binding_array<texture_2d<f32>, 2>;
@compute @workgroup_size(1)
fn main() {}
`
	module, err := compileWGSL(t, source)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if len(module.GlobalVariables) < 2 {
		t.Fatalf("expected at least 2 global variables, got %d", len(module.GlobalVariables))
	}

	// buf_array should be Storage space (binding array of struct)
	bufArray := module.GlobalVariables[0]
	if bufArray.Name != "buf_array" {
		t.Fatalf("expected buf_array, got %s", bufArray.Name)
	}
	if bufArray.Space != ir.SpaceStorage {
		t.Errorf("binding_array<Foo> space = %v, want Storage; buffer binding arrays should NOT be Handle space", bufArray.Space)
	}

	// tex_array should be Handle space (binding array of texture)
	texArray := module.GlobalVariables[1]
	if texArray.Name != "tex_array" {
		t.Fatalf("expected tex_array, got %s", texArray.Name)
	}
	if texArray.Space != ir.SpaceHandle {
		t.Errorf("binding_array<texture_2d> space = %v, want Handle; texture binding arrays should be Handle space", texArray.Space)
	}
}

// TestMatrixAliasScalarConstructorAnonymousType verifies that when a matrix alias
// constructor with scalar args creates column vectors (grouping), the final Compose
// uses an anonymous matrix type handle, not the named alias handle.
// This matches Rust naga behavior where Mat2(1,2,3,4) produces Compose(ty=anonymous_mat2x2f).
func TestMatrixAliasScalarConstructorAnonymousType(t *testing.T) {
	source := `
alias Mat2 = mat2x2<f32>;
@compute @workgroup_size(1)
fn main() {
    let f = Mat2(1.0, 2.0, 3.0, 4.0);
}
`
	module, err := compileWGSL(t, source)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if len(module.EntryPoints) == 0 {
		t.Fatal("no entry points")
	}
	fn := &module.EntryPoints[0].Function

	// Find the Compose expression for the matrix
	var matCompose *ir.ExprCompose
	for _, expr := range fn.Expressions {
		if c, ok := expr.Kind.(ir.ExprCompose); ok {
			if int(c.Type) < len(module.Types) {
				if _, isMat := module.Types[c.Type].Inner.(ir.MatrixType); isMat {
					matCompose = &c
					break
				}
			}
		}
	}
	if matCompose == nil {
		t.Fatal("no matrix Compose expression found")
	}

	// The Compose type should be anonymous (empty name), not the named alias
	composeTypeName := module.Types[matCompose.Type].Name
	if composeTypeName != "" {
		t.Errorf("matrix Compose type name = %q, want empty (anonymous); scalar grouping should use anonymous type", composeTypeName)
	}
}

func TestCallArgumentTypeMismatch(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "vec2 passed as scalar (issue #66)",
			src: `fn takes_scalar(n: u32) -> u32 { return n; }
@compute @workgroup_size(1)
fn main() {
    let v = vec2<u32>(1u, 2u);
    takes_scalar(v);
}`,
			wantErr: "type mismatch (expected u32, got vec2<u32>)",
		},
		{
			name: "scalar passed as vec2",
			src: `fn takes_vec(v: vec2<f32>) -> vec2<f32> { return v; }
@compute @workgroup_size(1)
fn main() {
    takes_vec(1.0);
}`,
			wantErr: "type mismatch (expected vec2<f32>, got f32)",
		},
		{
			name: "vec3 passed as vec4",
			src: `fn takes_vec4(v: vec4<f32>) -> vec4<f32> { return v; }
@compute @workgroup_size(1)
fn main() {
    let v = vec3<f32>(1.0, 2.0, 3.0);
    takes_vec4(v);
}`,
			wantErr: "type mismatch (expected vec4<f32>, got vec3<f32>)",
		},
		{
			name: "wrong argument count — too many",
			src: `fn takes_one(a: u32) -> u32 { return a; }
@compute @workgroup_size(1)
fn main() {
    takes_one(1u, 2u);
}`,
			wantErr: "expects 1 argument(s), got 2",
		},
		{
			name: "wrong argument count — too few",
			src: `fn takes_two(a: u32, b: u32) -> u32 { return a + b; }
@compute @workgroup_size(1)
fn main() {
    takes_two(1u);
}`,
			wantErr: "expects 2 argument(s), got 1",
		},
		{
			name: "correct call compiles",
			src: `fn add(a: u32, b: u32) -> u32 { return a + b; }
@compute @workgroup_size(1)
fn main() {
    let x = add(1u, 2u);
}`,
			wantErr: "",
		},
		{
			name: "abstract int concretizes to u32",
			src: `fn takes_u32(n: u32) -> u32 { return n; }
@compute @workgroup_size(1)
fn main() {
    let x = takes_u32(42);
}`,
			wantErr: "",
		},
		{
			name: "abstract float concretizes to f32",
			src: `fn takes_f32(n: f32) -> f32 { return n; }
@compute @workgroup_size(1)
fn main() {
    let x = takes_f32(3.14);
}`,
			wantErr: "",
		},
		{
			name: "multiple args — second wrong",
			src: `fn mixed(a: vec2<u32>, b: u32) -> u32 { return b; }
@compute @workgroup_size(1)
fn main() {
    let a = vec2<u32>(1u, 2u);
    let b = vec2<u32>(3u, 4u);
    mixed(a, b);
}`,
			wantErr: "argument 1: type mismatch (expected u32, got vec2<u32>)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileWGSL(t, tt.src)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, but compilation succeeded", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
