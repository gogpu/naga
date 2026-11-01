#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct Particle {
    vec2 pos;
    vec2 vel;
};
layout(std430) buffer type_1_block_0Compute { Particle _group_0_binding_0_cs[]; };


void main() {
    Particle p = Particle(vec2(0.0), vec2(0.0));
    p.pos = vec2(1.0, 2.0);
    p.vel = vec2(3.0, 4.0);
    vec2 gravity = vec2(0.1, 0.2);
    vec2 _e13 = p.vel;
    p.vel = (_e13 + gravity);
    vec2 _e16 = p.vel;
    p.vel = (_e16 * 0.9995);
    vec2 _e20 = p.pos;
    vec2 _e22 = p.vel;
    p.pos = (_e20 + (_e22 * 0.016));
    Particle _e28 = p;
    _group_0_binding_0_cs[0] = _e28;
    return;
}

