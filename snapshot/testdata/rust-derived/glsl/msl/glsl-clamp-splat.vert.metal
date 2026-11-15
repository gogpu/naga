// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct VertexOutput {
    metal::float4 gl_Position;
};

void main_1(
    thread metal::float2& a_pos_1,
    thread metal::float4& gl_Position
) {
    metal::float2 _e2 = a_pos_1;
    metal::float2 _e7 = metal::clamp(_e2, metal::float2(0.0), metal::float2(1.0));
    gl_Position = metal::float4(_e7.x, _e7.y, 0.0, 1.0);
    return;
}

struct main_Input {
    metal::float2 a_pos [[attribute(0)]];
};
struct main_Output {
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
) {
    metal::float2 a_pos_1 = {};
    metal::float4 gl_Position = {};
    const auto a_pos = varyings.a_pos;
    a_pos_1 = a_pos;
    main_1(a_pos_1, gl_Position);
    metal::float4 _e3 = gl_Position;
    const auto _tmp = VertexOutput {_e3};
    return main_Output { _tmp.gl_Position };
}
