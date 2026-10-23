#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    int mod_s = (5 - 2 * (5 / 2));
    ivec2 _e3 = ivec2(5);
    ivec2 _e4 = ivec2(2);
    ivec2 mod_s_vec = (_e3 - _e4 * (_e3 / _e4));
    uint mod_u = (5u - 2u * (5u / 2u));
    return;
}

