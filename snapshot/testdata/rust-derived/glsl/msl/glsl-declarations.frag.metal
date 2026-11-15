// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct VertexData {
    metal::float2 position;
    metal::float2 a;
};
struct FragmentData {
    metal::float2 position;
    metal::float2 a;
};
struct type_2 {
    metal::float4 inner[2];
};
struct TestStruct {
    float a;
    float b;
};
struct type_4 {
    float inner[2];
};
struct type_5 {
    type_4 inner[2];
};
struct type_6 {
    type_5 inner[2];
};
struct type_7 {
    type_6 inner[2];
};
struct type_8 {
    type_7 inner[2];
};
struct type_9 {
    type_8 inner[2];
};
struct type_10 {
    type_9 inner[2];
};
struct type_11 {
    float inner[3];
};
struct type_12 {
    float inner[4];
};
struct LightScatteringParams {
    float BetaRay;
    char _pad1[12];
    type_11 BetaMie;
    float HGg;
    char _pad3[12];
    type_12 DistanceMul;
    float BlendCoeff;
    char _pad5[12];
    metal::float3 SunDirection;
    metal::float3 SunColor;
};
struct type_14 {
    metal::float3 inner[2];
};
struct FragmentOutput {
    metal::float2 position;
    metal::float2 a;
    metal::float4 out_array;
    metal::float4 out_array_1;
};

void main_1(
    thread type_2& in_array_2,
    thread type_2& out_array,
    thread type_5& array_2d,
    thread type_10& array_toomanyd
) {
    type_14 positions = type_14 {{metal::float3(-1.0, 1.0, 0.0), metal::float3(-1.0, -1.0, 0.0)}};
    TestStruct strct = TestStruct {1.0, 2.0};
    metal::float4 from_input_array = {};
    float a_1 = {};
    float b = {};
    LightScatteringParams light_scattering_params = {};
    metal::float4 _e17 = in_array_2.inner[1];
    from_input_array = _e17;
    float _e21 = array_2d.inner[0].inner[0];
    a_1 = _e21;
    float _e30 = array_toomanyd.inner[0].inner[0].inner[0].inner[0].inner[0].inner[0].inner[0];
    b = _e30;
    out_array.inner[0] = metal::float4(2.0);
    return;
}

struct main_Input {
    metal::float2 position [[user(loc0), center_perspective]];
    metal::float2 a [[user(loc1), center_perspective]];
    metal::float4 in_array [[user(loc2), center_perspective]];
    metal::float4 in_array_1 [[user(loc3), center_perspective]];
};
struct main_Output {
    metal::float2 position [[color(0)]];
    metal::float2 a [[color(1)]];
    metal::float4 out_array [[color(2)]];
    metal::float4 out_array_1 [[color(3)]];
};
fragment main_Output main_(
  main_Input varyings [[stage_in]]
) {
    VertexData vert = {};
    FragmentData frag = {};
    type_2 in_array_2 = {};
    type_2 out_array = {};
    type_5 array_2d = {};
    type_10 array_toomanyd = {};
    const auto position = varyings.position;
    const auto a = varyings.a;
    const auto in_array = varyings.in_array;
    const auto in_array_1 = varyings.in_array_1;
    vert.position = position;
    vert.a = a;
    in_array_2.inner[0] = in_array;
    in_array_2.inner[1] = in_array_1;
    main_1(in_array_2, out_array, array_2d, array_toomanyd);
    metal::float2 _e12 = frag.position;
    metal::float2 _e14 = frag.a;
    metal::float4 _e17 = out_array.inner[0];
    metal::float4 _e19 = out_array.inner[1];
    const auto _tmp = FragmentOutput {_e12, _e14, _e17, _e19};
    return main_Output { _tmp.position, _tmp.a, _tmp.out_array, _tmp.out_array_1 };
}
