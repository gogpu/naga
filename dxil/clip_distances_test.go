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

// clipDistancesSource is a vertex shader writing @builtin(clip_distances)
// as array<f32, n>, through a local struct or as a Compose.
func clipDistancesSource(n int, compose bool) string {
	src := fmt.Sprintf(`enable clip_distances;
struct VOut {
    @builtin(position) pos: vec4<f32>,
    @builtin(clip_distances) clip: array<f32, %d>,
}
@vertex fn main(@builtin(vertex_index) vid: u32) -> VOut {
`, n)
	if compose {
		src += "    var c: array<f32, " + fmt.Sprint(n) + ">;\n"
		src += "    for (var i = 0u; i < " + fmt.Sprint(n) + "u; i++) { c[i] = f32(vid + i); }\n"
		return src + "    return VOut(vec4(0.0, 0.0, 0.0, 1.0), c);\n}\n"
	}
	src += "    var out: VOut;\n    out.pos = vec4(0.0, 0.0, 0.0, 1.0);\n"
	for i := range n {
		src += fmt.Sprintf("    out.clip[%d] = f32(vid) + %d.0;\n", i, i)
	}
	return src + "    return out;\n}\n"
}

// Every element of a clip_distances array is written (P1-DXIL-082). Only
// element 0 was stored for N >= 2, and IDxcValidator rejected the shader:
// "Not all elements of output SV_ClipDistance were written".
func TestClipDistancesEveryElementWritten(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		for _, compose := range []bool{false, true} {
			t.Run(fmt.Sprintf("n=%d compose=%v", n, compose), func(t *testing.T) {
				blob, _, err := Compile(lowerSource(t, clipDistancesSource(n, compose)), DefaultOptions())
				if err != nil {
					t.Fatalf("Compile: %v", err)
				}
				if err := Validate(blob, ValidateFull); err != nil {
					t.Fatalf("Validate: %v", err)
				}
			})
		}
	}
}

// A clip_distances array of 5 to 8 needs a second SV_ClipDistance signature
// element, which the emitter does not write yet (P1-DXIL-082): Compile
// refuses it with an unsupported error naming the size, instead of returning
// a container IDxcValidator rejects ("Not all elements of output
// SV_ClipDistance were written").
func TestClipDistancesOverFourRefused(t *testing.T) {
	for _, n := range []int{5, 6, 7, 8} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			_, _, err := Compile(lowerSource(t, clipDistancesSource(n, false)), DefaultOptions())
			if !errors.Is(err, backend.ErrUnsupported) {
				t.Fatalf("Compile: err = %v, want backend.ErrUnsupported", err)
			}
			if want := fmt.Sprintf("array<f32, %d>", n); !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %s", err, want)
			}
		})
	}
}
