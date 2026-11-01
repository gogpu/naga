// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import "testing"

// TestDeadIfSharedConditionCompiles (P1-DXIL-061): the shader of the card,
// two ifs on one condition with nothing live in the first, compiles to DXIL
// IDxcValidator accepts. The emitter used to meet the second if with no
// value for the condition ("function argument 0 not in value map").
func TestDeadIfSharedConditionCompiles(t *testing.T) {
	const src = `struct Out { v: i32 }
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) li: u32) {
    let c = li > 2u;
    if (c) { var t = 1; }
    if (c) { out.v = 7; }
}`
	blob, err := Compile(lowerSource(t, src), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
