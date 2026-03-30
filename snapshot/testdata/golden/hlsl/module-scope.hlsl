struct S {
    int x;
};

static const int Value = int(1);

Texture2D<float4> Texture : register(t0);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup0SamplerIndexArray : register(t0, space255);
static const SamplerState Sampler = nagaSamplerHeap[nagaGroup0SamplerIndexArray[1]];

void statement()
{
    return;
}

S ConstructS(int arg0) {
    S ret = (S)0;
    ret.x = arg0;
    return ret;
}

S returns()
{
    const S s = ConstructS(int(1));
    return s;
}

void call()
{
    statement();
    const S _e0 = returns();
    float4 s_1 = Texture.Sample(Sampler, (1.0).xx);
    return;
}

void main()
{
    call();
    statement();
    const S _e0 = returns();
    return;
}
