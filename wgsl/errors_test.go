package wgsl

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// parseError parses source and returns its *ParseError.
func parseError(t *testing.T, source string) *ParseError {
	t.Helper()
	_, _, err := Parse(source, ParseOptions{})
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error %v (%T), want a *ParseError", err, err)
	}
	return pe
}

// TestParseErrorShape: Error is the message alone, as Rust's Display; the
// primary label places it.
func TestParseErrorShape(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		line, column uint32
		text         string // the source under the primary label
	}{
		// A syntax error, labeled at the token the parser stopped on.
		{"parser", "fn () {}", 1, 4, "("},
		// A lowering error is labeled at its declaration; Rust labels the
		// expression (P2-WGSL-099).
		{"lowerer", "@vertex\nfn main() -> @builtin(position) vec4<f32> {\n    let x = unknownVar;\n    return vec4(0.0);\n}", 2, 1, ""},
		// Columns are bytes: two 2-byte runes before the error.
		{"utf8", "// éé\nfn () {}", 2, 4, "("},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pe := parseError(t, tc.source)
			if pe.Error() != pe.Message || strings.Contains(pe.Error(), "line") {
				t.Errorf("Error() = %q, want the message %q alone", pe.Error(), pe.Message)
			}
			loc, ok := pe.Location(tc.source)
			if !ok || loc.Line != tc.line || loc.Column != tc.column {
				t.Fatalf("Location = %+v, %v; want %d:%d (%q)", loc, ok, tc.line, tc.column, pe.Message)
			}
			span := pe.Labels[0].Span
			if got := tc.source[span.Start:span.End]; !strings.HasPrefix(got, tc.text) && got != "" {
				t.Errorf("label covers %q, want %q", got, tc.text)
			}
		})
	}
}

func TestParseErrorWithoutLabel(t *testing.T) {
	pe := &ParseError{Message: "m"}
	if _, ok := pe.Location("src"); ok {
		t.Error("Location of an error without labels is ok")
	}
	if got, want := pe.Format("src", "wgsl"), "error: m\n\n"; got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

// TestParseErrorFormat: Rust naga's codespan layout (tests/naga/wgsl_errors.rs).
func TestParseErrorFormat(t *testing.T) {
	const source = "const i32min = -0x80000000i;"
	pe := &ParseError{
		Message: "numeric literal not representable by target type: `0x80000000i`",
		Labels:  []Label{{Span: ir.Span{Start: 16, End: 27}, Text: "numeric literal not representable by target type"}},
	}
	want := "error: numeric literal not representable by target type: `0x80000000i`\n" +
		"  ┌─ wgsl:1:17\n" +
		"  │\n" +
		"1 │ const i32min = -0x80000000i;\n" +
		"  │                 ^^^^^^^^^^^ numeric literal not representable by target type\n" +
		"\n"
	if got := pe.Format(source, "wgsl"); got != want {
		t.Errorf("Format:\n%s\nwant:\n%s", got, want)
	}

	// Notes follow the labels, as Rust's `= note:` lines; a two-digit line
	// widens the gutter.
	src := strings.Repeat("\n", 9) + "    let value = sign(input);"
	pe = &ParseError{
		Message: "wrong type passed as argument #1 to `sign`",
		Labels:  []Label{{Span: ir.Span{Start: 21, End: 25}}},
		Notes:   []string{"`sign` accepts the following types for argument #1:"},
	}
	want = "error: wrong type passed as argument #1 to `sign`\n" +
		"   ┌─ shader.wgsl:10:13\n" +
		"   │\n" +
		"10 │     let value = sign(input);\n" +
		"   │             ^^^^\n" +
		"   │\n" +
		"   = note: `sign` accepts the following types for argument #1:\n" +
		"\n"
	if got := pe.Format(src, "shader.wgsl"); got != want {
		t.Errorf("Format:\n%s\nwant:\n%s", got, want)
	}
}

// TestParseMoreErrorsAreNotes: the first error is the ParseError (Rust stops
// there); the others the parser found are notes with their positions.
func TestParseMoreErrorsAreNotes(t *testing.T) {
	pe := parseError(t, "fn () {}\nfn () {}")
	if len(pe.Notes) == 0 || !strings.HasPrefix(pe.Notes[0], "2:4: ") {
		t.Errorf("notes %q, want the second error at 2:4", pe.Notes)
	}
}

func TestParseWarnings(t *testing.T) {
	source := "fn f() -> i32 { return 1; }\n@compute @workgroup_size(1) fn main() {\n    f();\n}"
	_, info, err := Parse(source, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range info.Warnings {
		if !w.Span.IsDefined() {
			t.Errorf("warning %q has no span", w.Message)
		}
	}
}
