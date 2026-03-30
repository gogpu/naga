#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

shared uint sized_comma[1];

shared uint sized_no_comma[1];

layout(std430) buffer type_2_block_0Compute { uint _group_0_binding_0_cs[]; };

layout(std430) buffer type_2_block_1Compute { uint _group_0_binding_1_cs[]; };


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        sized_comma = uint[1](0u);
        sized_no_comma = uint[1](0u);
    }
    memoryBarrierShared();
    barrier();
    uint _e4 = _group_0_binding_0_cs[0];
    sized_comma[0] = _e4;
    uint _e9 = _group_0_binding_1_cs[0];
    sized_no_comma[0] = _e9;
    uint _e14 = sized_comma[0];
    uint _e17 = sized_no_comma[0];
    _group_0_binding_1_cs[0] = (_e14 + _e17);
    return;
}

