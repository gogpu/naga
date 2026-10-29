// The srem-sign-rt operations with literal operands: naga's constant evaluator computes them.
@group(0) @binding(0) var<storage, read_write> dst: array<i32>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = -7i % 3i;
    dst[1] = 7i % -3i;
    dst[2] = -7i % -3i;
    dst[3] = 7i % 3i;
    dst[4] = i32(-2147483647 - 1) % 3i;
}
