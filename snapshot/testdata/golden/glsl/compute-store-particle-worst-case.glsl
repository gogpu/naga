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
    vec2 _e11 = p_1.pos;
    float r = length(_e11);
    float rSafe = max(r, 0.05);
    vec2 _e16 = p_1.pos;
    float _e26 = _group_0_binding_2_cs.dt;
    vec2 gravity = (((-(_e16) / vec2(((rSafe * rSafe) * rSafe))) * 0.15) * _e26);
    vec2 _e29 = p_1.vel;
    p_1.vel = (_e29 + gravity);
    vec2 _e33 = p_1.vel;
    p_1.vel = (_e33 * vec2(0.9995));
    vec2 _e38 = p_1.vel;
    float _e41 = _group_0_binding_2_cs.dt;
    vec2 _e43 = p_1.pos;
    p_1.pos = (_e43 + (_e38 * _e41));
    float _e47 = p_1.pos.x;
    if ((_e47 > 1.0)) {
        p_1.pos.x = 1.0;
        float _e56 = p_1.vel.x;
        p_1.vel.x = (_e56 * -0.8);
    }
    float _e60 = p_1.pos.x;
    if ((_e60 < -1.0)) {
        p_1.pos.x = -1.0;
        float _e69 = p_1.vel.x;
        p_1.vel.x = (_e69 * -0.8);
    }
    float _e73 = p_1.pos.y;
    if ((_e73 > 1.0)) {
        p_1.pos.y = 1.0;
        float _e82 = p_1.vel.y;
        p_1.vel.y = (_e82 * -0.8);
    }
    float _e86 = p_1.pos.y;
    if ((_e86 < -1.0)) {
        p_1.pos.y = -1.0;
        float _e95 = p_1.vel.y;
        p_1.vel.y = (_e95 * -0.8);
    }
    Particle _e99 = p_1;
    _group_0_binding_1_cs[i] = _e99;
    return;
}

