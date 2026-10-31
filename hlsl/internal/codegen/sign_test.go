// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"
)

// HLSL's sign returns int for every argument type, so sign of a float is
// converted back to the float type, as Rust naga does; without it a float
// vector got the bit patterns of -1, 0 and 1 on FXC, and asuint stored
// 0xFFFFFFFF for -1.0. sign of an integer is left as it is.
func TestSignOfFloatKeepsItsType(t *testing.T) {
	code := compileWGSLToHLSL(t, `
@group(0) @binding(0) var<storage, read_write> fo: array<vec4<f32>, 2>;
@group(0) @binding(1) var<storage, read_write> io: array<vec4<i32>, 2>;
@compute @workgroup_size(1) fn main() {
    let x = fo[0];
    fo[1] = sign(x);
    fo[0].x = sign(x.y);
    let i = io[0];
    io[1] = sign(i);
    io[0].x = sign(i.y);
}`, nil)
	for want, n := range map[string]int{"float4(sign(": 1, "float(sign(": 1, "sign(": 4} {
		if got := strings.Count(code, want); got != n {
			t.Errorf("%d of %q, want %d:\n%s", got, want, n, code)
		}
	}
}

// sign of an f16 is converted back to half.
func TestSignOfHalfKeepsItsType(t *testing.T) {
	opts := DefaultOptions()
	opts.ShaderModel = ShaderModel6_2
	code := compileWGSLToHLSL(t, `
enable f16;
@group(0) @binding(0) var<storage, read_write> ho: array<vec2<f16>, 2>;
@compute @workgroup_size(1) fn main() {
    let h = ho[0];
    ho[1] = sign(h);
    ho[0].x = sign(h.y);
}`, opts)
	for want, n := range map[string]int{"half2(sign(": 1, "half(sign(": 1, "sign(": 2} {
		if got := strings.Count(code, want); got != n {
			t.Errorf("%d of %q, want %d:\n%s", got, want, n, code)
		}
	}
}
