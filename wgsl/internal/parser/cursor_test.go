package parser

import "testing"

func TestParserCursor(t *testing.T) {
	p := NewParser(NewLexer("a b"))

	// previous before any advance is the zero Token: Kind TokenEOF, line 0.
	if got := p.previous(); got.Kind != TokenEOF || got.Line != 0 || got.Column != 0 {
		t.Fatalf("previous before advance = %+v, want the zero Token", got)
	}
	if got := p.peek(); got.Lexeme != "a" {
		t.Fatalf("peek = %q, want %q", got.Lexeme, "a")
	}
	if got := p.advance(); got.Lexeme != "a" {
		t.Fatalf("advance = %q, want %q", got.Lexeme, "a")
	}
	if got := p.peek(); got.Lexeme != "b" {
		t.Fatalf("peek = %q, want %q", got.Lexeme, "b")
	}
	if got := p.advance(); got.Lexeme != "b" {
		t.Fatalf("advance = %q, want %q", got.Lexeme, "b")
	}
	if !p.isAtEnd() {
		t.Fatalf("expected EOF after consuming both tokens")
	}
	// advance at EOF returns the last consumed token and does not move.
	if got := p.advance(); got.Lexeme != "b" {
		t.Fatalf("advance at EOF = %q, want %q", got.Lexeme, "b")
	}
	if !p.isAtEnd() {
		t.Fatalf("advance at EOF moved the cursor")
	}
}

func TestSplitDoesNotAdvance(t *testing.T) {
	tests := []struct {
		src    string
		split  func(*Parser)
		want   TokenKind
		lexeme string
	}{
		{">>", (*Parser).splitGreaterGreater, TokenGreater, ">"},
		{">=", (*Parser).splitGreaterEqual, TokenEqual, "="},
		{">>=", (*Parser).splitGreaterGreaterEqual, TokenGreaterEqual, ">="},
	}
	for _, tt := range tests {
		p := NewParser(NewLexer(tt.src))
		tt.split(p)
		cur := p.peek()
		if cur.Kind != tt.want || cur.Lexeme != tt.lexeme {
			t.Errorf("%s: after split cur = %v %q, want %v %q", tt.src, cur.Kind, cur.Lexeme, tt.want, tt.lexeme)
		}
		// The remainder sits at the second character of the original token.
		if cur.Column != 2 {
			t.Errorf("%s: after split column = %d, want 2", tt.src, cur.Column)
		}
		// Advancing consumes the remainder, and the next token is EOF.
		if got := p.advance(); got.Kind != tt.want {
			t.Errorf("%s: advance returned %v, want %v", tt.src, got.Kind, tt.want)
		}
		if !p.isAtEnd() {
			t.Errorf("%s: expected EOF after the split remainder", tt.src)
		}
	}
}
