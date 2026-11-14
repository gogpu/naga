// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct Data {
    metal::float4x4 m;
};

void main_1(
    thread metal::float4& color,
    constant Data& data
) {
    metal::float4x4 _e3 = data.m;
    color = metal::transpose(_e3)[3];
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
