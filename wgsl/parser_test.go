package wgsl

import (
	"testing"
)

// Helper function to parse source code
func parseSource(t *testing.T, source string) *Module {
	t.Helper()
	lexer := NewLexer(source)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatalf("Lexer error: %v", lexErr)
	}
	parser := NewParser(tokens)
	module, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	return module
}

// Helper function to try parsing (may return error)
func tryParseSource(t *testing.T, source string) (*Module, error) {
	t.Helper()
	lexer := NewLexer(source)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		return nil, lexErr
	}
	parser := NewParser(tokens)
	return parser.Parse()
}

func TestParseSimpleVertexShader(t *testing.T) {
	// Test from TASK-013
	source := `@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return vec4<f32>(pos, 1.0);
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got %q", fn.Name)
	}

	if len(fn.Attributes) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(fn.Attributes))
	} else if fn.Attributes[0].Name != "vertex" {
		t.Errorf("expected attribute 'vertex', got %q", fn.Attributes[0].Name)
	}

	if len(fn.Params) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(fn.Params))
	} else {
		param := fn.Params[0]
		if param.Name != "pos" {
			t.Errorf("expected parameter name 'pos', got %q", param.Name)
		}
		if len(param.Attributes) != 1 {
			t.Errorf("expected 1 parameter attribute, got %d", len(param.Attributes))
		}
	}

	if fn.ReturnType == nil {
		t.Error("expected return type, got nil")
	}

	if fn.Body == nil {
		t.Error("expected function body, got nil")
	} else if len(fn.Body.Statements) != 1 {
		t.Errorf("expected 1 statement, got %d", len(fn.Body.Statements))
	}
}

func TestParseStructDeclaration(t *testing.T) {
	source := `struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}`

	module := parseSource(t, source)

	if len(module.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(module.Structs))
	}

	s := module.Structs[0]
	if s.Name != "VertexOutput" {
		t.Errorf("expected struct name 'VertexOutput', got %q", s.Name)
	}

	if len(s.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(s.Members))
	}

	// Check first member
	if s.Members[0].Name != "position" {
		t.Errorf("expected first member 'position', got %q", s.Members[0].Name)
	}
	if len(s.Members[0].Attributes) != 1 {
		t.Errorf("expected 1 attribute on position, got %d", len(s.Members[0].Attributes))
	}

	// Check second member
	if s.Members[1].Name != "uv" {
		t.Errorf("expected second member 'uv', got %q", s.Members[1].Name)
	}
}

func TestParseGlobalVariable(t *testing.T) {
	source := `@group(0) @binding(0) var<uniform> transform: mat4x4<f32>;`

	module := parseSource(t, source)

	if len(module.GlobalVars) != 1 {
		t.Fatalf("expected 1 global variable, got %d", len(module.GlobalVars))
	}

	v := module.GlobalVars[0]
	if v.Name != "transform" {
		t.Errorf("expected variable name 'transform', got %q", v.Name)
	}
	if v.AddressSpace != "uniform" {
		t.Errorf("expected address space 'uniform', got %q", v.AddressSpace)
	}
	if len(v.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(v.Attributes))
	}
}

func TestParseConstDeclaration(t *testing.T) {
	source := `const PI: f32 = 3.14159;`

	module := parseSource(t, source)

	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant, got %d", len(module.Constants))
	}

	c := module.Constants[0]
	if c.Name != "PI" {
		t.Errorf("expected constant name 'PI', got %q", c.Name)
	}
	if c.Init == nil {
		t.Error("expected initializer, got nil")
	}
}

func TestParseCompleteVertexShader(t *testing.T) {
	// Complex shader from TASK-013
	source := `struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0) var<uniform> transform: mat4x4<f32>;

@vertex
fn vs_main(@location(0) pos: vec3<f32>, @location(1) uv: vec2<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.position = transform * vec4<f32>(pos, 1.0);
    out.uv = uv;
    return out;
}`

	module := parseSource(t, source)

	// Check struct
	if len(module.Structs) != 1 {
		t.Errorf("expected 1 struct, got %d", len(module.Structs))
	}

	// Check global variables
	if len(module.GlobalVars) != 1 {
		t.Errorf("expected 1 global variable, got %d", len(module.GlobalVars))
	}

	// Check function
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "vs_main" {
		t.Errorf("expected function name 'vs_main', got %q", fn.Name)
	}

	if len(fn.Params) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(fn.Params))
	}

	// Check body has expected statements
	if fn.Body == nil {
		t.Fatal("expected function body, got nil")
	}
	if len(fn.Body.Statements) != 4 {
		t.Errorf("expected 4 statements in body, got %d", len(fn.Body.Statements))
	}
}

func TestParseExpressions(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"binary add", "fn f() { let x = 1 + 2; }"},
		{"binary multiply", "fn f() { let x = a * b; }"},
		{"comparison", "fn f() { let x = a < b; }"},
		{"logical and", "fn f() { let x = a && b; }"},
		{"logical or", "fn f() { let x = a || b; }"},
		{"unary minus", "fn f() { let x = -y; }"},
		{"unary not", "fn f() { let x = !y; }"},
		{"member access", "fn f() { let x = a.b; }"},
		{"index access", "fn f() { let x = a[0]; }"},
		{"function call", "fn f() { let x = foo(a, b); }"},
		{"type constructor", "fn f() { let x = vec3<f32>(1.0, 2.0, 3.0); }"},
		{"complex expr", "fn f() { let x = a + b * c - d; }"},
		{"nested parens", "fn f() { let x = (a + b) * (c - d); }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("Parse error for %q: %v", tt.name, err)
			}
		})
	}
}

func TestParseStatements(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"return", "fn f() { return; }"},
		{"return value", "fn f() -> i32 { return 42; }"},
		{"var decl", "fn f() { var x: i32 = 1; }"},
		{"let decl", "fn f() { let x = 1; }"},
		{"assignment", "fn f() { x = 1; }"},
		{"compound assign", "fn f() { x += 1; }"},
		{"if", "fn f() { if true { return; } }"},
		{"if else", "fn f() { if true { return 1; } else { return 0; } }"},
		{"if else if", "fn f() { if a { } else if b { } else { } }"},
		{"for loop", "fn f() { for (var i = 0; i < 10; i += 1) { } }"},
		{"while loop", "fn f() { while x < 10 { x += 1; } }"},
		{"loop", "fn f() { loop { break; } }"},
		{"break", "fn f() { loop { break; } }"},
		{"continue", "fn f() { loop { continue; } }"},
		{"discard", "fn f() { discard; }"},
		{"block", "fn f() { { let x = 1; } }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("Parse error for %q: %v", tt.name, err)
			}
		})
	}
}

func TestParseTypeAlias(t *testing.T) {
	source := `alias Float4 = vec4<f32>;`

	module := parseSource(t, source)

	if len(module.Aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(module.Aliases))
	}

	a := module.Aliases[0]
	if a.Name != "Float4" {
		t.Errorf("expected alias name 'Float4', got %q", a.Name)
	}
}

func TestParseFragmentShader(t *testing.T) {
	source := `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, 0.0, 1.0);
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "fs_main" {
		t.Errorf("expected function name 'fs_main', got %q", fn.Name)
	}

	// Check fragment attribute
	hasFragment := false
	for _, attr := range fn.Attributes {
		if attr.Name == "fragment" {
			hasFragment = true
			break
		}
	}
	if !hasFragment {
		t.Error("expected @fragment attribute")
	}
}

func TestParseComputeShader(t *testing.T) {
	source := `@compute @workgroup_size(64, 1, 1)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
    let index = id.x;
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Name != "cs_main" {
		t.Errorf("expected function name 'cs_main', got %q", fn.Name)
	}

	// Should have @compute and @workgroup_size attributes
	if len(fn.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(fn.Attributes))
	}
}

func TestParseTextureSampler(t *testing.T) {
	source := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, uv);
}`

	module := parseSource(t, source)

	if len(module.GlobalVars) != 2 {
		t.Errorf("expected 2 global variables, got %d", len(module.GlobalVars))
	}

	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}
}

func TestParseArrayType(t *testing.T) {
	source := `var<private> arr: array<f32, 10>;`

	module := parseSource(t, source)

	if len(module.GlobalVars) != 1 {
		t.Fatalf("expected 1 global variable, got %d", len(module.GlobalVars))
	}

	v := module.GlobalVars[0]
	if v.Name != "arr" {
		t.Errorf("expected variable name 'arr', got %q", v.Name)
	}
	if v.AddressSpace != "private" {
		t.Errorf("expected address space 'private', got %q", v.AddressSpace)
	}

	arrayType, ok := v.Type.(*ArrayType)
	if !ok {
		t.Fatalf("expected ArrayType, got %T", v.Type)
	}
	if arrayType.Element == nil {
		t.Error("expected array element type")
	}
	if arrayType.Size == nil {
		t.Error("expected array size")
	}
}

func TestParseErrorRecovery(t *testing.T) {
	// Source with an error - missing function body
	source := `fn good() { return; }
fn bad(
fn another() { return; }`

	lexer := NewLexer(source)
	tokens, _ := lexer.Tokenize()
	parser := NewParser(tokens)
	module, _ := parser.Parse()

	// Parser should recover and parse what it can
	if module == nil {
		t.Fatal("expected module even with errors")
	}

	// Should have parsed at least 1 function (the first one)
	if len(module.Functions) < 1 {
		t.Errorf("expected at least 1 function, got %d", len(module.Functions))
	}
}

func TestParseEmptyModule(t *testing.T) {
	source := ``

	module := parseSource(t, source)

	if module == nil {
		t.Fatal("expected module, got nil")
	}
}

func TestParseEnableDirective(t *testing.T) {
	source := `enable f16;

fn f() -> f16 {
    return 1.0h;
}`

	module := parseSource(t, source)

	// Enable directive is skipped, function should be parsed
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}
}

func TestParseMatrixTypes(t *testing.T) {
	source := `fn f() {
    var m2: mat2x2<f32>;
    var m3: mat3x3<f32>;
    var m4: mat4x4<f32>;
    var m23: mat2x3<f32>;
    var m34: mat3x4<f32>;
}`

	_, err := tryParseSource(t, source)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
}

func TestParseBitwiseOperators(t *testing.T) {
	source := `fn f() {
    let a = x & y;
    let b = x | y;
    let c = x ^ y;
    let d = x << 2u;
    let e = x >> 2u;
    let f = ~x;
}`

	_, err := tryParseSource(t, source)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
}

func TestParsePointerType(t *testing.T) {
	source := `fn f(p: ptr<function, f32>) {
    *p = 1.0;
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if len(fn.Params) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(fn.Params))
	}

	ptrType, ok := fn.Params[0].Type.(*PtrType)
	if !ok {
		t.Fatalf("expected PtrType, got %T", fn.Params[0].Type)
	}
	if ptrType.AddressSpace != "function" {
		t.Errorf("expected address space 'function', got %q", ptrType.AddressSpace)
	}
}

// TestParseSwitchStatement tests parsing of switch statements (NAGA-002).
func TestParseSwitchStatement(t *testing.T) {
	source := `@fragment
fn main(@location(0) idx: u32) -> @location(0) vec4<f32> {
    var color: vec4<f32>;
    switch idx {
        case 0u: { color = vec4<f32>(1.0, 0.0, 0.0, 1.0); }
        case 1u: { color = vec4<f32>(0.0, 1.0, 0.0, 1.0); }
        default: { color = vec4<f32>(0.0, 0.0, 1.0, 1.0); }
    }
    return color;
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Body == nil || len(fn.Body.Statements) < 2 {
		t.Fatalf("expected at least 2 statements in function body")
	}

	// Second statement should be switch
	switchStmt, ok := fn.Body.Statements[1].(*SwitchStmt)
	if !ok {
		t.Fatalf("expected SwitchStmt, got %T", fn.Body.Statements[1])
	}

	if len(switchStmt.Cases) != 3 {
		t.Errorf("expected 3 cases, got %d", len(switchStmt.Cases))
	}

	// Check first case (0u)
	if switchStmt.Cases[0].IsDefault {
		t.Errorf("first case should not be default")
	}
	if len(switchStmt.Cases[0].Selectors) != 1 {
		t.Errorf("expected 1 selector in first case, got %d", len(switchStmt.Cases[0].Selectors))
	}

	// Check last case (default)
	if !switchStmt.Cases[2].IsDefault {
		t.Errorf("last case should be default")
	}
}

// TestParseLocalConst tests parsing of local const declarations (NAGA-002).
func TestParseLocalConst(t *testing.T) {
	source := `@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    const PI = 3.14159;
    let x = PI * 2.0;
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}`

	module := parseSource(t, source)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}

	fn := module.Functions[0]
	if fn.Body == nil || len(fn.Body.Statements) < 1 {
		t.Fatalf("expected at least 1 statement in function body")
	}

	// First statement should be const declaration
	constDecl, ok := fn.Body.Statements[0].(*ConstDecl)
	if !ok {
		t.Fatalf("expected ConstDecl, got %T", fn.Body.Statements[0])
	}

	if constDecl.Name != "PI" {
		t.Errorf("expected const name 'PI', got %q", constDecl.Name)
	}

	if constDecl.Init == nil {
		t.Errorf("const should have initializer")
	}
}
