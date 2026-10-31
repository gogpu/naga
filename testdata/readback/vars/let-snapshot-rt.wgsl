// A let of a local array or struct is the value of the variable where the let is: later stores to the variable
// don't change it. A var initialized with a struct constructor of run-time values holds those values.
// src = [seed, index] = [7, 1]. Expected:
//   arr let:           a[0] = 7, a[index] = 17 (arr is [8, 18] after the stores)
//   struct init:       s.a = 7, s.b.y = 9
//   struct let:        t.a = 7, t.b.x = 8 (s.a and s.b.x are stored after the let)
//   nested struct let: m.s.a = 7
//   array of vectors:  b[0].x = 7
//   the variable:      arr[0] = 8
// DXIL (P0-DXIL-062): the array let read the variable at the use (8, 18), the struct init and lets stored 0.
struct S { a: u32, b: vec2<u32> }
struct N { s: S, k: u32 }

@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let seed = src[0];
    let i = src[1];

    var arr = array<u32, 2>(seed, seed + 10u);
    let a = arr;
    arr[0] = arr[0] + 1u;
    arr[1] = arr[1] + 1u;
    dst[0] = a[0];
    dst[1] = a[i];

    var s = S(seed, vec2<u32>(seed + 1u, seed + 2u));
    dst[2] = s.a;
    dst[3] = s.b.y;
    let t = s;
    s.a = s.a + 100u;
    s.b.x = 0u;
    dst[4] = t.a;
    dst[5] = t.b.x;

    var n = N(S(seed, vec2<u32>(1u, 2u)), 3u);
    let m = n;
    n.s.a = 0u;
    dst[6] = m.s.a;

    var av = array<vec2<u32>, 2>(vec2<u32>(seed, 1u), vec2<u32>(2u, 3u));
    let b = av;
    av[0].x = 0u;
    dst[7] = b[0].x;

    dst[8] = arr[0];
}
