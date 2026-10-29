// step and sign on run-time vec4<f32>: every component on its own (DXIL computed component 0 only).
// step(e, x) = 1.0 where e <= x, else 0.0:
//   e = (0.5, 1.0, -1.0, 2.0), x = (0.5, 0.9, -0.5, 3.0) -> (1, 0, 1, 1)
// sign(v) = -1, 0 or 1:  v = (-2.5, 0.0, 3.0, -0.25) -> (-1, 0, 1, -1)
@group(0) @binding(0) var<storage, read> src: array<vec4<f32>, 3>;
@group(0) @binding(1) var<storage, read_write> dst: array<vec4<f32>, 3>;

@compute @workgroup_size(1)
fn main() {
    dst[0] = step(src[0], src[1]);
    dst[1] = sign(src[2]);
}
