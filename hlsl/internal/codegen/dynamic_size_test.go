// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"
)

// arrayLength() of a storage buffer whose binding has a
// DynamicStorageBufferSizeIndex counts the elements of the binding size in
// the dynamic offsets constant buffer, past the runtime array's offset in
// the struct; one without counts the elements of its view.
func TestArrayLengthOfDynamicStorageBinding(t *testing.T) {
	offset, size := uint32(0), uint32(1)
	opts := DefaultOptions()
	opts.BindingMap = map[ResourceBinding]BindTarget{
		{Group: 0, Binding: 0}: {Register: 0, DynamicStorageBufferOffsetsIndex: &offset, DynamicStorageBufferSizeIndex: &size},
		{Group: 0, Binding: 1}: {Register: 1},
	}
	opts.DynamicStorageBufferOffsetsTargets = map[uint32]OffsetsBindTarget{0: {Register: 2, Size: 2}}
	code := compileWGSLToHLSL(t, `
struct Moved { head: vec4<u32>, items: array<u32> }
@group(0) @binding(0) var<storage, read_write> moved: Moved;
@group(0) @binding(1) var<storage, read_write> fixed: array<u32>;
@compute @workgroup_size(1) fn main() {
    moved.items[0] = arrayLength(&moved.items);
    fixed[0] = arrayLength(&fixed);
}
`, opts)
	for _, want := range []string{
		"((__dynamic_buffer_offsets0._1 - 16) / 4)",
		"((NagaBufferLengthRW(fixed) - 0) / 4)",
		"uint _1;",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("%q not in the HLSL:\n%s", want, code)
		}
	}
}
