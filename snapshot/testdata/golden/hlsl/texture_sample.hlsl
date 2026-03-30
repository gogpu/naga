struct VertexOutput {
    float4 position : SV_Position;
    float2 uv : LOC0;
};

Texture2D<float4> t_diffuse : register(t0);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerState s_diffuse = nagaSamplerHeap[nagaGroup0SamplerIndexArray[1]];

struct VertexOutput_vs_main {
    float2 uv_1 : LOC0;
    float4 position : SV_Position;
};

struct FragmentInput_fs_main {
    float2 uv_2 : LOC0;
};

int naga_div(int lhs, int rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}

int naga_mod(int lhs, int rhs) {
    int divisor = ((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

VertexOutput ConstructVertexOutput(float4 arg0, float2 arg1) {
    VertexOutput ret = (VertexOutput)0;
    ret.position = arg0;
    ret.uv = arg1;
    return ret;
}

VertexOutput_vs_main vs_main(uint vi : SV_VertexID)
{
    float x = ((float(naga_div(int(vi), int(2))) * 4.0) - 1.0);
    float y = ((float(naga_mod(int(vi), int(2))) * 4.0) - 1.0);
    float2 pos = float2(x, y);
    float2 uv_3 = ((pos + (1.0).xx) * 0.5);
    const VertexOutput vertexoutput = ConstructVertexOutput(float4(pos, 0.0, 1.0), uv_3);
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.uv, vertexoutput.position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    float2 uv = fragmentinput_fs_main.uv_2;
    float4 color = t_diffuse.Sample(s_diffuse, uv);
    return color;
}
