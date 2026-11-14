// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct type_4 {
    metal::float4x4 inner[2];
};
struct Data {
    type_4 m;
};

float first_u0028_mf44_u005b_2_u005d_u003b(
    thread type_4& m
) {
    float _e6 = m.inner[1][0][0u];
    return _e6;
}

void main_1(
    thread metal::float4& color,
    constant Data& data
) {
    type_4 param = {};
    type_4 _e6 = data.m;
    param.inner[0] = _e6.inner[0];
    param.inner[1] = _e6.inner[1];
    float _e11 = first_u0028_mf44_u005b_2_u005d_u003b(param);
    color = metal::float4(_e11);
    return;
}

struct main_Output {
    metal::float4 member [[color(0)]];
};
fragment main_Output main_(
  constant Data& data [[user(fake0)]]
) {
    metal::float4 color = {};
    main_1(color, data);
    metal::float4 _e1 = color;
    return main_Output { _e1 };
}
