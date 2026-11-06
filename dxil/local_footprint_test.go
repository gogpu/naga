// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
)

// A declared array length is a literal in the source, and every whole-array
// initialization of a local is written element by element (DXIL has no
// aggregate store, and DXC unrolls the same zero store: 4097 stores for a
// float[4096], 24 s for a float[65536]). Without a bound a shader of a few
// dozen bytes took seconds per million elements and ran the process out of
// memory at eight million. Compile refuses an entry point whose function and
// private variables take more than maxFunctionVariableBytes, as WGSL lets an
// implementation do above its 8192-byte minimum ("Limits").
func TestCompileRefusesOversizedFunctionVariables(t *testing.T) {
	const n = 1 << 20
	tests := []struct{ name, decl, use string }{
		{"array of vectors", fmt.Sprintf("var a: array<vec2<f32>, %d>;", n), "a[0] = vec2(1.0); o[0] = a[0].x;"},
		{"array of scalars", fmt.Sprintf("var a: array<f32, %d>;", n), "a[0] = 1.0; o[0] = a[0];"},
		{"array of structs", fmt.Sprintf("var a = array<S, %d>();", n), "a[0].k = 1.0; o[0] = a[0].k;"},
		{"two locals together", "var a: array<f32, 12288>; var b: array<f32, 12288>;", "a[1] = 1.0; b[1] = 2.0; o[0] = a[1] + b[1];"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@group(0) @binding(0) var<storage, read_write> o: array<f32>;\nstruct S { p: vec2<f32>, k: f32 }\n" +
				"@compute @workgroup_size(1) fn main() {\n" + tt.decl + "\n" + tt.use + "\n}\n"
			_, _, err := Compile(lowerSource(t, src), DefaultOptions())
			if err == nil || !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), "bytes of function and private variables") {
				t.Fatalf("err = %v, want the variables refused as too large", err)
			}
		})
	}
	t.Run("private array", func(t *testing.T) {
		src := fmt.Sprintf("@group(0) @binding(0) var<storage, read_write> o: array<f32>;\nvar<private> a: array<f32, %d>;\n"+
			"@compute @workgroup_size(1) fn main() { a[0] = 1.0; o[0] = a[0]; }\n", n)
		_, _, err := Compile(lowerSource(t, src), DefaultOptions())
		if err == nil || !errors.Is(err, backend.ErrUnsupported) {
			t.Fatalf("err = %v, want the private array refused as too large", err)
		}
	})
}

// Variables up to the bound compile and validate; the bound is eight times
// the 8192 bytes WGSL requires an implementation to support.
func TestCompileAcceptsFunctionVariablesAtTheBound(t *testing.T) {
	words := maxFunctionVariableBytes / 4
	compileValidateEach(t, fmt.Sprintf(`@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@group(0) @binding(1) var<storage, read> idx: array<u32>;
@compute @workgroup_size(1) fn main() {
    let i = idx[0];
    var a: array<f32, %d>;
    var b: array<vec2<f32>, %d>;
    a[i %% %du] = 1.0;
    b[i %% %du] = vec2(2.0);
    o[0] = a[1] + b[3].y;
}
`, words/2, words/4, words/2, words/4))
}

// The byte size counted for each kind of type: WGSL's byte-size, an array's
// stride times its length, saturating rather than wrapping.
func TestFunctionVariableBytes(t *testing.T) {
	m := lowerSource(t, `
struct S { p: vec3<f32>, k: f32 }
@compute @workgroup_size(1) fn main() {
    var a: f32;
    var b: vec3<f32>;
    var c: mat4x4<f32>;
    var d: array<vec3<f32>, 10>;
    var e: S;
    var f: array<array<S, 4294967295>, 4294967295>;
}
`)
	fn := &m.EntryPoints[0].Function
	want := map[string]uint64{"a": 4, "b": 12, "c": 64, "d": 160, "e": 16, "f": variableBytesCap}
	for _, lv := range fn.LocalVars {
		if got := variableBytes(m, lv.Type); got != want[lv.Name] {
			t.Errorf("%s: %d bytes, want %d", lv.Name, got, want[lv.Name])
		}
	}
}
