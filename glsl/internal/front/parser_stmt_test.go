package front

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// bodyOf returns the statements of the function named name.
func bodyOf(t *testing.T, declaration, name string) []astStmt {
	t.Helper()
	return funcDecl(t, wantParsed(t, versioned(declaration)), name).Body
}

// mainBody returns the statements of main's body.
func mainBody(t *testing.T, body string) []astStmt {
	t.Helper()
	return bodyOf(t, "void main() {\n"+body+"\n}", "main")
}

// onlyStatement returns the single statement of main's body.
func onlyStatement(t *testing.T, body string) astStmt {
	t.Helper()
	stmts := mainBody(t, body)
	if len(stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(stmts))
	}
	return stmts[0]
}

// wantStmtKind asserts the kind of a statement and returns it.
func wantStmtKind[T any](t *testing.T, stmt astStmt) T {
	t.Helper()
	kind, ok := stmt.Kind.(T)
	if !ok {
		var want T
		t.Fatalf("statement kind = %T, want %T", stmt.Kind, want)
	}
	return kind
}

// rootKind returns the kind of the expression an expression statement evaluates.
func rootKind(t *testing.T, stmt astStmt) hirExpr {
	t.Helper()
	expr := wantStmtKind[stmtExpr](t, stmt).Expr
	return expr.Arena.get(expr.Root).Kind
}

func TestParseEmptyStatement(t *testing.T) {
	stmt := onlyStatement(t, ";")
	wantStmtKind[stmtEmpty](t, stmt)
	if !stmt.Span.IsDefined() {
		t.Error("the empty statement has no span")
	}
}

func TestParseBlockStatement(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		block := wantStmtKind[stmtBlock](t, onlyStatement(t, "{ }"))
		if len(block.Body) != 0 {
			t.Errorf("%d statements, want none", len(block.Body))
		}
	})
	t.Run("declaration and expression", func(t *testing.T) {
		block := wantStmtKind[stmtBlock](t, onlyStatement(t, "{ float a; a = 1.0; }"))
		if len(block.Body) != 2 {
			t.Fatalf("%d statements, want 2", len(block.Body))
		}
		wantStmtKind[stmtDecl](t, block.Body[0])
		wantStmtKind[stmtExpr](t, block.Body[1])
	})
	t.Run("nested", func(t *testing.T) {
		outer := wantStmtKind[stmtBlock](t, onlyStatement(t, "{ { } }"))
		if len(outer.Body) != 1 {
			t.Fatalf("%d statements, want 1", len(outer.Body))
		}
		wantStmtKind[stmtBlock](t, outer.Body[0])
	})
}

func TestParseIfStatement(t *testing.T) {
	t.Run("with else", func(t *testing.T) {
		iff := wantStmtKind[stmtIf](t, onlyStatement(t, "if (1 > 0) { discard; } else { break; }"))
		if _, ok := rootKind(t, astStmt{Kind: stmtExpr{Expr: iff.Cond}}).(hirBinary); !ok {
			t.Errorf("condition root = %T, want a binary expression",
				rootKind(t, astStmt{Kind: stmtExpr{Expr: iff.Cond}}))
		}
		if iff.Accept == nil || iff.Reject == nil {
			t.Fatalf("accept = %v, reject = %v, want both branches", iff.Accept, iff.Reject)
		}
		accept := wantStmtKind[stmtBlock](t, *iff.Accept)
		if len(accept.Body) != 1 {
			t.Errorf("%d accept statements, want 1", len(accept.Body))
		}
		wantStmtKind[stmtDiscard](t, accept.Body[0])
		reject := wantStmtKind[stmtBlock](t, *iff.Reject)
		if len(reject.Body) != 1 {
			t.Errorf("%d reject statements, want 1", len(reject.Body))
		}
		wantStmtKind[stmtBreak](t, reject.Body[0])
	})

	t.Run("without else", func(t *testing.T) {
		iff := wantStmtKind[stmtIf](t, onlyStatement(t, "if (1) discard;"))
		if iff.Reject != nil {
			t.Errorf("reject = %v, want none", iff.Reject)
		}
		wantStmtKind[stmtDiscard](t, *iff.Accept)
	})
}

func TestParseDanglingElse(t *testing.T) {
	// The else binds to the inner if, so the outer one has no reject branch.
	outer := wantStmtKind[stmtIf](t, onlyStatement(t, "if (1) if (0) discard; else break;"))
	if outer.Reject != nil {
		t.Fatalf("outer reject = %v, want none", outer.Reject)
	}
	inner := wantStmtKind[stmtIf](t, *outer.Accept)
	if inner.Reject == nil {
		t.Fatal("inner reject is missing; the else bound to the outer if")
	}
	wantStmtKind[stmtBreak](t, *inner.Reject)
}

func TestParseSwitchStatement(t *testing.T) {
	t.Run("labels and bodies", func(t *testing.T) {
		stmts := mainBody(t, "int x = 0;\n"+
			"switch (x) {\n"+
			"case 0:\n\tx = 1;\n\tbreak;\n"+
			"case 1:\n"+
			"default:\n\tx = 2;\n"+
			"}")
		if len(stmts) != 2 {
			t.Fatalf("%d statements, want a declaration and a switch", len(stmts))
		}
		sw := wantStmtKind[stmtSwitch](t, stmts[1])
		if len(sw.Cases) != 3 {
			t.Fatalf("%d cases, want 3", len(sw.Cases))
		}
		if sw.Cases[0].Label == nil {
			t.Error("case 0 has no label expression")
		}
		if got := len(sw.Cases[0].Body); got != 2 {
			t.Errorf("case 0 has %d statements, want 2", got)
		}
		if sw.Cases[1].Label == nil {
			t.Error("case 1 has no label expression")
		}
		if got := len(sw.Cases[1].Body); got != 0 {
			t.Errorf("case 1 has %d statements, want none", got)
		}
		if sw.Cases[2].Label != nil {
			t.Error("the default label carries a label expression")
		}
		if got := len(sw.Cases[2].Body); got != 1 {
			t.Errorf("default has %d statements, want 1", got)
		}
	})

	t.Run("no cases", func(t *testing.T) {
		sw := wantStmtKind[stmtSwitch](t, onlyStatement(t, "switch (1) { }"))
		if len(sw.Cases) != 0 {
			t.Errorf("%d cases, want none", len(sw.Cases))
		}
	})

	t.Run("constant case label", func(t *testing.T) {
		sw := wantStmtKind[stmtSwitch](t, onlyStatement(t, "switch (1) { case 3 + 4: break; }"))
		label := sw.Cases[0].Label
		if _, ok := label.Arena.get(label.Root).Kind.(hirBinary); !ok {
			t.Errorf("label root = %T, want a binary expression", label.Arena.get(label.Root).Kind)
		}
	})
}

func TestParseWhileStatement(t *testing.T) {
	whileStmt := wantStmtKind[stmtWhile](t, onlyStatement(t, "while (true) { }"))
	body := wantStmtKind[stmtBlock](t, whileStmt.Body)
	if len(body.Body) != 0 {
		t.Errorf("%d body statements, want none", len(body.Body))
	}
	if _, ok := whileStmt.Cond.Arena.get(whileStmt.Cond.Root).Kind.(hirLiteral); !ok {
		t.Errorf("condition root = %T, want a literal", whileStmt.Cond.Arena.get(whileStmt.Cond.Root).Kind)
	}
}

func TestParseDoStatement(t *testing.T) {
	doStmt := wantStmtKind[stmtDo](t, onlyStatement(t, "do { break; } while (false);"))
	body := wantStmtKind[stmtBlock](t, doStmt.Body)
	if len(body.Body) != 1 {
		t.Errorf("%d body statements, want 1", len(body.Body))
	}
	literal, ok := doStmt.Cond.Arena.get(doStmt.Cond.Root).Kind.(hirLiteral)
	if !ok {
		t.Fatalf("condition root = %T, want a literal", doStmt.Cond.Arena.get(doStmt.Cond.Root).Kind)
	}
	if value, isBool := literal.Value.(ir.LiteralBool); !isBool || bool(value) {
		t.Errorf("condition = %v, want the literal false", literal.Value)
	}
}

func TestParseForStatement(t *testing.T) {
	t.Run("empty clauses", func(t *testing.T) {
		forStmt := wantStmtKind[stmtFor](t, onlyStatement(t, "for (;;) { }"))
		if forStmt.InitDecl != nil || forStmt.InitExpr != nil {
			t.Errorf("init = %v, %v, want none", forStmt.InitDecl, forStmt.InitExpr)
		}
		if forStmt.Cond != nil || forStmt.Update != nil {
			t.Errorf("cond = %v, update = %v, want none", forStmt.Cond, forStmt.Update)
		}
		wantStmtKind[stmtBlock](t, forStmt.Body)
	})

	t.Run("declaration and both expressions", func(t *testing.T) {
		forStmt := wantStmtKind[stmtFor](t, onlyStatement(t, "for (int i = 0; i < 4; i++) { }"))
		if forStmt.InitDecl == nil {
			t.Fatal("the init clause carries no declaration")
		}
		init := wantDeclKind[declVarList](t, *forStmt.InitDecl)
		if len(init.Declarators) != 1 || init.Declarators[0].Name != "i" {
			t.Errorf("declarators = %+v, want one named i", init.Declarators)
		}
		if forStmt.Cond == nil {
			t.Fatal("the condition clause is missing")
		}
		if forStmt.Update == nil {
			t.Fatal("the update clause is missing")
		}
	})

	t.Run("expression init and empty tail", func(t *testing.T) {
		stmts := mainBody(t, "int i;\nfor (i = 0; ; ) { }")
		if len(stmts) != 2 {
			t.Fatalf("%d statements, want a declaration and a for", len(stmts))
		}
		forStmt := wantStmtKind[stmtFor](t, stmts[1])
		if forStmt.InitDecl != nil || forStmt.InitExpr == nil {
			t.Errorf("init = %v, %v, want an expression", forStmt.InitDecl, forStmt.InitExpr)
		}
		if forStmt.Cond != nil || forStmt.Update != nil {
			t.Errorf("cond = %v, update = %v, want none", forStmt.Cond, forStmt.Update)
		}
	})

	t.Run("condition only", func(t *testing.T) {
		forStmt := wantStmtKind[stmtFor](t, onlyStatement(t, "for (; 1 < 2; ) { }"))
		if forStmt.InitExpr != nil || forStmt.InitDecl != nil {
			t.Errorf("init = %v, %v, want none", forStmt.InitDecl, forStmt.InitExpr)
		}
		if forStmt.Cond == nil || forStmt.Update != nil {
			t.Errorf("cond = %v, update = %v, want a condition only", forStmt.Cond, forStmt.Update)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		stmts := mainBody(t, "int i;\nfor (i = 0; i < 1; i++) ;")
		wantStmtKind[stmtFor](t, stmts[1])
		wantStmtKind[stmtEmpty](t, wantStmtKind[stmtFor](t, stmts[1]).Body)
	})
}

func TestParseReturnStatement(t *testing.T) {
	t.Run("without a value", func(t *testing.T) {
		ret := wantStmtKind[stmtReturn](t, onlyStatement(t, "return;"))
		if ret.Value != nil {
			t.Errorf("value = %v, want none", ret.Value)
		}
	})
	t.Run("with a value", func(t *testing.T) {
		ret := wantStmtKind[stmtReturn](t, onlyStatement(t, "return 1 + 2;"))
		if ret.Value == nil {
			t.Fatal("the return carries no value")
		}
		if _, ok := ret.Value.Arena.get(ret.Value.Root).Kind.(hirBinary); !ok {
			t.Errorf("value root = %T, want a binary expression", ret.Value.Arena.get(ret.Value.Root).Kind)
		}
	})
}

func TestParseJumpStatements(t *testing.T) {
	wantStmtKind[stmtBreak](t, onlyStatement(t, "break;"))
	wantStmtKind[stmtContinue](t, onlyStatement(t, "continue;"))
	wantStmtKind[stmtDiscard](t, onlyStatement(t, "discard;"))
}

func TestParseExpressionStatement(t *testing.T) {
	t.Run("binary", func(t *testing.T) {
		if _, ok := rootKind(t, onlyStatement(t, "1 + 2;")).(hirBinary); !ok {
			t.Errorf("root = %T, want a binary expression", rootKind(t, onlyStatement(t, "1 + 2;")))
		}
	})
	t.Run("assignment", func(t *testing.T) {
		stmts := mainBody(t, "float x;\nx = 1.0;")
		if _, ok := rootKind(t, stmts[1]).(hirAssign); !ok {
			t.Errorf("root = %T, want an assignment", rootKind(t, stmts[1]))
		}
	})
	t.Run("comma operator", func(t *testing.T) {
		stmts := mainBody(t, "float x;\nx = 1.0, x = 2.0;")
		if _, ok := rootKind(t, stmts[1]).(hirSequence); !ok {
			t.Errorf("root = %T, want a sequence", rootKind(t, stmts[1]))
		}
	})
}

func TestParseDeclarationStatement(t *testing.T) {
	t.Run("type name is a declaration", func(t *testing.T) {
		wantStmtKind[stmtDecl](t, onlyStatement(t, "vec4 v;"))
	})
	t.Run("type name and parenthesis is a constructor", func(t *testing.T) {
		call, ok := rootKind(t, onlyStatement(t, "vec4(1.0);")).(hirCall)
		if !ok {
			t.Fatalf("root = %T, want a call", rootKind(t, onlyStatement(t, "vec4(1.0);")))
		}
		if call.Callee.Kind != calleeTypeName {
			t.Errorf("callee kind = %v, want a builtin type constructor", call.Callee.Kind)
		}
	})
	t.Run("struct definition and use", func(t *testing.T) {
		stmts := mainBody(t, "struct S { float x; };\nS s;")
		if len(stmts) != 2 {
			t.Fatalf("%d statements, want 2", len(stmts))
		}
		decl := wantStmtKind[stmtDecl](t, stmts[0])
		list := wantDeclKind[declVarList](t, decl.Decl)
		if list.Typ.StructDef == nil {
			t.Error("the first statement carries no struct definition")
		}
	})
	t.Run("qualified declaration", func(t *testing.T) {
		wantStmtKind[stmtDecl](t, onlyStatement(t, "const float k = 1.0;"))
	})
}

func TestParseStatementSpans(t *testing.T) {
	src := versioned("void main() { return 1; }")
	returnStmt := onlyStatementFrom(t, src, "main")
	if got := src[returnStmt.Span.Start:returnStmt.Span.End]; got != "return 1;" {
		t.Errorf("return span covers %q, want %q", got, "return 1;")
	}

	ifSrc := versioned("void main() { if (1) { } else { } }")
	ifStmt := onlyStatementFrom(t, ifSrc, "main")
	if got := ifSrc[ifStmt.Span.Start:ifStmt.Span.End]; got != "if (1) { } else { }" {
		t.Errorf("if span covers %q, want %q", got, "if (1) { } else { }")
	}

	blockSrc := versioned("void main() { { discard; } }")
	block := onlyStatementFrom(t, blockSrc, "main")
	if got := blockSrc[block.Span.Start:block.Span.End]; got != "{ discard; }" {
		t.Errorf("block span covers %q, want %q", got, "{ discard; }")
	}
}

// onlyStatementFrom returns the single statement of the named function of src,
// which is parsed as given.
func onlyStatementFrom(t *testing.T, src, name string) astStmt {
	t.Helper()
	stmts := funcDecl(t, wantParsed(t, src), name).Body
	if len(stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(stmts))
	}
	return stmts[0]
}

func TestStatementDepthLimit(t *testing.T) {
	depth := maxStatementDepth + 1
	src := versioned("void main() {" + strings.Repeat("{", depth) + strings.Repeat("}", depth) + "}")
	wantParseError(t, src, errStatementDepth)
}

func TestParseStatementErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "declaration without a semicolon",
			body: "float x = 1.0 }",
			want: []string{"Expected Comma or Semicolon, found RightBrace"},
		},
		{
			name: "if without a parenthesis",
			body: "if 1 discard;",
			want: []string{"Expected LeftParen, found IntConstant(Integer { value: 1, signed: true, width: 32 })"},
		},
		{
			name: "switch with a non-label",
			body: "switch (1) { break; }",
			want: []string{"Expected Case, Default or RightBrace, found Break"},
		},
		{
			name: "case without a colon",
			body: "switch (1) { case 0 }",
			want: []string{"Expected Colon, found RightBrace"},
		},
		{
			name: "for declaration without a semicolon",
			body: "for (int i = 0 i < 1; i++) { }",
			want: []string{"Expected Comma or Semicolon, found Identifier(\"i\")"},
		},
		{
			name: "unterminated block",
			body: "while (true) {",
			want: []string{"Unexpected end of file"},
		},
		{
			name: "do without while",
			body: "do { } (false);",
			want: []string{"Expected While, found LeftParen"},
		},
		{
			name: "return without a semicolon",
			body: "return 1 }",
			want: []string{"Expected Semicolon, found RightBrace"},
		},
		{
			name: "break without a semicolon",
			body: "break }",
			want: []string{"Expected Semicolon, found RightBrace"},
		},
		{
			name: "type name before a brace",
			body: "float",
			want: []string{"Expected identifier or Semicolon, found RightBrace"},
		},
		{
			name: "case outside a switch",
			body: "case 0: break;",
			want: []string{"Expected LeftParen, integer literal, float literal or bool literal, found Case"},
		},
		{
			name: "else without if",
			body: "if (1) else break;",
			want: []string{"Expected LeftParen, integer literal, float literal or bool literal, found Else"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantParseError(t, versioned("void main() {\n"+tt.body+"\n}"), tt.want...)
		})
	}

	t.Run("type name at the end of the stream", func(t *testing.T) {
		// The statement parser consumes a type name to tell a declaration from
		// a constructor, so the stream may end there.
		wantParseError(t, versioned("void main() { float"), "Unexpected end of file")
	})
}

func TestParseStatementErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"if condition without an expression", "if () discard;", "Expected LeftParen, integer " +
			"literal, float literal or bool literal, found RightParen"},
		{"if condition without a closing parenthesis", "if (1 discard;", "Expected RightParen, found Discard"},
		{"if without an accept statement", "if (1) }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightBrace"},
		{"if without a reject statement", "if (1) { } else }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightBrace"},
		{"switch without a brace", "switch (1) 1;", "Expected LeftBrace, found IntConstant(Integer { " +
			"value: 1, signed: true, width: 32 })"},
		{"case label without an expression", "switch (1) { case : }", "Expected LeftParen, integer " +
			"literal, float literal or bool literal, found Colon"},
		{"case body without a semicolon", "switch (1) { case 0: 1 }", "Expected Semicolon, found RightBrace"},
		{"while condition without a closing parenthesis", "while (1 discard;",
			"Expected RightParen, found Discard"},
		{"while without a body", "while (1) }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightBrace"},
		{"do without a body", "do } while (1);", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightBrace"},
		{"do condition without a closing parenthesis", "do { } while (1 discard;",
			"Expected RightParen, found Discard"},
		{"do without a semicolon", "do { } while (1)", "Expected Semicolon, found RightBrace"},
		{"for without a parenthesis", "for 1;", "Expected LeftParen, found IntConstant(Integer { " +
			"value: 1, signed: true, width: 32 })"},
		{"for init expression", "for (1 + ;;) { }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found Semicolon"},
		{"for condition expression", "for (; 1 + ;) { }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found Semicolon"},
		{"for update expression", "for (;; 1 +) { }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightParen"},
		{"for condition without a semicolon", "for (; 1 < 2 1) { }", "Expected Semicolon, " +
			"found IntConstant(Integer { value: 1, signed: true, width: 32 })"},
		{"for without a closing parenthesis", "for (;; 1", "Expected RightParen, found RightBrace"},
		{"for without a body", "for (;;) }", "Expected LeftParen, integer literal, " +
			"float literal or bool literal, found RightBrace"},
		{"expression statement without a semicolon", "1 + 2 }", "Expected Semicolon, found RightBrace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantParseError(t, versioned("void main() {\n"+tt.body+"\n}"), tt.want)
		})
	}
}
