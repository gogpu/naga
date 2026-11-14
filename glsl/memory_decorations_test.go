// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// memoryDecorationsModule parses a storage module and sets the decorations of
// its global.
func memoryDecorationsModule(t *testing.T, decorations ir.MemoryDecorations) *ir.Module {
	t.Helper()
	m, _, err := wgsl.Parse("@group(0) @binding(0) var<storage, read_write> data: array<u32>;\n"+
		"@compute @workgroup_size(1) fn main() { data[0] = 1u; }\n", wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m.GlobalVariables[0].MemoryDecorations = decorations
	return m
}

// TestMemoryDecorations: GLSL writes both decorations, as Rust naga's
// supported list does (back/glsl/mod.rs:506).
func TestMemoryDecorations(t *testing.T) {
	for _, decorations := range []ir.MemoryDecorations{
		ir.MemoryCoherent,
		ir.MemoryVolatile,
		ir.MemoryCoherent | ir.MemoryVolatile,
	} {
		if _, _, err := glsl.Compile(memoryDecorationsModule(t, decorations), glsl.Options{LangVersion: glsl.Version430}); err != nil {
			t.Errorf("decorations %b: %v", decorations, err)
		}
	}
}
