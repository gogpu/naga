struct Camera {
    row_major float4x4 ViewProj;
};

struct Transform {
    row_major float4x4 Model;
};

struct Sprite_size {
    float2 size;
};

struct VertexOutput {
    float2 v_Uv : LOC0;
    float4 gl_Position : SV_Position;
};

static float3 Vertex_Position_1 = (float3)0;
static float3 Vertex_Normal_1 = (float3)0;
static float2 Vertex_Uv_1 = (float2)0;
static float2 v_Uv = (float2)0;
cbuffer global : register(b0) { Camera global; }
cbuffer global_1 : register(b0, space2) { Transform global_1; }
cbuffer global_2 : register(b1, space2) { Sprite_size global_2; }
static float4 gl_Position = (float4)0;

struct VertexOutput_main {
    float2 v_Uv_1 : LOC0;
    float4 gl_Position_1 : SV_Position;
};

void main_1()
{
    float3 position = (float3)0;

    float2 _e9 = Vertex_Uv_1;
    v_Uv = _e9;
    float3 _e10 = Vertex_Position_1;
    float2 _e11 = global_2.size;
    position = (_e10 * float3(_e11.x, _e11.y, 1.0));
    float4x4 _e19 = global.ViewProj;
    float4x4 _e20 = global_1.Model;
    float3 _e22 = position;
    gl_Position = mul(float4(_e22.x, _e22.y, _e22.z, 1.0), mul(_e20, _e19));
    return;
}

VertexOutput ConstructVertexOutput(float2 arg0, float4 arg1) {
    VertexOutput ret = (VertexOutput)0;
    ret.v_Uv = arg0;
    ret.gl_Position = arg1;
    return ret;
}

VertexOutput_main main(float3 Vertex_Position : LOC0, float3 Vertex_Normal : LOC1, float2 Vertex_Uv : LOC2)
{
    Vertex_Position_1 = Vertex_Position;
    Vertex_Normal_1 = Vertex_Normal;
    Vertex_Uv_1 = Vertex_Uv;
    main_1();
    float2 _e7 = v_Uv;
    float4 _e9 = gl_Position;
    const VertexOutput vertexoutput = ConstructVertexOutput(_e7, _e9);
    const VertexOutput_main vertexoutput_1 = { vertexoutput.v_Uv, vertexoutput.gl_Position };
    return vertexoutput_1;
}
