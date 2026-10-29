// countLeadingZeros of run-time u32 and i32 values at 0 and at the edges.
// WGSL: the number of consecutive 0 bits starting from the most significant bit (32 for 0). Read as i32, a
// negative value has its sign bit set, so it gives 0.
// u32 0, 1, 0x00F00000, 0x7FFFFFFF, 0x80000000 -> 32, 31, 8, 1, 0
// i32 0, 1, -1, 0x7FFFFFFF, INT_MIN           -> 32, 31, 0, 1, 0
@group(0) @binding(0) var<storage, read> src: array<u32, 10>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < 5u; i++) {
        dst[i] = countLeadingZeros(src[i]);
        dst[5u + i] = bitcast<u32>(countLeadingZeros(bitcast<i32>(src[5u + i])));
    }
}
