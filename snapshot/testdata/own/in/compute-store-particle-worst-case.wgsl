// BUG-DXIL-005 worst-case reproducer. Faithful copy of the gogpu/examples/particles
// compute shader (self-contained — single buffer, trivial boundary). If this
// compiles and validates, the real particles example works end-to-end via the
// GOGPU_DX12_DXIL=1 path. Next agent MUST NOT skip this regression gate.

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

    // Gravity-like RMW on struct vector field.
    let r = length(p.pos);
    let rSafe = max(r, 0.05);
    let gravity = -p.pos / (rSafe * rSafe * rSafe) * 0.15 * params.dt;
    p.vel += gravity;

    // Scalar RMW of struct vector field.
    p.vel *= 0.9995;
    p.pos += p.vel * params.dt;

    // Scalar component assign + scalar component RMW on struct vector field.
    if (p.pos.x >  1.0) { p.pos.x =  1.0; p.vel.x *= -0.8; }
    if (p.pos.x < -1.0) { p.pos.x = -1.0; p.vel.x *= -0.8; }
    if (p.pos.y >  1.0) { p.pos.y =  1.0; p.vel.y *= -0.8; }
    if (p.pos.y < -1.0) { p.pos.y = -1.0; p.vel.y *= -0.8; }

    pout[i] = p;
}
