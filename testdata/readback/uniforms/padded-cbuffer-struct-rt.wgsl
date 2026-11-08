// A uniform struct with padding, read member by member. WGSL's uniform layout puts a: f32 at byte 0,
// b: vec2<f32> at 8, c: f32 at 16 and d: vec3<f32> at 32; the struct occupies 48 bytes (12 words).
// The expected words follow from the layout, not from a run: out = (a, b.x, c, d.z) = (1.5, 2.5, 4.5, 7.5).
// P0-DXIL-050: a padded cbuffer struct was read at the wrong offsets.
struct Padded {
    a: f32,
    b: vec2<f32>,
    c: f32,
    d: vec3<f32>,
}

@group(0) @binding(0) var<uniform> u: Padded;
@group(0) @binding(1) var<storage, read_write> out: array<f32, 4>;

@compute @workgroup_size(1)
fn main() {
    out[0] = u.a;
    out[1] = u.b.x;
    out[2] = u.c;
    out[3] = u.d.z;
}
