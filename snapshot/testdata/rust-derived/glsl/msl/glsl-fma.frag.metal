// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct Mat4x3_ {
    metal::float4 mx;
    metal::float4 my;
    metal::float4 mz;
};
struct FragmentOutput {
    metal::float4 o_color;
};

void Fma(
    thread Mat4x3_& d,
    Mat4x3_ m,
    float s
) {
    Mat4x3_ m_1 = {};
    float s_1 = {};
    m_1 = m;
    s_1 = s;
    Mat4x3_ _e6 = d;
    Mat4x3_ _e8 = m_1;
    float _e10 = s_1;
    d.mx = _e6.mx + (_e8.mx * _e10);
    Mat4x3_ _e14 = d;
    Mat4x3_ _e16 = m_1;
    float _e18 = s_1;
    d.my = _e14.my + (_e16.my * _e18);
    Mat4x3_ _e22 = d;
    Mat4x3_ _e24 = m_1;
    float _e26 = s_1;
    d.mz = _e22.mz + (_e24.mz * _e26);
    return;
}

void main_1(
    thread metal::float4& o_color
) {
    Mat4x3_ m1_ = Mat4x3_ {metal::float4(0.0), metal::float4(1.0), metal::float4(2.0)};
    Mat4x3_ m2_ = Mat4x3_ {metal::float4(0.0), metal::float4(1.0), metal::float4(2.0)};
    Mat4x3_ _e17 = m2_;
    Fma(m1_, _e17, 2.0);
    o_color.x = 1.0;
    o_color.y = 1.0;
    o_color.z = 1.0;
    o_color.w = 1.0;
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
