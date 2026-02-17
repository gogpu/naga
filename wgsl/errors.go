package wgsl

import (
	"fmt"
	"strings"
)

// SourceError represents an error with source location information.
type SourceError struct {
	Message string
	Span    Span
	Source  string // Original source code (for context display)
}

// Error implements the error interface.
func (e *SourceError) Error() string {
	if e.Span.Start.Line == 0 {
		return e.Message
	}
	return fmt.Sprintf("%d:%d: %s", e.Span.Start.Line, e.Span.Start.Column, e.Message)
}

// FormatWithContext returns the error message with source context.
// Shows the problematic line with a caret pointing to the error location.
func (e *SourceError) FormatWithContext() string {
	if e.Source == "" || e.Span.Start.Line == 0 {
		return e.Error()
	}

	lines := strings.Split(e.Source, "\n")
	lineNum := e.Span.Start.Line
	if lineNum < 1 || lineNum > len(lines) {
		return e.Error()
	}

	line := lines[lineNum-1]
	col := e.Span.Start.Column
	if col < 1 {
		col = 1
	}
	if col > len(line)+1 {
		col = len(line) + 1
	}

	// Build the error message with context
	var sb strings.Builder
	fmt.Fprintf(&sb, "error: %s\n", e.Message)
	fmt.Fprintf(&sb, "  --> line %d:%d\n", lineNum, col)
	sb.WriteString("   |\n")
	fmt.Fprintf(&sb, "%3d| %s\n", lineNum, line)
	fmt.Fprintf(&sb, "   | %s^\n", strings.Repeat(" ", col-1))

	return sb.String()
}

// NewSourceError creates a new SourceError.
func NewSourceError(message string, span Span, source string) *SourceError {
	return &SourceError{
		Message: message,
		Span:    span,
		Source:  source,
	}
}

// NewSourceErrorf creates a new SourceError with formatted message.
func NewSourceErrorf(span Span, source string, format string, args ...interface{}) *SourceError {
	return &SourceError{
		Message: fmt.Sprintf(format, args...),
		Span:    span,
		Source:  source,
	}
}

// SourceErrors represents a list of source errors.
type SourceErrors []*SourceError

// Error implements the error interface.
func (el SourceErrors) Error() string {
	if len(el) == 0 {
		return "no errors"
	}
	if len(el) == 1 {
		return el[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", el[0].Error(), len(el)-1)
}

// FormatAll returns all errors formatted with context.
func (el SourceErrors) FormatAll() string {
	var sb strings.Builder
	for i, e := range el {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(e.FormatWithContext())
	}
	return sb.String()
}

// Add adds an error to the list.
func (el *SourceErrors) Add(err *SourceError) {
	*el = append(*el, err)
}

// AddError adds an error with the given message and span.
func (el *SourceErrors) AddError(message string, span Span, source string) {
	el.Add(NewSourceError(message, span, source))
}

// Len returns the number of errors.
func (el SourceErrors) Len() int {
	return len(el)
}

// HasErrors returns true if there are any errors.
func (el SourceErrors) HasErrors() bool {
	return len(el) > 0
}
