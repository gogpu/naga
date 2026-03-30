struct Vertex {
    float2 position : LOC0;
};

struct NoteInstance {
    float2 position : LOC1;
};

struct VertexOutput {
    float4 position : SV_Position;
};

struct VertexOutput_vs_main {
    float4 position : SV_Position;
};

struct FragmentInput_fs_main {
    float2 position_2 : LOC1;
    float4 position_1 : SV_Position;
};

VertexOutput_vs_main vs_main(Vertex vertex, NoteInstance note)
{
    VertexOutput out_ = (VertexOutput)0;

    VertexOutput _e3 = out_;
    const VertexOutput vertexoutput = _e3;
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    VertexOutput in_ = { fragmentinput_fs_main.position_1 };
    NoteInstance note_1 = { fragmentinput_fs_main.position_2 };
    float3 position_3 = (1.0).xxx;
    return in_.position;
}
