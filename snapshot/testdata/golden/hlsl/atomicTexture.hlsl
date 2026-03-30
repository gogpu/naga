RWTexture2D<float4> image_u : register(u0, space0);
RWTexture2D<float4> image_s : register(u1, space0);

struct cs_main_Input {
    uint3 id : SV_GroupThreadID;
};

[numthreads(2, 1, 1)]
void cs_main(cs_main_Input _input, uint3 id : SV_GroupThreadID)
{
    
    int2 _e4 = int2(0, 0);
    Texture2D<float4> _e6 = Texture2D<float4>(image_u, _e4, 1u);
    int2 _e10 = int2(0, 0);
    Texture2D<float4> _e12 = Texture2D<float4>(image_u, _e10, 1u);
    int2 _e16 = int2(0, 0);
    Texture2D<float4> _e18 = Texture2D<float4>(image_u, _e16, 1u);
    int2 _e22 = int2(0, 0);
    Texture2D<float4> _e24 = Texture2D<float4>(image_u, _e22, 1u);
    int2 _e28 = int2(0, 0);
    Texture2D<float4> _e30 = Texture2D<float4>(image_u, _e28, 1u);
    int2 _e34 = int2(0, 0);
    Texture2D<float4> _e36 = Texture2D<float4>(image_u, _e34, 1u);
    int2 _e40 = int2(0, 0);
    Texture2D<float4> _e42 = Texture2D<float4>(image_s, _e40, 1);
    int2 _e46 = int2(0, 0);
    Texture2D<float4> _e48 = Texture2D<float4>(image_s, _e46, 1);
    int2 _e52 = int2(0, 0);
    Texture2D<float4> _e54 = Texture2D<float4>(image_s, _e52, 1);
    int2 _e58 = int2(0, 0);
    Texture2D<float4> _e60 = Texture2D<float4>(image_s, _e58, 1);
    int2 _e64 = int2(0, 0);
    Texture2D<float4> _e66 = Texture2D<float4>(image_s, _e64, 1);
    int2 _e70 = int2(0, 0);
    Texture2D<float4> _e72 = Texture2D<float4>(image_s, _e70, 1);
    return;
}
