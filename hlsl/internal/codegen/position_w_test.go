// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"regexp"
	"testing"
)

// positionInversion matches the reciprocal the fragment position input's w is
// rewritten with. The member name is left open: the namer may disambiguate it
// against the entry point argument of the same name.
var positionInversion = regexp.MustCompile(
	`float4\(fragmentinput_fs_main\.\w+\.xyz, 1\.0 / fragmentinput_fs_main\.\w+\.w\)`)

func mustMatchPositionInversion(t *testing.T, code string) {
	t.Helper()
	if !positionInversion.MatchString(code) {
		t.Errorf("expected the fragment position input's w to be inverted as %q\n--- output ---\n%s",
			positionInversion.String(), code)
	}
}

// In a fragment shader WGSL's position.w is the reciprocal of the clip-space
// w, while D3D's SV_Position.w carries the clip-space w itself, so the w of a
// position input has to be inverted. Tint does the same in its HLSL writer's
// shader_io raise pass (`vec4(pos.xyz, 1.0 / pos.w)`), while Rust naga
// (back/hlsl/conv.rs) maps Position straight to SV_Position and leaves w
// un-inverted — this is a deliberate, spec-first deviation from Rust.
func TestCompile_FragmentPositionWIsReciprocal(t *testing.T) {
	src := `
@fragment
fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) f32 {
    return pos.w;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustMatchPositionInversion(t, code)
}

// The same inversion inside a fragment input struct: the member initialization
// list reads the position member through the same reciprocal.
func TestCompile_FragmentPositionWIsReciprocalInStruct(t *testing.T) {
	src := `
struct FragmentIn {
    @location(0) uv: vec2<f32>,
    @builtin(position) pos: vec4<f32>,
}

@fragment
fn fs_main(in: FragmentIn) -> @location(0) f32 {
    return in.pos.w + in.uv.x;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustMatchPositionInversion(t, code)
}

// A vertex shader's position output is a clip-space position and must stay
// unchanged: only an input read of the position builtin is inverted.
func TestCompile_VertexPositionOutputIsNotInverted(t *testing.T) {
	src := `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(idx), 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustNotContain(t, code, []string{"1.0 / "})
}
