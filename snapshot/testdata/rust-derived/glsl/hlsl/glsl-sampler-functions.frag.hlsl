Texture2D<float> tex2D_ : register(t0, space1);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup1SamplerIndexArray : register(t1, space255);
static const SamplerComparisonState sampShadow = nagaComparisonSamplerHeap[nagaGroup1SamplerIndexArray[1]];

float CalcShadowPCF1_(Texture2D<float> T_P_t_TextureDepth, SamplerComparisonState S_P_t_TextureDepth, float3 t_ProjCoord)
{
    float3 t_ProjCoord_1 = (float3)0;
    float t_Res = 0.0;

    t_ProjCoord_1 = t_ProjCoord;
    float _e6 = t_Res;
    float3 _e7 = t_ProjCoord_1;
    float3 _e8 = _e7.xyz;
    float _e11 = T_P_t_TextureDepth.SampleCmp(S_P_t_TextureDepth, _e8.xy, _e8.z);
    t_Res = (_e6 + (_e11 * 0.2));
    float _e15 = t_Res;
    return _e15;
}

float CalcShadowPCF(Texture2D<float> T_P_t_TextureDepth_1, SamplerComparisonState S_P_t_TextureDepth_1, float3 t_ProjCoord_2, float t_Bias)
{
    float3 t_ProjCoord_3 = (float3)0;
    float t_Bias_1 = (float)0;

    t_ProjCoord_3 = t_ProjCoord_2;
    t_Bias_1 = t_Bias;
    float3 _e7 = t_ProjCoord_3;
    float _e9 = t_Bias_1;
    t_ProjCoord_3.z = (_e7.z + _e9);
    float3 _e11 = t_ProjCoord_3;
    const float _e13 = CalcShadowPCF1_(T_P_t_TextureDepth_1, S_P_t_TextureDepth_1, _e11.xyz);
    return _e13;
}

void main_1()
{
    const float _e4 = CalcShadowPCF1_(tex2D_, sampShadow, (0.0).xxx);
    const float _e8 = CalcShadowPCF(tex2D_, sampShadow, (0.0).xxx, 1.0);
    return;
}

void main()
{
    main_1();
    return;
}
