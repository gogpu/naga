// Run-time shift amounts, taken modulo the bit width as WGSL defines, scalar and vector shifts, and i32 abs.
// src = (3, 35, 0x80000001, 0xFFFFFFF9 = -7)
// 1u << 3 = 8; 1u << 35 = 1u << 3 = 8 (an LLVM shl by 35 is poison: DXIL masks the amount with 31)
// vec2(0x80000001, 6) >> vec2(35, 1) = (0x80000001 >> 3, 3) = (0x10000000, 3)
// i32(-8) >> 33 = -8 >> 1 = -4 = 0xFFFFFFFC; abs(-7) = 7; abs(INT_MIN) = INT_MIN (two's complement wrap)
// vec2(1, 3) << vec2(31, 34) = (0x80000000, 12), stored as their sum 0x8000000C
// Words 8 and 9 are not written.
@group(0) @binding(0) var<storage, read> src: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32, 10>;

@compute @workgroup_size(1)
fn main() {
    let a = src[0];
    let b = src[1];
    dst[0] = 1u << a;
    dst[1] = 1u << b;
    let v = vec2<u32>(src[2], 6u) >> vec2<u32>(b, 1u);
    dst[2] = v.x;
    dst[3] = v.y;
    dst[4] = bitcast<u32>(i32(-8 - i32(a) + 3) >> (b - 2u));
    dst[5] = bitcast<u32>(abs(bitcast<i32>(src[3])));
    dst[6] = bitcast<u32>(abs(bitcast<i32>(src[2] - 1u)));
    let w = vec2<u32>(1u, a) << vec2<u32>(31u, b - 1u);
    dst[7] = w.x + w.y;
}
