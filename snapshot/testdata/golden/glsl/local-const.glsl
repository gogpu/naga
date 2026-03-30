#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

const int gb = 4;
const uint gc = 4u;
const float gd = 4.0;


void const_in_fn() {
    return;
}

void main() {
    const_in_fn();
    return;
}

