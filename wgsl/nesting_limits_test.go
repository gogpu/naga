package wgsl

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestParseNestingLimits checks the public error of Rust naga's nesting
// limits (recursion depth 200, brace nesting 127) and that the deepest
// source Rust accepts also lowers and validates: the limits are only useful
// if nothing after the parser recurses past what they allow.
func TestParseNestingLimits(t *testing.T) {
	r := strings.Repeat
	cases := []struct {
		name      string
		src       func(n int) string
		maxOK     int // deepest Rust naga v30 accepts (measured with its CLI)
		wantMsg   string
		wantLabel string
		wantNote  string
	}{
		{
			name:     "parens",
			src:      func(n int) string { return "const x: i32 = " + r("(", n) + "1" + r(")", n) + ";" },
			maxOK:    198,
			wantMsg:  "internal WGSL front end error",
			wantNote: "Parser recursion limit exceeded",
		},
		{
			name: "calls",
			src: func(n int) string {
				return "fn g(a: i32) -> i32 { return a; }\nfn f() -> i32 { return " + r("g(", n) + "1" + r(")", n) + "; }"
			},
			maxOK:    197,
			wantMsg:  "internal WGSL front end error",
			wantNote: "Parser recursion limit exceeded",
		},
		{
			name:     "array types",
			src:      func(n int) string { return "var<private> x: " + r("array<", n) + "i32" + r(", 1>", n) + ";" },
			maxOK:    199,
			wantMsg:  "internal WGSL front end error",
			wantNote: "Parser recursion limit exceeded",
		},
		{
			name: "blocks",
			src: func(n int) string {
				return "fn f() -> i32 { var a = 0; " + r("{ a += 1; ", n) + r("} ", n) + "return a; }"
			},
			maxOK:     126,
			wantMsg:   "brace nesting limit reached",
			wantLabel: "limit reached at this brace",
			wantNote:  "nesting limit is currently set to 127",
		},
		// The chain limits go beyond Rust, which bounds none of these (they
		// overflow the stack in its later passes); the limits and texts are
		// Tint's (expression depth 512, statement chaining 127).
		{
			name:    "binary operator chain",
			src:     func(n int) string { return "fn f() -> i32 { let a = 1; return a" + r(" + a", n) + "; }" },
			maxOK:   512,
			wantMsg: "reached max expression depth of 512",
		},
		{
			name:    "unary operator chain",
			src:     func(n int) string { return "fn f() -> i32 { let a = 1; return " + r("- ", n) + "a; }" },
			maxOK:   512,
			wantMsg: "reached max expression depth of 512",
		},
		{
			name:    "else-if chain",
			src:     func(n int) string { return "fn f() { if true {} " + r("else if true {} ", n) + "else {} }" },
			maxOK:   125,
			wantMsg: "statement nesting depth / chaining length exceeds limit of 127",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod, _, err := Parse(tc.src(tc.maxOK), ParseOptions{})
			if err != nil {
				t.Fatalf("depth %d: want accepted, got %v", tc.maxOK, err)
			}
			if _, err := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("depth %d: validate: %v", tc.maxOK, err)
			}

			_, _, err = Parse(tc.src(tc.maxOK+1), ParseOptions{})
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("depth %d: want *ParseError, got %T %v", tc.maxOK+1, err, err)
			}
			if pe.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", pe.Message, tc.wantMsg)
			}
			// Tint's chain texts carry no note of their own; later notes are
			// the errors that follow from parser recovery.
			if tc.wantNote != "" && (len(pe.Notes) == 0 || pe.Notes[0] != tc.wantNote) {
				t.Errorf("notes = %q, want first %q", pe.Notes, tc.wantNote)
			}
			if len(pe.Labels) == 0 || pe.Labels[0].Text != tc.wantLabel {
				t.Errorf("labels = %+v, want first text %q", pe.Labels, tc.wantLabel)
			}
		})
	}
}

// TestParseDeepNestingDoesNotCrash: 500 000 nested parentheses used to end
// the process with Go's fatal "stack overflow", which recover() cannot catch.
func TestParseDeepNestingDoesNotCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a few hundred MB of tokens")
	}
	const huge = 1_000_000
	src := "const x: i32 = " + strings.Repeat("(", huge) + "1" + strings.Repeat(")", huge) + ";"
	_, _, err := Parse(src, ParseOptions{})
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Message != "internal WGSL front end error" {
		t.Fatalf("want the recursion-limit error, got %v", err)
	}
}

// TestParseLongChainsDoNotCrash: a million-long operator or else-if chain
// used to parse, then end the process with Go's fatal "stack overflow" in a
// recursive pass after the parser (lowering for else-if, the backends for
// operators). Each is now one parse error.
func TestParseLongChainsDoNotCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a few hundred MB of tokens")
	}
	const huge = 1_000_000
	r := strings.Repeat
	cases := []struct{ name, src, wantMsg string }{
		{"binary", "fn f() -> i32 { let a = 1; return a" + r(" + 1", huge) + "; }", "reached max expression depth of 512"},
		{"unary", "fn f() -> i32 { let a = 1; return " + r("- ", huge) + "a; }", "reached max expression depth of 512"},
		{"else-if", "fn f() { if true {} " + r("else if true {} ", huge) + "}",
			"statement nesting depth / chaining length exceeds limit of 127"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Parse(tc.src, ParseOptions{})
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Message != tc.wantMsg {
				t.Fatalf("want %q, got %v", tc.wantMsg, err)
			}
		})
	}
}
