struct Camera {
    row_major float4x4 ViewProj;
};

struct Transform {
    row_major float4x4 Model;
};

struct VertexOutput {
    float3 v_Position : LOC0;
    float3 v_Normal : LOC1;
    float2 v_Uv : LOC2;
    float4 gl_Position : SV_Position;
};

static float3 Vertex_Position_1 = (float3)0;
static float3 Vertex_Normal_1 = (float3)0;
static float2 Vertex_Uv_1 = (float2)0;
static float3 v_Position = (float3)0;
static float3 v_Normal = (float3)0;
static float2 v_Uv = (float2)0;
cbuffer global : register(b0) { Camera global; }
cbuffer global_1 : register(b0, space2) { Transform global_1; }
static float4 gl_Position = (float4)0;

struct VertexOutput_main {
    float3 v_Position_1 : LOC0;
    float3 v_Normal_1 : LOC1;
    float2 v_Uv_1 : LOC2;
    float4 gl_Position_1 : SV_Position;
};

void main_1()
{
    float4x4 _e10 = global_1.Model;
    float3 _e11 = Vertex_Normal_1;
    v_Normal = mul(float4(_e11.x, _e11.y, _e11.z, 1.0), _e10).xyz;
    float4x4 _e19 = global_1.Model;
    float3 _e27 = Vertex_Normal_1;
    v_Normal = mul(_e27, float3x3(_e19[0].xyz, _e19[1].xyz, _e19[2].xyz));
    float4x4 _e29 = global_1.Model;
    float3 _e30 = Vertex_Position_1;
    v_Position = mul(float4(_e30.x, _e30.y, _e30.z, 1.0), _e29).xyz;
    float2 _e38 = Vertex_Uv_1;
    v_Uv = _e38;
    float4x4 _e40 = global.ViewProj;
    float3 _e41 = v_Position;
    gl_Position = mul(float4(_e41.x, _e41.y, _e41.z, 1.0), _e40);
    return;
}

VertexOutput ConstructVertexOutput(float3 arg0, float3 arg1, float2 arg2, float4 arg3) {
    VertexOutput ret = (VertexOutput)0;
    ret.v_Position = arg0;
    ret.v_Normal = arg1;
    ret.v_Uv = arg2;
    ret.gl_Position = arg3;
    return ret;
}

VertexOutput_main main(float3 Vertex_Position : LOC0, float3 Vertex_Normal : LOC1, float2 Vertex_Uv : LOC2)
{
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    main_1();
    float3 _e7 = v_Position;
    float3 _e9 = v_Normal;
    float2 _e11 = v_Uv;
    float4 _e13 = gl_Position;
    const VertexOutput vertexoutput = ConstructVertexOutput(_e7, _e9, _e11, _e13);
    const VertexOutput_main vertexoutput_1 = { vertexoutput.v_Position, vertexoutput.v_Normal, vertexoutput.v_Uv, vertexoutput.gl_Position };
    return vertexoutput_1;
}
