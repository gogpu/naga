struct VertexOutput {
    float2 v_uv : LOC0;
    float4 gl_Position : SV_Position;
};

static const float c_scale = 1.2;

static float2 a_pos_1 = (float2)0;
static float2 a_uv_1 = (float2)0;
static float2 v_uv = (float2)0;
static float4 gl_Position = (float4)0;

struct VertexOutput_main {
    float2 v_uv_1 : LOC0;
    float4 gl_Position_1 : SV_Position;
};

void main_1()
{
    float2 _e4 = a_uv_1;
    v_uv = _e4;
    float2 _e6 = a_pos_1;
    float2 _e7 = (c_scale * _e6);
    gl_Position = float4(_e7.x, _e7.y, 0.0, 1.0);
    return;
}

VertexOutput ConstructVertexOutput(float2 arg0, float4 arg1) {
    VertexOutput ret = (VertexOutput)0;
    ret.v_uv = arg0;
    ret.gl_Position = arg1;
    return ret;
}

VertexOutput_main main(float2 a_pos : LOC0, float2 a_uv : LOC1)
{
    a_pos_1 = a_pos;
    a_uv_1 = a_uv;
    main_1();
    float2 _e5 = v_uv;
    float4 _e7 = gl_Position;
    const VertexOutput vertexoutput = ConstructVertexOutput(_e5, _e7);
    const VertexOutput_main vertexoutput_1 = { vertexoutput.v_uv, vertexoutput.gl_Position };
    return vertexoutput_1;
}
