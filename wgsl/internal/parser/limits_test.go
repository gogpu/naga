package parser

import (
	"strings"
	"testing"
)

// parseErrors lexes and parses source and returns the parser's errors.
func parseErrors(t *testing.T, source string) []ParseError {
	t.Helper()
	tokens, err := NewLexer(source).Tokenize()
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	p := NewParser(tokens)
	_, _ = p.Parse() // the error is p.Errors()[0]
	return p.Errors()
}

// nestingCase builds a source nested n levels deep in one construct.
type nestingCase struct {
	name string
	src  func(n int) string
	// maxOK is the deepest nesting Rust naga v30 (14328f49) accepts; maxOK+1
	// is refused with wantMsg. Each pair was measured with Rust's naga CLI.
	maxOK   int
	wantMsg string
}

func nestingCases() []nestingCase {
	r := strings.Repeat
	return []nestingCase{
		{
			// Global initializer: expression() at depth 1, one more per `(`.
			name:    "parens in a const initializer",
			src:     func(n int) string { return "const x: i32 = " + r("(", n) + "1" + r(")", n) + ";" },
			maxOK:   198,
			wantMsg: errInternalFrontEnd,
		},
		{
			// statement() at depth 1, the returned expression at depth 2.
			name:    "parens in a return",
			src:     func(n int) string { return "fn f() -> i32 { return " + r("(", n) + "1" + r(")", n) + "; }" },
			maxOK:   197,
			wantMsg: errInternalFrontEnd,
		},
		{
			name:    "parens on the right of an assignment",
			src:     func(n int) string { return "fn f() { var a = 0; a = " + r("(", n) + "0" + r(")", n) + "; }" },
			maxOK:   197,
			wantMsg: errInternalFrontEnd,
		},
		{
			// A for header's init is not a counted statement in Rust.
			name: "parens in a for-loop initializer",
			src: func(n int) string {
				return "fn f() { for (var i = " + r("(", n) + "0" + r(")", n) + "; i < 1; i++) {} }"
			},
			maxOK:   197,
			wantMsg: errInternalFrontEnd,
		},
		{
			name: "nested call arguments",
			src: func(n int) string {
				return "fn g(a: i32) -> i32 { return a; }\nfn f() -> i32 { return " + r("g(", n) + "1" + r(")", n) + "; }"
			},
			maxOK:   197,
			wantMsg: errInternalFrontEnd,
		},
		{
			// A call statement's arguments sit one level below the statement.
			name: "nested arguments of a call statement",
			src: func(n int) string {
				return "fn g(a: i32) -> i32 { return a; }\nfn f() { g(" + r("g(", n) + "1" + r(")", n) + "); }"
			},
			maxOK:   197,
			wantMsg: errInternalFrontEnd,
		},
		{
			name: "parens in a bitcast argument",
			src: func(n int) string {
				return "fn f() -> i32 { return bitcast<i32>(" + r("(", n) + "1" + r(")", n) + "); }"
			},
			maxOK:   196,
			wantMsg: errInternalFrontEnd,
		},
		{
			// The declared type is not counted; each template argument is.
			name:    "nested array types",
			src:     func(n int) string { return "var<private> x: " + r("array<", n) + "i32" + r(", 1>", n) + ";" },
			maxOK:   199,
			wantMsg: errInternalFrontEnd,
		},
		{
			name:    "nested array types in a constructor",
			src:     func(n int) string { return "const x = " + r("array<", n) + "i32" + r(", 1>", n) + "();" },
			maxOK:   198,
			wantMsg: errInternalFrontEnd,
		},
		{
			// The function body is brace level 1.
			name:    "nested blocks",
			src:     func(n int) string { return "fn f() { " + r("{", n) + r("}", n) + " }" },
			maxOK:   126,
			wantMsg: errBraceNestingLimit,
		},
		{
			name:    "nested ifs",
			src:     func(n int) string { return "fn f() { " + r("if true { ", n) + r("} ", n) + "}" },
			maxOK:   126,
			wantMsg: errBraceNestingLimit,
		},
		{
			name:    "nested loops",
			src:     func(n int) string { return "fn f() { " + r("loop { ", n) + r("break; } ", n) + "}" },
			maxOK:   126,
			wantMsg: errBraceNestingLimit,
		},
		{
			// A switch body and its case body are two levels.
			name:    "nested switches",
			src:     func(n int) string { return "fn f() { " + r("switch 0 { default { ", n) + r("} } ", n) + "}" },
			maxOK:   63,
			wantMsg: errBraceNestingLimit,
		},
	}
}

// chainCases are the constructs the parser reads in a loop, where Rust naga
// has no limit (its later passes overflow the stack on a long enough chain)
// and the limits are Tint's: expression depth 512 (resolver.cc
// kMaxExpressionDepth; Tint's MaxExpressionDepth_Pass/Fail tests use the same
// left-deep `b + b + …` chain) and statement chaining 127 (kMaxStatementDepth).
func chainCases() []nestingCase {
	r := strings.Repeat
	return []nestingCase{
		{
			name:    "binary operator chain",
			src:     func(n int) string { return "fn f() -> i32 { let a = 1; return a" + r(" + a", n) + "; }" },
			maxOK:   512,
			wantMsg: errExpressionDepth,
		},
		{
			name:    "chain in a global initializer",
			src:     func(n int) string { return "const x = 1" + r(" * 1", n) + ";" },
			maxOK:   512,
			wantMsg: errExpressionDepth,
		},
		{
			name:    "unary operator chain",
			src:     func(n int) string { return "fn f() -> i32 { let a = 1; return " + r("- ", n) + "a; }" },
			maxOK:   512,
			wantMsg: errExpressionDepth,
		},
		{
			name:    "logical not chain",
			src:     func(n int) string { return "fn f() -> bool { let a = true; return " + r("!", n) + "a; }" },
			maxOK:   512,
			wantMsg: errExpressionDepth,
		},
		{
			name:    "member access chain",
			src:     func(n int) string { return "fn f() { let a = v" + r(".x", n) + "; }" },
			maxOK:   512,
			wantMsg: errExpressionDepth,
		},
		{
			// The heights of a chain's operands add up: each operand here is a
			// parenthesized chain of 2, so the tree is three times as high.
			name:    "chain of parenthesized chains",
			src:     func(n int) string { return "const x = 1" + r(" + (1 + (1 + 1))", n) + ";" },
			maxOK:   510,
			wantMsg: errExpressionDepth,
		},
		{
			// The function body is level 1, the first `if` body level 2 and
			// the k-th `else if` body level 2+k.
			name:    "else-if chain",
			src:     func(n int) string { return "fn f() { if true {} " + r("else if true {} ", n) + "else {} }" },
			maxOK:   125,
			wantMsg: errStatementChaining,
		},
		{
			// A chain inside nested blocks shares their budget.
			name: "else-if chain in nested blocks",
			src: func(n int) string {
				return "fn f() { {{{{ if true {} " + r("else if true {} ", n) + "}}}} }"
			},
			maxOK:   121,
			wantMsg: errStatementChaining,
		},
	}
}

// TestNestingLimits checks Rust naga's limits: recursion depth 200
// (Parser::track_recursion) and brace nesting 127 (WGSL §2.4), at the exact
// depth where Rust starts to refuse, with Rust's texts.
func TestNestingLimits(t *testing.T) {
	for _, tc := range nestingCases() {
		t.Run(tc.name, func(t *testing.T) {
			if errs := parseErrors(t, tc.src(tc.maxOK)); len(errs) != 0 {
				t.Fatalf("depth %d: want accepted, got %v", tc.maxOK, errs[0])
			}
			errs := parseErrors(t, tc.src(tc.maxOK+1))
			if len(errs) == 0 {
				t.Fatalf("depth %d: want %q, got no error", tc.maxOK+1, tc.wantMsg)
			}
			checkLimitError(t, errs[0], tc.wantMsg)
		})
	}
}

// TestNestingLimitsHugeInput feeds a million levels of each construct. Before
// the limits existed this killed the process with Go's fatal, unrecoverable
// "stack overflow" (500 000 parentheses were enough); now each is one error.
func TestNestingLimitsHugeInput(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a few hundred MB of tokens")
	}
	const huge = 1_000_000
	for _, tc := range append(nestingCases(), chainCases()...) {
		t.Run(tc.name, func(t *testing.T) {
			errs := parseErrors(t, tc.src(huge))
			if len(errs) == 0 {
				t.Fatalf("want %q, got no error", tc.wantMsg)
			}
			checkLimitError(t, errs[0], tc.wantMsg)
		})
	}
}

func checkLimitError(t *testing.T, got ParseError, wantMsg string) {
	t.Helper()
	if got.Message != wantMsg {
		t.Fatalf("message = %q, want %q", got.Message, wantMsg)
	}
	switch wantMsg {
	case errInternalFrontEnd:
		if len(got.Notes) != 1 || got.Notes[0] != "Parser recursion limit exceeded" {
			t.Errorf("notes = %q, want Rust's [Parser recursion limit exceeded]", got.Notes)
		}
	case errBraceNestingLimit:
		if got.Token.Kind != TokenLeftBrace {
			t.Errorf("error at %s, want the `{` over the limit", got.Token.Kind)
		}
		if got.Label != "limit reached at this brace" {
			t.Errorf("label = %q, want Rust's", got.Label)
		}
		if len(got.Notes) != 1 || got.Notes[0] != "nesting limit is currently set to 127" {
			t.Errorf("notes = %q, want Rust's", got.Notes)
		}
	case errExpressionDepth, errStatementChaining:
		// Tint's texts, which carry no label or note.
		if got.Label != "" || len(got.Notes) != 0 {
			t.Errorf("label %q, notes %q: want none", got.Label, got.Notes)
		}
	}
}

// TestChainLimits: Rust reads prefix operators, operator chains and else-if
// chains in a loop and bounds none of them, so a long chain reaches its
// recursive later passes and overflows the stack there. We refuse them at
// Tint's limits instead (WGSL §2.4 lets an implementation refuse a program
// beyond its limits): the limit is accepted, one past it refused.
func TestChainLimits(t *testing.T) {
	for _, tc := range chainCases() {
		t.Run(tc.name, func(t *testing.T) {
			if errs := parseErrors(t, tc.src(tc.maxOK)); len(errs) != 0 {
				t.Fatalf("length %d: want accepted, got %v", tc.maxOK, errs[0])
			}
			errs := parseErrors(t, tc.src(tc.maxOK+1))
			if len(errs) == 0 {
				t.Fatalf("length %d: want %q, got no error", tc.maxOK+1, tc.wantMsg)
			}
			checkLimitError(t, errs[0], tc.wantMsg)
		})
	}
}

// TestUnaryChainShape: the loop must build the same tree the recursion did,
// the operator written first outermost.
func TestUnaryChainShape(t *testing.T) {
	m := parseSource(t, "fn f() -> i32 { let a = 1; return -~a; }")
	ret, ok := m.Functions[0].Body.Statements[1].(*ReturnStmt)
	if !ok {
		t.Fatalf("statement 1 is %T, want *ReturnStmt", m.Functions[0].Body.Statements[1])
	}
	outer, ok := ret.Value.(*UnaryExpr)
	if !ok || outer.Op != TokenMinus {
		t.Fatalf("outer = %#v, want unary minus", ret.Value)
	}
	inner, ok := outer.Operand.(*UnaryExpr)
	if !ok || inner.Op != TokenTilde {
		t.Fatalf("inner = %#v, want unary tilde", outer.Operand)
	}
	if id, ok := inner.Operand.(*Ident); !ok || id.Name != "a" {
		t.Fatalf("operand = %#v, want a", inner.Operand)
	}
}

// TestElseIfChainShape: the loop must nest the chain as the recursion did,
// each `else if` the Else of the one before, the final block last.
func TestElseIfChainShape(t *testing.T) {
	m := parseSource(t, "fn f() { if true {} else if false {} else if true { return; } else {} }")
	first, ok := m.Functions[0].Body.Statements[0].(*IfStmt)
	if !ok {
		t.Fatalf("statement is %T, want *IfStmt", m.Functions[0].Body.Statements[0])
	}
	second, ok := first.Else.(*IfStmt)
	if !ok {
		t.Fatalf("first.Else = %T, want *IfStmt", first.Else)
	}
	third, ok := second.Else.(*IfStmt)
	if !ok {
		t.Fatalf("second.Else = %T, want *IfStmt", second.Else)
	}
	if len(third.Body.Statements) != 1 {
		t.Fatalf("third body has %d statements, want 1", len(third.Body.Statements))
	}
	if _, ok := third.Else.(*BlockStmt); !ok {
		t.Fatalf("third.Else = %T, want *BlockStmt", third.Else)
	}

	m = parseSource(t, "fn f() { if true {} }")
	if only := m.Functions[0].Body.Statements[0].(*IfStmt); only.Else != nil {
		t.Fatalf("Else = %#v, want nil", only.Else)
	}
}
