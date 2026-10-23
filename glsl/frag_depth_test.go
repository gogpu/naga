// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestClampFragDepth: with ClampFragDepth, a fragment entry point that
// returns frag_depth, alone or in a struct, declares the range uniform and
// stores frag_depth clamped to it, on desktop GL and ES. Without the option,
// or in an entry point that doesn't write frag_depth, nothing changes.
func TestClampFragDepth(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		values []string
	}{
		"result": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 {
	if p.x > 2.0 { return 0.25; }
	return p.z * 2.0;
}`, []string{"0.25", "(p.z * 2.0)"}},
		"struct": {`
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z * 2.0); }`, []string{"_tmp_return.depth"}},
		"none": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`, nil},
	} {
		for _, version := range []glsl.Version{glsl.Version330, glsl.VersionES300} {
			for _, clamp := range []bool{false, true} {
				got, _, err := glsl.Compile(lowerWGSL(t, tc.source), glsl.Options{
					LangVersion:        version,
					EntryPoint:         "fs_main",
					ForceHighPrecision: true,
					ClampFragDepth:     clamp,
				})
				if err != nil {
					t.Fatalf("%s, %v, clamp %v: %v", name, version, clamp, err)
				}
				decl := "uniform vec2 " + glsl.FragDepthRangeUniform + ";"
				if version.ES {
					decl = "uniform highp vec2 " + glsl.FragDepthRangeUniform + ";"
				}
				want := clamp && tc.values != nil
				if strings.Contains(got, decl) != want {
					t.Errorf("%s, %v, clamp %v: declares the range: %v, want %v:\n%s", name, version, clamp, !want, want, got)
				}
				for _, v := range tc.values {
					store := fmt.Sprintf("gl_FragDepth = clamp(%s, naga_frag_depth_range.x, naga_frag_depth_range.y);", v)
					if strings.Contains(got, store) != want {
						t.Errorf("%s, %v, clamp %v: %q: %v, want %v:\n%s", name, version, clamp, store, !want, want, got)
					}
				}
			}
		}
	}
}
