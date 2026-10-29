// countLeadingZeros and countTrailingZeros of a run-time vec4<u32> and vec4<i32>: the vector paths
// (SPIR-V splats the 31/32 constants; DXIL has to scalarize).
// u32 (0, 1, 0x00F00000, 0x80000000): clz 32, 31, 8, 0; ctz 32, 0, 20, 31
// i32 (0, -1, 12, INT_MIN):           clz 32, 0, 28, 0; ctz 32, 0, 2, 31
@group(0) @binding(0) var<storage, read> src: array<vec4<u32>, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<vec4<u32>, 5>;

@compute @workgroup_size(1)
fn main() {
    let u = src[0];
    let s = bitcast<vec4<i32>>(src[1]);
    dst[0] = countLeadingZeros(u);
    dst[1] = countTrailingZeros(u);
    dst[2] = bitcast<vec4<u32>>(countLeadingZeros(s));
    dst[3] = bitcast<vec4<u32>>(countTrailingZeros(s));
}
