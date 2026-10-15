// BUG-DXIL-005 Phase 1: compound RMW on a struct vector field.
// Patterns: `p.vel += gravity;`, `p.vel *= 0.9995;`, `p.pos += p.vel * dt;`.
//
// Naga lowers these to Load(p.vel) + Binary + Store(p.vel, result).
// The Store LHS is AccessIndex(p, vel) (one-level, no component index) so it
// does NOT match tryStructMemberComponentStore (two-level). The generic store
// path then GEPs to a single scalar and stores one value, corrupting the vector.

struct Particle { pos: vec2<f32>, vel: vec2<f32>, }

@group(0) @binding(0) var<storage, read_write> out: array<Particle>;

@compute @workgroup_size(1)
fn main() {
    var p: Particle;
    p.pos = vec2<f32>(1.0, 2.0);
    p.vel = vec2<f32>(3.0, 4.0);
    let gravity = vec2<f32>(0.1, 0.2);
    p.vel += gravity;
    p.vel *= 0.9995;
    p.pos += p.vel * 0.016;
    out[0] = p;
}
