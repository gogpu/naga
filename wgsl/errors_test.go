package wgsl

import (
	"strings"
	"testing"
)

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

	p := NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Lower - should fail with unresolved identifier
	_, err = LowerWithSource(ast, source)
	if err == nil {
		t.Fatal("expected error for unresolved identifier")
	}

	// Error should contain position info
	errStr := err.Error()
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

	p := NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Lower - should fail
	_, err = LowerWithSource(ast, source)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
