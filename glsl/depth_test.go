// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"os"
	"regexp"
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

// TestRestrictedLoadsDeclareTheirLevel: under the Restrict image load policy
// a texelFetch reads the level clamped into a variable declared before it,
// also for a depth texture and for a texture a helper function takes as an
// argument, whose image type the writer reads off the argument.
func TestRestrictedLoadsDeclareTheirLevel(t *testing.T) {
	source := mustReadFile("testdata/depth/depth.wgsl")
	uses := regexp.MustCompile(`_e\d+_clamped_lod`)
	for _, ep := range []string{"loaded", "helpers"} {
		got, _, err := glsl.Compile(lowerWGSL(t, source), glsl.Options{
			LangVersion:         glsl.Version430,
			EntryPoint:          ep,
			BoundsCheckPolicies: glsl.BoundsCheckPolicies{ImageLoad: glsl.BoundsCheckRestrict},
		})
		if err != nil {
			t.Fatal(err)
		}
		names := uses.FindAllString(got, -1)
		if len(names) == 0 {
			t.Errorf("%s: no clamped level:\n%s", ep, got)
		}
		for _, name := range names {
			if !strings.Contains(got, "int "+name+" = clamp(") {
				t.Errorf("%s: %s is used but not declared:\n%s", ep, name, got)
			}
		}
	}
}

// TestUncalledFunctionOfBothKindsIsLeftOut compiles an entry point that
// doesn't call a function taking a depth texture it samples both with and
// without a comparison. The function is written for every entry point whose
// globals cover its own, but GLSL has no type for that argument, so it is
// left out instead of failing an entry point that never calls it.
func TestUncalledFunctionOfBothKindsIsLeftOut(t *testing.T) {
	const source = `
@group(0) @binding(0) var d: texture_depth_2d;
@group(0) @binding(1) var d2: texture_depth_2d;
@group(0) @binding(2) var s: sampler;
@group(0) @binding(3) var sc: sampler_comparison;
fn cmp(t: texture_depth_2d, c: sampler_comparison, uv: vec2<f32>) -> f32 { return textureSampleCompare(t, c, uv, 0.5); }
fn lvl(t: texture_depth_2d, sm: sampler, uv: vec2<f32>) -> f32 { return textureSampleLevel(t, sm, uv, 0); }
fn both(t: texture_depth_2d, uv: vec2<f32>) -> f32 { return cmp(t, sc, uv) + lvl(t, s, uv); }
fn calls_both(uv: vec2<f32>) -> f32 { return both(d2, uv); }
@fragment
fn mixed(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return vec4<f32>(cmp(d, sc, uv), lvl(d2, s, uv), textureSampleLevel(d2, s, uv, 1u), 1.0);
}
@fragment
fn other(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return vec4<f32>(both(d2, uv), 0.0, 0.0, 1.0);
}
`
	got, _, err := glsl.Compile(lowerWGSL(t, source), glsl.Options{LangVersion: glsl.Version430, EntryPoint: "mixed"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "both(") {
		t.Errorf("both, or calls_both calling it, is written:\n%s", got)
	}
	_, _, err = glsl.Compile(lowerWGSL(t, source), glsl.Options{LangVersion: glsl.Version430, EntryPoint: "other"})
	if err == nil || !strings.Contains(err.Error(), "different sampler types") {
		t.Errorf("other: err = %v, want a refusal", err)
	}
}
