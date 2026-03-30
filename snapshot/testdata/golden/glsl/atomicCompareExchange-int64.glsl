// === Entry Point: test_atomic_compare_exchange_i64 (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct _atomic_compare_exchange_result_Sint_8_ {
    int64_t old_value;
    bool exchanged;
};
struct _atomic_compare_exchange_result_Uint_8_ {
    uint64_t old_value;
    bool exchanged;
};
const uint SIZE = 128u;

layout(std430) buffer type_2_block_0Compute { int _group_0_binding_0_cs[128]; };


void main() {
    uint i = 0u;
    int64_t old = 0;
    bool exchanged = false;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e26 = i;
            i = (_e26 + 1u);
        }
        loop_init = false;
        uint _e2 = i;
        if ((_e2 < SIZE)) {
        } else {
            break;
        }
        {
            uint _e6 = i;
            int64_t _e8 = _group_0_binding_0_cs[_e6];
            old = _e8;
            exchanged = false;
            while(true) {
                bool _e12 = exchanged;
                if (!(_e12)) {
                } else {
                    break;
                }
                {
                    int64_t _e14 = old;
                    int64_t new = int((_e14 + 10L));
                    uint _e19 = i;
                    int64_t _e21 = old;
                    _atomic_compare_exchange_result_Sint_8_ _e22; _e22.old_value = atomicCompSwap(_group_0_binding_0_cs[_e19], _e21, new);
                    _e22.exchanged = (_e22.old_value == _e21);
                    old = _e22.old_value;
                    exchanged = _e22.exchanged;
                }
            }
        }
    }
    return;
}


// === Entry Point: test_atomic_compare_exchange_u64 (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct _atomic_compare_exchange_result_Sint_8_ {
    int64_t old_value;
    bool exchanged;
};
struct _atomic_compare_exchange_result_Uint_8_ {
    uint64_t old_value;
    bool exchanged;
};
const uint SIZE = 128u;

layout(std430) buffer type_4_block_0Compute { uint _group_0_binding_1_cs[128]; };


void main() {
    uint i_1 = 0u;
    uint64_t old_1 = 0u;
    bool exchanged_1 = false;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e26 = i_1;
            i_1 = (_e26 + 1u);
        }
        loop_init = false;
        uint _e2 = i_1;
        if ((_e2 < SIZE)) {
        } else {
            break;
        }
        {
            uint _e6 = i_1;
            uint64_t _e8 = _group_0_binding_1_cs[_e6];
            old_1 = _e8;
            exchanged_1 = false;
            while(true) {
                bool _e12 = exchanged_1;
                if (!(_e12)) {
                } else {
                    break;
                }
                {
                    uint64_t _e14 = old_1;
                    uint64_t new = uint((_e14 + 10uL));
                    uint _e19 = i_1;
                    uint64_t _e21 = old_1;
                    _atomic_compare_exchange_result_Uint_8_ _e22; _e22.old_value = atomicCompSwap(_group_0_binding_1_cs[_e19], _e21, new);
                    _e22.exchanged = (_e22.old_value == _e21);
                    old_1 = _e22.old_value;
                    exchanged_1 = _e22.exchanged;
                }
            }
        }
    }
    return;
}

