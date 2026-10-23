package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// A run-time index into an array value picks its element with a select
// over the index, as one into a vector or matrix value does: two selects
// for the three floats of a. It picked element 0 whatever the index, so
// a vertex stage drawing a fullscreen triangle from a let array put all
// three vertices at the first.
func TestRunTimeIndexIntoArrayValue(t *testing.T) {
	_, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
	let a = array<f32, 3>(1.0, 2.0, 3.0);
	data[0] = a[i];
}
`)
	selects := 0
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrSelect {
				selects++
			}
		}
	}
	if selects != 2 {
		t.Errorf("%d selects, want 2", selects)
	}
}
