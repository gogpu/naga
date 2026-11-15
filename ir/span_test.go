// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import "testing"

func TestSpanLocation(t *testing.T) {
	const src = "const a = 1;\nvar<private> b: f32;\n  x"
	for _, tc := range []struct {
		span Span
		want SourceLocation
	}{
		{Span{0, 5}, SourceLocation{Line: 1, Column: 1, Offset: 0, Length: 5}},
		{Span{13, 16}, SourceLocation{Line: 2, Column: 1, Offset: 13, Length: 3}},
		{Span{17, 24}, SourceLocation{Line: 2, Column: 5, Offset: 17, Length: 7}},
		// The newline ends its line: it is the last column, not the next line's first.
		{Span{12, 13}, SourceLocation{Line: 1, Column: 13, Offset: 12, Length: 1}},
		{Span{36, 37}, SourceLocation{Line: 3, Column: 3, Offset: 36, Length: 1}},
		// Past the end: clamped, no panic.
		{Span{90, 95}, SourceLocation{Line: 3, Column: 4, Offset: 37, Length: 5}},
	} {
		if got := tc.span.Location(src); got != tc.want {
			t.Errorf("%+v.Location = %+v, want %+v", tc.span, got, tc.want)
		}
	}
}

func TestSpanIsDefined(t *testing.T) {
	if (Span{}).IsDefined() {
		t.Error("the zero Span is defined")
	}
	if !(Span{0, 1}).IsDefined() || !(Span{3, 3}).IsDefined() {
		t.Error("a non-zero Span is undefined")
	}
}
