// dot4I8Packed and dot4U8Packed on run-time words. a = 0x0102FF80, b = 0x7F03FE81, low byte first:
// signed bytes a = (-128, -1, 2, 1), b = (-127, -2, 3, 127): 16256 + 2 + 6 + 127 = 16391
// unsigned bytes a = (128, 255, 2, 1), b = (129, 254, 3, 127): 16512 + 64770 + 6 + 127 = 81415
// Below SM 6.4 DXIL uses the unpack-and-dot polyfill, from 6.4 dx.op.dot4AddPacked. Words 2 and 3 are not written.
@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32, 4>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = dot4I8Packed(src[0], src[1]);
    dst[1] = i32(dot4U8Packed(src[0], src[1]));
}
