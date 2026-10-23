package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// countBinOps returns the number of binary operations op in fn.
func countBinOps(fn *module.Function, op int) int {
	n := 0
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrBinOp && len(in.Operands) == 3 && in.Operands[2] == op {
				n++
			}
		}
	}
	return n
}

// A unary operator on a vector applies to every component: -v of a vec3
// is three subtractions and ~u of a vec2<u32> two xors. It applied to the
// first component only, and the others took the values after it, as the
// y of -p.pos in the particles shader of naga's own tests did.
func TestUnaryOperatorsOnEveryComponent(t *testing.T) {
	_, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> f: array<vec3<f32>>;
@group(0) @binding(1) var<storage, read_write> u: array<vec2<u32>>;
@compute @workgroup_size(1) fn main() {
	f[1] = -f[0];
	u[1] = ~u[0];
}
`)
	if n := countBinOps(fn, 1); n != 3 {
		t.Errorf("%d subtractions for -v, want 3", n)
	}
	if n := countBinOps(fn, 12); n != 2 {
		t.Errorf("%d xors for ~u, want 2", n)
	}
}

// sign of a vector is the sign of every component, two selects each. It
// was the sign of the first component only.
func TestSignOfEveryComponent(t *testing.T) {
	_, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> f: array<vec3<f32>>;
@compute @workgroup_size(1) fn main() {
	f[1] = sign(f[0]);
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
	if selects != 6 {
		t.Errorf("%d selects, want 6", selects)
	}
}

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
