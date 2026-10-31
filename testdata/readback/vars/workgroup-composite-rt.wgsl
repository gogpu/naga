// var<workgroup> vectors, an array of vectors and a struct: zero at the start, then written and read per member.
// src = [5]. Expected: v.y before any write = 0; ws.b before any write = 0; after v = (5, 6): v.x + v.y = 11;
// av[1] = (5, 10, 15, 20) → av[1].z = 15; av[0] is never written → av[0].w = 0; after ws.a = 5: ws.a + ws.b = 5.
// DXIL (P1-DXIL-066): workgroup vectors are refused at compile time; a workgroup struct gives DXIL that
// IDxcValidator rejects ("Invalid record") or crashes on.
struct S {
    a: u32,
    b: u32,
}

@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<workgroup> v: vec2<u32>;
var<workgroup> av: array<vec4<u32>, 2>;
var<workgroup> ws: S;

@compute @workgroup_size(1)
fn main() {
    let o = src[0];
    dst[0] = v.y;
    dst[1] = ws.b;
    v = vec2<u32>(o, o + 1u);
    av[1] = vec4<u32>(o, o * 2u, o * 3u, o * 4u);
    ws.a = o;
    dst[2] = v.x + v.y;
    dst[3] = av[1].z;
    dst[4] = av[0].w;
    dst[5] = ws.a + ws.b;
}
