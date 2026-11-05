// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import "testing"

// TestTypedUAVLoadAdditionalFormatsFlag (P1-DXIL-096): a textureLoad from a
// storage texture whose element has more than one component sets the
// TypedUAVLoadAdditionalFormats shader flag (bit 13), as DXC does for a
// TextureLoad / BufferLoad on a typed UAV with CompCount > 1
// (DxilShaderFlags.cpp:604-627). A single-channel format does not. The
// validator recomputes the flag ("Flags must match usage"), so both cases
// are checked by validation; image/main was rejected with declared=0
// actual=8192.
func TestTypedUAVLoadAdditionalFormatsFlag(t *testing.T) {
	cases := map[string]string{
		"rgba8uint read": `
@group(0) @binding(0) var src: texture_storage_2d<rgba8uint, read>;
@group(0) @binding(1) var dst: texture_storage_2d<rgba8uint, write>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_id) id: vec3<u32>) {
    textureStore(dst, vec2<i32>(id.xy), textureLoad(src, vec2<i32>(id.xy)));
}`,
		"r32uint read": `
@group(0) @binding(0) var src: texture_storage_2d<r32uint, read>;
@group(0) @binding(1) var dst: texture_storage_2d<r32uint, write>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_id) id: vec3<u32>) {
    textureStore(dst, vec2<i32>(id.xy), textureLoad(src, vec2<i32>(id.xy)));
}`,
		"rgba32float read_write": `
@group(0) @binding(0) var img: texture_storage_2d<rgba32float, read_write>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_id) id: vec3<u32>) {
    textureStore(img, vec2<i32>(id.xy), textureLoad(img, vec2<i32>(id.xy)) + vec4(1.0));
}`,
		"rgba8unorm write only": `
@group(0) @binding(0) var dst: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_id) id: vec3<u32>) {
    textureStore(dst, vec2<i32>(id.xy), vec4(1.0));
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) { compileValidateEach(t, src) })
	}
}
