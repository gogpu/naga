// A local array of vectors declared without an initializer starts at zero, also when read by a run-time index.
// src = [1]. Expected:
//   xs[i].x + 7, xs[i].y + 9:            7, 9
//   ys[i].y after ys[0] = (3, 4):        0 (the element not written)
//   ys[i - 1].y:                         4
// DXIL (P0-DXIL-091): the zero store of a local was written only for types storableWhole accepts, which left out
// arrays of vectors, so an array of vectors nothing was stored to read uninitialized memory.
@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let i = src[0];

    var xs: array<vec2<u32>, 2>;
    let v = xs[i];
    dst[0] = v.x + 7u;
    dst[1] = v.y + 9u;

    var ys: array<vec2<u32>, 2>;
    ys[0] = vec2<u32>(3u, 4u);
    let w = ys[i];
    dst[2] = w.y;
    let u = ys[i - 1u];
    dst[3] = u.y;
}
