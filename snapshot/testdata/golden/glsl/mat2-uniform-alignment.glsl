#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct S {
    int a;
    mat2x2 b;
};
layout(std140) uniform S_block_0Compute { S _group_0_binding_0_cs; };


void main() {
    S v = _group_0_binding_0_cs;
    return;
}

