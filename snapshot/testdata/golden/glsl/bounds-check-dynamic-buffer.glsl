#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct T {
    uint t;
};
layout(std430) buffer type_block_0Compute { uint _group_0_binding_0_cs; };

layout(std430) buffer type_1_block_1Compute { uint _group_0_binding_1_cs[]; };

layout(std140) uniform type_2_block_2Compute { T _group_0_binding_2_cs[1]; };

layout(std430) buffer type_2_block_3Compute { T _group_0_binding_3_cs[1]; };

layout(std430) buffer type_2_block_4Compute { T _group_0_binding_4_cs[1]; };

layout(std430) buffer type_2_block_5Compute { T _group_1_binding_0_cs[1]; };


void main() {
    uint i = _group_0_binding_0_cs;
    uint _e7 = _group_0_binding_2_cs[i].t;
    _group_0_binding_1_cs[0] = _e7;
    uint _e13 = _group_0_binding_3_cs[i].t;
    _group_0_binding_1_cs[1] = _e13;
    uint _e19 = _group_0_binding_4_cs[i].t;
    _group_0_binding_1_cs[2] = _e19;
    uint _e25 = _group_1_binding_0_cs[i].t;
    _group_0_binding_1_cs[3] = _e25;
    return;
}

