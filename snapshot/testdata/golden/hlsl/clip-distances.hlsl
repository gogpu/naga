struct VertexOutput {
    float4 position : SV_Position;
    float clip_distances[1] : SV_ClipDistance;
};

struct VertexOutput_main {
    float4 position : SV_Position;
    float clip_distances0_ : SV_ClipDistance0;
};

VertexOutput_main main()
{
    VertexOutput out_ = (VertexOutput)0;

    out_.clip_distances[0] = 0.5;
    VertexOutput _e4 = out_;
    const VertexOutput vertexoutput = _e4;
    const VertexOutput_main vertexoutput_1 = { vertexoutput.position, vertexoutput.clip_distances[0] };
    return vertexoutput_1;
}
