package codegen

import (
	"strings"
	"testing"
)

// TestInt16HLSL: 16-bit integers (enable wgpu_int16) as Rust naga writes
// them for shader model 6.2: int16_t/uint16_t types and literals, a bitcast
// through the target's constructor (asint/asuint take only 32-bit values),
// and naga_div comparing against int16_t(-32768), the i16 minimum; naga
// compared against the i32 minimum, so -32768 / -1 was not guarded.
func TestInt16HLSL(t *testing.T) {
	code := wgslToHLSL(t, `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: array<i16, 4>;
@group(0) @binding(1) var<storage, read_write> u: array<u16, 4>;
@compute @workgroup_size(1) fn main() {
    o[0] = o[1] / o[2];
    u[0] = bitcast<u16>(o[3]);
    o[1] = i16(-7);
}`)
	for _, want := range []string{
		"int16_t naga_div(int16_t lhs, int16_t rhs)",
		"(lhs == int16_t(-32768) & rhs == -1)",
		"uint16_t(",
		"int16_t(-7)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in:\n%s", want, code)
		}
	}
	for _, bad := range []string{"asuint(", "int(-2147483647 - 1)"} {
		if strings.Contains(code, bad) {
			t.Errorf("unexpected %q in:\n%s", bad, code)
		}
	}
}
