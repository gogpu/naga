struct VertexInput {
    float3 position : LOC0;
    float3 color : LOC1;
};

struct VertexOutput {
    float4 position : SV_Position;
    float3 color : LOC0;
};

struct VertexOutput_vs_main {
    float3 color_1 : LOC0;
    float4 position : SV_Position;
};

struct FragmentInput_fs_main {
    float3 color_2 : LOC0;
};

VertexOutput_vs_main vs_main(VertexInput in_)
{
    VertexOutput out_ = (VertexOutput)0;

    out_.position = float4(in_.position, 1.0);
    out_.color = in_.color;
    VertexOutput _e8 = out_;
    const VertexOutput vertexoutput = _e8;
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.color, vertexoutput.position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    float3 color = fragmentinput_fs_main.color_2;
    return float4(color, 1.0);
}
