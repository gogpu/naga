package wgsl

import (
	"testing"
)

func TestLexerBasicTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenKind
	}{
		{"+ - * /", []TokenKind{TokenPlus, TokenMinus, TokenStar, TokenSlash, TokenEOF}},
		{"( ) { }", []TokenKind{TokenLeftParen, TokenRightParen, TokenLeftBrace, TokenRightBrace, TokenEOF}},
		{"[ ] , .", []TokenKind{TokenLeftBracket, TokenRightBracket, TokenComma, TokenDot, TokenEOF}},
		{": ; @", []TokenKind{TokenColon, TokenSemicolon, TokenAt, TokenEOF}},
	}

	for _, tt := range tests {
		lexer := NewLexer(tt.input)
		tokens, err := lexer.Tokenize()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}

		if len(tokens) != len(tt.expected) {
			t.Errorf("Expected %d tokens, got %d", len(tt.expected), len(tokens))
			continue
		}

		for i, tok := range tokens {
			if tok.Kind != tt.expected[i] {
				t.Errorf("Token %d: expected %v, got %v", i, tt.expected[i], tok.Kind)
			}
		}
	}
}

func TestLexerOperators(t *testing.T) {
	input := "== != <= >= && || << >> -> ++ --"
	expected := []TokenKind{
		TokenEqualEqual, TokenBangEqual, TokenLessEqual, TokenGreaterEqual,
		TokenAmpAmp, TokenPipePipe, TokenLessLess, TokenGreaterGreater,
		TokenArrow, TokenPlusPlus, TokenMinusMinus, TokenEOF,
	}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, tok := range tokens {
		if tok.Kind != expected[i] {
			t.Errorf("Token %d: expected %v, got %v", i, expected[i], tok.Kind)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	input := "fn struct var let const return if else for while"
	expected := []TokenKind{
		TokenFn, TokenStruct, TokenVar, TokenLet, TokenConst,
		TokenReturn, TokenIf, TokenElse, TokenFor, TokenWhile, TokenEOF,
	}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, tok := range tokens {
		if tok.Kind != expected[i] {
			t.Errorf("Token %d: expected %v, got %v", i, expected[i], tok.Kind)
		}
	}
}

func TestLexerTypes(t *testing.T) {
	input := "f32 i32 u32 vec2 vec3 vec4 mat4x4 bool"
	expected := []TokenKind{
		TokenF32, TokenI32, TokenU32, TokenVec2, TokenVec3, TokenVec4,
		TokenMat4x4, TokenBool, TokenEOF,
	}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, tok := range tokens {
		if tok.Kind != expected[i] {
			t.Errorf("Token %d: expected %v, got %v", i, expected[i], tok.Kind)
		}
	}
}

func TestLexerNumbers(t *testing.T) {
	tests := []struct {
		input    string
		kind     TokenKind
		lexeme   string
	}{
		{"123", TokenIntLiteral, "123"},
		{"0x1F", TokenIntLiteral, "0x1F"},
		{"1.5", TokenFloatLiteral, "1.5"},
		{"1e10", TokenFloatLiteral, "1e10"},
		{"1.5e-3", TokenFloatLiteral, "1.5e-3"},
		{"42u", TokenIntLiteral, "42u"},
		{"3.14f", TokenFloatLiteral, "3.14f"},
	}

	for _, tt := range tests {
		lexer := NewLexer(tt.input)
		tokens, err := lexer.Tokenize()
		if err != nil {
			t.Errorf("Input %q: unexpected error: %v", tt.input, err)
			continue
		}

		if len(tokens) != 2 { // number + EOF
			t.Errorf("Input %q: expected 2 tokens, got %d", tt.input, len(tokens))
			continue
		}

		if tokens[0].Kind != tt.kind {
			t.Errorf("Input %q: expected kind %v, got %v", tt.input, tt.kind, tokens[0].Kind)
		}

		if tokens[0].Lexeme != tt.lexeme {
			t.Errorf("Input %q: expected lexeme %q, got %q", tt.input, tt.lexeme, tokens[0].Lexeme)
		}
	}
}

func TestLexerIdentifiers(t *testing.T) {
	input := "foo _bar baz123 my_variable"
	expected := []string{"foo", "_bar", "baz123", "my_variable"}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tokens) != len(expected)+1 { // identifiers + EOF
		t.Fatalf("Expected %d tokens, got %d", len(expected)+1, len(tokens))
	}

	for i, name := range expected {
		if tokens[i].Kind != TokenIdent {
			t.Errorf("Token %d: expected Ident, got %v", i, tokens[i].Kind)
		}
		if tokens[i].Lexeme != name {
			t.Errorf("Token %d: expected %q, got %q", i, name, tokens[i].Lexeme)
		}
	}
}

func TestLexerComments(t *testing.T) {
	input := `foo // this is a comment
bar /* block comment */ baz
/* nested /* comments */ work */
qux`

	expected := []string{"foo", "bar", "baz", "qux"}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	identTokens := make([]Token, 0)
	for _, tok := range tokens {
		if tok.Kind == TokenIdent {
			identTokens = append(identTokens, tok)
		}
	}

	if len(identTokens) != len(expected) {
		t.Fatalf("Expected %d identifiers, got %d", len(expected), len(identTokens))
	}

	for i, name := range expected {
		if identTokens[i].Lexeme != name {
			t.Errorf("Identifier %d: expected %q, got %q", i, name, identTokens[i].Lexeme)
		}
	}
}

func TestLexerFunction(t *testing.T) {
	input := `@vertex
fn main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}`

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Just verify we got tokens without errors
	if len(tokens) < 10 {
		t.Errorf("Expected more tokens for function, got %d", len(tokens))
	}

	// Check first few tokens
	expectedStart := []TokenKind{
		TokenAt, TokenIdent, // @vertex
		TokenFn, TokenIdent, TokenLeftParen, // fn main(
	}

	for i, expected := range expectedStart {
		if tokens[i].Kind != expected {
			t.Errorf("Token %d: expected %v, got %v (lexeme: %q)",
				i, expected, tokens[i].Kind, tokens[i].Lexeme)
		}
	}
}
