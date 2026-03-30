#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer Globals_block_0Compute {
    uint a;
    uint b[10];
    uint c[];
} _group_0_binding_0_cs;


uint fetch_add_atomic() {
    uint _e3 = atomicAdd(_group_0_binding_0_cs.a, 1u);
    return _e3;
}

uint fetch_add_atomic_static_sized_array(int i) {
    uint _e5 = atomicAdd(_group_0_binding_0_cs.b[i], 1u);
    return _e5;
}

uint fetch_add_atomic_dynamic_sized_array(int i_1) {
    uint _e5 = atomicAdd(_group_0_binding_0_cs.c[i_1], 1u);
    return _e5;
}

uint exchange_atomic() {
    uint _e3 = atomicExchange(_group_0_binding_0_cs.a, 1u);
    return _e3;
}

uint exchange_atomic_static_sized_array(int i_2) {
    uint _e5 = atomicExchange(_group_0_binding_0_cs.b[i_2], 1u);
    return _e5;
}

uint exchange_atomic_dynamic_sized_array(int i_3) {
    uint _e5 = atomicExchange(_group_0_binding_0_cs.c[i_3], 1u);
    return _e5;
}

uint fetch_add_atomic_dynamic_sized_array_static_index() {
    uint _e4 = atomicAdd(_group_0_binding_0_cs.c[1000], 1u);
    return _e4;
}

uint exchange_atomic_dynamic_sized_array_static_index() {
    uint _e4 = atomicExchange(_group_0_binding_0_cs.c[1000], 1u);
    return _e4;
}

void main() {
    uint _e0 = fetch_add_atomic();
    uint _e2 = fetch_add_atomic_static_sized_array(1);
    uint _e4 = fetch_add_atomic_dynamic_sized_array(1);
    uint _e5 = exchange_atomic();
    uint _e7 = exchange_atomic_static_sized_array(1);
    uint _e9 = exchange_atomic_dynamic_sized_array(1);
    uint _e10 = fetch_add_atomic_dynamic_sized_array_static_index();
    uint _e11 = exchange_atomic_dynamic_sized_array_static_index();
    return;
}

