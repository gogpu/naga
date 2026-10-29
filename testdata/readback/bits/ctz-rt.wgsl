// countTrailingZeros of run-time u32 and i32 values at 0 and at the edges.
// WGSL: the number of consecutive 0 bits starting from the least significant bit (32 for 0).
// u32 0, 1, 8, 0x00F00000, 0x80000000 -> 32, 0, 3, 20, 31
// i32 0, 1, -1, 12, INT_MIN          -> 32, 0, 0, 2, 31
@group(0) @binding(0) var<storage, read> src: array<u32, 10>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < 5u; i++) {
        dst[i] = countTrailingZeros(src[i]);
        dst[5u + i] = bitcast<u32>(countTrailingZeros(bitcast<i32>(src[5u + i])));
    }
}
