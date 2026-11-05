// exp and log of run-time values, rounded to thousandths so that the few ulps a GPU's exp2 / log2 may differ by do
// not change the result. src = (0.0, 1.0, 2.0, 10.0).
// exp(0) = 1 and log(1) = 0 exactly; round(exp(1) * 1000) = 2718; round(exp(2) * 1000) = 7389;
// round(log(10) * 1000) = 2303; round(log(exp(2)) * 1000) = 2000. Words 6 and 7 are not written.
@group(0) @binding(0) var<storage, read> src: array<f32, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32, 8>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = i32(exp(src[0]));
    dst[1] = i32(log(src[1]));
    let e = exp(vec2(src[1], src[2]));
    dst[2] = i32(round(e.x * 1000.0));
    dst[3] = i32(round(e.y * 1000.0));
    dst[4] = i32(round(log(src[3]) * 1000.0));
    dst[5] = i32(round(log(e.y) * 1000.0));
}
