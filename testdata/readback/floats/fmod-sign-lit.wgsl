// The fmod-sign-rt operations with literal operands: naga's constant evaluator computes them.
@group(0) @binding(0) var<storage, read_write> dst: array<f32>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = -1.5f % 1.0f;
    dst[1] = 1.5f % -1.0f;
    dst[2] = -7.5f % 2.0f;
    dst[3] = 7.5f % -2.0f;
    dst[4] = -7.5f % -2.0f;
}
