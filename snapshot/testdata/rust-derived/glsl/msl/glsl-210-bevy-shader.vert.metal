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
struct VertexOutput {
    metal::packed_float3 v_Position;
    metal::packed_float3 v_Normal;
    metal::float2 v_Uv;
    metal::float4 gl_Position;
};

void main_1(
    thread metal::float3& Vertex_Position_1,
    thread metal::float3& Vertex_Normal_1,
    thread metal::float2& Vertex_Uv_1,
    thread metal::float3& v_Position,
    thread metal::float3& v_Normal,
    thread metal::float2& v_Uv,
    constant Camera& global,
    constant Transform& global_1,
    thread metal::float4& gl_Position
) {
    metal::float4x4 _e10 = global_1.Model;
    metal::float3 _e11 = Vertex_Normal_1;
    v_Normal = (_e10 * metal::float4(_e11.x, _e11.y, _e11.z, 1.0)).xyz;
    metal::float4x4 _e19 = global_1.Model;
    metal::float3 _e27 = Vertex_Normal_1;
    v_Normal = metal::float3x3(_e19[0].xyz, _e19[1].xyz, _e19[2].xyz) * _e27;
    metal::float4x4 _e29 = global_1.Model;
    metal::float3 _e30 = Vertex_Position_1;
    v_Position = (_e29 * metal::float4(_e30.x, _e30.y, _e30.z, 1.0)).xyz;
    metal::float2 _e38 = Vertex_Uv_1;
    v_Uv = _e38;
    metal::float4x4 _e40 = global.ViewProj;
    metal::float3 _e41 = v_Position;
    gl_Position = _e40 * metal::float4(_e41.x, _e41.y, _e41.z, 1.0);
    return;
}

struct main_Input {
    metal::float3 Vertex_Position [[attribute(0)]];
    metal::float3 Vertex_Normal [[attribute(1)]];
    metal::float2 Vertex_Uv [[attribute(2)]];
};
struct main_Output {
    metal::float3 v_Position [[user(loc0), center_perspective]];
    metal::float3 v_Normal [[user(loc1), center_perspective]];
    metal::float2 v_Uv [[user(loc2), center_perspective]];
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
, constant Camera& global [[user(fake0)]]
, constant Transform& global_1 [[user(fake0)]]
) {
    metal::float3 Vertex_Position_1 = {};
    metal::float3 Vertex_Normal_1 = {};
    metal::float2 Vertex_Uv_1 = {};
    metal::float3 v_Position = {};
    metal::float3 v_Normal = {};
    metal::float2 v_Uv = {};
    metal::float4 gl_Position = {};
    const auto Vertex_Position = varyings.Vertex_Position;
    const auto Vertex_Normal = varyings.Vertex_Normal;
    const auto Vertex_Uv = varyings.Vertex_Uv;
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    main_1(Vertex_Position_1, Vertex_Normal_1, Vertex_Uv_1, v_Position, v_Normal, v_Uv, global, global_1, gl_Position);
    metal::float3 _e7 = v_Position;
    metal::float3 _e9 = v_Normal;
    metal::float2 _e11 = v_Uv;
    metal::float4 _e13 = gl_Position;
    const auto _tmp = VertexOutput {_e7, _e9, _e11, _e13};
    return main_Output { _tmp.v_Position, _tmp.v_Normal, _tmp.v_Uv, _tmp.gl_Position };
}
