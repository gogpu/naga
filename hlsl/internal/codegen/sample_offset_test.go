// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import "testing"

// TestSampleLevelOffsetDimension pins the type of the wrapper around a
// texture sample's offset: it follows the offset, int for 1D and int3 for
// 3D. A fixed int2 widened a 1D offset and dropped a 3D offset's z, so the
// 3D texture was read at the wrong texel (BUG-HLSL-SAMPLE-OFFSET-001; Rust
// naga c079e55f, test 9717-texture-sample-level-offset).
func TestSampleLevelOffsetDimension(t *testing.T) {
	const source = `
@group(0) @binding(0) var sampled_texture_1d: texture_1d<f32>;
@group(0) @binding(1) var sampled_texture_2d: texture_2d<f32>;
@group(0) @binding(2) var sampled_texture_3d: texture_3d<f32>;
@group(0) @binding(3) var texture_sampler: sampler;

@fragment
fn main() -> @location(0) vec4<f32> {
    let sample_1d = textureSampleLevel(sampled_texture_1d, texture_sampler, 0.5, 0.0, -1);
    let sample_2d = textureSampleLevel(sampled_texture_2d, texture_sampler, vec2<f32>(0.5), 0.0, vec2<i32>(-1, 2));
    let sample_3d = textureSampleLevel(sampled_texture_3d, texture_sampler, vec3<f32>(0.5), 0.0, vec3<i32>(-1, 2, -3));
    return sample_1d + sample_2d + sample_3d;
}
`
	code := compileWGSLToHLSL(t, source, nil)
	mustContain(t, code, []string{
		"sampled_texture_1d.SampleLevel(texture_sampler, 0.5, 0.0, int(int(-1)));",
		"sampled_texture_2d.SampleLevel(texture_sampler, (0.5).xx, 0.0, int2(int2(int(-1), int(2))));",
		"sampled_texture_3d.SampleLevel(texture_sampler, (0.5).xxx, 0.0, int3(int3(int(-1), int(2), int(-3))));",
	})
}
