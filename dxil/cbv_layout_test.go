package dxil

import (
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// TestConstantBufferReadsAtWGSLOffsets compiles loads from a uniform struct
// and requires the bytes each one reads, from the register of each
// dx.op.cbufferLoadLegacy and the component of each extractvalue of it: a
// two-row matrix has its columns 8 bytes apart, a whole struct is read
// member by member at the offsets WGSL gives them, past the padding, and a
// run-time index into a vector or the columns of a two-row matrix reads
// all of them for a select to pick from.
func TestConstantBufferReadsAtWGSLOffsets(t *testing.T) {
	const decl = `
struct U {
    a: vec2<f32>,
    m2: mat2x2<f32>,
    b: f32,
    m33: mat3x3<f32>,
    c: vec3<f32>,
    d: u32,
}
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;
@group(0) @binding(2) var<storage, read_write> idx: array<u32>;
@compute @workgroup_size(1) fn main() {
`
	for _, tc := range []struct {
		body    string
		want    []uint32
		selects bool
	}{
		{"out[0] = u.m2[1].y;", []uint32{20}, false},
		{"out[0] = u.m33[2].y;", []uint32{68}, false},
		{"out[0] = u.m2[idx[0]].y;", []uint32{8, 12, 16, 20}, true},
		{"out[0] = u.c[idx[0]];", []uint32{80, 84, 88}, true},
		{"let w = u; out[0] = w.b + f32(w.d);", []uint32{0, 4, 8, 12, 16, 20, 24, 32, 36, 40, 48, 52, 56, 64, 68, 72, 80, 84, 88, 92}, false},
	} {
		mod, fn := emitMainModule(t, decl+tc.body+"\n}\n")
		reads, selects := cbvReads(mod, fn)
		if !slices.Equal(reads, tc.want) {
			t.Errorf("%s: reads bytes %v, want %v", tc.body, reads, tc.want)
		}
		if (selects > 0) != tc.selects {
			t.Errorf("%s: %d selects", tc.body, selects)
		}
	}
}

// cbvReads returns the byte offsets the extractvalues of the
// dx.op.cbufferLoadLegacy calls with a constant register read in fn,
// sorted, and the number of selects.
func cbvReads(mod *module.Module, fn *module.Function) (reads []uint32, selects int) {
	constants := map[int]int64{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			constants[c.ValueID] = c.IntValue
		}
	}
	registers := map[int]int64{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			switch {
			case in.Kind == module.InstrCall && in.CalledFunc != nil && strings.HasPrefix(in.CalledFunc.Name, "dx.op.cbufferLoadLegacy"):
				if reg, ok := constants[in.Operands[2]]; ok {
					registers[in.ValueID] = reg
				}
			case in.Kind == module.InstrExtractVal:
				if reg, ok := registers[in.Operands[0]]; ok {
					reads = append(reads, uint32(reg*16)+uint32(in.Operands[1])*4)
				}
			case in.Kind == module.InstrSelect:
				selects++
			}
		}
	}
	slices.Sort(reads)
	return reads, selects
}
