package snapshot_test

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ronParser parses Rust RON (Rusty Object Notation) text into Go values.
// It's a hand-written recursive descent parser handling the subset of RON
// used in Rust naga IR snapshot files.
type ronParser struct {
	input []rune
	pos   int
}

func newRONParser(data string) *ronParser {
	return &ronParser{input: []rune(data), pos: 0}
}

func (p *ronParser) atEnd() bool {
	return p.pos >= len(p.input)
}

func (p *ronParser) peek() rune {
	if p.atEnd() {
		return 0
	}
	return p.input[p.pos]
}

func (p *ronParser) advance() rune {
	ch := p.input[p.pos]
	p.pos++
	return ch
}

func (p *ronParser) skipWhitespaceAndComments() {
	for !p.atEnd() {
		ch := p.peek()
		if unicode.IsSpace(ch) {
			p.advance()
			continue
		}
		// Skip line comments //
		if ch == '/' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '/' {
			for !p.atEnd() && p.peek() != '\n' {
				p.advance()
			}
			continue
		}
		break
	}
}

func (p *ronParser) expect(ch rune) error {
	p.skipWhitespaceAndComments()
	if p.atEnd() {
		return fmt.Errorf("expected '%c' but got EOF", ch)
	}
	got := p.advance()
	if got != ch {
		return fmt.Errorf("expected '%c' but got '%c' at pos %d", ch, got, p.pos-1)
	}
	return nil
}

func (p *ronParser) tryConsume(ch rune) bool {
	p.skipWhitespaceAndComments()
	if !p.atEnd() && p.peek() == ch {
		p.advance()
		return true
	}
	return false
}

// parseIdent reads an identifier (alphanumeric + '_').
// Also handles Rust raw identifiers: r#None, r#Some, etc.
func (p *ronParser) parseIdent() (string, error) {
	p.skipWhitespaceAndComments()
	// Handle Rust raw identifier syntax: r#identifier
	if !p.atEnd() && p.peek() == 'r' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '#' {
		p.advance() // skip 'r'
		p.advance() // skip '#'
	}
	start := p.pos
	for !p.atEnd() && (unicode.IsLetter(p.peek()) || unicode.IsDigit(p.peek()) || p.peek() == '_') {
		p.advance()
	}
	if p.pos == start {
		if p.atEnd() {
			return "", fmt.Errorf("expected identifier but got EOF")
		}
		return "", fmt.Errorf("expected identifier but got '%c' at pos %d", p.peek(), p.pos)
	}
	return string(p.input[start:p.pos]), nil
}

// parseString reads a quoted string "...".
func (p *ronParser) parseString() (string, error) {
	p.skipWhitespaceAndComments()
	if err := p.expect('"'); err != nil {
		return "", err
	}
	var sb strings.Builder
	for !p.atEnd() {
		ch := p.advance()
		if ch == '"' {
			return sb.String(), nil
		}
		if ch == '\\' {
			if p.atEnd() {
				return "", fmt.Errorf("unterminated string escape")
			}
			esc := p.advance()
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteRune('\\')
				sb.WriteRune(esc)
			}
		} else {
			sb.WriteRune(ch)
		}
	}
	return "", fmt.Errorf("unterminated string")
}

// parseNumber reads a number (integer or float, possibly negative).
func (p *ronParser) parseNumber() (string, error) {
	p.skipWhitespaceAndComments()
	start := p.pos
	if !p.atEnd() && (p.peek() == '-' || p.peek() == '+') {
		p.advance()
	}
	for !p.atEnd() && unicode.IsDigit(p.peek()) {
		p.advance()
	}
	if !p.atEnd() && p.peek() == '.' {
		p.advance()
		for !p.atEnd() && unicode.IsDigit(p.peek()) {
			p.advance()
		}
	}
	// Scientific notation
	if !p.atEnd() && (p.peek() == 'e' || p.peek() == 'E') {
		p.advance()
		if !p.atEnd() && (p.peek() == '-' || p.peek() == '+') {
			p.advance()
		}
		for !p.atEnd() && unicode.IsDigit(p.peek()) {
			p.advance()
		}
	}
	if p.pos == start {
		return "", fmt.Errorf("expected number at pos %d", p.pos)
	}
	return string(p.input[start:p.pos]), nil
}

func (p *ronParser) parseUint() (uint32, error) {
	s, err := p.parseNumber()
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse uint %q: %w", s, err)
	}
	return uint32(v), nil
}

func (p *ronParser) parseInt() (int32, error) {
	s, err := p.parseNumber()
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse int %q: %w", s, err)
	}
	return int32(v), nil
}

func (p *ronParser) parseFloat64() (float64, error) {
	s, err := p.parseNumber()
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %w", s, err)
	}
	return v, nil
}

func (p *ronParser) parseFloat32() (float32, error) {
	s, err := p.parseNumber()
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %w", s, err)
	}
	return float32(v), nil
}

// parseSome parses Some(value) or None, returning (value, true) or ("", false).
func (p *ronParser) parseSomeNone() (bool, error) {
	p.skipWhitespaceAndComments()
	ident, err := p.parseIdent()
	if err != nil {
		return false, err
	}
	if ident == "None" {
		return false, nil
	}
	if ident == "Some" {
		return true, nil
	}
	return false, fmt.Errorf("expected Some or None, got %q", ident)
}

// parseSomeString parses Some("string") or None.
func (p *ronParser) parseSomeString() (*string, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	s, err := p.parseString()
	if err != nil {
		return nil, err
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return &s, nil
}

// parseSomeUint parses Some(N) or None.
func (p *ronParser) parseSomeUint() (*uint32, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	v, err := p.parseUint()
	if err != nil {
		return nil, err
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return &v, nil
}

// parseSomeExprHandle parses Some(N) or None for expression handles.
func (p *ronParser) parseSomeExprHandle() (*uint32, error) {
	return p.parseSomeUint()
}

// parseFieldSep consumes an optional comma after a field.
func (p *ronParser) parseFieldSep() {
	p.tryConsume(',')
}

// skipValue skips any RON value: strings, numbers, identifiers, nested () [] {}.
func (p *ronParser) skipValue() error {
	p.skipWhitespaceAndComments()
	if p.atEnd() {
		return fmt.Errorf("unexpected end of input in skipValue")
	}
	ch := p.peek()
	switch ch {
	case '(':
		return p.skipBracketed('(', ')')
	case '[':
		return p.skipBracketed('[', ']')
	case '{':
		return p.skipBracketed('{', '}')
	case '"':
		_, err := p.parseString()
		return err
	default:
		// Number, identifier, or None/Some(...)
		ident, err := p.parseIdent()
		if err != nil {
			// Try number
			_, err2 := p.parseNumber()
			return err2
		}
		if ident == "Some" {
			p.skipWhitespaceAndComments()
			if !p.atEnd() && p.peek() == '(' {
				return p.skipBracketed('(', ')')
			}
		}
		return nil
	}
}

// skipBracketed skips a balanced pair of brackets including contents.
func (p *ronParser) skipBracketed(open, close rune) error {
	if err := p.expect(open); err != nil {
		return err
	}
	depth := 1
	for depth > 0 && !p.atEnd() {
		ch := p.peek()
		switch {
		case ch == '"':
			if _, err := p.parseString(); err != nil {
				return err
			}
			continue
		case ch == open:
			depth++
		case ch == close:
			depth--
		}
		p.advance()
	}
	return nil
}

// context returns a debug string showing current position.
func (p *ronParser) context() string {
	start := p.pos
	end := p.pos + 40
	if end > len(p.input) {
		end = len(p.input)
	}
	return string(p.input[start:end])
}
