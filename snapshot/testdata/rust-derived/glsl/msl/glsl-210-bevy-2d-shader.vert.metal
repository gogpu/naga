// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct Camera {
    metal::float4x4 ViewProj;
};
struct Transform {
    metal::float4x4 Model;
};
struct Sprite_size {
    metal::float2 size;
};
struct VertexOutput {
    metal::float2 v_Uv;
    metal::float4 gl_Position;
};

void main_1(
    thread metal::float3& Vertex_Position_1,
    thread metal::float2& Vertex_Uv_1,
    thread metal::float2& v_Uv,
    constant Camera& global,
    constant Transform& global_1,
    constant Sprite_size& global_2,
    thread metal::float4& gl_Position
) {
    metal::float3 position = {};
    metal::float2 _e9 = Vertex_Uv_1;
    v_Uv = _e9;
    metal::float3 _e10 = Vertex_Position_1;
    metal::float2 _e11 = global_2.size;
    position = _e10 * metal::float3(_e11.x, _e11.y, 1.0);
    metal::float4x4 _e19 = global.ViewProj;
    metal::float4x4 _e20 = global_1.Model;
    metal::float3 _e22 = position;
    gl_Position = (_e19 * _e20) * metal::float4(_e22.x, _e22.y, _e22.z, 1.0);
    return;
}

struct main_Input {
    metal::float3 Vertex_Position [[attribute(0)]];
    metal::float3 Vertex_Normal [[attribute(1)]];
    metal::float2 Vertex_Uv [[attribute(2)]];
};
struct main_Output {
    metal::float2 v_Uv [[user(loc0), center_perspective]];
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
, constant Camera& global [[user(fake0)]]
, constant Transform& global_1 [[user(fake0)]]
, constant Sprite_size& global_2 [[user(fake0)]]
) {
    metal::float3 Vertex_Position_1 = {};
    metal::float3 Vertex_Normal_1 = {};
    metal::float2 Vertex_Uv_1 = {};
    metal::float2 v_Uv = {};
    metal::float4 gl_Position = {};
    const auto Vertex_Position = varyings.Vertex_Position;
    const auto Vertex_Normal = varyings.Vertex_Normal;
    const auto Vertex_Uv = varyings.Vertex_Uv;
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    main_1(Vertex_Position_1, Vertex_Uv_1, v_Uv, global, global_1, global_2, gl_Position);
    metal::float2 _e7 = v_Uv;
    metal::float4 _e9 = gl_Position;
    const auto _tmp = VertexOutput {_e7, _e9};
    return main_Output { _tmp.v_Uv, _tmp.gl_Position };
}
