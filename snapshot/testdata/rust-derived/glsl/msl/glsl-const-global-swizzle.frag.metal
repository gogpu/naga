// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_Target;
};
constant metal::float2 blank = metal::float2(0.0, 1.0);

void main_1(
    thread metal::float2& v_Uv_1,
    thread metal::float4& o_Target
) {
    metal::float2 col = {};
    metal::float2 _e3 = v_Uv_1;
    col = _e3.xy * blank;
    metal::float2 _e7 = col;
    o_Target = metal::float4(_e7.x, _e7.y, 0.0, 1.0);
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
) {
    metal::float2 v_Uv_1 = {};
    metal::float4 o_Target = {};
    const auto v_Uv = varyings.v_Uv;
    v_Uv_1 = v_Uv;
    main_1(v_Uv_1, o_Target);
    metal::float4 _e3 = o_Target;
    const auto _tmp = FragmentOutput {_e3};
    return main_Output { _tmp.o_Target };
}
