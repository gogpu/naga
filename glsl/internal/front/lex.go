package front

import "github.com/gogpu/naga/ir"

// glslToken is one classified GLSL token: a literal, an identifier, punctuation,
// a keyword or a type name, with its place in the source.
type glslToken struct {
	kind tokenKind
	text string // the spelling of an identifier, a punctuation token or a type name

	typeName *typeName // kind == tokTypeName

	intValue  uint64
	intSigned bool
	intWidth  int32

	floatValue float64
	floatWidth int32

	boolValue bool

	span ir.Span
	line uint32
}

// isPunct reports whether t is the punctuation token s.
func (t glslToken) isPunct(s string) bool { return t.kind == tokPunct && t.text == s }

// lex classifies the preprocessor's tokens into GLSL tokens.
func lex(tokens []ppToken) []glslToken {
	out := make([]glslToken, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, classify(t))
	}
	return out
}

// classify turns one preprocessor token into its GLSL token.
func classify(t ppToken) glslToken {
	tok := glslToken{span: t.span, line: t.line}
	switch t.kind {
	case ppInt:
		tok.kind = tokInt
		tok.intValue, tok.intSigned, tok.intWidth = t.intValue, t.intSigned, t.intWidth
	case ppFloat:
		tok.kind = tokFloat
		tok.floatValue, tok.floatWidth = t.floatValue, t.floatWidth
	case ppPunct:
		tok.kind, tok.text = tokPunct, t.text
	case ppIdent:
		switch t.text {
		case "true", "false":
			tok.kind, tok.boolValue = tokBool, t.text == "true"
		default:
			if kind, ok := keywords[t.text]; ok {
				tok.kind = kind
			} else if tn, ok := parseTypeName(t.text); ok {
				// The spelling is kept: a combined sampler constructor is a
				// call of the builtin of that name (design §11).
				tok.kind, tok.typeName, tok.text = tokTypeName, &tn, t.text
			} else {
				tok.kind, tok.text = tokIdent, t.text
			}
		}
	default:
		// ppHash and ppNewline never reach the lexer: the directive processor
		// consumed them.
	}
	return tok
}
