package codegen

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// countZerosShader takes countLeadingZeros and countTrailingZeros of u32,
// i32 and vec3<i32>.
const countZerosShader = `
@group(0) @binding(0) var<storage, read_write> u: array<u32>;
@group(0) @binding(1) var<storage, read_write> i: array<i32>;
@group(0) @binding(2) var<storage, read_write> v: array<vec3<i32>>;

@compute @workgroup_size(1)
fn main() {
    u[1] = countLeadingZeros(u[0]);
    i[1] = countLeadingZeros(i[0]);
    v[1] = countLeadingZeros(v[0]);
    u[2] = countTrailingZeros(u[0]);
    i[2] = countTrailingZeros(i[0]);
    v[2] = countTrailingZeros(v[0]);
}
`

// countLeadingZeros(x) is 31 - (index of the highest set bit), which is 32
// for zero and 0 for a negative i32, and countTrailingZeros(x) is the index
// of the lowest set bit, 32 for zero. FindUMsb and FindILsb return -1 for
// zero, so the leading count is 31 - FindUMsb (never FindSMsb, which counts
// from the sign bit) and the trailing count is UMin(FindILsb, 32).
func TestCountZeros(t *testing.T) {
	module, _, err := wgsl.Parse(countZerosShader, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	spv, err := NewBackend(DefaultOptions()).Compile(module)
	if err != nil {
		t.Fatal(err)
	}
	const opExtInst, opISub = 12, 130
	const uMin, findILsb, findSMsb, findUMsb = 38, 73, 74, 75
	words := make([]uint32, len(spv)/4)
	for k := range words {
		words[k] = binary.LittleEndian.Uint32(spv[k*4:])
	}
	msbs, lsbs := map[uint32]bool{}, map[uint32]bool{}
	var subtracted, clamped int
	for k := 5; k < len(words); {
		n, op := int(words[k]>>16), words[k]&0xffff
		if op == opExtInst {
			switch words[k+4] {
			case findSMsb:
				t.Error("SPIR-V uses FindSMsb, which counts from the sign bit")
			case findUMsb:
				msbs[words[k+2]] = true
			case findILsb:
				lsbs[words[k+2]] = true
			case uMin:
				if lsbs[words[k+5]] {
					clamped++
				}
			}
		}
		if op == opISub && msbs[words[k+4]] {
			subtracted++
		}
		k += n
	}
	if len(msbs) != 3 || subtracted != 3 {
		t.Errorf("SPIR-V: %d FindUMsb, %d subtracted from 31, want 3 and 3", len(msbs), subtracted)
	}
	if len(lsbs) != 3 || clamped != 3 {
		t.Errorf("SPIR-V: %d FindILsb, %d clamped to 32, want 3 and 3", len(lsbs), clamped)
	}
}
