#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

struct Particle {
    vec2 pos;
    vec2 vel;
};
struct Params {
    float dt;
    uint count;
};
layout(std430) readonly buffer type_3_block_0Compute { Particle _group_0_binding_0_cs[]; };

layout(std430) buffer type_3_block_1Compute { Particle _group_0_binding_1_cs[]; };

layout(std140) uniform Params_block_2Compute { Params _group_0_binding_2_cs; };


void main() {
    uvec3 id = gl_GlobalInvocationID;
    Particle p_1 = Particle(vec2(0.0), vec2(0.0));
    uint i = id.x;
    uint _e4 = _group_0_binding_2_cs.count;
    if ((i >= _e4)) {
        return;
    }
    Particle _e8 = _group_0_binding_0_cs[i];
    p_1 = _e8;
    vec2 _e12 = p_1.vel;
    p_1.vel = (_e12 * vec2(0.9995));
    vec2 _e17 = p_1.vel;
    float _e20 = _group_0_binding_2_cs.dt;
    vec2 _e22 = p_1.pos;
    p_1.pos = (_e22 + (_e17 * _e20));
    Particle _e26 = p_1;
    _group_0_binding_1_cs[i] = _e26;
    return;
}

