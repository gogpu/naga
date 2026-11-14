// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package msl_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
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

// TestMemoryDecorations: MSL writes a coherent module and refuses a volatile
// one, as Rust naga's supported list does (back/msl/mod.rs:879, "No
// MEMORY_DECORATION_VOLATILE" — Metal has no volatile).
func TestMemoryDecorations(t *testing.T) {
	if _, _, err := msl.Compile(memoryDecorationsModule(t, ir.MemoryCoherent), msl.Options{}); err != nil {
		t.Fatalf("coherent module: %v", err)
	}
	_, _, err := msl.Compile(memoryDecorationsModule(t, ir.MemoryVolatile), msl.Options{})
	if err == nil {
		t.Fatal("a volatile module compiled; MSL cannot express volatile")
	}
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("error = %v, want it to wrap backend.ErrUnsupported", err)
	}
}
