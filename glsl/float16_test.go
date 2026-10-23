// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestFloat16IsRefused: GLSL has no 16-bit float type and Rust naga's
// writer refuses f16 (UnsupportedScalar), so an entry point that writes
// one, in a struct, a conversion or a variable, is refused instead of
// written with 32-bit floats in its place, which in a buffer read other
// bytes than the f16 layout. A module that enables f16 but doesn't use it
// compiles.
func TestFloat16IsRefused(t *testing.T) {
	const prefix = "enable f16;\n@group(0) @binding(0) var<storage, read_write> s: array<f32>;\n"
	for name, src := range map[string]string{
		"struct member": `enable f16;
struct S { a: f32, b: f16 }
@group(0) @binding(0) var<storage, read_write> s: S;
@compute @workgroup_size(1) fn main() { s.a = 1.0; }`,
		"conversion": prefix + `@compute @workgroup_size(1) fn main() { s[0] = f32(f16(s[1]) * f16(s[2])); }`,
		"variable":   prefix + `@compute @workgroup_size(1) fn main() { var h = 1.5h; h *= f16(s[1]); s[0] = f32(h); }`,
	} {
		_, _, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.Version430, EntryPoint: "main"})
		if err == nil || !strings.Contains(err.Error(), "16-bit float") {
			t.Errorf("%s: err = %v, want the 16-bit float error", name, err)
		}
	}
	unused := prefix + `@compute @workgroup_size(1) fn main() { s[0] = 1.0; }`
	if _, _, err := glsl.Compile(lowerWGSL(t, unused), glsl.Options{LangVersion: glsl.Version430, EntryPoint: "main"}); err != nil {
		t.Errorf("f16 enabled but unused: %v", err)
	}
}
