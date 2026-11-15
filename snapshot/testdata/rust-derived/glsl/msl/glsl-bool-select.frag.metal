// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_color;
};

float TevPerCompGT(
    float a,
    float b
) {
    float a_1 = {};
    float b_1 = {};
    a_1 = a;
    b_1 = b;
    float _e4 = a_1;
    float _e5 = b_1;
    return (_e4 > _e5) ? 1.0 : 0.0;
}

metal::float3 TevPerCompGT_1(
    metal::float3 a_2,
    metal::float3 b_2
) {
    metal::float3 a_3 = {};
    metal::float3 b_3 = {};
    a_3 = a_2;
    b_3 = b_2;
    metal::float3 _e4 = a_3;
    metal::float3 _e5 = b_3;
    return metal::select(metal::float3(0.0), metal::float3(1.0), _e4 > _e5);
}

void main_1(
    thread metal::float4& o_color
) {
    metal::float3 _e5 = TevPerCompGT_1(metal::float3(3.0), metal::float3(5.0));
    o_color.x = _e5.x;
    o_color.y = _e5.y;
    o_color.z = _e5.z;
    float _e15 = TevPerCompGT(3.0, 5.0);
    o_color.w = _e15;
    return;
}

struct main_Output {
    metal::float4 o_color [[color(0)]];
};
fragment main_Output main_(
) {
    metal::float4 o_color = {};
    main_1(o_color);
    metal::float4 _e1 = o_color;
    const auto _tmp = FragmentOutput {_e1};
    return main_Output { _tmp.o_color };
}
