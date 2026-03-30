TextureCubeArray<float> texture_ : register(t0);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerComparisonState texture_sampler = nagaComparisonSamplerHeap[nagaGroup0SamplerIndexArray[1]];

float main() : SV_Target0
{
    float3 pos = (0.0).xxx;
    float _e6 = texture_.SampleCmpLevelZero(texture_sampler, float4(pos, int(0)), 0.0);
    return _e6;
}
