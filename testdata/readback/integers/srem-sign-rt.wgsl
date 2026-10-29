// Signed % of run-time i32 values with negative operands on either side.
// WGSL: e1 - truncate(e1 / e2) * e2, so a non-zero result has the sign of e1.
// -7 % 3 = -1, 7 % -3 = 1, -7 % -3 = -1, 7 % 3 = 1, INT_MIN % 3 = -2
// (INT_MIN / 3 truncates to -715827882; -2147483648 + 2147483646 = -2)
@group(0) @binding(0) var<storage, read> src: array<i32, 10>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < 5u; i++) {
        dst[i] = src[2u * i] % src[2u * i + 1u];
    }
}
