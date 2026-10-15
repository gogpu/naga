#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct _atomic_compare_exchange_result_Uint_4_ {
    uint old_value;
    bool exchanged;
};
const int o = 2;

shared uint a;


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        a = 0u;
    }
    memoryBarrierShared();
    barrier();
    _atomic_compare_exchange_result_Uint_4_ _e5; _e5.old_value = atomicCompSwap(a, uint(o), 1u);
    _e5.exchanged = (_e5.old_value == uint(o));
    return;
}

