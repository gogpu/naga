// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import "strings"

// Span is a byte range [Start, End) of a shader's source text, as Rust
// naga's Span. The zero Span is undefined: no source position is known.
// Frontends, error labels and diagnostics share this one type.
type Span struct {
	Start uint32
	End   uint32
}

// IsDefined reports whether s names a source position, as Rust's
// Span::is_defined: the zero Span does not.
func (s Span) IsDefined() bool {
	return s != Span{}
}

// SourceLocation is a Span's position in a source text, as Rust naga's
// SourceLocation.
type SourceLocation struct {
	// Line is the 1-based line of Start.
	Line uint32
	// Column is the 1-based byte position of Start in its line.
	Column uint32
	// Offset is Start, the byte offset in the whole source.
	Offset uint32
	// Length is End - Start in bytes.
	Length uint32
}

// Location returns the line and column of s in source, as Rust's
// Span::location. A Span past the end of source is clamped to it.
func (s Span) Location(source string) SourceLocation {
	start := min(int(s.Start), len(source))
	prefix := source[:start]
	lineStart := strings.LastIndexByte(prefix, '\n') + 1
	length := uint32(0)
	if s.End > s.Start {
		length = s.End - s.Start
	}
	return SourceLocation{
		Line:   uint32(strings.Count(prefix, "\n")) + 1,
		Column: uint32(start-lineStart) + 1,
		Offset: uint32(start),
		Length: length,
	}
}
