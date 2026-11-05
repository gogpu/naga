// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestClipDistanceCount: TranslationInfo.ClipDistanceCount is the size N of
// the `out float gl_ClipDistance[N];` re-declaration of a vertex entry point
// that writes @builtin(clip_distances), and 0 for every other entry point.
// Rust naga: ReflectionInfo.clip_distance_count (back/glsl/mod.rs, set in
// back/glsl/writer.rs where the re-declaration is written). wgpu-hal's GLES
// backend needs the count to enable GL_CLIP_DISTANCE0..N-1.
func TestClipDistanceCount(t *testing.T) {
	vertex := func(n int) string {
		return fmt.Sprintf(`enable clip_distances;
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @builtin(clip_distances) clip_distances: array<f32, %d>,
}
@vertex
fn main() -> VertexOutput {
    var out: VertexOutput;
    out.clip_distances[0] = 0.5;
    return out;
}`, n)
	}
	const (
		vertexWithoutClip = `@vertex fn main() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }`
		fragment          = `@fragment fn main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`
	)
	for _, tc := range []struct {
		name   string
		source string
		want   uint32
		decl   string // re-declaration the GLSL must contain, empty for none
	}{
		{"vertex N=1", vertex(1), 1, "out float gl_ClipDistance[1];"},
		{"vertex N=8", vertex(8), 8, "out float gl_ClipDistance[8];"},
		{"vertex without clip distances", vertexWithoutClip, 0, ""},
		{"fragment", fragment, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, info, err := glsl.Compile(lowerWGSL(t, tc.source), glsl.Options{LangVersion: glsl.Version450})
			if err != nil {
				t.Fatal(err)
			}
			if info.ClipDistanceCount != tc.want {
				t.Errorf("ClipDistanceCount = %d, want %d", info.ClipDistanceCount, tc.want)
			}
			if tc.decl != "" && !strings.Contains(src, tc.decl) {
				t.Errorf("GLSL lacks %q:\n%s", tc.decl, src)
			}
			if tc.decl == "" && strings.Contains(src, "gl_ClipDistance") {
				t.Errorf("GLSL re-declares gl_ClipDistance without a clip_distances output:\n%s", src)
			}
		})
	}
}
