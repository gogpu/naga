// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"
)

const localAggregatePrelude = `
@group(0) @binding(0) var<storage, read_write> data: array<i32, 8>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
struct S { a: i32, b: i32 }
struct V { p: vec2<f32>, k: i32 }
struct T { a: array<i32, 2>, k: i32 }
struct Inner { delicious: i32 }
struct Outer { om_nom_nom: Inner, thing: u32 }
`

// Function-local aggregates of every shape compile to modules IDxcValidator
// accepts (P1-DXIL-064, P1-DXIL-065, P1-DXIL-071, P1-DXIL-079). The emitter
// kept a struct local in one alloca and an array of structs had no layout:
// a member of an element (`ss[i].b`, `ss[i].a = x`, `arr[1].a`) was refused
// ("AccessIndex 0 into ir.PointerType"), a struct member that is a struct
// got a GEP of the wrong type ("Explicit gep type does not match pointee
// type"), a run-time Compose stored to a struct with an array member wrote
// the wrong slots ("Explicit load/store type does not match"), and a column
// of a local matrix read by component (`m[1].y`) was refused.
var localAggregateCases = []struct{ name, body string }{
	{"array of structs, constant element member (065)", `
    var arr = array<S, 2>(S(1, 2), S(3, 4));
    data[0] = arr[1].a;`},
	{"array of structs, run-time element member read (065)", `
    var ss = array<S, 2>(S(1, 2), S(3, 4));
    data[0] = ss[idx[0] % 2u].b;`},
	{"array of structs, run-time element member write (065)", `
    var ss = array<S, 2>(S(1, 2), S(3, 4));
    ss[idx[0] % 2u].a = 9;
    data[0] = ss[0].a + ss[1].a;`},
	{"array of structs with a vector member (065)", `
    var vs = array<V, 3>(V(vec2(1.0, 2.0), 1), V(vec2(3.0, 4.0), 2), V(vec2(5.0, 6.0), 3));
    let i = idx[0] % 3u;
    vs[i].p.y = 7.0;
    data[0] = i32(vs[i].p.y) + vs[i].k;`},
	{"array of matrices (065)", `
    var ms = array<mat2x2<f32>, 2>(mat2x2(1.0, 2.0, 3.0, 4.0), mat2x2(5.0, 6.0, 7.0, 8.0));
    data[0] = i32(ms[idx[0] % 2u][1].y);`},
	{"local matrix read by column component (065)", `
    var m = mat2x2(f32(idx[0]), 2.0, 3.0, 4.0);
    data[0] = i32(m[1].y + m[idx[1] % 2u].x);`},
	{"struct member that is a struct, copied (071)", `
    var thing = Outer();
    thing.thing = idx[0];
    var inner = thing.om_nom_nom;
    var delishus = inner.delicious;
    if (thing.thing != u32(delishus)) { data[1] = 5; }
    data[0] = thing.om_nom_nom.delicious;`},
	{"run-time Compose stored to a struct with an array member (064)", `
    var t = T(array<i32, 2>(i32(idx[0]), 1), 2);
    data[0] = t.k;
    data[1] = t.a[1];
    data[2] = t.a[idx[1] % 2u];`},
	{"local array<u32, 6> indexed by a loop counter (079)", `
    var strides: array<u32, 6>;
    for (var d = 0u; d < 6u; d++) { strides[d] = idx[d % 4u] + d; }
    var coords: array<u32, 6>;
    var rem = idx[0];
    for (var d = 0u; d < idx[1]; d++) {
        coords[d] = rem / max(strides[d], 1u);
        rem = rem % max(strides[d], 1u);
    }
    data[0] = i32(coords[idx[2] % 6u]);`},
}

func TestLocalAggregatesValidate(t *testing.T) {
	for _, tt := range localAggregateCases {
		t.Run(tt.name, func(t *testing.T) {
			src := localAggregatePrelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
			blob, _, err := Compile(lowerSource(t, src), DefaultOptions())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

// The values the straight-line local aggregates give (P1-DXIL-064 and the
// zero-initialization case of P1-DXIL-065's log). A zero-initialized struct
// with a struct member started uninitialized when the emitter could not
// store its zero value: a read of a part then read undefined memory.
func TestLocalAggregateValues(t *testing.T) {
	tests := []struct {
		name, body string
		want       map[int64]string
	}{
		{"run-time Compose into a struct with an array member", `
    var t = T(array<i32, 2>(i32(idx[0]), 1), 2);
    data[0] = t.k;
    data[1] = t.a[1];
    data[2] = t.a[0];`, map[int64]string{0: "c2", 4: "c1"}},
		{"zero-initialized struct with a struct member", `
    var o: Outer;
    data[0] = o.om_nom_nom.delicious;
    data[1] = i32(o.thing);`, map[int64]string{0: "c0", 4: "c0"}},
		{"zero-initialized array of structs", `
    var ss: array<Outer, 2>;
    data[0] = ss[1].om_nom_nom.delicious;`, map[int64]string{0: "c0"}},
		{"constant-initialized array of structs", `
    var ss = array<S, 2>(S(1, 2), S(3, 4));
    data[0] = ss[1].a;
    data[1] = ss[0].b;`, map[int64]string{0: "c3", 4: "c2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := localAggregatePrelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
			mod, fn := emitMainModule(t, src)
			got := straightLineStores(t, mod, fn)
			for off, want := range tt.want {
				if got[off] != want {
					t.Errorf("store at byte %d = %q, want %q (all: %v)", off, got[off], want, got)
				}
			}
		})
	}
}
