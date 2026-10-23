package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// An element of a local array that holds a copy of a function's result is
// loaded whole, from its first scalar in the flattened array: a[0][1] of
// an array<array<f32, 2>, 3> reads flat index 1. The copy's element was
// loaded as one scalar at the element's index, and its second component
// was the value after that load.
func TestInlinedLocalCopiesLoadWholeElements(t *testing.T) {
	mod, fn := emitMainModule(t, `
fn ret_array() -> array<f32, 2> {
    return array<f32, 2>(1.0, 2.0);
}
fn ret_array_array() -> array<array<f32, 2>, 3> {
    return array<array<f32, 2>, 3>(ret_array(), ret_array(), ret_array());
}
@fragment fn main() -> @location(0) vec4<f32> {
    let a = ret_array_array();
    return vec4<f32>(a[0][0], a[0][1], 0.0, 1.0);
}
`)
	defs := map[int]*module.Instruction{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.HasValue {
				defs[in.ValueID] = in
			}
		}
	}
	var got []int64
	for _, v := range callOperands(fn, "dx.op.storeOutput", 4)[:2] {
		load := defs[v]
		if load == nil || load.Kind != module.InstrLoad {
			t.Fatalf("output component %%%d isn't a load", v)
		}
		gep := defs[load.Operands[0]]
		if gep == nil || gep.Kind != module.InstrGEP {
			t.Fatalf("load %%%d isn't from a GEP", v)
		}
		got = append(got, intConstants(mod, gep.Operands[len(gep.Operands)-1:])...)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("a[0][0] and a[0][1] read flat indices %v, want [0 1]", got)
	}
}
