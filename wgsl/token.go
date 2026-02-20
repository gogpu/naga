// Package wgsl provides WGSL (WebGPU Shading Language) parsing.
package wgsl

// TokenKind represents the type of token.
type TokenKind uint8

const (
	TokenEOF TokenKind = iota
	TokenError

	// Literals
	TokenIdent
	TokenIntLiteral
	TokenFloatLiteral
	TokenBoolLiteral

	// Operators
	TokenPlus                // +
	TokenMinus               // -
	TokenStar                // *
	TokenSlash               // /
	TokenPercent             // %
	TokenAmpersand           // &
	TokenPipe                // |
	TokenCaret               // ^
	TokenTilde               // ~
	TokenBang                // !
	TokenEqual               // =
	TokenLess                // <
	TokenGreater             // >
	TokenDot                 // .
	TokenComma               // ,
	TokenColon               // :
	TokenSemicolon           // ;
	TokenAt                  // @
	TokenArrow               // ->
	TokenPlusPlus            // ++
	TokenMinusMinus          // --
	TokenEqualEqual          // ==
	TokenBangEqual           // !=
	TokenLessEqual           // <=
	TokenGreaterEqual        // >=
	TokenAmpAmp              // &&
	TokenPipePipe            // ||
	TokenLessLess            // <<
	TokenGreaterGreater      // >>
	TokenPlusEqual           // +=
	TokenMinusEqual          // -=
	TokenStarEqual           // *=
	TokenSlashEqual          // /=
	TokenPercentEqual        // %=
	TokenAmpEqual            // &=
	TokenPipeEqual           // |=
	TokenCaretEqual          // ^=
	TokenLessLessEqual       // <<=
	TokenGreaterGreaterEqual // >>=

	// Delimiters
	TokenLeftParen    // (
	TokenRightParen   // )
	TokenLeftBrace    // {
	TokenRightBrace   // }
	TokenLeftBracket  // [
	TokenRightBracket // ]

	// Keywords
	TokenAlias
	TokenBreak
	TokenCase
	TokenConst
	TokenConstAssert
	TokenContinue
	TokenContinuing
	TokenDefault
	TokenDiagnostic
	TokenDiscard
	TokenElse
	TokenEnable
	TokenFalse
	TokenFn
	TokenFor
	TokenIf
	TokenLet
	TokenLoop
	TokenOverride
	TokenReturn
	TokenStruct
	TokenSwitch
	TokenTrue
	TokenVar
	TokenWhile

	// Reserved keywords
	TokenNull
	TokenSelf
	TokenSuper
	TokenTrait
	TokenType
	TokenUsing

	// Type keywords
	TokenBool
	TokenF16
	TokenF32
	TokenI32
	TokenU32
	TokenVec2
	TokenVec3
	TokenVec4
	TokenMat2x2
	TokenMat2x3
	TokenMat2x4
	TokenMat3x2
	TokenMat3x3
	TokenMat3x4
	TokenMat4x2
	TokenMat4x3
	TokenMat4x4
	TokenArray
	TokenAtomic
	TokenPtr
	TokenSampler
	TokenSamplerComparison
	TokenTexture1d
	TokenTexture2d
	TokenTexture2dArray
	TokenTexture3d
	TokenTextureCube
	TokenTextureCubeArray
	TokenTextureMultisampled2d
	TokenTextureStorage1d
	TokenTextureStorage2d
	TokenTextureStorage2dArray
	TokenTextureStorage3d
	TokenTextureDepth2d
	TokenTextureDepth2dArray
	TokenTextureDepthCube
	TokenTextureDepthCubeArray
	TokenTextureDepthMultisampled2d
)

// tokenNames maps token kinds to their string representations.
var tokenNames = map[TokenKind]string{
	TokenEOF:          "EOF",
	TokenError:        "Error",
	TokenIdent:        "Ident",
	TokenIntLiteral:   "IntLiteral",
	TokenFloatLiteral: "FloatLiteral",
	TokenBoolLiteral:  "BoolLiteral",

	// Operators
	TokenPlus:                "+",
	TokenMinus:               "-",
	TokenStar:                "*",
	TokenSlash:               "/",
	TokenPercent:             "%",
	TokenAmpersand:           "&",
	TokenPipe:                "|",
	TokenCaret:               "^",
	TokenTilde:               "~",
	TokenBang:                "!",
	TokenEqual:               "=",
	TokenLess:                "<",
	TokenGreater:             ">",
	TokenDot:                 ".",
	TokenComma:               ",",
	TokenColon:               ":",
	TokenSemicolon:           ";",
	TokenAt:                  "@",
	TokenArrow:               "->",
	TokenPlusPlus:            "++",
	TokenMinusMinus:          "--",
	TokenEqualEqual:          "==",
	TokenBangEqual:           "!=",
	TokenLessEqual:           "<=",
	TokenGreaterEqual:        ">=",
	TokenAmpAmp:              "&&",
	TokenPipePipe:            "||",
	TokenLessLess:            "<<",
	TokenGreaterGreater:      ">>",
	TokenPlusEqual:           "+=",
	TokenMinusEqual:          "-=",
	TokenStarEqual:           "*=",
	TokenSlashEqual:          "/=",
	TokenPercentEqual:        "%=",
	TokenAmpEqual:            "&=",
	TokenPipeEqual:           "|=",
	TokenCaretEqual:          "^=",
	TokenLessLessEqual:       "<<=",
	TokenGreaterGreaterEqual: ">>=",

	// Delimiters
	TokenLeftParen:    "(",
	TokenRightParen:   ")",
	TokenLeftBrace:    "{",
	TokenRightBrace:   "}",
	TokenLeftBracket:  "[",
	TokenRightBracket: "]",

	// Keywords
	TokenAlias:       "alias",
	TokenBreak:       "break",
	TokenCase:        "case",
	TokenConst:       "const",
	TokenConstAssert: "const_assert",
	TokenContinue:    "continue",
	TokenContinuing:  "continuing",
	TokenDefault:     "default",
	TokenDiagnostic:  "diagnostic",
	TokenDiscard:     "discard",
	TokenElse:        "else",
	TokenEnable:      "enable",
	TokenFalse:       "false",
	TokenFn:          "fn",
	TokenFor:         "for",
	TokenIf:          "if",
	TokenLet:         "let",
	TokenLoop:        "loop",
	TokenOverride:    "override",
	TokenReturn:      "return",
	TokenStruct:      "struct",
	TokenSwitch:      "switch",
	TokenTrue:        "true",
	TokenVar:         "var",
	TokenWhile:       "while",
}

// String returns the string representation of the token kind.
func (k TokenKind) String() string {
	if name, ok := tokenNames[k]; ok {
		return name
	}
	return "Unknown"
}

// Token represents a lexical token.
type Token struct {
	Kind   TokenKind
	Lexeme string
	Line   int
	Column int
}

// Span represents a source code location span.
type Span struct {
	Start  Position
	End    Position
	Source string // Source file name or identifier
}

// Position represents a position in source code.
type Position struct {
	Line   int
	Column int
	Offset int
}
