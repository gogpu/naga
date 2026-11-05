package codegen

import "testing"

// TestInt16MSL: 16-bit integers (enable wgpu_int16) as Rust naga writes
// them. Arithmetic on short/ushort promotes to int in C++, so the wrapping
// bitcasts truncate with static_cast first (put_bitcasted_expression);
// conversions and bitcasts name short/ushort, not int/uint; the helpers
// wrap their literal operands in the type (`short(1)`) and compare against
// static_cast<short>(-32768), the i16 minimum.
func TestInt16MSL(t *testing.T) {
	code := compileWGSLWithOpts(t, `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: array<i16, 4>;
@group(0) @binding(1) var<storage, read_write> w: array<u32, 4>;
@compute @workgroup_size(1) fn main() {
    o[0] = o[1] + o[2];
    o[1] = o[2] / o[3];
    o[2] = i16(w[0]);
    w[1] = u32(bitcast<u16>(o[3]));
    o[3] = abs(o[0]);
}`, DefaultOptions())
	for _, want := range []string{
		"as_type<short>(static_cast<ushort>(as_type<ushort>(static_cast<ushort>(",
		"short naga_div(short lhs, short rhs) {",
		"return lhs / metal::select(rhs, short(1), (lhs == static_cast<short>(-32768) & rhs == short(-1)) | (rhs == short(0)));",
		"return metal::select(as_type<short>(static_cast<ushort>(-as_type<ushort>(val))), val, val >= short(0));",
		"static_cast<short>(",
		"as_type<ushort>(",
	} {
		mustContainMSL(t, code, want)
	}
}
