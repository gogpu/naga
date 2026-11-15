// Package errfmt renders parse errors against their source, in Rust naga's
// layout (codespan): the message, then each label under a copy of its line
// with carets, then the notes. The source frontends' public ParseError types
// delegate here, so wgsl.ParseError and glsl.ParseError render alike. Nothing
// in this package is exported from naga: formatting is behavior, not
// vocabulary (design §3, R3).
package errfmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// Location returns the line and column of the first label in source, and
// false when there is no label, as Rust's ParseError::location.
func Location(labels []frontend.Label, source string) (ir.SourceLocation, bool) {
	if len(labels) == 0 {
		return ir.SourceLocation{}, false
	}
	return labels[0].Span.Location(source), true
}

// Format renders message with the labels that name a source position,
// attributed to path:
//
//	error: expected identifier, found "("
//	  ┌─ wgsl:1:4
//	  │
//	1 │ fn () {}
//	  │    ^ expected identifier
//
// Each label is drawn under its own copy of its line. A label's undefined
// span is skipped, as Rust's emit_to_string_with_path does.
func Format(message string, labels []frontend.Label, notes []string, source, path string) string {
	defined := make([]frontend.Label, 0, len(labels))
	for _, label := range labels {
		if label.Span.IsDefined() {
			defined = append(defined, label)
		}
	}
	lines := strings.Split(source, "\n")
	width := 1
	for _, label := range defined {
		width = max(width, len(strconv.Itoa(int(label.Span.Location(source).Line))))
	}
	pad := strings.Repeat(" ", width+1)

	var sb strings.Builder
	fmt.Fprintf(&sb, "error: %s\n", message)
	if len(defined) > 0 {
		loc := defined[0].Span.Location(source)
		fmt.Fprintf(&sb, "%s┌─ %s:%d:%d\n%s│\n", pad, path, loc.Line, loc.Column, pad)
		for _, label := range defined {
			loc := label.Span.Location(source)
			text := lines[min(int(loc.Line), len(lines))-1]
			fmt.Fprintf(&sb, "%*d │ %s\n", width, loc.Line, text)
			// Carets cover the span's characters on its line, at least one.
			lineStart := int(loc.Offset) - int(loc.Column-1)
			startCol := utf8.RuneCountInString(source[lineStart:loc.Offset])
			end := min(int(loc.Offset+loc.Length), lineStart+len(text))
			carets := max(1, utf8.RuneCountInString(source[loc.Offset:max(end, int(loc.Offset))]))
			fmt.Fprintf(&sb, "%s│ %s%s", pad, strings.Repeat(" ", startCol), strings.Repeat("^", carets))
			if label.Text != "" {
				sb.WriteString(" " + label.Text)
			}
			sb.WriteString("\n")
		}
	}
	if len(notes) > 0 {
		if len(defined) > 0 {
			fmt.Fprintf(&sb, "%s│\n", pad)
		}
		for _, note := range notes {
			fmt.Fprintf(&sb, "%s= note: %s\n", pad, note)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// SourceLines is the line index of one source text: the byte offset where each
// line starts. Its Location answers exactly as [ir.Span.Location] but in
// O(log lines), so a frontend that reports one diagnostic per line does not
// rescan the source per error — the quadratic cost the deep validation of the
// GLSL frontend measured (100k diagnostics over a 1.4 MB source: 8.2 s).
// Bytes past math.MaxUint32 are outside the span domain; a location there
// saturates at the last addressable offset instead of wrapping.
type SourceLines struct {
	starts []uint32
}

// NewSourceLines indexes source for repeated location lookups. Only the first
// math.MaxUint32 bytes are indexed: ir.Span offsets are uint32, so no span can
// name a position past them, and a uint32 entry cannot hold one.
func NewSourceLines(source string) SourceLines {
	starts := make([]uint32, 0, 1+strings.Count(source, "\n"))
	starts = append(starts, 0)
	for i := 0; i < len(source); i++ {
		if source[i] != '\n' {
			continue
		}
		pos := uint64(i) + 1
		if pos > math.MaxUint32 {
			break
		}
		starts = append(starts, uint32(pos))
	}
	return SourceLines{starts: starts}
}

// Location returns the line and column of s, with [ir.Span.Location]'s
// semantics: 1-based line and column, a span past the end clamped to it.
//
// The values stay in the span's own uint32 domain: the clamp and the line
// arithmetic never pass through an int, so nothing can narrow or truncate on
// the way. A source longer than a span can address saturates instead of
// truncating, the one case where the two domains cannot meet.
func (l SourceLines) Location(source string, s ir.Span) ir.SourceLocation {
	size, fits := narrow32(len(source))
	if !fits {
		// ir.Span offsets are uint32: bytes past math.MaxUint32 are beyond
		// every span, so the location saturates at the last addressable byte.
		size = math.MaxUint32
	}
	start := min(s.Start, size)
	lineIdx := l.lineIndex(start)
	lineStart := l.starts[lineIdx-1]
	line, fits := narrow32(lineIdx)
	if !fits {
		// The same saturation for the line number: more than math.MaxUint32
		// lines cannot be numbered in an ir.SourceLocation.
		line = math.MaxUint32
	}
	length := uint32(0)
	if s.End > s.Start {
		length = s.End - s.Start
	}
	return ir.SourceLocation{
		Line:   line,
		Column: start - lineStart + 1,
		Offset: start,
		Length: length,
	}
}

// lineIndex returns the 1-based line that holds the byte offset start: the
// last line whose first byte is not past it. The first line starts at byte
// zero, so the result is at least one and at most the number of lines.
func (l SourceLines) lineIndex(start uint32) int {
	lo, hi := 0, len(l.starts)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if l.starts[mid] <= start {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// narrow32 narrows a non-negative count to uint32 and reports whether the
// value fits. A caller gets exact arithmetic when it does; when it does not,
// the value is outside the span domain and the caller decides what that means.
func narrow32(v int) (uint32, bool) {
	if v < 0 || uint64(v) > math.MaxUint32 {
		return 0, false
	}
	return uint32(v), true
}
