package naga_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
)

const countLeadingZerosShader = `
@group(0) @binding(0) var<storage, read_write> u: array<u32>;
@group(0) @binding(1) var<storage, read_write> i: array<i32>;
@group(0) @binding(2) var<storage, read_write> v: array<vec3<i32>>;

@compute @workgroup_size(1)
fn main() {
    u[1] = countLeadingZeros(u[0]);
    i[1] = countLeadingZeros(i[0]);
    v[1] = countLeadingZeros(v[0]);
}
`

// countLeadingZeros(x) is 31 - (index of the highest set bit), which is 32
// for zero and 0 for a negative i32. Each backend has to subtract from 31
// and treat a signed argument as unsigned.
func TestCountLeadingZeros(t *testing.T) {
	spv, err := naga.Compile(countLeadingZerosShader)
	if err != nil {
		t.Fatal(err)
	}
	const opExtInst, opISub, findSMsb, findUMsb = 12, 130, 74, 75
	words := make([]uint32, len(spv)/4)
	for k := range words {
		words[k] = binary.LittleEndian.Uint32(spv[k*4:])
	}
	msbs := map[uint32]bool{}
	var subtracted int
	for k := 5; k < len(words); {
		n, op := int(words[k]>>16), words[k]&0xffff
		if op == opExtInst && words[k+4] == findSMsb {
			t.Error("SPIR-V uses FindSMsb, which counts from the sign bit")
		}
		if op == opExtInst && words[k+4] == findUMsb {
			msbs[words[k+2]] = true
		}
		if op == opISub && msbs[words[k+4]] {
			subtracted++
		}
		k += n
	}
	if len(msbs) != 3 || subtracted != 3 {
		t.Errorf("SPIR-V: %d FindUMsb, %d subtracted from 31, want 3 and 3", len(msbs), subtracted)
	}

	ast, err := naga.Parse(countLeadingZerosShader)
	if err != nil {
		t.Fatal(err)
	}
	module, err := naga.LowerWithSource(ast, countLeadingZerosShader)
	if err != nil {
		t.Fatal(err)
	}

	h, _, err := hlsl.Compile(module, hlsl.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(h, "(31u - firstbithigh(asuint("); got != 3 {
		t.Errorf("HLSL has %d (31u - firstbithigh(asuint(...))), want 3:\n%s", got, h)
	}

	opts := glsl.DefaultOptions()
	opts.LangVersion = glsl.Version430
	g, _, err := glsl.Compile(module, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"uint((31 - findMSB(", "(31 - findMSB(uint(", "(31 - findMSB(uvec3("} {
		if !strings.Contains(g, want) {
			t.Errorf("GLSL lacks %q:\n%s", want, g)
		}
	}
}
