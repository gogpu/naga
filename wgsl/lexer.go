package wgsl

import (
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes WGSL source code.
type Lexer struct {
	source string
	pos    int
	line   int
	column int
	start  int
	tokens []Token
}

// NewLexer creates a new lexer for the given source.
func NewLexer(source string) *Lexer {
	// Estimate ~1 token per 6 characters of source.
	estTokens := len(source) / 6
	if estTokens < 16 {
		estTokens = 16
	}
	return &Lexer{
		source: source,
		pos:    0,
		line:   1,
		column: 1,
		tokens: make([]Token, 0, estTokens),
	}
}

// Tokenize returns all tokens from the source.
func (l *Lexer) Tokenize() ([]Token, error) {
	for !l.isAtEnd() {
		l.start = l.pos
		if err := l.scanToken(); err != nil {
			return nil, err
		}
	}

	l.tokens = append(l.tokens, Token{
		Kind:   TokenEOF,
		Line:   l.line,
		Column: l.column,
	})

	return l.tokens, nil
}

func (l *Lexer) scanToken() error {
	r := l.advance()

	switch r {
	// Single-character tokens
	case '(':
		l.addToken(TokenLeftParen)
	case ')':
		l.addToken(TokenRightParen)
	case '{':
		l.addToken(TokenLeftBrace)
	case '}':
		l.addToken(TokenRightBrace)
	case '[':
		l.addToken(TokenLeftBracket)
	case ']':
		l.addToken(TokenRightBracket)
	case ',':
		l.addToken(TokenComma)
	case '.':
		l.addToken(TokenDot)
	case ':':
		l.addToken(TokenColon)
	case ';':
		l.addToken(TokenSemicolon)
	case '@':
		l.addToken(TokenAt)
	case '~':
		l.addToken(TokenTilde)
	case '%':
		if l.match('=') {
			l.addToken(TokenPercentEqual)
		} else {
			l.addToken(TokenPercent)
		}
	case '^':
		if l.match('=') {
			l.addToken(TokenCaretEqual)
		} else {
			l.addToken(TokenCaret)
		}

	// Operators that could be one or two characters
	case '+':
		if l.match('+') {
			l.addToken(TokenPlusPlus)
		} else if l.match('=') {
			l.addToken(TokenPlusEqual)
		} else {
			l.addToken(TokenPlus)
		}
	case '-':
		if l.match('-') {
			l.addToken(TokenMinusMinus)
		} else if l.match('=') {
			l.addToken(TokenMinusEqual)
		} else if l.match('>') {
			l.addToken(TokenArrow)
		} else {
			l.addToken(TokenMinus)
		}
	case '*':
		if l.match('=') {
			l.addToken(TokenStarEqual)
		} else {
			l.addToken(TokenStar)
		}
	case '/':
		if l.match('/') {
			// Line comment
			for l.peek() != '\n' && !l.isAtEnd() {
				l.advance()
			}
		} else if l.match('*') {
			// Block comment
			l.blockComment()
		} else if l.match('=') {
			l.addToken(TokenSlashEqual)
		} else {
			l.addToken(TokenSlash)
		}
	case '=':
		if l.match('=') {
			l.addToken(TokenEqualEqual)
		} else {
			l.addToken(TokenEqual)
		}
	case '!':
		if l.match('=') {
			l.addToken(TokenBangEqual)
		} else {
			l.addToken(TokenBang)
		}
	case '<':
		if l.match('<') {
			if l.match('=') {
				l.addToken(TokenLessLessEqual)
			} else {
				l.addToken(TokenLessLess)
			}
		} else if l.match('=') {
			l.addToken(TokenLessEqual)
		} else {
			l.addToken(TokenLess)
		}
	case '>':
		if l.match('>') {
			if l.match('=') {
				l.addToken(TokenGreaterGreaterEqual)
			} else {
				l.addToken(TokenGreaterGreater)
			}
		} else if l.match('=') {
			l.addToken(TokenGreaterEqual)
		} else {
			l.addToken(TokenGreater)
		}
	case '&':
		if l.match('&') {
			l.addToken(TokenAmpAmp)
		} else if l.match('=') {
			l.addToken(TokenAmpEqual)
		} else {
			l.addToken(TokenAmpersand)
		}
	case '|':
		if l.match('|') {
			l.addToken(TokenPipePipe)
		} else if l.match('=') {
			l.addToken(TokenPipeEqual)
		} else {
			l.addToken(TokenPipe)
		}

	// Whitespace
	case ' ', '\r', '\t':
		// Ignore whitespace
	case '\n':
		l.line++
		l.column = 1

	default:
		if isDigit(r) {
			l.number()
		} else if isAlpha(r) || r == '_' {
			l.identifier()
		} else {
			l.addToken(TokenError)
		}
	}

	return nil
}

func (l *Lexer) blockComment() {
	depth := 1
	for depth > 0 && !l.isAtEnd() {
		if l.peek() == '/' && l.peekNext() == '*' {
			l.advance()
			l.advance()
			depth++
		} else if l.peek() == '*' && l.peekNext() == '/' {
			l.advance()
			l.advance()
			depth--
		} else {
			if l.peek() == '\n' {
				l.line++
				l.column = 0
			}
			l.advance()
		}
	}
}

func (l *Lexer) number() {
	// Check for hex, octal, or binary prefix
	if l.source[l.start] == '0' && l.pos < len(l.source) {
		next := l.peek()
		if next == 'x' || next == 'X' {
			l.advance()
			for isHexDigit(l.peek()) {
				l.advance()
			}
			// Integer suffixes (u for unsigned, i for signed)
			if l.peek() == 'i' || l.peek() == 'u' {
				l.advance()
			}
			l.addToken(TokenIntLiteral)
			return
		}
	}

	for isDigit(l.peek()) {
		l.advance()
	}

	// Look for fractional part.
	// WGSL allows "1." as a float literal (no trailing digit required).
	// We treat "N." as float when followed by a digit or not an identifier-start char.
	// "1.x" is member access (int 1, then .x), but "1." "1.0" "1.5" are floats.
	nextAfterDot := l.peekNext()
	if l.peek() == '.' && !isAlpha(nextAfterDot) && nextAfterDot != '_' {
		l.advance() // consume '.'
		for isDigit(l.peek()) {
			l.advance()
		}
		// Look for exponent
		if l.peek() == 'e' || l.peek() == 'E' {
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
			for isDigit(l.peek()) {
				l.advance()
			}
		}
		// Float suffix
		if l.peek() == 'f' || l.peek() == 'h' {
			l.advance()
		}
		l.addToken(TokenFloatLiteral)
		return
	}

	// Check for exponent without decimal point
	if l.peek() == 'e' || l.peek() == 'E' {
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		for isDigit(l.peek()) {
			l.advance()
		}
		l.addToken(TokenFloatLiteral)
		return
	}

	// Float suffix without decimal point: 1f, 1h are valid WGSL float literals
	if l.peek() == 'f' || l.peek() == 'h' {
		l.advance()
		l.addToken(TokenFloatLiteral)
		return
	}

	// Integer suffixes
	if l.peek() == 'i' || l.peek() == 'u' {
		l.advance()
	}

	l.addToken(TokenIntLiteral)
}

func (l *Lexer) identifier() {
	for isAlphaNumeric(l.peek()) || l.peek() == '_' {
		l.advance()
	}

	text := l.source[l.start:l.pos]
	kind := l.lookupKeyword(text)
	l.addToken(kind)
}

var keywords = map[string]TokenKind{
	"alias":        TokenAlias,
	"break":        TokenBreak,
	"case":         TokenCase,
	"const":        TokenConst,
	"const_assert": TokenConstAssert,
	"continue":     TokenContinue,
	"continuing":   TokenContinuing,
	"default":      TokenDefault,
	"diagnostic":   TokenDiagnostic,
	"discard":      TokenDiscard,
	"else":         TokenElse,
	"enable":       TokenEnable,
	"false":        TokenFalse,
	"fn":           TokenFn,
	"for":          TokenFor,
	"if":           TokenIf,
	"let":          TokenLet,
	"loop":         TokenLoop,
	"override":     TokenOverride,
	"return":       TokenReturn,
	"struct":       TokenStruct,
	"switch":       TokenSwitch,
	"true":         TokenTrue,
	"var":          TokenVar,
	"while":        TokenWhile,

	// Types
	"bool":                          TokenBool,
	"f16":                           TokenF16,
	"f32":                           TokenF32,
	"i32":                           TokenI32,
	"u32":                           TokenU32,
	"vec2":                          TokenVec2,
	"vec3":                          TokenVec3,
	"vec4":                          TokenVec4,
	"mat2x2":                        TokenMat2x2,
	"mat2x3":                        TokenMat2x3,
	"mat2x4":                        TokenMat2x4,
	"mat3x2":                        TokenMat3x2,
	"mat3x3":                        TokenMat3x3,
	"mat3x4":                        TokenMat3x4,
	"mat4x2":                        TokenMat4x2,
	"mat4x3":                        TokenMat4x3,
	"mat4x4":                        TokenMat4x4,
	"array":                         TokenArray,
	"atomic":                        TokenAtomic,
	"ptr":                           TokenPtr,
	"sampler":                       TokenSampler,
	"sampler_comparison":            TokenSamplerComparison,
	"texture_1d":                    TokenTexture1d,
	"texture_2d":                    TokenTexture2d,
	"texture_2d_array":              TokenTexture2dArray,
	"texture_3d":                    TokenTexture3d,
	"texture_cube":                  TokenTextureCube,
	"texture_cube_array":            TokenTextureCubeArray,
	"texture_multisampled_2d":       TokenTextureMultisampled2d,
	"texture_storage_1d":            TokenTextureStorage1d,
	"texture_storage_2d":            TokenTextureStorage2d,
	"texture_storage_2d_array":      TokenTextureStorage2dArray,
	"texture_storage_3d":            TokenTextureStorage3d,
	"texture_depth_2d":              TokenTextureDepth2d,
	"texture_depth_2d_array":        TokenTextureDepth2dArray,
	"texture_depth_cube":            TokenTextureDepthCube,
	"texture_depth_cube_array":      TokenTextureDepthCubeArray,
	"texture_depth_multisampled_2d": TokenTextureDepthMultisampled2d,
}

func (l *Lexer) lookupKeyword(text string) TokenKind {
	if kind, ok := keywords[text]; ok {
		return kind
	}
	if text == "true" || text == "false" {
		return TokenBoolLiteral
	}
	return TokenIdent
}

func (l *Lexer) addToken(kind TokenKind) {
	l.tokens = append(l.tokens, Token{
		Kind:   kind,
		Lexeme: l.source[l.start:l.pos],
		Line:   l.line,
		Column: l.column - (l.pos - l.start),
	})
}

func (l *Lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.source[l.pos:])
	l.pos += size
	l.column++
	return r
}

func (l *Lexer) peek() rune {
	if l.isAtEnd() {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.source[l.pos:])
	return r
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.source) {
		return 0
	}
	_, size := utf8.DecodeRuneInString(l.source[l.pos:])
	r, _ := utf8.DecodeRuneInString(l.source[l.pos+size:])
	return r
}

func (l *Lexer) match(expected rune) bool {
	if l.isAtEnd() {
		return false
	}
	r, size := utf8.DecodeRuneInString(l.source[l.pos:])
	if r != expected {
		return false
	}
	l.pos += size
	l.column++
	return true
}

func (l *Lexer) isAtEnd() bool {
	return l.pos >= len(l.source)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isAlpha(r rune) bool {
	return unicode.IsLetter(r)
}

func isAlphaNumeric(r rune) bool {
	return isAlpha(r) || isDigit(r)
}
