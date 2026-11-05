// A local array of arrays, or of structs, read by index holds its initializer, or zero without one.
// src = [1, 0]. Expected:
//   xs[i][j] (xs = ((1, 2), (3, 4))):   3
//   xs[j][i]:                           2
//   xs[1][1]:                           4
//   zs[i][j] + 7 (var zs: 2x2 array):   7
//   ss[i].b (ss = (S(5, 6), S(7, 8))):  8 (through a let of the element)
// DXIL (P0-DXIL-091): an array of arrays or of structs that was never stored resolved only whole-variable loads to
// its initializer; an indexed load went to an alloca nothing was stored to, and read uninitialized memory. A
// zero-initialized array of arrays got no zero store.
struct S { a: u32, b: u32 }

@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let i = src[0];
    let j = src[1];

    var xs = array<array<u32, 2>, 2>(array<u32, 2>(1u, 2u), array<u32, 2>(3u, 4u));
    dst[0] = xs[i][j];
    dst[1] = xs[j][i];
    dst[2] = xs[1][1];

    var zs: array<array<u32, 2>, 2>;
    dst[3] = zs[i][j] + 7u;

    var ss = array<S, 2>(S(5u, 6u), S(7u, 8u));
    let s = ss[i];
    dst[4] = s.b;
}
