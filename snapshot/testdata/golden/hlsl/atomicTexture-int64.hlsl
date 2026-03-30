RWTexture2D<float4> image : register(u0, space0);

struct cs_main_Input {
    uint3 id : SV_GroupThreadID;
};

[numthreads(2, 1, 1)]
void cs_main(cs_main_Input _input, uint3 id : SV_GroupThreadID)
{
    
    int2 _e4 = int2(0, 0);
    Texture2D<float4> _e6 = Texture2D<float4>(image, _e4, 1UL);
    GroupMemoryBarrierWithGroupSync();
    int2 _e10 = int2(0, 0);
    Texture2D<float4> _e12 = Texture2D<float4>(image, _e10, 1UL);
    return;
}
