// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"os"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestRestrictBoundsChecks compares the GLSL of shaders that index arrays,
// vectors and matrices in every address space, compiled with the Restrict
// index and buffer policies, with testdata/bounds: each index that may be
// out of bounds is clamped to the last element, by length() for a
// runtime-sized array. bounds-check-restrict.wgsl is Rust naga's test of
// the policy; its MSL clamps the same indices. Regenerate with
// UPDATE_GOLDEN=1 and review the diff.
func TestRestrictBoundsChecks(t *testing.T) {
	for _, tc := range []struct{ source, golden string }{
		{"../snapshot/testdata/rust/in/wgsl/bounds-check-restrict.wgsl", "testdata/bounds/bounds-check-restrict.comp"},
		{"testdata/bounds/spaces.wgsl", "testdata/bounds/spaces.comp"},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			got, _, err := glsl.Compile(lowerWGSL(t, mustReadFile(tc.source)), glsl.Options{
				LangVersion: glsl.Version430,
				EntryPoint:  "main",
				BoundsCheckPolicies: glsl.BoundsCheckPolicies{
					Index:  glsl.BoundsCheckRestrict,
					Buffer: glsl.BoundsCheckRestrict,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(tc.golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want := mustReadFile(tc.golden); got != want {
				t.Errorf("output differs from %s:\n%s", tc.golden, got)
			}
		})
	}
}

func TestReadZeroSkipWriteIndicesAreRefused(t *testing.T) {
	module := lowerWGSL(t, `@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`)
	for _, p := range []glsl.BoundsCheckPolicies{
		{Index: glsl.BoundsCheckReadZeroSkipWrite},
		{Buffer: glsl.BoundsCheckReadZeroSkipWrite},
	} {
		if _, _, err := glsl.Compile(module, glsl.Options{BoundsCheckPolicies: p}); err == nil {
			t.Errorf("%+v compiled", p)
		}
	}
}
