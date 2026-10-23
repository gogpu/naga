// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestDepthTextureDeclarations compares the GLSL of the entry points of
// testdata/depth/depth.wgsl with goldens there: a depth texture that is
// sampled without a comparison or loaded is a sampler2D, sampler2DArray or
// samplerCube that reads the depth from its first component, with an
// integer level converted to float, and one sampled with a comparison a
// shadow sampler. A function argument takes the type of the textures passed
// to it. Regenerate with UPDATE_GOLDEN=1 and review the diff.
func TestDepthTextureDeclarations(t *testing.T) {
	source := mustReadFile("testdata/depth/depth.wgsl")
	for _, ep := range []string{"sampled", "loaded", "compared", "helpers"} {
		t.Run(ep, func(t *testing.T) {
			golden := "testdata/depth/" + ep + ".frag"
			got, _, err := glsl.Compile(lowerWGSL(t, source), glsl.Options{LangVersion: glsl.Version430, EntryPoint: ep})
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want := mustReadFile(golden); got != want {
				t.Errorf("output differs from %s:\n%s", golden, got)
			}
			if _, _, err := glsl.Compile(lowerWGSL(t, source), glsl.Options{LangVersion: glsl.VersionES310, EntryPoint: ep, ForceHighPrecision: true}); err != nil {
				t.Errorf("ES 3.10: %v", err)
			}
		})
	}
}

// TestDepthTextureOfBothKindsIsRefused: GLSL has no sampler type for a
// depth texture that is sampled with a comparison and also loaded, or for a
// function argument passed depth textures of both kinds.
func TestDepthTextureOfBothKindsIsRefused(t *testing.T) {
	const globals = `
@group(0) @binding(0) var depth: texture_depth_2d;
@group(0) @binding(1) var shadow: texture_depth_2d;
@group(0) @binding(2) var sc: sampler_comparison;
fn size(t: texture_depth_2d) -> vec2<u32> { return textureDimensions(t); }
`
	for name, body := range map[string]string{
		"compared and loaded": `textureSampleCompare(shadow, sc, p.xy, 0.5) + textureLoad(shadow, vec2<i32>(p.xy), 0)`,
		"both kinds passed":   `textureSampleCompare(shadow, sc, p.xy, 0.5) + textureLoad(depth, vec2<i32>(p.xy), 0) + f32(size(depth).x + size(shadow).x)`,
	} {
		source := globals + "@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return vec4<f32>(" + body + "); }"
		_, _, err := glsl.Compile(lowerWGSL(t, source), glsl.Options{LangVersion: glsl.Version430, EntryPoint: "fs_main"})
		if err == nil || !strings.Contains(err.Error(), "different sampler types") {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
	}
}
