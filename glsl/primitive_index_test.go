// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestPrimitiveIndexCastAndExtension: gl_PrimitiveID is an int and GLSL ES has
// no implicit int to uint conversion, so WGSL's u32 primitive_index is read as
// uint(gl_PrimitiveID). Below GLSL 1.50 and ES 3.20 gl_PrimitiveID comes from
// GL_ARB_geometry_shader4 / GL_OES_geometry_shader, as Rust naga's
// PRIMITIVE_INDEX feature writes it (BUG-GLSL-PRIMITIVE-ID-001; wgpu found the
// ES failure on Mesa llvmpipe ES 3.2).
func TestPrimitiveIndexCastAndExtension(t *testing.T) {
	module := lowerWGSL(t, `
enable primitive_index;
@fragment fn main(@builtin(primitive_index) index: u32) -> @location(0) vec4<f32> {
    return vec4<f32>(f32(index));
}`)
	for _, tc := range []struct {
		version   glsl.Version
		extension string // "" = none
	}{
		{glsl.VersionES310, "#extension GL_OES_geometry_shader : require"},
		{glsl.VersionES320, ""},
		{glsl.Version330, ""},
	} {
		code, _, err := glsl.Compile(module, glsl.Options{LangVersion: tc.version, BoundsCheckPolicies: uncheckedPolicies})
		if err != nil {
			t.Fatalf("%v: %v", tc.version, err)
		}
		if !strings.Contains(code, "uint index = uint(gl_PrimitiveID);") {
			t.Errorf("%v: want uint(gl_PrimitiveID) in:\n%s", tc.version, code)
		}
		hasExt := strings.Contains(code, "_geometry_shader")
		switch {
		case tc.extension != "" && !strings.Contains(code, tc.extension):
			t.Errorf("%v: want %q in:\n%s", tc.version, tc.extension, code)
		case tc.extension == "" && hasExt:
			t.Errorf("%v: no geometry-shader extension expected in:\n%s", tc.version, code)
		}
	}
}
