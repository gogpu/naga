// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct Globals {
    metal::float4x4 view_matrix;
};
struct VertexPushConstants {
    metal::float4x4 world_matrix;
};
struct VertexOutput {
    metal::float4 frag_color;
    metal::float4 gl_Position;
};

void main_1(
    constant Globals& global,
    constant VertexPushConstants& global_1,
    thread metal::float2& position_1,
    thread metal::float4& color_1,
    thread metal::float4& frag_color,
    thread metal::float4& gl_Position
) {
    metal::float4 _e7 = color_1;
    frag_color = _e7;
    metal::float4x4 _e9 = global.view_matrix;
    metal::float4x4 _e10 = global_1.world_matrix;
    metal::float2 _e12 = position_1;
    gl_Position = (_e9 * _e10) * metal::float4(_e12.x, _e12.y, 0.0, 1.0);
    metal::float4 _e20 = gl_Position;
    metal::float4 _e22 = gl_Position;
    gl_Position.z = (_e20.z + _e22.w) / 2.0;
    return;
}

struct main_Input {
    metal::float2 position [[attribute(0)]];
    metal::float4 color [[attribute(1)]];
};
struct main_Output {
    metal::float4 frag_color [[user(loc0), center_perspective]];
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
, constant Globals& global [[user(fake0)]]
, constant VertexPushConstants& global_1 [[user(fake0)]]
) {
    metal::float2 position_1 = {};
    metal::float4 color_1 = {};
    metal::float4 frag_color = {};
    metal::float4 gl_Position = {};
    const auto position = varyings.position;
    const auto color = varyings.color;
    position_1 = position;
    color_1 = color;
    main_1(global, global_1, position_1, color_1, frag_color, gl_Position);
    metal::float4 _e5 = frag_color;
    metal::float4 _e7 = gl_Position;
    const auto _tmp = VertexOutput {_e5, _e7};
    return main_Output { _tmp.frag_color, _tmp.gl_Position };
}
