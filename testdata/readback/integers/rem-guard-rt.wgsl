// % where WGSL defines a result the hardware instruction does not: a zero divisor and INT_MIN % -1 give 0.
// i32: 5 % 0 = 0, -5 % 0 = 0, INT_MIN % -1 = 0; u32: 5 % 0 = 0
@group(0) @binding(0) var<storage, read> src: array<i32, 6>;
@group(0) @binding(1) var<storage, read> usrc: array<u32, 2>;
@group(0) @binding(2) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < 3u; i++) {
        dst[i] = bitcast<u32>(src[2u * i] % src[2u * i + 1u]);
    }
    dst[3] = usrc[0] % usrc[1];
}
