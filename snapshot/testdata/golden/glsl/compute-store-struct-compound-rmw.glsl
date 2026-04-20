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
    Particle p_1 = Particle(vec2(0.0), vec2(0.0));
    p_1.pos = vec2(1.0, 2.0);
    p_1.vel = vec2(3.0, 4.0);
    vec2 gravity = vec2(0.1, 0.2);
    vec2 _e13 = p_1.vel;
    p_1.vel = (_e13 + gravity);
    vec2 _e17 = p_1.vel;
    p_1.vel = (_e17 * vec2(0.9995));
    vec2 _e22 = p_1.vel;
    vec2 _e25 = p_1.pos;
    p_1.pos = (_e25 + (_e22 * 0.016));
    Particle _e29 = p_1;
    _group_0_binding_0_cs[0] = _e29;
    return;
}

