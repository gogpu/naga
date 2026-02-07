package wgsl

import (
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

	// Verify module structure
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
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

	// Verify function
	fn := module.Functions[0]
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

	// Verify function body has a return statement
	if len(fn.Body) != 1 {
		t.Errorf("expected 1 statement in body, got %d", len(fn.Body))
	}

	if stmt, ok := fn.Body[0].Kind.(ir.StmtReturn); ok {
		if stmt.Value == nil {
			t.Error("expected return value, got nil")
		}
	} else {
		t.Errorf("expected StmtReturn, got %T", fn.Body[0].Kind)
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
