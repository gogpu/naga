#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct _atomic_compare_exchange_result_Sint_4_ {
    int old_value;
    bool exchanged;
};
struct _atomic_compare_exchange_result_Uint_4_ {
    uint old_value;
    bool exchanged;
};
layout(std430) buffer type_block_0Compute { int _group_0_binding_0_cs; };

layout(std430) buffer type_1_block_1Compute { uint _group_0_binding_1_cs; };


void test_atomic_i32_() {
    _group_0_binding_0_cs = 1;
    _atomic_compare_exchange_result_Sint_4_ _e5; _e5.old_value = atomicCompSwap(_group_0_binding_0_cs, 1, 1);
    _e5.exchanged = (_e5.old_value == 1);
    _atomic_compare_exchange_result_Sint_4_ _e9; _e9.old_value = atomicCompSwap(_group_0_binding_0_cs, 1, 1);
    _e9.exchanged = (_e9.old_value == 1);
    int _e12 = atomicAdd(_group_0_binding_0_cs, 1);
    int _e15 = atomicAdd(_group_0_binding_0_cs, -1);
    int _e18 = atomicAnd(_group_0_binding_0_cs, 1);
    int _e21 = atomicXor(_group_0_binding_0_cs, 1);
    int _e24 = atomicOr(_group_0_binding_0_cs, 1);
    int _e27 = atomicMin(_group_0_binding_0_cs, 1);
    int _e30 = atomicMax(_group_0_binding_0_cs, 1);
    int _e33 = atomicExchange(_group_0_binding_0_cs, 1);
    return;
}

void test_atomic_u32_() {
    _group_0_binding_1_cs = 1u;
    _atomic_compare_exchange_result_Uint_4_ _e5; _e5.old_value = atomicCompSwap(_group_0_binding_1_cs, 1u, 1u);
    _e5.exchanged = (_e5.old_value == 1u);
    _atomic_compare_exchange_result_Uint_4_ _e9; _e9.old_value = atomicCompSwap(_group_0_binding_1_cs, 1u, 1u);
    _e9.exchanged = (_e9.old_value == 1u);
    uint _e12 = atomicAdd(_group_0_binding_1_cs, 1u);
    uint _e15 = atomicAdd(_group_0_binding_1_cs, -1u);
    uint _e18 = atomicAnd(_group_0_binding_1_cs, 1u);
    uint _e21 = atomicXor(_group_0_binding_1_cs, 1u);
    uint _e24 = atomicOr(_group_0_binding_1_cs, 1u);
    uint _e27 = atomicMin(_group_0_binding_1_cs, 1u);
    uint _e30 = atomicMax(_group_0_binding_1_cs, 1u);
    uint _e33 = atomicExchange(_group_0_binding_1_cs, 1u);
    return;
}

void main() {
    test_atomic_i32_();
    test_atomic_u32_();
    return;
}

