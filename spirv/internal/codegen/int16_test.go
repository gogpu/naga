package codegen

import "testing"

// TestInt16SPIRV: a 16-bit integer type declares Int16 and the 16-bit
// storage capabilities with SPV_KHR_16bit_storage (Rust naga's make_scalar
// and request_type_capabilities); an i16 constant is sign-extended into its
// word (SPIR-V "Literal": narrower signed types are sign-extended; Rust
// writes value as i32 as u32); a u16 constant is zero-extended.
func TestInt16SPIRV(t *testing.T) {
	spv := compileWGSLForCapabilityTest(t, `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: array<i16, 2>;
@group(0) @binding(1) var<storage, read_write> u: array<u16, 2>;
@compute @workgroup_size(1) fn main() { o[0] = o[1] + i16(-2); u[0] = u[1] + u16(65535); }`)
	assertValidSPIRV(t, spv)
	instrs := decodeSPIRVInstructions(spv)
	for _, c := range []Capability{CapabilityInt16, CapabilityStorageBuffer16BitAccess, CapabilityUniformAndStorageBuffer16BitAccess} {
		if !hasCapability(instrs, c) {
			t.Errorf("missing capability %v", c)
		}
	}
	var sawNeg, sawMax bool
	for _, in := range instrs {
		if in.opcode == OpConstant && len(in.words) == 4 {
			switch in.words[3] {
			case 0xFFFFFFFE:
				sawNeg = true
			case 0xFFFF:
				sawMax = true
			}
		}
	}
	if !sawNeg || !sawMax {
		t.Errorf("constants: sign-extended i16 -2 %v, zero-extended u16 65535 %v; want both", sawNeg, sawMax)
	}
}
