#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    vec4 poly_1 = vec4(0.0);
    int k_1 = 0;
    int j_1 = 0;
    float _e9 = poly_1.x;
    int _e10 = j_1;
    int _e13 = k_1;
    poly_1.x = (_e9 + (vec3[2](vec3(0.0), vec3(0.0))[_e10].y * vec3[2](vec3(0.0), vec3(0.0))[_e13].z));
    return;
}

