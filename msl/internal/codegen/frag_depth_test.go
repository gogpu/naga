// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"runtime"
	"strings"
	"testing"
)

// TestClampFragDepth: with ClampFragDepth, a fragment entry point that
// returns frag_depth, alone or in a struct, takes the range at the buffer
// slot given and returns frag_depth clamped to it, on every return. Without
// the option, or in an entry point that doesn't write frag_depth, nothing
// changes.
func TestClampFragDepth(t *testing.T) {
	const param = "constant metal::float2& _frag_depth_range [[buffer(5)]]"
	for name, tc := range map[string]struct {
		source string
		values []string
	}{
		"result": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 {
	if p.x > 2.0 { return 0.25; }
	return p.z * 2.0;
}`, []string{"0.25", "p.z * 2.0"}},
		"struct": {`
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z * 2.0); }`, []string{"_tmp.depth"}},
		"none": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`, nil},
	} {
		for _, clamp := range []bool{false, true} {
			opts := DefaultOptions()
			if clamp {
				opts.ClampFragDepth = &ClampFragDepth{Buffer: 5}
			}
			got := compileWGSLWithOpts(t, tc.source, opts)
			want := clamp && tc.values != nil
			if strings.Contains(got, param) != want {
				t.Errorf("%s, clamp %v: takes the range: %v, want %v:\n%s", name, clamp, !want, want, got)
			}
			for _, v := range tc.values {
				clamped := "metal::clamp(" + v + ", _frag_depth_range.x, _frag_depth_range.y)"
				if strings.Contains(got, clamped) != want {
					t.Errorf("%s, clamp %v: %q: %v, want %v:\n%s", name, clamp, clamped, !want, want, got)
				}
			}
			if runtime.GOOS == "darwin" {
				verifyMSLWithXcrun(t, got)
			}
		}
	}
}
