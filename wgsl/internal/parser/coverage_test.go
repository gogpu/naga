package parser

import (
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// Override declarations
// -----------------------------------------------------------------------

func TestParseOverrideDecl(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"typed with default", "@id(0) override width: f32 = 640.0;"},
		{"typed no default", "override height: u32;"},
		{"untyped with default", "override scale = 2.0;"},
		{"multiple overrides", "override a: f32 = 1.0;\noverride b: i32;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := parseSource(t, tt.source)
			if len(module.Overrides) == 0 {
				t.Error("expected at least 1 override")
			}
		})
	}
}

func TestParseOverrideDeclStructure(t *testing.T) {
	source := `@id(42) override gamma: f32 = 2.2;`
	module := parseSource(t, source)
	if len(module.Overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(module.Overrides))
	}
	o := module.Overrides[0]
	if o.Name != "gamma" {
		t.Errorf("expected name 'gamma', got %q", o.Name)
	}
	if o.Init == nil {
		t.Error("expected initializer")
	}
	if len(o.Attributes) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(o.Attributes))
	}
}

// -----------------------------------------------------------------------
// Bitcast expression
// -----------------------------------------------------------------------

func TestParseBitcast(t *testing.T) {
	source := `fn f() { let x = bitcast<u32>(1.0); }`
	module := parseSource(t, source)
	if len(module.Functions) != 1 {
		t.Fatal("expected 1 function")
	}
	fn := module.Functions[0]
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Statements))
	}
	constDecl, ok := fn.Body.Statements[0].(*ConstDecl)
	if !ok {
		t.Fatalf("expected ConstDecl, got %T", fn.Body.Statements[0])
	}
	_, ok = constDecl.Init.(*BitcastExpr)
	if !ok {
		t.Fatalf("expected BitcastExpr, got %T", constDecl.Init)
	}
}

// -----------------------------------------------------------------------
// Increment / Decrement operators
// -----------------------------------------------------------------------

func TestParseIncrementDecrement(t *testing.T) {
	tests := []struct {
		name string
		src  string
		op   TokenKind
	}{
		{"increment", "fn f() { var x: i32 = 0; x++; }", TokenPlusEqual},
		{"decrement", "fn f() { var x: i32 = 0; x--; }", TokenMinusEqual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := parseSource(t, tt.src)
			fn := module.Functions[0]
			// var decl + assign(++/--)
			if len(fn.Body.Statements) < 2 {
				t.Fatalf("expected at least 2 statements, got %d", len(fn.Body.Statements))
			}
			assign, ok := fn.Body.Statements[1].(*AssignStmt)
			if !ok {
				t.Fatalf("expected AssignStmt, got %T", fn.Body.Statements[1])
			}
			if assign.Op != tt.op {
				t.Errorf("expected op %v, got %v", tt.op, assign.Op)
			}
		})
	}
}

// -----------------------------------------------------------------------
// All assignment operators
// -----------------------------------------------------------------------

func TestParseAllAssignOps(t *testing.T) {
	ops := []struct {
		symbol string
		kind   TokenKind
	}{
		{"=", TokenEqual},
		{"+=", TokenPlusEqual},
		{"-=", TokenMinusEqual},
		{"*=", TokenStarEqual},
		{"/=", TokenSlashEqual},
		{"%=", TokenPercentEqual},
		{"&=", TokenAmpEqual},
		{"|=", TokenPipeEqual},
		{"^=", TokenCaretEqual},
		{"<<=", TokenLessLessEqual},
		{">>=", TokenGreaterGreaterEqual},
	}
	for _, op := range ops {
		t.Run(op.symbol, func(t *testing.T) {
			source := "fn f() { var x: i32 = 0; x " + op.symbol + " 1; }"
			module := parseSource(t, source)
			fn := module.Functions[0]
			if len(fn.Body.Statements) < 2 {
				t.Fatalf("expected at least 2 statements, got %d", len(fn.Body.Statements))
			}
			assign, ok := fn.Body.Statements[1].(*AssignStmt)
			if !ok {
				t.Fatalf("expected AssignStmt, got %T", fn.Body.Statements[1])
			}
			if assign.Op != op.kind {
				t.Errorf("expected op %v, got %v", op.kind, assign.Op)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Expression statement (function call as statement)
// -----------------------------------------------------------------------

func TestParseExprStmt(t *testing.T) {
	source := `fn f() { foo(1, 2); }`
	module := parseSource(t, source)
	fn := module.Functions[0]
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Statements))
	}
	_, ok := fn.Body.Statements[0].(*ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", fn.Body.Statements[0])
	}
}

// -----------------------------------------------------------------------
// Type specs: atomic, sampler, sampler_comparison, binding_array
// -----------------------------------------------------------------------

func TestParseAtomicType(t *testing.T) {
	source := `var<workgroup> counter: atomic<u32>;`
	module := parseSource(t, source)
	if len(module.GlobalVars) != 1 {
		t.Fatal("expected 1 global var")
	}
	named, ok := module.GlobalVars[0].Type.(*NamedType)
	if !ok {
		t.Fatalf("expected NamedType, got %T", module.GlobalVars[0].Type)
	}
	if named.Name != "atomic" {
		t.Errorf("expected 'atomic', got %q", named.Name)
	}
}

func TestParseSamplerTypes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"sampler", "@group(0) @binding(0) var s: sampler;"},
		{"sampler_comparison", "@group(0) @binding(0) var s: sampler_comparison;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

func TestParseBindingArrayType(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		hasSize bool
	}{
		{
			"with size",
			"@group(0) @binding(0) var textures: binding_array<texture_2d<f32>, 4>;",
			true,
		},
		{
			"without size",
			"@group(0) @binding(0) var textures: binding_array<texture_2d<f32>>;",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := parseSource(t, tt.source)
			if len(module.GlobalVars) != 1 {
				t.Fatal("expected 1 global var")
			}
			baType, ok := module.GlobalVars[0].Type.(*BindingArrayType)
			if !ok {
				t.Fatalf("expected BindingArrayType, got %T", module.GlobalVars[0].Type)
			}
			if (baType.Size != nil) != tt.hasSize {
				t.Errorf("hasSize = %v, want %v", baType.Size != nil, tt.hasSize)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Pointer type with access mode
// -----------------------------------------------------------------------

func TestParsePtrTypeWithAccessMode(t *testing.T) {
	source := `fn f(p: ptr<storage, f32, read_write>) {}`
	module := parseSource(t, source)
	fn := module.Functions[0]
	if len(fn.Params) != 1 {
		t.Fatal("expected 1 parameter")
	}
	ptrType, ok := fn.Params[0].Type.(*PtrType)
	if !ok {
		t.Fatalf("expected PtrType, got %T", fn.Params[0].Type)
	}
	if ptrType.AccessMode != "read_write" {
		t.Errorf("expected access mode 'read_write', got %q", ptrType.AccessMode)
	}
}

// -----------------------------------------------------------------------
// Texture type keywords
// -----------------------------------------------------------------------

func TestParseTextureTypes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"texture_1d", "@group(0) @binding(0) var t: texture_1d<f32>;"},
		{"texture_2d", "@group(0) @binding(0) var t: texture_2d<f32>;"},
		{"texture_2d_array", "@group(0) @binding(0) var t: texture_2d_array<f32>;"},
		{"texture_3d", "@group(0) @binding(0) var t: texture_3d<f32>;"},
		{"texture_cube", "@group(0) @binding(0) var t: texture_cube<f32>;"},
		{"texture_cube_array", "@group(0) @binding(0) var t: texture_cube_array<f32>;"},
		{"texture_multisampled_2d", "@group(0) @binding(0) var t: texture_multisampled_2d<f32>;"},
		{"texture_depth_2d", "@group(0) @binding(0) var t: texture_depth_2d;"},
		{"texture_depth_2d_array", "@group(0) @binding(0) var t: texture_depth_2d_array;"},
		{"texture_depth_cube", "@group(0) @binding(0) var t: texture_depth_cube;"},
		{"texture_depth_cube_array", "@group(0) @binding(0) var t: texture_depth_cube_array;"},
		{"texture_depth_multisampled_2d", "@group(0) @binding(0) var t: texture_depth_multisampled_2d;"},
		{"texture_storage_1d", "@group(0) @binding(0) var t: texture_storage_1d<rgba8unorm, write>;"},
		{"texture_storage_2d", "@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;"},
		{"texture_storage_2d_array", "@group(0) @binding(0) var t: texture_storage_2d_array<rgba8unorm, write>;"},
		{"texture_storage_3d", "@group(0) @binding(0) var t: texture_storage_3d<rgba8unorm, write>;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Type constructor expressions (vec, mat) — primary() type keyword path
// -----------------------------------------------------------------------

func TestParseTypeConstructorExpressions(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"vec2", "fn f() { let x = vec2<f32>(1.0, 2.0); }"},
		{"vec3", "fn f() { let x = vec3<f32>(1.0, 2.0, 3.0); }"},
		{"vec4", "fn f() { let x = vec4<f32>(1.0, 2.0, 3.0, 4.0); }"},
		{"mat2x2", "fn f() { let x = mat2x2<f32>(1.0, 0.0, 0.0, 1.0); }"},
		{"mat3x3", "fn f() { let x = mat3x3<f32>(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0); }"},
		{"bool", "fn f() { let x = bool(1); }"},
		{"i32", "fn f() { let x = i32(1.0); }"},
		{"u32", "fn f() { let x = u32(1); }"},
		{"f32", "fn f() { let x = f32(1); }"},
		{"f16", "fn f() { let x = f16(1.0); }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Unary operators: address-of (&), dereference (*), bitwise-not (~)
// -----------------------------------------------------------------------

func TestParseUnaryOps(t *testing.T) {
	tests := []struct {
		name string
		src  string
		op   TokenKind
	}{
		{"address_of", "fn f() { var x: i32; let p = &x; }", TokenAmpersand},
		{"deref", "fn f(p: ptr<function, i32>) { let v = *p; }", TokenStar},
		{"bitwise_not", "fn f() { let x = ~0u; }", TokenTilde},
		{"negate", "fn f() { let x = -1.0; }", TokenMinus},
		{"logical_not", "fn f() { let x = !true; }", TokenBang},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.src)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// DependencyOrder
// -----------------------------------------------------------------------

func TestDependencyOrderSimple(t *testing.T) {
	decls := []Decl{
		&ConstDecl{Name: "B", Init: &Ident{Name: "A"}},
		&ConstDecl{Name: "A", Init: &Literal{Kind: TokenIntLiteral, Value: "1"}},
	}
	ordered := DependencyOrder(decls)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(ordered))
	}
	// A should come before B since B depends on A
	first, ok := ordered[0].(*ConstDecl)
	if !ok {
		t.Fatalf("expected ConstDecl, got %T", ordered[0])
	}
	if first.Name != "A" {
		t.Errorf("expected A first, got %s", first.Name)
	}
}

func TestDependencyOrderCycle(t *testing.T) {
	// Cycles should not cause infinite loops
	decls := []Decl{
		&ConstDecl{Name: "A", Init: &Ident{Name: "B"}},
		&ConstDecl{Name: "B", Init: &Ident{Name: "A"}},
	}
	ordered := DependencyOrder(decls)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(ordered))
	}
}

func TestDependencyOrderEmpty(t *testing.T) {
	ordered := DependencyOrder(nil)
	if len(ordered) != 0 {
		t.Errorf("expected 0 decls, got %d", len(ordered))
	}
}

func TestDependencyOrderNoDeps(t *testing.T) {
	// No dependencies — source order preserved
	decls := []Decl{
		&ConstDecl{Name: "X", Init: &Literal{Kind: TokenIntLiteral, Value: "1"}},
		&ConstDecl{Name: "Y", Init: &Literal{Kind: TokenIntLiteral, Value: "2"}},
		&ConstDecl{Name: "Z", Init: &Literal{Kind: TokenIntLiteral, Value: "3"}},
	}
	ordered := DependencyOrder(decls)
	if len(ordered) != 3 {
		t.Fatalf("expected 3, got %d", len(ordered))
	}
	names := []string{
		ordered[0].(*ConstDecl).Name,
		ordered[1].(*ConstDecl).Name,
		ordered[2].(*ConstDecl).Name,
	}
	if names[0] != "X" || names[1] != "Y" || names[2] != "Z" {
		t.Errorf("expected source order X,Y,Z, got %v", names)
	}
}

func TestDependencyOrderStructDeps(t *testing.T) {
	// Struct B uses struct A as member type
	decls := []Decl{
		&StructDecl{
			Name: "B",
			Members: []*StructMember{
				{Name: "a", Type: &NamedType{Name: "A"}},
			},
		},
		&StructDecl{
			Name:    "A",
			Members: []*StructMember{{Name: "x", Type: &NamedType{Name: "f32"}}},
		},
	}
	ordered := DependencyOrder(decls)
	// A should come first
	first := ordered[0].(*StructDecl)
	if first.Name != "A" {
		t.Errorf("expected A first, got %s", first.Name)
	}
}

func TestDependencyOrderFunctionDeps(t *testing.T) {
	decls := []Decl{
		&FunctionDecl{
			Name: "bar",
			Body: &BlockStmt{
				Statements: []Stmt{
					&ExprStmt{Expr: &CallExpr{Func: &Ident{Name: "foo"}, Args: nil}},
				},
			},
		},
		&FunctionDecl{
			Name: "foo",
			Body: &BlockStmt{
				Statements: []Stmt{
					&ReturnStmt{},
				},
			},
		},
	}
	ordered := DependencyOrder(decls)
	first := ordered[0].(*FunctionDecl)
	if first.Name != "foo" {
		t.Errorf("expected foo first, got %s", first.Name)
	}
}

func TestDependencyOrderAlias(t *testing.T) {
	decls := []Decl{
		&AliasDecl{Name: "MyVec", Type: &NamedType{Name: "MyStruct"}},
		&StructDecl{Name: "MyStruct", Members: []*StructMember{{Name: "x", Type: &NamedType{Name: "f32"}}}},
	}
	ordered := DependencyOrder(decls)
	first := ordered[0].(*StructDecl)
	if first.Name != "MyStruct" {
		t.Errorf("expected MyStruct first, got %s", first.Name)
	}
}

func TestDependencyOrderOverrideDeps(t *testing.T) {
	decls := []Decl{
		&OverrideDecl{Name: "B", Init: &Ident{Name: "A"}},
		&OverrideDecl{Name: "A", Init: &Literal{Kind: TokenIntLiteral, Value: "1"}},
	}
	ordered := DependencyOrder(decls)
	first := ordered[0].(*OverrideDecl)
	if first.Name != "A" {
		t.Errorf("expected A first, got %s", first.Name)
	}
}

func TestDependencyOrderVarDeps(t *testing.T) {
	decls := []Decl{
		&VarDecl{Name: "v", Type: &NamedType{Name: "MyStruct"}},
		&StructDecl{Name: "MyStruct", Members: []*StructMember{{Name: "x", Type: &NamedType{Name: "f32"}}}},
	}
	ordered := DependencyOrder(decls)
	first := ordered[0].(*StructDecl)
	if first.Name != "MyStruct" {
		t.Errorf("expected MyStruct first, got %s", first.Name)
	}
}

func TestDependencyOrderStmtDeps(t *testing.T) {
	// Test dependency collection through all statement types:
	// for, while, loop, switch, assign, if, block, break if, expr stmt
	decls := []Decl{
		&FunctionDecl{
			Name: "user",
			Body: &BlockStmt{
				Statements: []Stmt{
					&ForStmt{
						Init:      &VarDecl{Name: "i", Type: &NamedType{Name: "i32"}, Init: &Ident{Name: "START"}},
						Condition: &BinaryExpr{Left: &Ident{Name: "i"}, Op: TokenLess, Right: &Ident{Name: "END"}},
						Update:    &AssignStmt{Left: &Ident{Name: "i"}, Op: TokenPlusEqual, Right: &Literal{Kind: TokenIntLiteral, Value: "1"}},
						Body:      &BlockStmt{},
					},
					&WhileStmt{
						Condition: &Ident{Name: "cond"},
						Body:      &BlockStmt{},
					},
					&LoopStmt{
						Body:       &BlockStmt{},
						Continuing: &BlockStmt{Statements: []Stmt{&BreakIfStmt{Condition: &Ident{Name: "done"}}}},
					},
					&SwitchStmt{
						Selector: &Ident{Name: "val"},
						Cases: []*SwitchCaseClause{
							{Selectors: []Expr{&Ident{Name: "CASE_A"}}, Body: &BlockStmt{}},
						},
					},
					&IfStmt{
						Condition: &Ident{Name: "flag"},
						Body:      &BlockStmt{},
						Else: &BlockStmt{
							Statements: []Stmt{&ReturnStmt{Value: &Ident{Name: "result"}}},
						},
					},
				},
			},
		},
		&ConstDecl{Name: "START", Init: &Literal{Kind: TokenIntLiteral, Value: "0"}},
	}
	ordered := DependencyOrder(decls)
	if len(ordered) != 2 {
		t.Fatalf("expected 2, got %d", len(ordered))
	}
	// START should come before user
	first := ordered[0].(*ConstDecl)
	if first.Name != "START" {
		t.Errorf("expected START first, got %s", first.Name)
	}
}

// -----------------------------------------------------------------------
// collectExprDeps — exercise all Expr branches
// -----------------------------------------------------------------------

func TestDependencyOrderExprBranches(t *testing.T) {
	// ConstructExpr, IndexExpr, MemberExpr in dep collection
	decls := []Decl{
		&FunctionDecl{
			Name: "user",
			Body: &BlockStmt{
				Statements: []Stmt{
					&ExprStmt{Expr: &ConstructExpr{
						Type: &NamedType{Name: "MyStruct"},
						Args: []Expr{&Literal{Kind: TokenIntLiteral, Value: "1"}},
					}},
					&ExprStmt{Expr: &IndexExpr{
						Expr:  &Ident{Name: "arr"},
						Index: &Literal{Kind: TokenIntLiteral, Value: "0"},
					}},
					&ExprStmt{Expr: &MemberExpr{
						Expr:   &Ident{Name: "obj"},
						Member: "field",
					}},
				},
			},
		},
	}
	ordered := DependencyOrder(decls)
	if len(ordered) != 1 {
		t.Fatalf("expected 1, got %d", len(ordered))
	}
}

// -----------------------------------------------------------------------
// collectTypeRefs — Array with size, PtrType
// -----------------------------------------------------------------------

func TestDependencyOrderArrayAndPtrTypeDeps(t *testing.T) {
	decls := []Decl{
		&VarDecl{
			Name: "a",
			Type: &ArrayType{
				Element: &NamedType{Name: "MyStruct"},
				Size:    &Ident{Name: "SIZE"},
			},
		},
		&VarDecl{
			Name: "p",
			Type: &PtrType{
				AddressSpace: "function",
				PointeeType:  &NamedType{Name: "MyStruct"},
			},
		},
		&StructDecl{Name: "MyStruct", Members: []*StructMember{{Name: "x", Type: &NamedType{Name: "f32"}}}},
	}
	ordered := DependencyOrder(decls)
	// MyStruct should come before a and p
	first := ordered[0].(*StructDecl)
	if first.Name != "MyStruct" {
		t.Errorf("expected MyStruct first, got %s", first.Name)
	}
}

// -----------------------------------------------------------------------
// Error recovery and error paths
// -----------------------------------------------------------------------

func TestParseMultipleErrors(t *testing.T) {
	// Multiple errors — parser should recover
	source := `fn bad1( fn bad2( fn good() { return; }`
	module, err := tryParseSource(t, source)
	if err == nil {
		t.Fatal("expected parse errors")
	}
	if module == nil {
		t.Fatal("expected module even with errors")
	}
	// Should have recovered and parsed the good function
	if len(module.Functions) < 1 {
		t.Errorf("expected at least 1 function after recovery, got %d", len(module.Functions))
	}
}

func TestParseUnexpectedDeclaration(t *testing.T) {
	// Unexpected token at top level
	source := `+ fn f() {}`
	_, err := tryParseSource(t, source)
	if err == nil {
		t.Fatal("expected error for unexpected token")
	}
}

func TestParseMissingFunctionBody(t *testing.T) {
	source := `fn f()`
	_, err := tryParseSource(t, source)
	if err == nil {
		t.Fatal("expected error for missing function body")
	}
}

// -----------------------------------------------------------------------
// Token String() coverage
// -----------------------------------------------------------------------

func TestTokenKindString(t *testing.T) {
	tests := []struct {
		kind TokenKind
		want string
	}{
		{TokenEOF, "EOF"},
		{TokenIdent, "Ident"},
		{TokenIntLiteral, "IntLiteral"},
		{TokenFloatLiteral, "FloatLiteral"},
		{TokenBoolLiteral, "BoolLiteral"},
		{TokenPlus, "+"},
		{TokenMinus, "-"},
		{TokenStar, "*"},
		{TokenSlash, "/"},
		{TokenPercent, "%"},
		{TokenAmpersand, "&"},
		{TokenPipe, "|"},
		{TokenCaret, "^"},
		{TokenTilde, "~"},
		{TokenBang, "!"},
		{TokenEqual, "="},
		{TokenLess, "<"},
		{TokenGreater, ">"},
		{TokenDot, "."},
		{TokenComma, ","},
		{TokenColon, ":"},
		{TokenSemicolon, ";"},
		{TokenAt, "@"},
		{TokenArrow, "->"},
		{TokenPlusPlus, "++"},
		{TokenMinusMinus, "--"},
		{TokenEqualEqual, "=="},
		{TokenBangEqual, "!="},
		{TokenLessEqual, "<="},
		{TokenGreaterEqual, ">="},
		{TokenAmpAmp, "&&"},
		{TokenPipePipe, "||"},
		{TokenLessLess, "<<"},
		{TokenGreaterGreater, ">>"},
		{TokenPlusEqual, "+="},
		{TokenMinusEqual, "-="},
		{TokenStarEqual, "*="},
		{TokenSlashEqual, "/="},
		{TokenPercentEqual, "%="},
		{TokenAmpEqual, "&="},
		{TokenPipeEqual, "|="},
		{TokenCaretEqual, "^="},
		{TokenLessLessEqual, "<<="},
		{TokenGreaterGreaterEqual, ">>="},
		{TokenLeftParen, "("},
		{TokenRightParen, ")"},
		{TokenLeftBrace, "{"},
		{TokenRightBrace, "}"},
		{TokenLeftBracket, "["},
		{TokenRightBracket, "]"},
		{TokenFn, "fn"},
		{TokenStruct, "struct"},
		{TokenReturn, "return"},
		{TokenIf, "if"},
		{TokenElse, "else"},
		{TokenFor, "for"},
		{TokenWhile, "while"},
		{TokenLoop, "loop"},
		{TokenSwitch, "switch"},
		{TokenCase, "case"},
		{TokenDefault, "default"},
		{TokenBreak, "break"},
		{TokenContinue, "continue"},
		{TokenContinuing, "continuing"},
		{TokenDiscard, "discard"},
		{TokenVar, "var"},
		{TokenLet, "let"},
		{TokenConst, "const"},
		{TokenConstAssert, "const_assert"},
		{TokenAlias, "alias"},
		{TokenOverride, "override"},
		{TokenTrue, "true"},
		{TokenFalse, "false"},
		{TokenDiagnostic, "diagnostic"},
		{TokenEnable, "enable"},
		{TokenError, "Error"},
		{TokenKind(255), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("TokenKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// SourceError coverage
// -----------------------------------------------------------------------

func TestSourceErrorFormatWithContext(t *testing.T) {
	src := "fn main() {\n  let x = bad;\n}"
	err := NewSourceError("unknown identifier", Span{
		Start: Position{Line: 2, Column: 11},
	}, src)

	formatted := err.FormatWithContext()
	if !strings.Contains(formatted, "unknown identifier") {
		t.Error("expected error message in formatted output")
	}
	if !strings.Contains(formatted, "line 2:11") {
		t.Error("expected line reference in formatted output")
	}
	if !strings.Contains(formatted, "^") {
		t.Error("expected caret in formatted output")
	}
}

func TestSourceErrorNoSource(t *testing.T) {
	err := NewSourceError("test error", Span{Start: Position{Line: 1, Column: 1}}, "")
	result := err.FormatWithContext()
	if result != err.Error() {
		t.Errorf("expected Error() output when no source, got %q", result)
	}
}

func TestSourceErrorZeroLine(t *testing.T) {
	err := NewSourceError("test error", Span{}, "some source")
	if err.Error() != "test error" {
		t.Errorf("expected plain message for zero line, got %q", err.Error())
	}
}

func TestSourceErrorf(t *testing.T) {
	err := NewSourceErrorf(Span{Start: Position{Line: 1, Column: 1}}, "", "error at %s", "foo")
	if err.Message != "error at foo" {
		t.Errorf("unexpected message: %q", err.Message)
	}
}

func TestSourceErrors(t *testing.T) {
	var errs SourceErrors
	if errs.HasErrors() {
		t.Error("empty errors should not HasErrors")
	}
	if errs.Error() != "no errors" {
		t.Errorf("unexpected Error(): %q", errs.Error())
	}

	errs.AddError("first error", Span{Start: Position{Line: 1, Column: 1}}, "src")
	if !errs.HasErrors() {
		t.Error("should HasErrors after AddError")
	}
	if errs.Len() != 1 {
		t.Errorf("Len() = %d, want 1", errs.Len())
	}
	if errs.Error() != "1:1: first error" {
		t.Errorf("unexpected Error(): %q", errs.Error())
	}

	errs.Add(NewSourceError("second", Span{Start: Position{Line: 2, Column: 1}}, "src"))
	if errs.Len() != 2 {
		t.Errorf("Len() = %d, want 2", errs.Len())
	}
	if !strings.Contains(errs.Error(), "1 more") {
		t.Errorf("expected 'more errors' in %q", errs.Error())
	}

	formatted := errs.FormatAll()
	if !strings.Contains(formatted, "first error") || !strings.Contains(formatted, "second") {
		t.Errorf("FormatAll missing errors: %q", formatted)
	}
}

func TestSourceErrorOutOfRangeLine(t *testing.T) {
	err := NewSourceError("error", Span{Start: Position{Line: 100, Column: 1}}, "single line")
	result := err.FormatWithContext()
	// Should fall back to plain Error() since line 100 doesn't exist
	if result != err.Error() {
		t.Errorf("expected fallback for out-of-range line, got %q", result)
	}
}

func TestSourceErrorLargeColumn(t *testing.T) {
	err := NewSourceError("error", Span{Start: Position{Line: 1, Column: 999}}, "short")
	result := err.FormatWithContext()
	// Should still produce output (clamped column)
	if !strings.Contains(result, "^") {
		t.Error("expected caret even with large column")
	}
}

func TestSourceErrorZeroColumn(t *testing.T) {
	err := NewSourceError("error", Span{Start: Position{Line: 1, Column: 0}}, "code")
	result := err.FormatWithContext()
	if !strings.Contains(result, "^") {
		t.Error("expected caret for zero column")
	}
}

// -----------------------------------------------------------------------
// Switch with default first
// -----------------------------------------------------------------------

func TestParseSwitchDefaultFirst(t *testing.T) {
	source := `fn f(x: i32) {
    switch x {
        case default, 5: { return; }
        case 1: { return; }
    }
}`
	module := parseSource(t, source)
	fn := module.Functions[0]
	switchStmt, ok := fn.Body.Statements[0].(*SwitchStmt)
	if !ok {
		t.Fatalf("expected SwitchStmt, got %T", fn.Body.Statements[0])
	}
	if len(switchStmt.Cases) < 1 {
		t.Fatal("expected at least 1 case")
	}
	if !switchStmt.Cases[0].IsDefault {
		t.Error("first case should be default")
	}
	if !switchStmt.Cases[0].DefaultFirst {
		t.Error("first case should have DefaultFirst=true")
	}
}

// -----------------------------------------------------------------------
// Loop with continuing block
// -----------------------------------------------------------------------

func TestParseLoopContinuing(t *testing.T) {
	source := `fn f() {
    var i = 0;
    loop {
        if i >= 10 { break; }
        i++;
        continuing {
            i++;
        }
    }
}`
	module := parseSource(t, source)
	fn := module.Functions[0]
	// Find loop statement
	var loop *LoopStmt
	for _, s := range fn.Body.Statements {
		if l, ok := s.(*LoopStmt); ok {
			loop = l
			break
		}
	}
	if loop == nil {
		t.Fatal("expected loop statement")
	}
	if loop.Continuing == nil {
		t.Error("expected continuing block")
	}
	if len(loop.Continuing.Statements) == 0 {
		t.Error("expected statements in continuing block")
	}
}

// -----------------------------------------------------------------------
// For loop variants
// -----------------------------------------------------------------------

func TestParseForLoopVariants(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"full", "fn f() { for (var i = 0; i < 10; i++) { } }"},
		{"empty init", "fn f() { var i = 0; for (; i < 10; i++) { } }"},
		{"empty update", "fn f() { for (var i = 0; i < 10;) { i++; } }"},
		{"empty condition", "fn f() { for (var i = 0;; i++) { break; } }"},
		{"minimal", "fn f() { for (;;) { break; } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.source)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Array type without template args (inferred)
// -----------------------------------------------------------------------

func TestParseInferredArrayType(t *testing.T) {
	source := `fn f() { let a = array(1, 2, 3); }`
	_, err := tryParseSource(t, source)
	if err != nil {
		t.Errorf("parse error: %v", err)
	}
}

// -----------------------------------------------------------------------
// Nested generic types (exercises >> splitting)
// -----------------------------------------------------------------------

func TestParseNestedGenerics(t *testing.T) {
	source := `var<private> a: array<vec3<f32>, 4>;`
	_, err := tryParseSource(t, source)
	if err != nil {
		t.Errorf("parse error: %v", err)
	}
}

// -----------------------------------------------------------------------
// Let statement with explicit type
// -----------------------------------------------------------------------

func TestParseLetWithType(t *testing.T) {
	source := `fn f() { let x: f32 = 1.0; }`
	module := parseSource(t, source)
	fn := module.Functions[0]
	constDecl, ok := fn.Body.Statements[0].(*ConstDecl)
	if !ok {
		t.Fatalf("expected ConstDecl, got %T", fn.Body.Statements[0])
	}
	if constDecl.Type == nil {
		t.Error("expected explicit type on let")
	}
}

// -----------------------------------------------------------------------
// Global const with type annotation
// -----------------------------------------------------------------------

func TestParseGlobalConstWithType(t *testing.T) {
	source := `const PI: f32 = 3.14;`
	module := parseSource(t, source)
	if len(module.Constants) != 1 {
		t.Fatal("expected 1 constant")
	}
	c := module.Constants[0]
	if c.Type == nil {
		t.Error("expected type annotation on const")
	}
	if !c.IsConst {
		t.Error("expected IsConst = true")
	}
}

// -----------------------------------------------------------------------
// Let declaration at module scope
// -----------------------------------------------------------------------

func TestParseLetModuleScope(t *testing.T) {
	source := `let x = 42;`
	module := parseSource(t, source)
	if len(module.Constants) != 1 {
		t.Fatal("expected 1 constant")
	}
	c := module.Constants[0]
	if c.Name != "x" {
		t.Errorf("expected name 'x', got %q", c.Name)
	}
}

// -----------------------------------------------------------------------
// Global var with access mode
// -----------------------------------------------------------------------

func TestParseGlobalVarAccessMode(t *testing.T) {
	source := `@group(0) @binding(0) var<storage, read_write> buf: array<f32>;`
	module := parseSource(t, source)
	if len(module.GlobalVars) != 1 {
		t.Fatal("expected 1 global var")
	}
	v := module.GlobalVars[0]
	if v.AddressSpace != "storage" {
		t.Errorf("expected address space 'storage', got %q", v.AddressSpace)
	}
	if v.AccessMode != "read_write" {
		t.Errorf("expected access mode 'read_write', got %q", v.AccessMode)
	}
}

// -----------------------------------------------------------------------
// Comparison and equality operators
// -----------------------------------------------------------------------

func TestParseComparisonOps(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"less", "fn f() { let x = 1 < 2; }"},
		{"greater", "fn f() { let x = 2 > 1; }"},
		{"less_equal", "fn f() { let x = 1 <= 2; }"},
		{"greater_equal", "fn f() { let x = 2 >= 1; }"},
		{"equal", "fn f() { let x = 1 == 1; }"},
		{"not_equal", "fn f() { let x = 1 != 2; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.src)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Shift operators
// -----------------------------------------------------------------------

func TestParseShiftOps(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"left_shift", "fn f() { let x = 1u << 2u; }"},
		{"right_shift", "fn f() { let x = 4u >> 1u; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tryParseSource(t, tt.src)
			if err != nil {
				t.Errorf("parse error: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Struct member with multiple attributes
// -----------------------------------------------------------------------

func TestParseStructMemberMultiAttrs(t *testing.T) {
	source := `struct S {
    @align(16) @size(32) x: f32,
}`
	module := parseSource(t, source)
	if len(module.Structs) != 1 {
		t.Fatal("expected 1 struct")
	}
	if len(module.Structs[0].Members) != 1 {
		t.Fatal("expected 1 member")
	}
	if len(module.Structs[0].Members[0].Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(module.Structs[0].Members[0].Attributes))
	}
}

// -----------------------------------------------------------------------
// AST node Pos() methods (coverage for marker methods)
// -----------------------------------------------------------------------

func TestASTNodePos(t *testing.T) {
	span := Span{Start: Position{Line: 5, Column: 10}}

	nodes := []Node{
		&StructDecl{Span: span},
		&FunctionDecl{Span: span},
		&VarDecl{Span: span},
		&ConstDecl{Span: span},
		&OverrideDecl{Span: span},
		&AliasDecl{Span: span},
		&ConstAssertDecl{Span: span},
		&NamedType{Span: span},
		&ArrayType{Span: span},
		&BindingArrayType{Span: span},
		&PtrType{Span: span},
		&BlockStmt{Span: span},
		&ReturnStmt{Span: span},
		&IfStmt{Span: span},
		&ForStmt{Span: span},
		&WhileStmt{Span: span},
		&LoopStmt{Span: span},
		&BreakStmt{Span: span},
		&BreakIfStmt{Span: span},
		&ContinueStmt{Span: span},
		&DiscardStmt{Span: span},
		&AssignStmt{Span: span},
		&ExprStmt{Span: span},
		&SwitchStmt{Span: span},
		&Ident{Span: span},
		&Literal{Span: span},
		&BinaryExpr{Span: span},
		&UnaryExpr{Span: span},
		&CallExpr{Span: span},
		&IndexExpr{Span: span},
		&MemberExpr{Span: span},
		&ConstructExpr{Span: span},
		&BitcastExpr{Span: span},
	}

	for _, n := range nodes {
		pos := n.Pos()
		if pos.Start.Line != 5 || pos.Start.Column != 10 {
			t.Errorf("%T.Pos() = %+v, want line=5 col=10", n, pos)
		}
	}
}

// -----------------------------------------------------------------------
// Interface marker methods (forces coverage of declNode/stmtNode/etc)
// -----------------------------------------------------------------------

func TestASTMarkerMethods(t *testing.T) {
	// These calls just exercise the marker methods for coverage.
	// They are no-op methods that implement interfaces.
	var decls []Decl = []Decl{
		&StructDecl{},
		&FunctionDecl{},
		&VarDecl{},
		&ConstDecl{},
		&OverrideDecl{},
		&AliasDecl{},
		&ConstAssertDecl{},
	}
	for _, d := range decls {
		d.declNode()
		_ = d.Pos()
	}

	var stmts []Stmt = []Stmt{
		&VarDecl{},
		&ConstDecl{},
		&ConstAssertDecl{},
		&BlockStmt{},
		&ReturnStmt{},
		&IfStmt{},
		&ForStmt{},
		&WhileStmt{},
		&LoopStmt{},
		&BreakStmt{},
		&BreakIfStmt{},
		&ContinueStmt{},
		&DiscardStmt{},
		&AssignStmt{},
		&ExprStmt{},
		&SwitchStmt{},
	}
	for _, s := range stmts {
		s.stmtNode()
		_ = s.Pos()
	}

	var types []Type = []Type{
		&NamedType{},
		&ArrayType{},
		&BindingArrayType{},
		&PtrType{},
	}
	for _, tp := range types {
		tp.typeNode()
		_ = tp.Pos()
	}

	var exprs []Expr = []Expr{
		&Ident{},
		&Literal{},
		&BinaryExpr{},
		&UnaryExpr{},
		&CallExpr{},
		&IndexExpr{},
		&MemberExpr{},
		&ConstructExpr{},
		&BitcastExpr{},
	}
	for _, e := range exprs {
		e.exprNode()
		_ = e.Pos()
	}
}

// -----------------------------------------------------------------------
// isBuiltinName coverage
// -----------------------------------------------------------------------

func TestIsBuiltinName(t *testing.T) {
	builtins := []string{
		"bool", "i32", "u32", "f32", "f16", "vec2", "vec3", "vec4",
		"mat2x2", "mat4x4", "array", "atomic", "ptr", "sampler",
		"texture_2d", "texture_cube", "texture_storage_2d", "true", "false",
	}
	for _, b := range builtins {
		if !isBuiltinName(b) {
			t.Errorf("isBuiltinName(%q) = false, want true", b)
		}
	}

	nonBuiltins := []string{"MyStruct", "custom_type", "foo", "bar"}
	for _, nb := range nonBuiltins {
		if isBuiltinName(nb) {
			t.Errorf("isBuiltinName(%q) = true, want false", nb)
		}
	}
}

// -----------------------------------------------------------------------
// Deref assignment: *p = value
// -----------------------------------------------------------------------

func TestParseDerefAssignment(t *testing.T) {
	source := `fn f(p: ptr<function, f32>) { *p = 1.0; }`
	module := parseSource(t, source)
	fn := module.Functions[0]
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Statements))
	}
	assign, ok := fn.Body.Statements[0].(*AssignStmt)
	if !ok {
		t.Fatalf("expected AssignStmt, got %T", fn.Body.Statements[0])
	}
	unary, ok := assign.Left.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr on left, got %T", assign.Left)
	}
	if unary.Op != TokenStar {
		t.Errorf("expected * op, got %v", unary.Op)
	}
}

// -----------------------------------------------------------------------
// Semicolons between declarations
// -----------------------------------------------------------------------

func TestParseSemicolonsBetweenDecls(t *testing.T) {
	// WGSL allows optional semicolons between/after declarations
	source := `struct S { x: f32, };; const A = 1;; fn f() {};`
	module := parseSource(t, source)
	if len(module.Structs) != 1 {
		t.Errorf("expected 1 struct, got %d", len(module.Structs))
	}
	if len(module.Constants) != 1 {
		t.Errorf("expected 1 constant, got %d", len(module.Constants))
	}
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}
}

// -----------------------------------------------------------------------
// Nested block statement
// -----------------------------------------------------------------------

func TestParseNestedBlock(t *testing.T) {
	source := `fn f() { { { let x = 1; } } }`
	module := parseSource(t, source)
	fn := module.Functions[0]
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Statements))
	}
	_, ok := fn.Body.Statements[0].(*BlockStmt)
	if !ok {
		t.Fatalf("expected BlockStmt, got %T", fn.Body.Statements[0])
	}
}
