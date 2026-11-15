// Package front implements the GLSL input frontend: GLSL source -> naga IR.
//
// This file is the preprocessor: the GLSL ES 3.20 specification's §3.10
// logical phases 4-8 (line-ending normalization, backslash-newline splicing,
// comment replacement, tokenization) followed by the directive processor and
// macro expansion. It is independent Go; pp-rs 0.2.1, which Rust naga uses,
// was read as the behavioral reference.
package front

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// Limits of the preprocessor. They bound untrusted input: a macro bomb or a
// deeply nested conditional is an error, never an allocation the input chooses.
const (
	// maxPPTokens is the absolute cap on the preprocessed stream, source
	// tokens and macro expansion together.
	maxPPTokens = 1 << 22
	// ppTokenBudgetFactor and ppTokenBudgetBase give a source its headroom:
	// four times its own token count plus 64K. A small shader that expands to
	// a huge one stops at its budget; a legitimately macro-heavy shader has
	// room to grow.
	ppTokenBudgetFactor = 4
	ppTokenBudgetBase   = 1 << 16
	// maxPPNesting bounds #if/#ifdef nesting.
	maxPPNesting = 1024
	// maxExpansionDepth bounds nested macro expansion beyond what the
	// being-expanded set already prevents.
	maxExpansionDepth = 128
)

// ppKind is the kind of a preprocessed token.
type ppKind uint8

const (
	ppIdent ppKind = iota
	ppInt
	ppFloat
	ppPunct
	ppHash
	ppNewline
)

// ppToken is one preprocessor token with its place in the source. Spans are
// byte offsets into the original text, so a diagnostic points through
// comments, splices and macro expansion at the text it came from.
type ppToken struct {
	kind ppKind
	// text is the spelling of an identifier or a punctuation token.
	text string

	// Integer literal fields (kind ppInt).
	intValue  uint64
	intSigned bool
	intWidth  int32

	// Float literal fields (kind ppFloat).
	floatValue float32
	floatWidth int32

	span ir.Span
	line uint32 // 1-based physical line of the token's first byte
	// lineOffset is the #line shift in effect where the token was read. It is
	// what __LINE__ adds to the physical line; a macro body takes the
	// invocation's offset, not its definition's.
	lineOffset int64
	// startOfLine is true when only whitespace precedes the token on its line,
	// which is what makes '#' a directive.
	startOfLine bool
	// leadingSpace is true when whitespace precedes the token. It separates a
	// function-like macro definition (`F(x)`) from an object-like one (`F (x)`).
	leadingSpace bool
}

// isPunct reports whether t is the punctuation token s.
func (t ppToken) isPunct(s string) bool { return t.kind == ppPunct && t.text == s }

// ppError is one preprocessor diagnostic. The frontend reports the first as the
// ParseError and the rest as notes (one *ParseError per parse, design R1).
type ppError struct {
	Message string
	Span    ir.Span
}

// ppProfile is the profile of the module's #version directive.
type ppProfile uint8

const (
	ppProfileCore ppProfile = iota
	ppProfileES
)

func (p ppProfile) String() string {
	if p == ppProfileES {
		return "es"
	}
	return "core"
}

// ppVersion is the module's #version directive.
type ppVersion struct {
	Value   uint16
	Profile ppProfile
	Span    ir.Span
}

// ppExtensionBehavior is the behavior named by an #extension directive.
type ppExtensionBehavior uint8

const (
	ppExtensionRequire ppExtensionBehavior = iota
	ppExtensionEnable
	ppExtensionWarn
	ppExtensionDisable
)

func (b ppExtensionBehavior) String() string {
	switch b {
	case ppExtensionRequire:
		return "require"
	case ppExtensionEnable:
		return "enable"
	case ppExtensionWarn:
		return "warn"
	default:
		return "disable"
	}
}

// ppExtension is one #extension directive.
type ppExtension struct {
	Name     string
	Behavior ppExtensionBehavior
	Span     ir.Span
}

// ppPragma is one #pragma directive: its name and its argument tokens.
type ppPragma struct {
	Name string
	Args []ppToken
	Span ir.Span
}

// ppResult is what preprocess produces.
type ppResult struct {
	// Tokens is the preprocessed stream: no directives, no newlines.
	Tokens []ppToken
	// Version is the #version directive; HasVersion reports whether one was
	// present (the frontend requires it).
	Version    ppVersion
	HasVersion bool
	Extensions []ppExtension
	Pragmas    []ppPragma
	Errors     []ppError
}

// ppMacro is one defined macro.
type ppMacro struct {
	name         string
	functionLike bool
	params       []string
	body         []ppToken
}

// preprocess runs the GLSL preprocessor over source with the caller's
// object-like defines (as from -D name=value; a define's body has no source
// span and takes the invocation's span when it is substituted).
func preprocess(source string, defines map[string]string) ppResult {
	if uint64(len(source)) > math.MaxUint32 {
		return ppResult{Errors: []ppError{{Message: "source is longer than 4 GiB"}}}
	}
	tokens, errs := tokenize(source)
	d := &directiveState{macros: map[string]*ppMacro{}}
	for name, value := range defines {
		valueTokens, valueErrs := tokenize(value)
		d.errors = append(d.errors, prefixErrors(valueErrs, "in define "+name+": ")...)
		body := make([]ppToken, 0, len(valueTokens))
		for _, t := range valueTokens {
			if t.kind == ppNewline {
				continue
			}
			t.span, t.line = ir.Span{}, 0
			body = append(body, t)
		}
		d.macros[name] = &ppMacro{name: name, body: body}
	}
	out := d.process(tokens)
	return ppResult{
		Tokens:     out,
		Version:    d.version,
		HasVersion: d.hasVersion,
		Extensions: d.extensions,
		Pragmas:    d.pragmas,
		Errors:     append(errs, d.errors...),
	}
}

// prefixErrors rewrites the messages of errs with prefix.
func prefixErrors(errs []ppError, prefix string) []ppError {
	for i := range errs {
		errs[i].Message = prefix + errs[i].Message
	}
	return errs
}

// scanner reads the source through logical phases 4-7: line endings are
// normalized, backslash-newline is spliced out, and comments become one space.
type scanner struct {
	src    string
	off    int
	line   uint32
	errors []ppError
}

func newScanner(src string) *scanner {
	return &scanner{src: src, line: 1}
}

// readAt returns the next character from off, its offset, and the line the
// character after it starts on. A carriage return, a following line feed, and
// a backslash before a newline are consumed as normalization/splicing require.
func (s *scanner) readAt(off int, line uint32) (r rune, next int, nextLine uint32, ok bool) {
	for {
		if off >= len(s.src) {
			return 0, off, line, false
		}
		r = rune(s.src[off])
		width := 1
		if r >= 0x80 {
			decoded, size := decodeRune(s.src[off:])
			r, width = decoded, size
		}
		next = off + width
		switch r {
		case '\r':
			nextLine = line + 1
			if next < len(s.src) && s.src[next] == '\n' {
				next++
			}
			return '\n', next, nextLine, true
		case '\n':
			return '\n', next, line + 1, true
		case '\\':
			// Splice: a backslash immediately before a newline is removed
			// together with it (phase 6).
			if next < len(s.src) && s.src[next] == '\n' {
				off, line = next+1, line+1
				continue
			}
			if next < len(s.src) && s.src[next] == '\r' {
				after := next + 1
				if after < len(s.src) && s.src[after] == '\n' {
					after++
				}
				off, line = after, line+1
				continue
			}
			return r, next, line, true
		default:
			return r, next, line, true
		}
	}
}

// decodeRune decodes one UTF-8 rune; an invalid sequence is one byte U+FFFD.
func decodeRune(s string) (rune, int) {
	for i := 1; i < len(s) && i < 4; i++ {
		if s[i]&0xC0 != 0x80 {
			r := []rune(s[:i])
			if len(r) == 1 && r[0] != '�' {
				return r[0], i
			}
			return '�', 1
		}
	}
	return '�', 1
}

func (s *scanner) pos() (off int, line uint32) { return s.off, s.line }

// peek returns the next character without consuming it.
func (s *scanner) peek() (rune, bool) {
	r, _, _, ok := s.readAt(s.off, s.line)
	return r, ok
}

// peekN returns the n-th character after the current one (0 is the next).
func (s *scanner) peekN(n int) (rune, bool) {
	off, line := s.off, s.line
	var r rune
	var ok bool
	for i := 0; i <= n; i++ {
		r, off, line, ok = s.readAt(off, line)
		if !ok {
			return 0, false
		}
	}
	return r, true
}

// advance consumes and returns the next character.
func (s *scanner) advance() rune {
	r, next, nextLine, ok := s.readAt(s.off, s.line)
	if !ok {
		return 0
	}
	s.off, s.line = next, nextLine
	return r
}

// errorf records a diagnostic at span.
func (s *scanner) errorf(span ir.Span, format string, args ...any) {
	s.errors = append(s.errors, ppError{Message: fmt.Sprintf(format, args...), Span: span})
}

// skipTrivia consumes whitespace and comments (phase 7) and reports whether any
// was seen.
func (s *scanner) skipTrivia() bool {
	space := false
	for {
		r, ok := s.peek()
		if !ok {
			return space
		}
		switch r {
		case ' ', '\t', '\v', '\f':
			s.advance()
			space = true
			continue
		case '/':
			after, ok := s.peekN(1)
			if !ok {
				return space
			}
			start := s.off
			switch after {
			case '/':
				s.skipLineComment()
				space = true
				continue
			case '*':
				s.skipBlockComment(uint32(start))
				space = true
				continue
			}
			return space
		}
		return space
	}
}

// skipLineComment consumes // to the end of the line, not including the newline.
func (s *scanner) skipLineComment() {
	s.advance() // '/'
	s.advance() // '/'
	for {
		r, ok := s.peek()
		if !ok || r == '\n' {
			return
		}
		s.advance()
	}
}

// skipBlockComment consumes /* through */, reporting an unterminated comment.
func (s *scanner) skipBlockComment(start uint32) {
	s.advance() // '/'
	s.advance() // '*'
	for {
		r, ok := s.peek()
		if !ok {
			s.errorf(ir.Span{Start: start, End: uint32(s.off)}, "unterminated comment")
			return
		}
		if r == '*' {
			if after, ok := s.peekN(1); ok && after == '/' {
				s.advance()
				s.advance()
				return
			}
		}
		s.advance()
	}
}

// tokenize turns the source into preprocessor tokens, newlines included. It is
// phase 8: keywords are not classified here; the lexer does that.
func tokenize(src string) ([]ppToken, []ppError) {
	s := newScanner(src)
	var out []ppToken
	startOfLine := true
	for {
		leading := s.skipTrivia()
		r, ok := s.peek()
		if !ok {
			break
		}
		start, line := s.pos()
		if r == '\n' {
			s.advance()
			out = append(out, ppToken{
				kind: ppNewline, span: ir.Span{Start: uint32(start), End: uint32(s.off)},
				line: line, startOfLine: startOfLine, leadingSpace: leading,
			})
			startOfLine = true
			continue
		}
		tok, ok := s.token(r)
		if !ok {
			continue
		}
		tok.startOfLine = startOfLine
		tok.leadingSpace = leading
		out = append(out, tok)
		startOfLine = false
	}
	return out, s.errors
}
