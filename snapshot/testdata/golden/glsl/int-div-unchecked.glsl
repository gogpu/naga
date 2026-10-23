#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    int div_s = (5 / 2);
    int mod_s = (5 - 2 * (5 / 2));
    uint div_u = (5u / 2u);
    uint mod_u = (5u - 2u * (5u / 2u));
    return;
}

