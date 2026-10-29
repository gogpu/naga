// countTrailingZeros of the ctz-rt values as literals: naga's constant evaluator computes them.
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = countTrailingZeros(0u);
    dst[1] = countTrailingZeros(1u);
    dst[2] = countTrailingZeros(8u);
    dst[3] = countTrailingZeros(0x00F00000u);
    dst[4] = countTrailingZeros(0x80000000u);
    dst[5] = bitcast<u32>(countTrailingZeros(0i));
    dst[6] = bitcast<u32>(countTrailingZeros(1i));
    dst[7] = bitcast<u32>(countTrailingZeros(-1i));
    dst[8] = bitcast<u32>(countTrailingZeros(12i));
    dst[9] = bitcast<u32>(countTrailingZeros(i32(-2147483647 - 1)));
}
