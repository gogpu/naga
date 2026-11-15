// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import (
	"sort"
	"strings"
)

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
//
// Location scans the source prefix on every call; a frontend that reports one
// diagnostic per line over a large source pays O(lines × source). Build a
// [SourceLines] index once and use [SourceLines.Location] there instead.
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

// SourceLines is the line index of one source text: the byte offset where each
// line starts. [SourceLines.Location] answers exactly as [Span.Location] but
// in O(log lines), so a frontend reporting many diagnostics does not rescan
// the source per error.
type SourceLines struct {
	starts []uint32
}

// NewSourceLines indexes source for repeated location lookups.
func NewSourceLines(source string) SourceLines {
	starts := make([]uint32, 0, 1+strings.Count(source, "\n"))
	starts = append(starts, 0)
	for i := 0; i < len(source); i++ {
		if source[i] == '\n' {
			starts = append(starts, uint32(i+1))
		}
	}
	return SourceLines{starts: starts}
}

// Location returns the line and column of s, with [Span.Location]'s semantics:
// 1-based line and column, a span past the end clamped to it.
func (l SourceLines) Location(source string, s Span) SourceLocation {
	start := min(int(s.Start), len(source))
	line := sort.Search(len(l.starts), func(i int) bool { return int(l.starts[i]) > start })
	lineStart := int(l.starts[line-1])
	length := uint32(0)
	if s.End > s.Start {
		length = s.End - s.Start
	}
	return SourceLocation{
		Line:   uint32(line),
		Column: uint32(start-lineStart) + 1,
		Offset: uint32(start),
		Length: length,
	}
}
