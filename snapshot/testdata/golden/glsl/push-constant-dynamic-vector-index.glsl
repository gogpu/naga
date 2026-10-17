#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct Immediates {
    vec4 v;
    mat4x4 m;
};
uniform Immediates _immediates_binding_cs;

layout(std430) buffer type_3_block_0Compute { float _group_0_binding_0_cs[]; };


void main() {
    uint idx = gl_LocalInvocationIndex;
    float v_c = _immediates_binding_cs.v.x;
    float v_v = _immediates_binding_cs.v[idx];
    float m_cc = _immediates_binding_cs.m[0][0];
    float m_cv = _immediates_binding_cs.m[0][idx];
    _group_0_binding_0_cs[0] = v_c;
    _group_0_binding_0_cs[1] = v_v;
    _group_0_binding_0_cs[2] = m_cc;
    _group_0_binding_0_cs[3] = m_cv;
    return;
}

