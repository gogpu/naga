// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// A local var with a constant initializer that is read by index keeps the
// initializer (P0-DXIL-084). The emitter resolved the loads of a never-stored
// local to its initializer (init-only), but an indexed load goes to an
// alloca, and that alloca never got the initializer: `var xs = array(...);
// xs[i]` read uninitialized memory while IDxcValidator accepted the module
// (a two-array vertex shader drew nothing on DX12 with DXIL). DXC stores a
// local's initializer after its alloca; Mesa lowers function temporaries'
// initializers to stores at the top of the function
// (nir_lower_variable_initializers).
func TestLocalArrayConstantInitializerStored(t *testing.T) {
	const prelude = `
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
const cp = array<vec2<f32>, 3>(vec2(-1.0, 0.5), vec2(1.0, 0.25), vec2(2.0, 3.0));
`
	tests := []struct{ name, body string }{
		{"scalar array, run-time index", `
    var xs = array<f32, 6>(-1.0, 1.0, -1.0, 1.0, 1.0, -1.0);
    data[0] = xs[idx[0] % 6u];`},
		{"two scalar arrays, run-time index", `
    var xs = array<f32, 6>(-1.0, 1.0, -1.0, 1.0, 1.0, -1.0);
    var ys = array<f32, 6>(-1.0, -1.0, 1.0, -1.0, 1.0, 1.0);
    let i = idx[0] % 6u;
    data[0] = xs[i] + ys[i];`},
		{"vec2 array, run-time index", `
    var p = array<vec2<f32>, 6>(vec2(-1.0, 0.5), vec2(1.0, 0.25), vec2(-1.0), vec2(1.0), vec2(1.0), vec2(-1.0));
    let v = p[idx[0] % 6u];
    data[0] = v.x;
    data[1] = v.y;`},
		{"vec2 array from a named constant, run-time index", `
    var p = cp;
    let v = p[idx[0] % 3u];
    data[0] = v.x;
    data[1] = v.y;`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := emitMain(t, prelude+"@compute @workgroup_size(1) fn main() {"+tt.body+"\n}\n")
			for ptr, first := range firstAccessToAllocas(fn) {
				if first != module.InstrStore {
					t.Errorf("alloca %%%d is read before anything is stored to it: the constant initializer is lost", ptr)
				}
			}
		})
	}
}

// The constant-index form of P0-DXIL-084: the element read is the one of the
// constant initializer (`var xs = array(1, 2, 3); xs[1]` is 2).
func TestLocalArrayConstantInitializerConstantIndex(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"scalar array", `
    var xs = array<i32, 3>(1, 2, 3);
    data[0] = xs[1];`, "c2"},
		{"vec2 array", `
    var p = array<vec2<i32>, 3>(vec2(1, 2), vec2(3, 4), vec2(5, 6));
    let v = p[1];
    data[0] = v.y;`, "c4"},
		{"vec2 array from a named constant", `
    var p = cp;
    let v = p[2];
    data[0] = v.x;`, "c5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> data: array<i32, 4>;
const cp = array<vec2<i32>, 3>(vec2(1, 2), vec2(3, 4), vec2(5, 6));
@compute @workgroup_size(1) fn main() {`+tt.body+"\n}\n")
			if got := straightLineStores(t, mod, fn)[0]; got != tt.want {
				t.Errorf("data[0] is %s, want %s (from the constant initializer)", got, tt.want)
			}
		})
	}
}
