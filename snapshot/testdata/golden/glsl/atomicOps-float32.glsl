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


void main() {
    uvec3 id = gl_LocalInvocationID;
    _group_0_binding_0_cs = 1.5;
    _group_0_binding_1_cs[1] = 1.5;
    _group_0_binding_2_cs.atomic_scalar = 1.5;
    _group_0_binding_2_cs.atomic_arr[1] = 1.5;
    memoryBarrierShared();
    barrier();
    float l0_ = _group_0_binding_0_cs;
    float l1_ = _group_0_binding_1_cs[1];
    float l2_ = _group_0_binding_2_cs.atomic_scalar;
    float l3_ = _group_0_binding_2_cs.atomic_arr[1];
    memoryBarrierShared();
    barrier();
    float _e27 = atomicAdd(_group_0_binding_0_cs, 1.5);
    float _e31 = atomicAdd(_group_0_binding_1_cs[1], 1.5);
    float _e35 = atomicAdd(_group_0_binding_2_cs.atomic_scalar, 1.5);
    float _e40 = atomicAdd(_group_0_binding_2_cs.atomic_arr[1], 1.5);
    memoryBarrierShared();
    barrier();
    float _e43 = atomicExchange(_group_0_binding_0_cs, 1.5);
    float _e47 = atomicExchange(_group_0_binding_1_cs[1], 1.5);
    float _e51 = atomicExchange(_group_0_binding_2_cs.atomic_scalar, 1.5);
    float _e56 = atomicExchange(_group_0_binding_2_cs.atomic_arr[1], 1.5);
    return;
}

