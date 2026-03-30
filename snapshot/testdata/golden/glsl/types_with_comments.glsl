#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct TestR {
    uint test_m;
};
struct TestS {
    uint test_m;
};
const uint test_c = 1u;

shared mat2x2 w_mem2_;


void test_f() {
    return;
}

void test_g() {
    return;
}

void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        w_mem2_ = mat2x2(0.0);
    }
    memoryBarrierShared();
    barrier();
    mat2x2 phony = w_mem2_;
    test_g();
    return;
}

