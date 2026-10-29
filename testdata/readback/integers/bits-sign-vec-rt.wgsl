// extractBits, insertBits and sign on run-time integer vectors: every component on its own.
// e = (0xF0F0F0F0, 0x12345678, 0x00000FF0, 0)
// extractBits(e, 4u, 8u) as u32: (0x0F, 0x67, 0xFF, 0)
// extractBits(i32(e), 4u, 8u): bit 7 of the 8 bits is copied up: (15, 103, -1, 0)
// insertBits(f, n, 8u, 8u), f = (0xFFFFFFFF, 0, 0x12345678, 0xAAAAAAAA), n = (0, 0xAB, 0xFFFFFF01, 0x55):
//   (0xFFFF00FF, 0x0000AB00, 0x12340178, 0xAAAA55AA)
// sign(i32 (-7, 0, 5, INT_MIN)) = (-1, 0, 1, -1)
@group(0) @binding(0) var<storage, read> src: array<vec4<u32>, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<vec4<u32>, 5>;

@compute @workgroup_size(1)
fn main() {
    let e = src[0];
    dst[0] = extractBits(e, 4u, 8u);
    dst[1] = bitcast<vec4<u32>>(extractBits(bitcast<vec4<i32>>(e), 4u, 8u));
    dst[2] = insertBits(src[1], src[2], 8u, 8u);
    dst[3] = bitcast<vec4<u32>>(sign(bitcast<vec4<i32>>(src[3])));
}
