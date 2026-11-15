// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct ColorMaterial_color {
    metal::float4 Color;
};
struct FragmentOutput {
    metal::float4 o_Target;
};

void main_1(
    thread metal::float4& o_Target,
    constant ColorMaterial_color& global
) {
    metal::float4 color = {};
    metal::float4 _e3 = global.Color;
    color = _e3;
    metal::float4 _e5 = color;
    o_Target = _e5;
    return;
}

struct main_Input {
    metal::float2 v_Uv [[user(loc0), center_perspective]];
};
struct main_Output {
    metal::float4 o_Target [[color(0)]];
};
fragment main_Output main_(
  main_Input varyings [[stage_in]]
, constant ColorMaterial_color& global [[user(fake0)]]
) {
    metal::float2 v_Uv_1 = {};
    metal::float4 o_Target = {};
    const auto v_Uv = varyings.v_Uv;
    v_Uv_1 = v_Uv;
    main_1(o_Target, global);
    metal::float4 _e3 = o_Target;
    const auto _tmp = FragmentOutput {_e3};
    return main_Output { _tmp.o_Target };
}
