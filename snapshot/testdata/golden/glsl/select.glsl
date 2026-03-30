#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    ivec2 x0_1 = ivec2(1, 2);
    vec2 i1_1 = vec2(0.0);
    int _e12 = x0_1.x;
    int _e14 = x0_1.y;
    i1_1 = ((_e12 < _e14) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    return;
}

