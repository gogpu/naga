// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
)

// TestScalarWidthRefusalIsUnsupported: SupportedCapabilities lists
// SHADER_INT64, SHADER_INT16 and SHADER_FLOAT16 as Rust naga's GLSL list does
// (back/glsl/mod.rs supported_capabilities), but the writer refuses 64-bit
// and 16-bit integer and 16-bit float types, as Rust's writer does
// ("GLSL has no 64-bit integer type"). The refusal wraps
// backend.ErrUnsupported, so a caller can tell it from a bug.
func TestScalarWidthRefusalIsUnsupported(t *testing.T) {
	for name, src := range map[string]string{
		"i64": `@group(0) @binding(0) var<storage, read_write> s: array<f32>;
@compute @workgroup_size(1) fn main() { var x = i64(s[0]); s[1] = f32(x); }`,
		"i16": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> s: array<f32>;
@compute @workgroup_size(1) fn main() { var x = i16(s[0]); s[1] = f32(x); }`,
		"f16": `enable f16;
@group(0) @binding(0) var<storage, read_write> s: array<f32>;
@compute @workgroup_size(1) fn main() { var x = f16(s[0]); s[1] = f32(x); }`,
	} {
		_, _, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.Version430})
		if !errors.Is(err, backend.ErrUnsupported) {
			t.Errorf("%s: err = %v, want backend.ErrUnsupported", name, err)
		}
	}
}
