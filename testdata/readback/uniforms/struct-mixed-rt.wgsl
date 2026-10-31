// One struct with scalar, vec3, vec2, mat2x3, array<vec3> and u32 members, read from a uniform buffer and copied
// member by member into a storage buffer of the same type. WGSL layout (uniform = storage here): a 0, b 16, c 28,
// d 32, m 48 (columns 16 apart), e 80 (elements 16 apart), f 112, size 128.
// u holds a = 1, b = (2, 3, 4), c = 5, d = (6, 7), m = [(8, 9, 10), (11, 12, 13)], e = [(14, 15, 16), (17, 18, 19)],
// f = 3; s starts as zeros.
// Expected s: c = 6 at word 7, d = (6, 7) at 8, m[1] = (11, 12, 13) at 16, e[1] = (17, 18, 19) at 24, f = 3 at 28,
// every other word still 0. out = (u.a + s.b.z, u.c + s.d.y, u.m[1].z + s.e[1].y, f32(u.f) + f32(s.f))
// = (1 + 0, 5 + 7, 13 + 18, 3 + 3) = (1, 12, 31, 6).
struct S {
    a: f32,
    b: vec3<f32>,
    c: f32,
    d: vec2<f32>,
    m: mat2x3<f32>,
    e: array<vec3<f32>, 2>,
    f: u32,
}

@group(0) @binding(0) var<uniform> u: S;
@group(0) @binding(1) var<storage, read_write> s: S;
@group(0) @binding(2) var<storage, read_write> out: vec4<f32>;

@compute @workgroup_size(1)
fn main() {
    s.c = u.c + 1.0;
    s.d = u.d;
    s.e[1] = u.e[1];
    s.m[1] = u.m[1];
    s.f = u.f;
    out = vec4<f32>(u.a + s.b.z, u.c + s.d.y, u.m[1].z + s.e[1].y, f32(u.f) + f32(s.f));
}
