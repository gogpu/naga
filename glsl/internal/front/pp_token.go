package front

import (
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// spanOf builds the span of the bytes [start, end).
func spanOf(start, end int) ir.Span {
	return ir.Span{Start: uint32(start), End: uint32(end)}
}

// isIdentStart reports whether r starts an identifier. GLSL identifiers are
// ASCII: a letter or an underscore.
func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isIdentContinue reports whether r continues an identifier.
func isIdentContinue(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// punctuations are the GLSL operators, longest first, so a scan can take the
// longest match.
var punctuations = []string{
	"<<=", ">>=",
	"++", "--", "&&", "||", "^^", "==", "!=", "<=", ">=", "<<", ">>",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"(", ")", "{", "}", "[", "]", "<", ">", ",", ";", ":", "=", "!", "-", "~",
	"+", "*", "/", "%", "|", "^", "&", "?",
}

// token scans one token whose first character is r. It reports false (after
// recording the error) for a character that starts no GLSL token.
func (s *scanner) token(r rune) (ppToken, bool) {
	start, line := s.pos()
	switch {
	case isIdentStart(r):
		var b strings.Builder
		for {
			rr, ok := s.peek()
			if !ok || !isIdentContinue(rr) {
				break
			}
			b.WriteRune(rr)
			s.advance()
		}
		return ppToken{kind: ppIdent, text: b.String(), span: spanOf(start, s.off), line: line}, true
	case isDigit(r):
		return s.number(r, start, line), true
	case r == '.':
		if rr, ok := s.peekN(1); !ok || !isDigit(rr) {
			s.advance()
			return ppToken{kind: ppPunct, text: ".", span: spanOf(start, s.off), line: line}, true
		}
		return s.number(r, start, line), true
	case r == '#':
		s.advance()
		return ppToken{kind: ppHash, text: "#", span: spanOf(start, s.off), line: line}, true
	default:
		if tok, ok := s.punctuation(start, line); ok {
			return tok, true
		}
		s.advance()
		s.errorf(spanOf(start, s.off), "unexpected character %q", r)
		return ppToken{}, false
	}
}

// punctuation takes the longest punctuation token at the current position.
func (s *scanner) punctuation(start int, line uint32) (ppToken, bool) {
	for _, p := range punctuations {
		matches := true
		for i := 0; i < len(p); i++ {
			r, ok := s.peekN(i)
			if !ok || r != rune(p[i]) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		for i := 0; i < len(p); i++ {
			s.advance()
		}
		return ppToken{kind: ppPunct, text: p, span: spanOf(start, s.off), line: line}, true
	}
	return ppToken{}, false
}

// takeWhile consumes and returns the characters satisfying pred.
func (s *scanner) takeWhile(pred func(rune) bool) string {
	var b strings.Builder
	for {
		r, ok := s.peek()
		if !ok || !pred(r) {
			return b.String()
		}
		b.WriteRune(r)
		s.advance()
	}
}

// number scans an integer or floating-point literal whose first character is
// first, already the current character. The rules follow pp-rs: decimal,
// 0x-hexadecimal and leading-zero octal integers with a u/U suffix; floats with
// a fraction, an exponent and an f/F suffix. The 64- and 16-bit suffixes are
// refused as Rust naga refuses them.
func (s *scanner) number(first rune, start int, line uint32) ppToken {
	var raw strings.Builder // the literal text, with the radix prefix's 0 kept
	base := 10
	isFloat := false

	raw.WriteRune(first)
	s.advance()

	if first == '0' {
		if r, ok := s.peek(); ok && (r == 'x' || r == 'X') {
			s.advance()
			raw.WriteString(s.takeWhile(isHexDigit))
			base = 16
		} else if r, ok := s.peek(); ok && isDigit(r) {
			base = 8
		}
	}

	if first == '.' {
		isFloat = true
	} else {
		raw.WriteString(s.takeWhile(isDigit))
		if r, ok := s.peek(); ok && r == '.' {
			s.advance()
			raw.WriteRune('.')
			isFloat = true
		}
	}
	if isFloat {
		raw.WriteString(s.takeWhile(isDigit))
	}

	// An exponent only belongs to a decimal or octal literal: 0x1E is a hex
	// integer, not a float.
	if isFloat || base == 8 || base == 10 {
		if r, ok := s.peek(); ok && (r == 'e' || r == 'E') {
			s.advance()
			raw.WriteRune('e')
			isFloat = true
			if r, ok := s.peek(); ok && (r == '+' || r == '-') {
				s.advance()
				raw.WriteRune(r)
			}
			raw.WriteString(s.takeWhile(isDigit))
		}
	}

	span := func() ir.Span { return spanOf(start, s.off) }

	width := int32(32)
	if isFloat {
		if r, ok := s.peek(); ok {
			switch r {
			case 'f', 'F':
				s.advance()
			case 'l', 'L', 'h', 'H':
				// GLSL 4.60 §4.1.4: the double suffix is "lf" and the
				// half-float suffix "hf" (the float16 extension); a bare l or
				// h is not a suffix, and the tokenizer reads it as part of the
				// next token.
				s.advance()
				next, ok := s.peek()
				if !ok || (next != 'f' && next != 'F') {
					s.errorf(span(), "invalid floating-point literal suffix: %q must be followed by 'f' (lf, hf)", string(r))
					break
				}
				s.advance()
				if r == 'l' || r == 'L' {
					width = 64
				} else {
					width = 16
				}
			}
		}
		text := raw.String()
		// Go's strconv wants a digit on both sides of the point where C and
		// Rust accept "5." and "1.e3".
		if i := strings.IndexByte(text, '.'); i >= 0 && (i+1 == len(text) || text[i+1] == 'e' || text[i+1] == 'E') {
			text = text[:i+1] + "0" + text[i+1:]
		}
		// Parse at full precision: a 32-bit literal is narrowed when it
		// becomes an IR literal, and a double keeps its bits.
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			s.errorf(span(), "invalid floating-point literal %q", text)
		}
		return ppToken{kind: ppFloat, floatValue: value, floatWidth: width, span: span(), line: line}
	}

	signed := true
	if r, ok := s.peek(); ok && (r == 'u' || r == 'U') {
		s.advance()
		signed = false
	}
	if r, ok := s.peek(); ok {
		switch r {
		case 'l', 'L':
			s.advance()
			s.errorf(span(), "unsupported 64-bit integer literal")
		case 's', 'S':
			s.advance()
			s.errorf(span(), "unsupported 16-bit integer literal")
		}
	}

	text := raw.String()
	if base != 10 {
		text = text[1:] // drop the leading 0 of the radix prefix
	}
	value, err := strconv.ParseUint(text, base, 64)
	if err != nil || text == "" {
		s.errorf(span(), "invalid or overflowing integer literal")
		value = 0
	}
	return ppToken{kind: ppInt, intValue: value, intSigned: signed, intWidth: 32, span: span(), line: line}
}
