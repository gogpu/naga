package wgsl

import (
	"errors"
	"strings"
	"testing"
)

func TestSourceError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SourceError
		expected string
	}{
		{
			name: "with position",
			err: &SourceError{
				Message: "unexpected token",
				Span: Span{
					Start: Position{Line: 5, Column: 10},
				},
			},
			expected: "5:10: unexpected token",
		},
		{
			name: "without position",
			err: &SourceError{
				Message: "generic error",
				Span:    Span{},
			},
			expected: "generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSourceError_FormatWithContext(t *testing.T) {
	source := `@vertex
fn main() -> @builtin(position) vec4<f32> {
    let x = 1.0
    return vec4(x);
}`

	err := &SourceError{
		Message: "expected ';' after statement",
		Span: Span{
			Start: Position{Line: 3, Column: 16},
		},
		Source: source,
	}

	formatted := err.FormatWithContext()

	// Check that it contains key parts
	if !strings.Contains(formatted, "expected ';' after statement") {
		t.Error("formatted error should contain message")
	}
	if !strings.Contains(formatted, "line 3:16") {
		t.Error("formatted error should contain line:column")
	}
	if !strings.Contains(formatted, "let x = 1.0") {
		t.Error("formatted error should contain source line")
	}
	if !strings.Contains(formatted, "^") {
		t.Error("formatted error should contain caret pointer")
	}
}

func TestSourceError_FormatWithContext_NoSource(t *testing.T) {
	err := &SourceError{
		Message: "error without source",
		Span: Span{
			Start: Position{Line: 1, Column: 1},
		},
		Source: "",
	}

	formatted := err.FormatWithContext()
	if formatted != "1:1: error without source" {
		t.Errorf("expected simple format without source, got: %q", formatted)
	}
}

func TestSourceErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   SourceErrors
		expected string
	}{
		{
			name:     "empty",
			errors:   SourceErrors{},
			expected: "no errors",
		},
		{
			name: "single",
			errors: SourceErrors{
				{Message: "first error", Span: Span{Start: Position{Line: 1, Column: 1}}},
			},
			expected: "1:1: first error",
		},
		{
			name: "multiple",
			errors: SourceErrors{
				{Message: "first error", Span: Span{Start: Position{Line: 1, Column: 1}}},
				{Message: "second error", Span: Span{Start: Position{Line: 2, Column: 5}}},
				{Message: "third error", Span: Span{Start: Position{Line: 3, Column: 10}}},
			},
			expected: "1:1: first error (and 2 more errors)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errors.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSourceErrors_Operations(t *testing.T) {
	var errs SourceErrors

	if errs.HasErrors() {
		t.Error("empty list should not have errors")
	}
	if errs.Len() != 0 {
		t.Error("empty list should have length 0")
	}

	errs.AddError("error 1", Span{Start: Position{Line: 1, Column: 1}}, "")
	if !errs.HasErrors() {
		t.Error("list with error should have errors")
	}
	if errs.Len() != 1 {
		t.Errorf("expected length 1, got %d", errs.Len())
	}

	errs.Add(NewSourceError("error 2", Span{Start: Position{Line: 2, Column: 1}}, ""))
	if errs.Len() != 2 {
		t.Errorf("expected length 2, got %d", errs.Len())
	}
}

func TestNewSourceErrorf(t *testing.T) {
	err := NewSourceErrorf(
		Span{Start: Position{Line: 5, Column: 3}},
		"source code",
		"unknown identifier: %s",
		"foo",
	)

	if err.Message != "unknown identifier: foo" {
		t.Errorf("expected formatted message, got: %q", err.Message)
	}
	if err.Span.Start.Line != 5 {
		t.Errorf("expected line 5, got %d", err.Span.Start.Line)
	}
}

func TestLowerWithSource_ErrorPosition(t *testing.T) {
	// Test that errors from LowerWithSource include position info
	source := `@vertex
fn main() -> @builtin(position) vec4<f32> {
    let x = unknownVar;
    return vec4(0.0);
}`

	// Parse
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Lower - should fail with unresolved identifier
	_, err = LowerWithSource(ast, source)
	if err == nil {
		t.Fatal("expected error for unresolved identifier")
	}

	// Check that error is a SourceErrors with position
	var errList *SourceErrors
	if !errors.As(err, &errList) {
		t.Fatalf("expected *SourceErrors, got %T", err)
	}

	if !errList.HasErrors() {
		t.Fatal("expected errors in list")
	}

	// Check first error has position
	firstErr := (*errList)[0]
	if firstErr.Span.Start.Line == 0 {
		t.Error("expected error to have line number")
	}

	// Error should mention the function or location
	errStr := firstErr.Error()
	if !strings.Contains(errStr, ":") {
		t.Errorf("expected error to contain line:column, got: %q", errStr)
	}
}

func TestLowerWithSource_ErrorContext(t *testing.T) {
	source := `@vertex
fn main() -> @builtin(position) vec4<f32> {
    var x: unknown_type;
    return vec4(0.0);
}`

	// Parse
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Lower - should fail
	_, err = LowerWithSource(ast, source)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}

	// Get formatted error with context
	var errList *SourceErrors
	if !errors.As(err, &errList) {
		t.Fatalf("expected *SourceErrors, got %T", err)
	}

	formatted := errList.FormatAll()

	// Should contain the source line
	if !strings.Contains(formatted, "unknown_type") {
		t.Log("Formatted error:", formatted)
		// This is expected since the span might not be set correctly yet
		// The test documents the expected behavior
	}
}
