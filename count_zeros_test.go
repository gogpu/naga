package naga_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
)

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
// of the lowest set bit, 32 for zero. The bit-finding built-ins of each
// backend return -1 for zero, so the backends read their result and a signed
// argument as unsigned, then subtract it from 31 or clamp it to 32.
func TestCountZeros(t *testing.T) {
	spv, err := naga.Compile(countZerosShader)
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

	ast, err := naga.Parse(countZerosShader)
	if err != nil {
		t.Fatal(err)
	}
	module, err := naga.LowerWithSource(ast, countZerosShader)
	if err != nil {
		t.Fatal(err)
	}

	h, _, err := hlsl.Compile(module, hlsl.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"(31u - firstbithigh(asuint(", "min(32u, firstbitlow(asuint("} {
		if got := strings.Count(h, want); got != 3 {
			t.Errorf("HLSL has %d %q, want 3:\n%s", got, want, h)
		}
	}

	opts := glsl.DefaultOptions()
	opts.LangVersion = glsl.Version430
	g, _, err := glsl.Compile(module, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"uint((31 - findMSB(", "(31 - findMSB(uint(", "(31 - findMSB(uvec3(",
		"min(uint(findLSB(", "int(min(uint(findLSB(", "ivec3(min(uvec3(findLSB(",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("GLSL lacks %q:\n%s", want, g)
		}
	}
}
