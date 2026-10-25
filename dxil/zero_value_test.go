package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// The zero of a 64-bit integer is a 64-bit constant. It was an i32 zero,
// which a store into 64-bit memory doesn't take.
func TestZeroValueOf64BitIntegerIs64Bit(t *testing.T) {
	mod, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> out: array<vec2<u64>>;
@compute @workgroup_size(1) fn main() {
	out[0] = vec2<u64>();
}
`)
	widths := map[int]uint{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			widths[c.ValueID] = c.ConstType.IntBits
		}
	}
	values := append(callOperands(fn, "dx.op.rawBufferStore.i64", 4), callOperands(fn, "dx.op.rawBufferStore.i64", 5)...)
	if len(values) == 0 {
		t.Fatal("no 64-bit raw buffer store")
	}
	for _, v := range values {
		if bits, ok := widths[v]; ok && bits != 64 {
			t.Errorf("stored zero is an i%d constant, want i64", bits)
		}
	}
}
