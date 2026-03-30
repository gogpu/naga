#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    vec3 v3_ = vec3(1.0, 2.0, 3.0);
    vec3 v3b = vec3(4.0, 5.0, 6.0);
    vec3 n = normalize(v3_);
    float l = length(v3_);
    float d = dot(v3_, v3b);
    vec3 c = cross(v3_, v3b);
    float cl = clamp(0.5, 0.0, 1.0);
    vec3 m = mix(v3_, v3b, 0.5);
    float s = smoothstep(0.0, 1.0, 0.5);
    return;
}

