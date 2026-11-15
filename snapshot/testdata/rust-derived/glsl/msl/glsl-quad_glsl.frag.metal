// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_color;
};

void main_1(
    thread metal::float4& o_color
) {
    o_color = metal::float4(1.0, 1.0, 1.0, 1.0);
    return;
}

struct main_Input {
    metal::float2 v_uv [[user(loc0), center_perspective]];
};
struct main_Output {
    metal::float4 o_color [[color(0)]];
};
fragment main_Output main_(
  main_Input varyings [[stage_in]]
) {
    metal::float2 v_uv_1 = {};
    metal::float4 o_color = {};
    const auto v_uv = varyings.v_uv;
    v_uv_1 = v_uv;
    main_1(o_color);
    metal::float4 _e3 = o_color;
    const auto _tmp = FragmentOutput {_e3};
    return main_Output { _tmp.o_color };
}
