// Workgroup memory that DXIL splits into scalars, as DXC does (P1-DXIL-066): an array of structs with a vector
// member, a matrix, an array of arrays of vectors and a vector indexed at run time, each written by every
// invocation and read back by another one after the barrier.
// Invocation i reads, with j = 3 - i:
//   ps[j].id + ps[j].v.y = (j + 10) + 3j       → 22, 18, 14, 10
//   g[j / 2][j % 2].y = 100 + j                 → 103, 102, 101, 100
//   c[(i + 1) % 4] = 7 * ((i + 1) % 4)          → 7, 14, 21, 0
//   m[i % 2].x * 10 + m[i % 2].y, m = (1,2 | 3,4) → 12, 34, 12, 34
struct P {
    id: u32,
    v: vec2<u32>,
}

@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

var<workgroup> ps: array<P, 4>;
var<workgroup> m: mat2x2<f32>;
var<workgroup> g: array<array<vec2<u32>, 2>, 2>;
var<workgroup> c: vec4<u32>;

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) i: u32) {
    ps[i] = P(i + 10u, vec2<u32>(i * 2u, i * 3u));
    g[i / 2u][i % 2u] = vec2<u32>(i, 100u + i);
    c[i] = i * 7u;
    if i == 0u {
        m = mat2x2<f32>(1.0, 2.0, 3.0, 4.0);
    }
    workgroupBarrier();
    let j = 3u - i;
    let q = ps[j];
    let col = m[i % 2u];
    dst[i * 4u] = q.id + q.v.y;
    dst[i * 4u + 1u] = g[j / 2u][j % 2u].y;
    dst[i * 4u + 2u] = c[(i + 1u) % 4u];
    dst[i * 4u + 3u] = u32(col.x * 10.0 + col.y);
}
