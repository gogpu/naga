// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package textutil

import "testing"

func TestIndentWriter_WriteLine(t *testing.T) {
	tests := []struct {
		name   string
		indent int
		format string
		args   []any
		want   string
	}{
		{
			name:   "no indent, no args",
			indent: 0,
			format: "hello",
			args:   nil,
			want:   "hello\n",
		},
		{
			name:   "one indent level",
			indent: 1,
			format: "hello",
			args:   nil,
			want:   "    hello\n",
		},
		{
			name:   "two indent levels",
			indent: 2,
			format: "hello",
			args:   nil,
			want:   "        hello\n",
		},
		{
			name:   "with format args",
			indent: 1,
			format: "x = %d;",
			args:   []any{42},
			want:   "    x = 42;\n",
		},
		{
			name:   "with string format",
			indent: 0,
			format: "fn %s() {",
			args:   []any{"main"},
			want:   "fn main() {\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w IndentWriter
			w.Indent = tt.indent
			w.WriteLine(tt.format, tt.args...)
			got := w.Out.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndentWriter_WriteIndent(t *testing.T) {
	tests := []struct {
		name   string
		indent int
		want   string
	}{
		{"zero", 0, ""},
		{"one", 1, "    "},
		{"two", 2, "        "},
		{"three", 3, "            "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w IndentWriter
			w.Indent = tt.indent
			w.WriteIndent()
			got := w.Out.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndentWriter_PushPopIndent(t *testing.T) {
	var w IndentWriter

	if w.Indent != 0 {
		t.Fatalf("initial Indent = %d, want 0", w.Indent)
	}

	w.PushIndent()
	if w.Indent != 1 {
		t.Fatalf("after PushIndent, Indent = %d, want 1", w.Indent)
	}

	w.PushIndent()
	if w.Indent != 2 {
		t.Fatalf("after second PushIndent, Indent = %d, want 2", w.Indent)
	}

	w.PopIndent()
	if w.Indent != 1 {
		t.Fatalf("after PopIndent, Indent = %d, want 1", w.Indent)
	}

	w.PopIndent()
	if w.Indent != 0 {
		t.Fatalf("after second PopIndent, Indent = %d, want 0", w.Indent)
	}

	// PopIndent below zero should stay at zero
	w.PopIndent()
	if w.Indent != 0 {
		t.Fatalf("PopIndent below zero should stay at 0, got %d", w.Indent)
	}
}

func TestIndentWriter_MultipleWrites(t *testing.T) {
	var w IndentWriter

	w.WriteLine("fn main() {")
	w.PushIndent()
	w.WriteLine("let x = %d;", 42)
	w.WriteLine("return x;")
	w.PopIndent()
	w.WriteLine("}")

	want := "fn main() {\n    let x = 42;\n    return x;\n}\n"
	got := w.Out.String()
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
