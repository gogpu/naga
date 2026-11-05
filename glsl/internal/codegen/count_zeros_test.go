package codegen

import (
	"strings"
	"testing"
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
// of the lowest set bit, 32 for zero. findMSB and findLSB return -1 for zero,
// and findMSB of a negative int counts from the sign bit, so the argument is
// read as unsigned, the leading count is 31 minus the result and the trailing
// count is clamped to 32 as unsigned.
func TestCountZeros(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version430
	g, _, err := compileWGSLHelper(countZerosShader, opts)
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
