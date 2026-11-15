// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


float CalcShadowPCF1_(
    metal::depth2d<float, metal::access::sample> T_P_t_TextureDepth,
    metal::sampler S_P_t_TextureDepth,
    metal::float3 t_ProjCoord
) {
    metal::float3 t_ProjCoord_1 = {};
    float t_Res = 0.0;
    t_ProjCoord_1 = t_ProjCoord;
    float _e6 = t_Res;
    metal::float3 _e7 = t_ProjCoord_1;
    metal::float3 _e8 = _e7.xyz;
    float _e11 = T_P_t_TextureDepth.sample_compare(S_P_t_TextureDepth, _e8.xy, _e8.z);
    t_Res = _e6 + (_e11 * 0.2);
    float _e15 = t_Res;
    return _e15;
}

float CalcShadowPCF(
    metal::depth2d<float, metal::access::sample> T_P_t_TextureDepth_1,
    metal::sampler S_P_t_TextureDepth_1,
    metal::float3 t_ProjCoord_2,
    float t_Bias
) {
    metal::float3 t_ProjCoord_3 = {};
    float t_Bias_1 = {};
    t_ProjCoord_3 = t_ProjCoord_2;
    t_Bias_1 = t_Bias;
    metal::float3 _e7 = t_ProjCoord_3;
    float _e9 = t_Bias_1;
    t_ProjCoord_3.z = _e7.z + _e9;
    metal::float3 _e11 = t_ProjCoord_3;
    float _e13 = CalcShadowPCF1_(T_P_t_TextureDepth_1, S_P_t_TextureDepth_1, _e11.xyz);
    return _e13;
}

void main_1(
    metal::depth2d<float, metal::access::sample> tex2D,
    metal::sampler sampShadow
) {
    float _e4 = CalcShadowPCF1_(tex2D, sampShadow, metal::float3(0.0));
    float _e8 = CalcShadowPCF(tex2D, sampShadow, metal::float3(0.0), 1.0);
    return;
}

fragment void main_(
  metal::depth2d<float, metal::access::sample> tex2D [[user(fake0)]]
, metal::sampler sampShadow [[user(fake0)]]
) {
    main_1(tex2D, sampShadow);
    return;
}
