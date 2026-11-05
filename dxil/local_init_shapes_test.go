// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

const indexedLocalPrelude = `
@group(0) @binding(0) var<storage, read_write> data: array<i32, 4>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
struct S { a: i32, b: i32 }
`

// The initial value of a local whose parts are addressed by an index or a
// component is stored before the first access, whatever the local's type
// (P0-DXIL-091). The emitter resolved the loads of a never-stored local to
// its initializer (init-only) or to zero (zero-store), but only a load of the
// whole variable resolves that way: a read of a part goes to an alloca, which
// never got the value, so `v[i]` of a vector, `xs[i][j]` of an array of
// arrays, an element of a zero-initialized array of vectors and an element of
// an array of structs read uninitialized memory while IDxcValidator accepted
// the module. DXC keeps such a local in an alloca with its initializer stored
// after it (and later folds a never-written one into a constant table).
func TestIndexedLocalInitializerStored(t *testing.T) {
	tests := []struct{ name, body string }{
		{"vector, run-time index", `
    var v = vec4<i32>(1, 2, 3, 4);
    data[0] = v[idx[0] % 4u];`},
		{"vector, constant component", `
    var v = vec4<i32>(1, 2, 3, 4);
    data[0] = v.y;`},
		{"zero-initialized vector, run-time index", `
    var v: vec4<i32>;
    data[0] = v[idx[0] % 4u] + 7;`},
		{"array of arrays, run-time indices", `
    var xs = array<array<i32, 2>, 2>(array<i32, 2>(1, 2), array<i32, 2>(3, 4));
    data[0] = xs[idx[0] % 2u][idx[1] % 2u];`},
		{"zero-initialized array of vectors, run-time index", `
    var xs: array<vec2<i32>, 2>;
    let v = xs[idx[0] % 2u];
    data[0] = v.y + 7;`},
		{"zero-initialized array of arrays, run-time indices", `
    var xs: array<array<i32, 2>, 2>;
    data[0] = xs[idx[0] % 2u][idx[1] % 2u] + 7;`},
		{"array of structs, run-time index", `
    var ss = array<S, 2>(S(1, 2), S(3, 4));
    let s = ss[idx[0] % 2u];
    data[0] = s.b;`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := emitMain(t, indexedLocalPrelude+"@compute @workgroup_size(1) fn main() {"+tt.body+"\n}\n")
			for ptr, first := range firstAccessToAllocas(fn) {
				if first != module.InstrStore {
					t.Errorf("alloca %%%d is read before anything is stored to it: the initial value is lost", ptr)
				}
			}
		})
	}
}

// The value an indexed read of such a local gives is the initializer's
// (P0-DXIL-091). Constant indices let the stores be followed symbolically;
// the run-time vector index reads a select over the components, each of
// which must be a stored constant.
func TestIndexedLocalInitializerValues(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"vector, run-time index", `
    var v = vec4<i32>(1, 2, 3, 4);
    data[0] = v[idx[0] % 4u];`, ""},
		{"vector, constant component", `
    var v = vec4<i32>(1, 2, 3, 4);
    data[0] = v.y;`, "c2"},
		{"array of arrays", `
    var xs = array<array<i32, 2>, 2>(array<i32, 2>(1, 2), array<i32, 2>(3, 4));
    data[0] = xs[1][0];`, "c3"},
		{"zero-initialized array of vectors", `
    var xs: array<vec2<i32>, 2>;
    let v = xs[1];
    data[0] = v.y;`, "c0"},
		{"array of structs", `
    var xs = array<S, 2>(S(1, 2), S(3, 4));
    let s = xs[1];
    data[0] = s.b;`, "c4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, fn := emitMainModule(t, indexedLocalPrelude+"@compute @workgroup_size(1) fn main() {"+tt.body+"\n}\n")
			got := straightLineStores(t, mod, fn)[0]
			if strings.Contains(got, "undef") {
				t.Fatalf("data[0] is %s: it reads memory nothing was stored to", got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("data[0] is %s, want %s (from the initializer)", got, tt.want)
			}
		})
	}
}
