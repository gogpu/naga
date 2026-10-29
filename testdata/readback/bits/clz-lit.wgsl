// countLeadingZeros of the clz-rt values as literals: naga's constant evaluator computes them.
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = countLeadingZeros(0u);
    dst[1] = countLeadingZeros(1u);
    dst[2] = countLeadingZeros(0x00F00000u);
    dst[3] = countLeadingZeros(0x7FFFFFFFu);
    dst[4] = countLeadingZeros(0x80000000u);
    dst[5] = bitcast<u32>(countLeadingZeros(0i));
    dst[6] = bitcast<u32>(countLeadingZeros(1i));
    dst[7] = bitcast<u32>(countLeadingZeros(-1i));
    dst[8] = bitcast<u32>(countLeadingZeros(0x7FFFFFFFi));
    dst[9] = bitcast<u32>(countLeadingZeros(i32(-2147483647 - 1)));
}
