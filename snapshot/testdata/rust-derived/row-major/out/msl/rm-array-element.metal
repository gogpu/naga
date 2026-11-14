// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct type_2 {
    metal::float4x4 inner[2];
};
struct Data {
    type_2 m;
};

void main_1(
    constant Data& data,
    thread metal::float4& color
) {
    metal::float4x4 m = {};
    metal::float4x4 _e6 = data.m.inner[1];
    m = metal::transpose(_e6);
    metal::float4 _e9 = m[0];
    color = _e9;
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
