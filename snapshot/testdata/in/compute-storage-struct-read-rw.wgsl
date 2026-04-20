// Regression test for BUG-DXIL-004: compute shader reading read-only
// storage of struct elements (not vec4). Mirrors gogpu/examples/particles.

struct Particle { pos: vec2<f32>, vel: vec2<f32>, }
struct Params { dt: f32, count: u32, }

@group(0) @binding(0) var<storage, read>       pin:    array<Particle>;
@group(0) @binding(1) var<storage, read_write> pout:   array<Particle>;
@group(0) @binding(2) var<uniform>             params: Params;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let i = id.x;
    if (i >= params.count) { return; }
    var p = pin[i];
    p.vel *= 0.9995;
    p.pos += p.vel * params.dt;
    pout[i] = p;
}
