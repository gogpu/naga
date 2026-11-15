// Package errfmt renders parse errors against their source, in Rust naga's
// layout (codespan): the message, then each label under a copy of its line
// with carets, then the notes. The source frontends' public ParseError types
// delegate here, so wgsl.ParseError and glsl.ParseError render alike. Nothing
// in this package is exported from naga: formatting is behavior, not
// vocabulary (design §3, R3).
package errfmt

import (
	"fmt"
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
