#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct InStorage {
    vec4 a[10];
};
struct InUniform {
    vec4 a[20];
};
layout(std430) readonly buffer InStorage_block_0Compute { InStorage _group_0_binding_0_cs; };

layout(std140) uniform InUniform_block_1Compute { InUniform _group_0_binding_1_cs; };

uniform sampler2DArray _group_0_binding_2_cs;

shared float in_workgroup[30];

float in_private[40] = float[40](0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0);


vec4 mock_function(ivec2 c, int i, int l) {
    vec4 in_function[2] = vec4[2](vec4(0.707, 0.0, 0.0, 1.0), vec4(0.0, 0.707, 0.0, 1.0));
    vec4 _e18 = _group_0_binding_0_cs.a[i];
    vec4 _e22 = _group_0_binding_1_cs.a[i];
    vec4 _e25 = texelFetch(_group_0_binding_2_cs, ivec3(c, i), l);
    float _e29 = in_workgroup[i];
    float _e34 = in_private[i];
    vec4 _e38 = in_function[i];
    return (((((_e18 + _e22) + _e25) + vec4(_e29)) + vec4(_e34)) + _e38);
}

void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        in_workgroup = float[30](0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0);
    }
    memoryBarrierShared();
    barrier();
    vec4 _e5 = mock_function(ivec2(1, 2), 3, 4);
    return;
}

