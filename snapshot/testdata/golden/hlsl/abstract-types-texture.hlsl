Texture2D<float4> t : register(t0);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerState s = nagaSamplerHeap[nagaGroup0SamplerIndexArray[1]];
Texture2D<float> d : register(t2);
static const SamplerComparisonState c = nagaComparisonSamplerHeap[nagaGroup0SamplerIndexArray[3]];
RWTexture2D<unorm float4> st : register(u4);

void color()
{
    float4 phony = t.Sample(s, float2(1.0, 2.0));
    float4 phony_1 = t.Sample(s, float2(1.0, 2.0), int2(int2(int(3), int(4))));
    float4 phony_2 = t.SampleLevel(s, float2(1.0, 2.0), 0.0);
    float4 phony_3 = t.SampleLevel(s, float2(1.0, 2.0), 0.0);
    float4 phony_4 = t.SampleGrad(s, float2(1.0, 2.0), float2(3.0, 4.0), float2(5.0, 6.0));
    float4 phony_5 = t.SampleBias(s, float2(1.0, 2.0), 1.0);
    return;
}

void depth()
{
    float phony_6 = d.SampleLevel(s, float2(1.0, 2.0), int(1));
    float phony_7 = d.SampleCmp(c, float2(1.0, 2.0), 0.0);
    float4 phony_8 = d.GatherCmp(c, float2(1.0, 2.0), 0.0);
    return;
}

void storage()
{
    st[int2(int(0), int(1))] = float4(2.0, 3.0, 4.0, 5.0);
    return;
}

void main()
{
    color();
    depth();
    storage();
    return;
}
