// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"runtime"
	"strings"
	"testing"
)

// TestFixedSampleMask: with FixedSampleMask, a fragment entry point ANDs the
// mask into the sample_mask it returns, on every return, and one that
// returns no sample_mask, or nothing at all, returns the mask as a
// [[sample_mask]] output. Without the option, or in a vertex entry point,
// nothing changes.
func TestFixedSampleMask(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		want   []string
	}{
		"color": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`,
			[]string{"uint fixed_sample_mask [[sample_mask]];", "return fs_mainOutput { p, 5u };"}},
		"struct": {`
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z); }`,
			[]string{"uint fixed_sample_mask [[sample_mask]];", "_tmp.depth, 5u };"}},
		"own mask": {`
struct Out { @location(0) color: vec4<f32>, @builtin(sample_mask) mask: u32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out {
	if p.x > 2.0 { return Out(p, 3u); }
	return Out(p, 6u);
}`, []string{"(_tmp.mask & 5u)"}},
		"own mask result": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(sample_mask) u32 { return u32(p.x); }`,
			[]string{"(naga_f2u32(p.x) & 5u)"}},
		"void": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) {
	if p.x > 2.0 { return; }
	if p.y > 2.0 { discard; }
}`, []string{"struct fs_mainOutput {", "uint fixed_sample_mask [[sample_mask]];", "fragment fs_mainOutput fs_main(", "return fs_mainOutput { 5u };"}},
		"vertex": {`
@vertex fn vs_main() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }`, nil},
	} {
		for _, fixed := range []bool{false, true} {
			opts := DefaultOptions()
			if fixed {
				mask := uint32(5)
				opts.FixedSampleMask = &mask
			}
			got := compileWGSLWithOpts(t, tc.source, opts)
			for _, w := range tc.want {
				if strings.Contains(got, w) != fixed {
					t.Errorf("%s, mask %v: %q: %v, want %v:\n%s", name, fixed, w, !fixed, fixed, got)
				}
			}
			if fixed && strings.Contains(got, "fixed_sample_mask") && strings.Count(got, "[[sample_mask]]") != 1 {
				t.Errorf("%s: more than one [[sample_mask]] output:\n%s", name, got)
			}
			if fixed && name == "void" && !strings.Contains(got, "\n    return fs_mainOutput { 5u };\n}\n") {
				t.Errorf("void: no return at the end, after the branch that discards:\n%s", got)
			}
			if fixed && name == "own mask" && strings.Count(got, "& 5u)") != 2 {
				t.Errorf("own mask: want the mask on both returns:\n%s", got)
			}
			if runtime.GOOS == "darwin" {
				verifyMSLWithXcrun(t, got)
			}
		}
	}
}
