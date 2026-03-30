#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    vec4 v4_ = vec4(1.0, 2.0, 3.0, 4.0);
    float x = v4_.x;
    float y = v4_.y;
    float z = v4_.z;
    float w = v4_.w;
    vec2 xy = v4_.xy;
    vec3 xyz = v4_.xyz;
    vec4 xyzw = v4_.xyzw;
    vec2 zw = v4_.zw;
    vec2 xx = v4_.xx;
    vec3 yyy = v4_.yyy;
    vec4 wzyx = v4_.wzyx;
    vec2 yx = v4_.yx;
    vec3 zxy = v4_.zxy;
    vec2 v2_ = vec2(1.0, 2.0);
    vec2 v2_yx = v2_.yx;
    vec2 v2_xx = v2_.xx;
    vec3 v3_ = vec3(1.0, 2.0, 3.0);
    vec2 v3_zx = v3_.zx;
    vec3 v3_yxz = v3_.yxz;
    return;
}

