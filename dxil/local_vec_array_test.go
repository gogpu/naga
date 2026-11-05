// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import "testing"

// A component of an element of a local array of vectors, picked by a
// run-time index and used directly as a value (P1-DXIL-081): `p[i].x`. A whole
// element (`let q = p[i]; q.x`) compiled, but the component read gave a
// malformed record ("Invalid record") or "has no value for component 1".
func TestLocalVectorArrayComponentValidates(t *testing.T) {
	const prelude = `
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
`
	tests := []struct{ name, body string }{
		{"array constructor of p[i].x", `
    var p = array<vec2<f32>, 3>(vec2(1.0, 2.0), vec2(3.0, 4.0), vec2(5.0, 6.0));
    let i = idx[0] % 3u;
    let a = array<f32, 1>(p[i].x);
    data[0] = a[0];`},
		{"p[i].y stored", `
    var p = array<vec2<f32>, 3>(vec2(1.0, 2.0), vec2(3.0, 4.0), vec2(5.0, 6.0));
    data[0] = p[idx[0] % 3u].y;`},
		{"p[i].y in a vector with another array", `
    var p = array<vec2<f32>, 6>(vec2(1.0, 2.0), vec2(3.0, 4.0), vec2(5.0, 6.0),
                                vec2(7.0, 8.0), vec2(9.0, 10.0), vec2(11.0, 12.0));
    var xs = array<f32, 6>(1.0, 2.0, 3.0, 4.0, 5.0, 6.0);
    let v = idx[1] % 6u;
    let o = vec4(xs[v], p[v].y, 0.0, 1.0);
    data[0] = o.x;
    data[1] = o.y;`},
		{"p[i][j] both run-time", `
    var p = array<vec3<f32>, 2>(vec3(1.0, 2.0, 3.0), vec3(4.0, 5.0, 6.0));
    data[0] = p[idx[0] % 2u][idx[1] % 3u];`},
		{"p[i].x written", `
    var p = array<vec2<f32>, 3>(vec2(1.0, 2.0), vec2(3.0, 4.0), vec2(5.0, 6.0));
    p[idx[0] % 3u].x = 9.0;
    data[0] = p[1].x + p[idx[1] % 3u].y;`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := prelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
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
