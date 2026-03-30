Texture2D<float4> u_texture : register(t0);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerState u_sampler = nagaSamplerHeap[nagaGroup0SamplerIndexArray[1]];

struct FragmentInput_main {
    float2 uv_1 : LOC0;
};

float4 main(FragmentInput_main fragmentinput_main) : SV_Target0
{
    float2 uv = fragmentinput_main.uv_1;
    float4 _e3 = u_texture.Sample(u_sampler, uv);
    return _e3;
}
