// BUG-DXIL-005 Phase 1: scalar component assign into a struct vector field.
// This is the pattern: `p.vel.x = 1.0;` from gogpu/examples/particles.
// Goes through tryStructMemberComponentStore path.

struct Particle { pos: vec2<f32>, vel: vec2<f32>, }

@group(0) @binding(0) var<storage, read_write> out: array<Particle>;

@compute @workgroup_size(1)
fn main() {
    var p: Particle;
    p.pos = vec2<f32>(0.0, 0.0);
    p.vel = vec2<f32>(0.0, 0.0);
    p.pos.x = 1.0;
    p.pos.y = 2.0;
    p.vel.x = 3.0;
    p.vel.y = 4.0;
    out[0] = p;
}
