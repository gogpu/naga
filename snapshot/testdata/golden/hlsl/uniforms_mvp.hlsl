struct Uniforms {
    row_major float4x4 model;
    row_major float4x4 view;
    row_major float4x4 projection;
};

struct VertexOutput {
    float4 position : SV_Position;
    float3 world_pos : LOC0;
};

cbuffer uniforms : register(b0) { Uniforms uniforms; }

struct VertexOutput_vs_main {
    float3 world_pos_1 : LOC0;
    float4 position_1 : SV_Position;
};

struct FragmentInput_fs_main {
    float3 world_pos_2 : LOC0;
};

VertexOutput_vs_main vs_main(float3 position : LOC0)
{
    VertexOutput out_ = (VertexOutput)0;

    float4x4 _e3 = uniforms.model;
    float4 world = mul(float4(position, 1.0), _e3);
    float4x4 _e9 = uniforms.projection;
    float4x4 _e12 = uniforms.view;
    float4 clip_ = mul(world, mul(_e12, _e9));
    out_.position = clip_;
    out_.world_pos = world.xyz;
    VertexOutput _e19 = out_;
    const VertexOutput vertexoutput = _e19;
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.world_pos, vertexoutput.position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    float3 world_pos = fragmentinput_fs_main.world_pos_2;
    float3 color = ((normalize(world_pos) * 0.5) + (0.5).xxx);
    return float4(color, 1.0);
}
