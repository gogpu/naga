// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct Data {
    metal::float4x4 m;
};

void main_1(
    constant Data& data,
    thread metal::float4& color
) {
    metal::float4x4 m = {};
    metal::float4x4 _e4 = data.m;
    m = metal::transpose(_e4);
    metal::float4 _e7 = m[1];
    color = _e7;
    return;
}

struct main_Output {
    metal::float4 member [[color(0)]];
};
fragment main_Output main_(
  constant Data& data [[user(fake0)]]
) {
    metal::float4 color = {};
    main_1(data, color);
    metal::float4 _e1 = color;
    return main_Output { _e1 };
}
