struct VertexOutput {
    float4 gl_Position : SV_Position;
};

static float2 a_pos_1 = (float2)0;
static float4 gl_Position = (float4)0;

struct VertexOutput_main {
    float4 gl_Position_1 : SV_Position;
};

void main_1()
{
    float2 _e2 = a_pos_1;
    float2 _e7 = clamp(_e2, (0.0).xx, (1.0).xx);
    gl_Position = float4(_e7.x, _e7.y, 0.0, 1.0);
    return;
}

VertexOutput ConstructVertexOutput(float4 arg0) {
    VertexOutput ret = (VertexOutput)0;
    ret.gl_Position = arg0;
    return ret;
}

VertexOutput_main main(float2 a_pos : LOC0)
{
    a_pos_1 = a_pos;
    main_1();
    float4 _e3 = gl_Position;
    const VertexOutput vertexoutput = ConstructVertexOutput(_e3);
    const VertexOutput_main vertexoutput_1 = { vertexoutput.gl_Position };
    return vertexoutput_1;
}
