#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;


void main() {
    uvec3 id = gl_GlobalInvocationID;
    uint x_1 = 0u;
    x_1 = (id.x * 2u);
    return;
}

