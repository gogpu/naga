#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct S {
    int a;
    mat2x2 b;
};
struct S_std140_ {
    int a;
    vec2 b_0;
    vec2 b_1;
};
S S_from_std140_(S_std140_ v) {
    return S(v.a, mat2x2(v.b_0, v.b_1));
}

layout(std140) uniform S_block_0Compute { S_std140_ _group_0_binding_0_cs; };


void main() {
    S v = S_from_std140_(_group_0_binding_0_cs);
    return;
}

