#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 2, local_size_y = 1, local_size_z = 1) in;

struct Struct {
    uint atomic_scalar;
    uint atomic_arr[2];
};
layout(std430) buffer type_block_0Compute { uint _group_0_binding_0_cs; };

layout(std430) buffer type_1_block_1Compute { uint _group_0_binding_1_cs[2]; };

layout(std430) buffer Struct_block_2Compute { Struct _group_0_binding_2_cs; };

layout(std140) uniform type_2_block_3Compute { uint64_t _group_0_binding_3_cs; };


void main() {
    uvec3 id = gl_LocalInvocationID;
    uint64_t _e3 = _group_0_binding_3_cs;
    atomicMax(_group_0_binding_0_cs, _e3);
    uint64_t _e7 = _group_0_binding_3_cs;
    atomicMax(_group_0_binding_1_cs[1], (1uL + _e7));
    atomicMax(_group_0_binding_2_cs.atomic_scalar, 1uL);
    atomicMax(_group_0_binding_2_cs.atomic_arr[1], uint(id.x));
    memoryBarrierShared();
    barrier();
    uint64_t _e20 = _group_0_binding_3_cs;
    atomicMin(_group_0_binding_0_cs, _e20);
    uint64_t _e24 = _group_0_binding_3_cs;
    atomicMin(_group_0_binding_1_cs[1], (1uL + _e24));
    atomicMin(_group_0_binding_2_cs.atomic_scalar, 1uL);
    atomicMin(_group_0_binding_2_cs.atomic_arr[1], uint(id.x));
    return;
}

