// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// TestF64MathResultTypes pins the result types of length, distance and
// determinant on double-precision operands. Their result is the operand's
// scalar kind and width, so an f64 argument yields an f64 result: a module that
// declares these results f32 is invalid (spirv-val: "OpExtInst result type is
// not the same as Result Type"). The source is inline rather than in the corpus
// because the check needs the validator, not a golden.
func TestF64MathResultTypes(t *testing.T) {
	source := `
var<private> v: vec3<f64>;
var<private> w: vec3<f64>;
var<private> m: mat3x3<f64>;

@fragment
fn main() -> @location(0) vec4<f32> {
    let length_v = length(v);
    let distance_vw = distance(v, w);
    let det_m = determinant(m);
    return vec4<f32>(f32(length_v), f32(distance_vw), f32(det_m), 1.0);
}
`
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	binary, err := NewBackend(Options{}).Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(binary) == 0 {
		t.Fatal("compile produced no binary")
	}
	validateWithVulkanSDK(t, binary)
}
