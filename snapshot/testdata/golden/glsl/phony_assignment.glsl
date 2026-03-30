#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std140) uniform type_block_0Compute { float _group_0_binding_0_cs; };


int five() {
    return 5;
}

void main() {
    uvec3 id = gl_GlobalInvocationID;
    float phony = _group_0_binding_0_cs;
    float phony_1 = _group_0_binding_0_cs;
    int _e6 = five();
    int _e7 = five();
    float phony_2 = _group_0_binding_0_cs;
    return;
}

