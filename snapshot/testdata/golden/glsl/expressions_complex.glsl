#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    float expr1_ = ((1.0 * 2.0) + (3.0 * 4.0));
    float expr2_ = ((1.0 + 2.0) * (3.0 - 4.0));
    float expr3_ = ((1.0 / 2.0) - (3.0 / 4.0));
    float expr4_ = (((1.0 + 2.0) + 3.0) * 4.0);
    float s1_ = (true ? 2.0 : 1.0);
    vec3 s2_ = vec3(0.0);
    int s3_ = ((1.0 > 2.0) ? 20 : 10);
    vec3 v1_ = vec3(1.0, 2.0, 3.0);
    vec3 v2_ = vec3(4.0, 3.0, 2.0);
    vec3 result = ((v1_ * v2_) + vec3(1.0));
    float l = (length(v1_) * dot(v1_, v2_));
    vec3 n = normalize((v1_ + v2_));
    return;
}

