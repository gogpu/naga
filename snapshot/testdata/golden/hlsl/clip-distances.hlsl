struct VertexOutput {
    float4 position : SV_Position;
    float clip_distances[1] : SV_Position;
};

struct VertexOutput_main {
    float4 position : SV_Position;
    float[1] clip_distances : SV_Position;
};

VertexOutput_main main()
{
    VertexOutput out_ = (VertexOutput)0;

    out_.clip_distances[0] = 0.5;
    VertexOutput _e4 = out_;
    const VertexOutput vertexoutput = _e4;
    const VertexOutput_main vertexoutput_1 = { vertexoutput.position, vertexoutput.clip_distances };
    return vertexoutput_1;
}
