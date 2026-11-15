// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct CameraViewProj {
    metal::float4x4 ViewProj;
};
struct Transform {
    metal::float4x4 Model;
};
struct VertexOutput {
    metal::packed_float3 v_WorldPosition;
    metal::packed_float3 v_WorldNormal;
    metal::float2 v_Uv;
    metal::float4 v_WorldTangent;
    metal::float4 gl_Position;
};

void main_1(
    thread metal::float3& Vertex_Position_1,
    thread metal::float3& Vertex_Normal_1,
    thread metal::float2& Vertex_Uv_1,
    thread metal::float4& Vertex_Tangent_1,
    thread metal::float3& v_WorldPosition,
    thread metal::float3& v_WorldNormal,
    thread metal::float2& v_Uv,
    constant CameraViewProj& global,
    thread metal::float4& v_WorldTangent,
    constant Transform& global_1,
    thread metal::float4& gl_Position
) {
    metal::float4 world_position = {};
    metal::float4x4 _e12 = global_1.Model;
    metal::float3 _e13 = Vertex_Position_1;
    world_position = _e12 * metal::float4(_e13.x, _e13.y, _e13.z, 1.0);
    metal::float4 _e21 = world_position;
    v_WorldPosition = _e21.xyz;
    metal::float4x4 _e23 = global_1.Model;
    metal::float3 _e31 = Vertex_Normal_1;
    v_WorldNormal = metal::float3x3(_e23[0].xyz, _e23[1].xyz, _e23[2].xyz) * _e31;
    metal::float2 _e33 = Vertex_Uv_1;
    v_Uv = _e33;
    metal::float4x4 _e34 = global_1.Model;
    metal::float4 _e42 = Vertex_Tangent_1;
    metal::float3 _e44 = metal::float3x3(_e34[0].xyz, _e34[1].xyz, _e34[2].xyz) * _e42.xyz;
    metal::float4 _e45 = Vertex_Tangent_1;
    v_WorldTangent = metal::float4(_e44.x, _e44.y, _e44.z, _e45.w);
    metal::float4x4 _e52 = global.ViewProj;
    metal::float4 _e53 = world_position;
    gl_Position = _e52 * _e53;
    return;
}

struct main_Input {
    metal::float3 Vertex_Position [[attribute(0)]];
    metal::float3 Vertex_Normal [[attribute(1)]];
    metal::float2 Vertex_Uv [[attribute(2)]];
    metal::float4 Vertex_Tangent [[attribute(3)]];
};
struct main_Output {
    metal::float3 v_WorldPosition [[user(loc0), center_perspective]];
    metal::float3 v_WorldNormal [[user(loc1), center_perspective]];
    metal::float2 v_Uv [[user(loc2), center_perspective]];
    metal::float4 v_WorldTangent [[user(loc3), center_perspective]];
    metal::float4 gl_Position [[position]];
};
vertex main_Output main_(
  main_Input varyings [[stage_in]]
, constant CameraViewProj& global [[user(fake0)]]
, constant Transform& global_1 [[user(fake0)]]
) {
    metal::float3 Vertex_Position_1 = {};
    metal::float3 Vertex_Normal_1 = {};
    metal::float2 Vertex_Uv_1 = {};
    metal::float4 Vertex_Tangent_1 = {};
    metal::float3 v_WorldPosition = {};
    metal::float3 v_WorldNormal = {};
    metal::float2 v_Uv = {};
    metal::float4 v_WorldTangent = {};
    metal::float4 gl_Position = {};
    const auto Vertex_Position = varyings.Vertex_Position;
    const auto Vertex_Normal = varyings.Vertex_Normal;
    const auto Vertex_Uv = varyings.Vertex_Uv;
    const auto Vertex_Tangent = varyings.Vertex_Tangent;
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    Vertex_Tangent_1 = Vertex_Tangent;
    main_1(Vertex_Position_1, Vertex_Normal_1, Vertex_Uv_1, Vertex_Tangent_1, v_WorldPosition, v_WorldNormal, v_Uv, global, v_WorldTangent, global_1, gl_Position);
    metal::float3 _e9 = v_WorldPosition;
    metal::float3 _e11 = v_WorldNormal;
    metal::float2 _e13 = v_Uv;
    metal::float4 _e15 = v_WorldTangent;
    metal::float4 _e17 = gl_Position;
    const auto _tmp = VertexOutput {_e9, _e11, _e13, _e15, _e17};
    return main_Output { _tmp.v_WorldPosition, _tmp.v_WorldNormal, _tmp.v_Uv, _tmp.v_WorldTangent, _tmp.gl_Position };
}
