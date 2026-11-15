struct CameraViewProj {
    row_major float4x4 ViewProj;
};

struct Transform {
    row_major float4x4 Model;
};

struct VertexOutput {
    float3 v_WorldPosition : LOC0;
    float3 v_WorldNormal : LOC1;
    float2 v_Uv : LOC2;
    float4 v_WorldTangent : LOC3;
    float4 gl_Position : SV_Position;
};

static float3 Vertex_Position_1 = (float3)0;
static float3 Vertex_Normal_1 = (float3)0;
static float2 Vertex_Uv_1 = (float2)0;
static float4 Vertex_Tangent_1 = (float4)0;
static float3 v_WorldPosition = (float3)0;
static float3 v_WorldNormal = (float3)0;
static float2 v_Uv = (float2)0;
cbuffer global : register(b0) { CameraViewProj global; }
static float4 v_WorldTangent = (float4)0;
cbuffer global_1 : register(b0, space2) { Transform global_1; }
static float4 gl_Position = (float4)0;

struct VertexOutput_main {
    float3 v_WorldPosition_1 : LOC0;
    float3 v_WorldNormal_1 : LOC1;
    float2 v_Uv_1 : LOC2;
    float4 v_WorldTangent_1 : LOC3;
    float4 gl_Position_1 : SV_Position;
};

void main_1()
{
    float4 world_position = (float4)0;

    float4x4 _e12 = global_1.Model;
    float3 _e13 = Vertex_Position_1;
    world_position = mul(float4(_e13.x, _e13.y, _e13.z, 1.0), _e12);
    float4 _e21 = world_position;
    v_WorldPosition = _e21.xyz;
    float4x4 _e23 = global_1.Model;
    float3 _e31 = Vertex_Normal_1;
    v_WorldNormal = mul(_e31, float3x3(_e23[0].xyz, _e23[1].xyz, _e23[2].xyz));
    float2 _e33 = Vertex_Uv_1;
    v_Uv = _e33;
    float4x4 _e34 = global_1.Model;
    float4 _e42 = Vertex_Tangent_1;
    float3 _e44 = mul(_e42.xyz, float3x3(_e34[0].xyz, _e34[1].xyz, _e34[2].xyz));
    float4 _e45 = Vertex_Tangent_1;
    v_WorldTangent = float4(_e44.x, _e44.y, _e44.z, _e45.w);
    float4x4 _e52 = global.ViewProj;
    float4 _e53 = world_position;
    gl_Position = mul(_e53, _e52);
    return;
}

VertexOutput ConstructVertexOutput(float3 arg0, float3 arg1, float2 arg2, float4 arg3, float4 arg4) {
    VertexOutput ret = (VertexOutput)0;
    ret.v_WorldPosition = arg0;
    ret.v_WorldNormal = arg1;
    ret.v_Uv = arg2;
    ret.v_WorldTangent = arg3;
    ret.gl_Position = arg4;
    return ret;
}

VertexOutput_main main(float3 Vertex_Position : LOC0, float3 Vertex_Normal : LOC1, float2 Vertex_Uv : LOC2, float4 Vertex_Tangent : LOC3)
{
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    Vertex_Tangent_1 = Vertex_Tangent;
    main_1();
    float3 _e9 = v_WorldPosition;
    float3 _e11 = v_WorldNormal;
    float2 _e13 = v_Uv;
    float4 _e15 = v_WorldTangent;
    float4 _e17 = gl_Position;
    const VertexOutput vertexoutput = ConstructVertexOutput(_e9, _e11, _e13, _e15, _e17);
    const VertexOutput_main vertexoutput_1 = { vertexoutput.v_WorldPosition, vertexoutput.v_WorldNormal, vertexoutput.v_Uv, vertexoutput.v_WorldTangent, vertexoutput.gl_Position };
    return vertexoutput_1;
}
