// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package textutil provides shared text writing utilities for naga codegen backends.
//
// All three text backends (GLSL, HLSL, MSL) need indent-aware text writing.
// This package extracts the common IndentWriter to eliminate duplication.
package textutil

import (
	"fmt"
	"strings"
)

// IndentWriter writes indented text to a strings.Builder.
// Embed this in backend Writer structs to get indent-aware output methods.
type IndentWriter struct {
	// Out is the output buffer.
	Out strings.Builder

	// Indent is the current indentation level (each level = 4 spaces).
	Indent int
}

// WriteLine writes indented text followed by a newline.
// If args are provided, format is treated as a fmt.Fprintf format string.
//
//nolint:goprintffuncname
func (w *IndentWriter) WriteLine(format string, args ...any) {
	w.WriteIndent()
	if len(args) == 0 {
		w.Out.WriteString(format)
	} else {
		fmt.Fprintf(&w.Out, format, args...)
	}
	w.Out.WriteByte('\n')
}

// WriteIndent writes the current indentation (4 spaces per level).
func (w *IndentWriter) WriteIndent() {
	for i := 0; i < w.Indent; i++ {
		w.Out.WriteString("    ")
	}
}

// PushIndent increases indentation by one level.
func (w *IndentWriter) PushIndent() {
	w.Indent++
}

// PopIndent decreases indentation by one level. Does not go below zero.
func (w *IndentWriter) PopIndent() {
	if w.Indent > 0 {
		w.Indent--
	}
}
