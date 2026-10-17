struct VertexInput {
    nointerpolation int3 chunk : LOC0;
    nointerpolation uint texture_index : LOC1;
};

struct VertexOutput {
    float4 clip_position : SV_Position;
};

struct VertexOutput_vs_main {
    float4 clip_position : SV_Position;
};

VertexOutput_vs_main vs_main(VertexInput in_)
{
    VertexOutput out_ = (VertexOutput)0;

    float3 position = float3(asint(asuint(in_.chunk) - asuint((int(5)).xxx)));
    VertexOutput _e7 = out_;
    const VertexOutput vertexoutput = _e7;
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.clip_position };
    return vertexoutput_1;
}
