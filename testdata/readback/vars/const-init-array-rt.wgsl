// A local var array with a constant initializer holds it when read by index: run-time index, constant index,
// arrays of vectors (splat and literal elements), a named constant, and inside a loop.
// src = [4, 1]. Expected:
//   xs[i], ys[i]:          14, 24
//   xs[1]:                 11
//   p[j] (= (60, 61)):     60, 61; p[j - 1].y (splat vec2(9u)): 9
//   q = cp, q[j + 1]:      50, 51
//   t[k] in a loop:        7, 8
// DXIL (P0-DXIL-084): the init-only shortcut resolved only whole-variable loads to the initializer; an indexed
// load went to an alloca nothing was stored to, and read uninitialized memory.
const cp = array<vec2<u32>, 3>(vec2(30u, 31u), vec2(40u, 41u), vec2(50u, 51u));

@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let i = src[0];
    let j = src[1];

    var xs = array<u32, 6>(10u, 11u, 12u, 13u, 14u, 15u);
    var ys = array<u32, 6>(20u, 21u, 22u, 23u, 24u, 25u);
    dst[0] = xs[i];
    dst[1] = ys[i % 6u];
    dst[2] = xs[1];

    var p = array<vec2<u32>, 3>(vec2(9u), vec2(60u, 61u), vec2(70u, 71u));
    let v = p[j];
    dst[3] = v.x;
    dst[4] = v.y;
    let s = p[j - 1u];
    dst[5] = s.y;

    var q = cp;
    let w = q[j + 1u];
    dst[6] = w.x;
    dst[7] = w.y;

    var t = array<u32, 2>(7u, 8u);
    for (var k = 0u; k < 2u; k++) {
        dst[8u + k] = t[k];
    }
}
