// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct A {
    half a_1_;
    char _pad1[2];
    metal::half2 a_vec2_;
    metal::half3 a_vec3_;
    metal::half4 a_vec4_;
};
struct B {
    half b_1_;
    char _pad1[2];
    metal::half2 b_vec2_;
    metal::half3 b_vec3_;
    metal::half4 b_vec4_;
    metal::half2x2 b_mat2_;
    metal::half2x3 b_mat2x3_;
    metal::half2x4 b_mat2x4_;
    metal::half3x2 b_mat3x2_;
    char _pad8[4];
    metal::half3x3 b_mat3_;
    metal::half3x4 b_mat3x4_;
    metal::half4x2 b_mat4x2_;
    metal::half4x3 b_mat4x3_;
    metal::half4x4 b_mat4_;
};

void main_1(
    constant A& global,
    device B& global_1
) {
    half _e16 = global.a_1_;
    global_1.b_1_ = _e16;
    metal::half2 _e17 = global.a_vec2_;
    global_1.b_vec2_ = _e17;
    metal::half3 _e18 = global.a_vec3_;
    global_1.b_vec3_ = _e18;
    metal::half4 _e19 = global.a_vec4_;
    global_1.b_vec4_ = _e19;
    return;
}

kernel void main_(
  constant A& global [[user(fake0)]]
, device B& global_1 [[user(fake0)]]
) {
    main_1(global, global_1);
    return;
}
