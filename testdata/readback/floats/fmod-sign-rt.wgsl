// f32 % of run-time values with negative operands on either side.
// WGSL: e1 - e2 * trunc(e1 / e2), so the result has the sign of e1 (a floored remainder, OpFMod, gives the
// sign of e2). The quotients are not near an integer, so an approximate division still truncates the same way.
// -1.5 % 1 = -0.5, 1.5 % -1 = 0.5, -7.5 % 2 = -1.5, 7.5 % -2 = 1.5, -7.5 % -2 = -1.5
@group(0) @binding(0) var<storage, read> src: array<f32, 10>;
@group(0) @binding(1) var<storage, read_write> dst: array<f32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < 5u; i++) {
        dst[i] = src[2u * i] % src[2u * i + 1u];
    }
}
