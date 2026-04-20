#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

struct Params {
    float scale;
    float offset;
};
layout(std430) readonly buffer type_2_block_0Compute { vec4 _group_0_binding_0_cs[]; };

layout(std430) buffer type_2_block_1Compute { vec4 _group_0_binding_1_cs[]; };

layout(std140) uniform Params_block_2Compute { Params _group_0_binding_2_cs; };


void main() {
    uvec3 gid = gl_GlobalInvocationID;
    uint i = gid.x;
    uint n = uint(_group_0_binding_0_cs.length());
    if ((i >= n)) {
        return;
    }
    vec4 v = _group_0_binding_0_cs[i];
    float _e13 = _group_0_binding_2_cs.scale;
    float _e17 = _group_0_binding_2_cs.offset;
    _group_0_binding_1_cs[i] = vec4(((v.xyz * _e13) + vec3(_e17)), v.w);
    return;
}

