Texture2D<float4> t1_ : register(t0);
Texture2D<float4> t2_ : register(t1);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerState s1_ = nagaSamplerHeap[nagaGroup0SamplerIndexArray[2]];
static const SamplerState s2_ = nagaSamplerHeap[nagaGroup0SamplerIndexArray[3]];
cbuffer uniformOne : register(b4) { float2 uniformOne; }
cbuffer uniformTwo : register(b0, space1) { float2 uniformTwo; }

struct FragmentInput_entry_point_one {
    float4 pos_1 : SV_Position;
};

float4 entry_point_one(FragmentInput_entry_point_one fragmentinput_entry_point_one) : SV_Target0
{
    float4 pos = fragmentinput_entry_point_one.pos_1;
    float4 _e4 = t1_.Sample(s1_, pos.xy);
    return _e4;
}

float4 entry_point_two() : SV_Target0
{
    float2 _e3 = uniformOne;
    float4 _e4 = t1_.Sample(s1_, _e3);
    return _e4;
}

float4 entry_point_three() : SV_Target0
{
    float2 _e3 = uniformTwo;
    float2 _e5 = uniformOne;
    float4 _e7 = t1_.Sample(s1_, (_e3 + _e5));
    float2 _e11 = uniformOne;
    float4 _e12 = t2_.Sample(s2_, _e11);
    return (_e7 + _e12);
}
