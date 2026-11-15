// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct VertexOutput {
    metal::float2 v_uv;
    metal::float4 gl_Position;
};
constant float c_scale = 1.2;

void main_1(
    thread metal::float2& a_pos_1,
    thread metal::float2& a_uv_1,
    thread metal::float2& v_uv,
    thread metal::float4& gl_Position
) {
    metal::float2 _e4 = a_uv_1;
    v_uv = _e4;
    metal::float2 _e6 = a_pos_1;
    metal::float2 _e7 = c_scale * _e6;
    gl_Position = metal::float4(_e7.x, _e7.y, 0.0, 1.0);
    return;
}

struct main_Input {
    metal::float2 a_pos [[attribute(0)]];
    metal::float2 a_uv [[attribute(1)]];
};
struct main_Output {
    metal::float2 v_uv [[user(loc0), center_perspective]];
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
) {
    metal::float2 a_pos_1 = {};
    metal::float2 a_uv_1 = {};
    metal::float2 v_uv = {};
    metal::float4 gl_Position = {};
    const auto a_pos = varyings.a_pos;
    const auto a_uv = varyings.a_uv;
    a_pos_1 = a_pos;
    a_uv_1 = a_uv;
    main_1(a_pos_1, a_uv_1, v_uv, gl_Position);
    metal::float2 _e5 = v_uv;
    metal::float4 _e7 = gl_Position;
    const auto _tmp = VertexOutput {_e5, _e7};
    return main_Output { _tmp.v_uv, _tmp.gl_Position };
}
