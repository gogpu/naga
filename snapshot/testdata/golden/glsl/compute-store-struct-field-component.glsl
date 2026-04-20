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
    p_1.pos = vec2(0.0, 0.0);
    p_1.vel = vec2(0.0, 0.0);
    p_1.pos.x = 1.0;
    p_1.pos.y = 2.0;
    p_1.vel.x = 3.0;
    p_1.vel.y = 4.0;
    Particle _e23 = p_1;
    _group_0_binding_0_cs[0] = _e23;
    return;
}

