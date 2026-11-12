package wgsl

import (
	"errors"
	"strings"
	"testing"
)

// TestParseRejectsUnterminatedBlockComment: a block comment must be closed;
// Rust naga's UnterminatedBlockComment has the same message and points at the
// opening `/*`.
func TestParseRejectsUnterminatedBlockComment(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"at start", "/*"},
		{"after code", "fn f() {} /* trailing"},
		{"one of two nested levels", "/* /* */"},
		{"nested inside", "/* a /* b */ c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Parse(tc.source, ParseOptions{})
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error %v (%T), want a *ParseError", err, err)
			}
			const want = "unterminated block comment"
			if !strings.Contains(parseErr.Message, want) {
				t.Errorf("message = %q, want it to contain %q", parseErr.Message, want)
			}
		})
	}
}

// TestParseAcceptsClosedBlockComments: the valid forms still lex.
func TestParseAcceptsClosedBlockComments(t *testing.T) {
	for _, source := range []string{
		"/* closed */ fn f() {}",
		"/* nested /* inner */ still a comment */ fn f() {}",
		"fn f() {} // line comment",
	} {
		if _, _, err := Parse(source, ParseOptions{}); err != nil {
			t.Errorf("Parse(%q): %v", source, err)
		}
	}
}
