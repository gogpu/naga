package wgsl

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/lower"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// ParseOptions configures [Parse]. The zero value is Rust naga's
// front::wgsl::Options::new(): every extension the parser knows is accepted,
// and validation (valid.Validate) decides which capabilities a module may use.
type ParseOptions struct{}

// ParseInfo is what [Parse] reports besides the module.
type ParseInfo struct {
	// Warnings are the diagnostics that do not stop compilation, such as an
	// unused value of a call whose result must be used.
	Warnings []Diagnostic
}

// Diagnostic is a message about a span of the source.
type Diagnostic struct {
	Message string
	Span    ir.Span
}

// Label marks a span of the source in a [ParseError].
type Label struct {
	Span ir.Span
	Text string
}

// ParseError is the error [Parse] returns, in Rust naga's shape: a message, the
// spans it is about (the first is the primary one) and notes. Error returns the
// message alone, as Rust's Display; [ParseError.Location] and
// [ParseError.Format] place it in the source.
type ParseError struct {
	Message string
	Labels  []Label
	Notes   []string
}

// Error returns the message.
func (e *ParseError) Error() string {
	return e.Message
}

// Location returns the line and column of the primary label in source, and
// false when the error has no label, as Rust's ParseError::location.
func (e *ParseError) Location(source string) (ir.SourceLocation, bool) {
	if len(e.Labels) == 0 {
		return ir.SourceLocation{}, false
	}
	return e.Labels[0].Span.Location(source), true
}

// Format renders the error with the source lines it labels, attributed to
// path, in the layout of Rust naga's emit_to_string_with_path (codespan):
//
//	error: expected identifier, found "("
//	  ┌─ wgsl:1:4
//	  │
//	1 │ fn () {}
//	  │    ^ expected identifier
//
// Each label is drawn under its own copy of its line.
func (e *ParseError) Format(source, path string) string {
	var labels []Label
	for _, l := range e.Labels {
		if l.Span.IsDefined() {
			labels = append(labels, l)
		}
	}
	lines := strings.Split(source, "\n")
	width := 1
	for _, l := range labels {
		width = max(width, len(strconv.Itoa(int(l.Span.Location(source).Line))))
	}
	pad := strings.Repeat(" ", width+1)

	var sb strings.Builder
	fmt.Fprintf(&sb, "error: %s\n", e.Message)
	if len(labels) > 0 {
		loc := labels[0].Span.Location(source)
		fmt.Fprintf(&sb, "%s┌─ %s:%d:%d\n%s│\n", pad, path, loc.Line, loc.Column, pad)
		for _, l := range labels {
			loc := l.Span.Location(source)
			text := lines[min(int(loc.Line), len(lines))-1]
			fmt.Fprintf(&sb, "%*d │ %s\n", width, loc.Line, text)
			// Carets cover the span's characters on its line, at least one.
			lineStart := int(loc.Offset) - int(loc.Column-1)
			startCol := utf8.RuneCountInString(source[lineStart:loc.Offset])
			end := min(int(loc.Offset+loc.Length), lineStart+len(text))
			carets := max(1, utf8.RuneCountInString(source[loc.Offset:max(end, int(loc.Offset))]))
			fmt.Fprintf(&sb, "%s│ %s%s", pad, strings.Repeat(" ", startCol), strings.Repeat("^", carets))
			if l.Text != "" {
				sb.WriteString(" " + l.Text)
			}
			sb.WriteString("\n")
		}
	}
	if len(e.Notes) > 0 {
		if len(labels) > 0 {
			fmt.Fprintf(&sb, "%s│\n", pad)
		}
		for _, n := range e.Notes {
			fmt.Fprintf(&sb, "%s= note: %s\n", pad, n)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// Parse parses WGSL source and lowers it to an IR module, as Rust naga's
// front::wgsl::Frontend::parse. A syntax or semantic error is a *ParseError.
// The module is not validated: call valid.Validate with the capabilities the
// device has.
//
// Parse refuses, with a *ParseError and without recursing deeper, a source
// over 4 GiB and one that nests beyond these limits:
//   - 200 levels of parser recursion (expressions, statements and template
//     arguments, as Rust naga counts them; about 197 nested parentheses);
//   - 127 levels of braces in a function, the WGSL minimum and Rust's limit;
//   - 1024 levels of statements once lowered, each `else if` one level;
//   - expressions 4096 operators deep (operator and member-access chains).
//
// Each limit is above what Rust naga compiles before it overflows its own
// stack, and the deepest program they admit needs a few MiB of goroutine
// stack through the validator and every backend.
func Parse(source string, options ParseOptions) (*ir.Module, ParseInfo, error) {
	// Spans are 32-bit byte offsets, as Rust naga's.
	if uint64(len(source)) > math.MaxUint32 {
		return nil, ParseInfo{}, &ParseError{Message: "source is longer than 4 GiB"}
	}
	p := parser.NewParser(parser.NewLexer(source))
	ast, err := p.Parse()
	if err != nil {
		return nil, ParseInfo{}, fromParserErrors(source, p.Errors(), err)
	}
	result, err := lower.LowerWithWarnings(ast, source)
	if err != nil {
		return nil, ParseInfo{}, fromLowerError(source, err)
	}
	var info ParseInfo
	for _, w := range result.Warnings {
		info.Warnings = append(info.Warnings, Diagnostic{Message: w.Message, Span: spanOf(source, w.Span)})
	}
	return result.Module, info, nil
}

// fromParserErrors converts the parser's errors: the first is the error,
// the others are notes (Rust stops at the first).
func fromParserErrors(source string, errs []parser.ParseError, err error) *ParseError {
	if len(errs) == 0 {
		return &ParseError{Message: err.Error()}
	}
	tokenSpan := func(tok parser.Token) ir.Span {
		start := offsetOf(source, tok.Line, tok.Column)
		return ir.Span{Start: start, End: start + uint32(len(tok.Lexeme))} //nolint:gosec // Parse refuses a source over 4 GiB
	}
	pe := &ParseError{
		Message: errs[0].Message,
		Labels:  []Label{{Span: tokenSpan(errs[0].Token), Text: errs[0].Label}},
		Notes:   append([]string(nil), errs[0].Notes...),
	}
	for _, e := range errs[1:] {
		pe.Notes = append(pe.Notes, fmt.Sprintf("%d:%d: %s", e.Token.Line, e.Token.Column, e.Message))
	}
	return pe
}

// fromLowerError converts the lowerer's errors, the first being the error
// and the others notes.
func fromLowerError(source string, err error) *ParseError {
	var list *parser.SourceErrors
	if !errors.As(err, &list) || len(*list) == 0 {
		var one *parser.SourceError
		if !errors.As(err, &one) {
			return &ParseError{Message: err.Error()}
		}
		list = &parser.SourceErrors{one}
	}
	first := (*list)[0]
	pe := &ParseError{Message: first.Message}
	if span := spanOf(source, first.Span); span.IsDefined() {
		pe.Labels = []Label{{Span: span}}
	}
	for _, e := range (*list)[1:] {
		pe.Notes = append(pe.Notes, e.Error())
	}
	return pe
}

// spanOf converts a parser span (1-based line and column in runes) to bytes.
// A span without an end is the start alone.
func spanOf(source string, s parser.Span) ir.Span {
	if s.Start.Line == 0 {
		return ir.Span{}
	}
	start := offsetOf(source, s.Start.Line, s.Start.Column)
	end := start
	if s.End.Line > 0 {
		end = max(start, offsetOf(source, s.End.Line, s.End.Column))
	}
	return ir.Span{Start: start, End: end}
}

// offsetOf returns the byte offset of a 1-based line and column; the
// parser counts columns in runes.
func offsetOf(source string, line, column int) uint32 {
	off := 0
	for l := 1; l < line; l++ {
		i := strings.IndexByte(source[off:], '\n')
		if i < 0 {
			return uint32(len(source)) //nolint:gosec // Parse refuses a source over 4 GiB
		}
		off += i + 1
	}
	for c := 1; c < column && off < len(source) && source[off] != '\n'; c++ {
		_, size := utf8.DecodeRuneInString(source[off:])
		off += size
	}
	return uint32(off)
}
