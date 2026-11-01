// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestImmediatesKernelParameter: a compute entry point that reads a
// var<immediate> global takes it as a constant reference at its immediates
// buffer slot, as Rust naga's MSL writer does for AddressSpace::Immediate
// (EntryPointResources.immediates_buffer). P1-MSL-097: the kernel used to
// read the global without declaring the parameter, which Metal rejects.
func TestImmediatesKernelParameter(t *testing.T) {
	const src = `
struct Params { scale: f32, idx: u32 }
var<immediate> pc: Params;
@group(0) @binding(0) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main() {
    out[pc.idx] = pc.scale;
}`
	slot := uint8(3)
	outSlot := uint8(0)
	sizesSlot := uint8(30)
	opts := DefaultOptions()
	opts.PerEntryPointMap = map[string]EntryPointResources{"main": {
		ImmediatesBuffer: &slot,
		SizesBuffer:      &sizesSlot,
		Resources:        map[ir.ResourceBinding]BindTarget{{Group: 0, Binding: 0}: {Buffer: &outSlot}},
	}}
	got := compileWGSLWithOpts(t, src, opts)
	if !strings.Contains(got, "constant Params& pc [[buffer(3)]]") {
		t.Errorf("kernel does not take pc at its immediates slot:\n%s", got)
	}
}
