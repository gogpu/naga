// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import "testing"

// TestBuiltinIndexIntoLocalArrayCompiles (P1-DXIL-106): a compute built-in
// argument used as a run-time index into a local array compiles to DXIL
// IDxcValidator accepts. The emitter used to lose the argument's value and
// fail with "function argument 0 not in value map".
func TestBuiltinIndexIntoLocalArrayCompiles(t *testing.T) {
	const src = `@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 4>;
    a[i % 4u] = 1.0;
    o[0] = a[1];
}`
	blob, _, err := Compile(lowerSource(t, src), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
