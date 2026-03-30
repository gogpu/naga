#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void thing() {
    return;
}

void with_diagnostic() {
    return;
}

void main() {
    thing();
    with_diagnostic();
    return;
}

