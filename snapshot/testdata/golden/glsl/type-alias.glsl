#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    vec3 a = vec3(0.0, 0.0, 0.0);
    vec3 c = vec3(0.0);
    vec3 b = vec3(vec2(0.0), 0.0);
    vec3 d = vec3(vec2(0.0), 0.0);
    ivec3 e = ivec3(d);
    mat2x2 f = mat2x2(vec2(1.0, 2.0), vec2(3.0, 4.0));
    mat3x3 g = mat3x3(a, a, a);
    return;
}

