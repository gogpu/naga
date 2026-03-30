#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    vec4 poly_1 = vec4(0.0);
    int k_1 = 0;
    int j_1 = 0;
    int _e9 = j_1;
    int _e12 = k_1;
    float _e16 = poly_1.x;
    poly_1.x = (_e16 + (vec3[2](vec3(0.0), vec3(0.0))[_e9].y * vec3[2](vec3(0.0), vec3(0.0))[_e12].z));
    return;
}

