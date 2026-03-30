TextureCubeArray<float> point_shadow_textures : register(t4);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerComparisonState point_shadow_textures_sampler = nagaComparisonSamplerHeap[nagaGroup0SamplerIndexArray[5]];

float4 fragment() : SV_Target0
{
    float3 frag_ls = float3(1.0, 1.0, 2.0);
    float a = point_shadow_textures.SampleCmp(point_shadow_textures_sampler, float4(frag_ls, int(1)), 1.0);
    return float4(a, 1.0, 1.0, 1.0);
}
