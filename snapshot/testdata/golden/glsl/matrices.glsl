#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    mat4x4 identity = mat4x4(vec4(1.0, 0.0, 0.0, 0.0), vec4(0.0, 1.0, 0.0, 0.0), vec4(0.0, 0.0, 1.0, 0.0), vec4(0.0, 0.0, 0.0, 1.0));
    mat2x2 m2_ = mat2x2(vec2(1.0, 0.0), vec2(0.0, 1.0));
    mat3x3 m3_ = mat3x3(vec3(1.0, 0.0, 0.0), vec3(0.0, 1.0, 0.0), vec3(0.0, 0.0, 1.0));
    mat4x3 m4x3_ = mat4x3(vec3(1.0, 0.0, 0.0), vec3(0.0, 1.0, 0.0), vec3(0.0, 0.0, 1.0), vec3(0.0, 0.0, 0.0));
    vec4 v = vec4(1.0, 2.0, 3.0, 1.0);
    vec4 transformed = (identity * v);
    mat4x4 combined = (identity * identity);
    mat2x2 t2_ = transpose(m2_);
    vec4 col0_ = identity[0];
    vec4 col1_ = identity[1];
    float element = identity[2].w;
    mat2x2 scaled = (m2_ * 2.0LF);
    return;
}

