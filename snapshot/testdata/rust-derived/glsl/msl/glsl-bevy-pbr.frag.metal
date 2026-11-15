// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct PointLight {
    metal::float4 pos;
    metal::float4 color;
    metal::float4 lightParams;
};
struct DirectionalLight {
    metal::float4 direction;
    metal::float4 color;
};
struct CameraPosition {
    metal::float4 CameraPos;
};
struct type_5 {
    PointLight inner[10];
};
struct type_6 {
    DirectionalLight inner[1];
};
struct Lights {
    metal::float4 AmbientColor;
    metal::uint4 NumLights;
    type_5 PointLights;
    type_6 DirectionalLights;
};
struct StandardMaterial_base_color {
    metal::float4 base_color;
};
struct StandardMaterial_roughness {
    float perceptual_roughness;
};
struct StandardMaterial_metallic {
    float metallic;
};
struct StandardMaterial_reflectance {
    float reflectance;
};
struct StandardMaterial_emissive {
    metal::float4 emissive;
};
struct FragmentOutput {
    metal::float4 o_Target;
};
constant int MAX_POINT_LIGHTS = 10;
constant int MAX_DIRECTIONAL_LIGHTS = 1;
constant float PI = 3.1415927;

float pow5_(
    float x
) {
    float x_1 = {};
    float x2_ = {};
    x_1 = x;
    float _e2 = x_1;
    float _e3 = x_1;
    x2_ = _e2 * _e3;
    float _e6 = x2_;
    float _e7 = x2_;
    float _e9 = x_1;
    return (_e6 * _e7) * _e9;
}

float getDistanceAttenuation(
    float distanceSquare,
    float inverseRangeSquared
) {
    float distanceSquare_1 = {};
    float inverseRangeSquared_1 = {};
    float factor = {};
    float smoothFactor = {};
    float attenuation = {};
    distanceSquare_1 = distanceSquare;
    inverseRangeSquared_1 = inverseRangeSquared;
    float _e4 = distanceSquare_1;
    float _e5 = inverseRangeSquared_1;
    factor = _e4 * _e5;
    float _e9 = factor;
    float _e10 = factor;
    smoothFactor = metal::clamp(1.0 - (_e9 * _e10), 0.0, 1.0);
    float _e17 = smoothFactor;
    float _e18 = smoothFactor;
    attenuation = _e17 * _e18;
    float _e21 = attenuation;
    float _e24 = distanceSquare_1;
    return (_e21 * 1.0) / metal::max(_e24, 0.001);
}

float D_GGX(
    float roughness,
    float NoH,
    metal::float3 h
) {
    float roughness_1 = {};
    float NoH_1 = {};
    float oneMinusNoHSquared = {};
    float a = {};
    float k = {};
    float d = {};
    roughness_1 = roughness;
    NoH_1 = NoH;
    float _e5 = NoH_1;
    float _e6 = NoH_1;
    oneMinusNoHSquared = 1.0 - (_e5 * _e6);
    float _e10 = NoH_1;
    float _e11 = roughness_1;
    a = _e10 * _e11;
    float _e14 = roughness_1;
    float _e15 = oneMinusNoHSquared;
    float _e16 = a;
    float _e17 = a;
    k = _e14 / (_e15 + (_e16 * _e17));
    float _e22 = k;
    float _e23 = k;
    d = (_e22 * _e23) * 0.31830987;
    float _e28 = d;
    return _e28;
}

float V_SmithGGXCorrelated(
    float roughness_2,
    float NoV,
    float NoL
) {
    float roughness_3 = {};
    float NoV_1 = {};
    float NoL_1 = {};
    float a2_ = {};
    float lambdaV = {};
    float lambdaL = {};
    float v = {};
    roughness_3 = roughness_2;
    NoV_1 = NoV;
    NoL_1 = NoL;
    float _e6 = roughness_3;
    float _e7 = roughness_3;
    a2_ = _e6 * _e7;
    float _e10 = NoL_1;
    float _e11 = NoV_1;
    float _e12 = a2_;
    float _e13 = NoV_1;
    float _e16 = NoV_1;
    float _e18 = a2_;
    lambdaV = _e10 * metal::sqrt(((_e11 - (_e12 * _e13)) * _e16) + _e18);
    float _e23 = NoV_1;
    float _e24 = NoL_1;
    float _e25 = a2_;
    float _e26 = NoL_1;
    float _e29 = NoL_1;
    float _e31 = a2_;
    lambdaL = _e23 * metal::sqrt(((_e24 - (_e25 * _e26)) * _e29) + _e31);
    float _e37 = lambdaV;
    float _e38 = lambdaL;
    v = 0.5 / (_e37 + _e38);
    float _e42 = v;
    return _e42;
}

metal::float3 F_Schlick(
    metal::float3 f0_,
    float f90_,
    float VoH
) {
    float f90_1 = {};
    float VoH_1 = {};
    f90_1 = f90_;
    VoH_1 = VoH;
    float _e5 = f90_1;
    float _e9 = VoH_1;
    float _e11 = pow5_(1.0 - _e9);
    return f0_ + ((metal::float3(_e5) - f0_) * _e11);
}

float F_Schlick_1(
    float f0_1,
    float f90_2,
    float VoH_2
) {
    float f0_2 = {};
    float f90_3 = {};
    float VoH_3 = {};
    f0_2 = f0_1;
    f90_3 = f90_2;
    VoH_3 = VoH_2;
    float _e6 = f0_2;
    float _e7 = f90_3;
    float _e8 = f0_2;
    float _e11 = VoH_3;
    float _e13 = pow5_(1.0 - _e11);
    return _e6 + ((_e7 - _e8) * _e13);
}

metal::float3 fresnel(
    metal::float3 f0_3,
    float LoH
) {
    metal::float3 f0_4 = {};
    float LoH_1 = {};
    float f90_4 = {};
    f0_4 = f0_3;
    LoH_1 = LoH;
    metal::float3 _e4 = f0_4;
    f90_4 = metal::clamp(metal::dot(_e4, metal::float3(16.5)), 0.0, 1.0);
    metal::float3 _e12 = f0_4;
    float _e13 = f90_4;
    float _e14 = LoH_1;
    metal::float3 _e15 = F_Schlick(_e12, _e13, _e14);
    return _e15;
}

metal::float3 specular(
    metal::float3 f0_5,
    float roughness_4,
    metal::float3 h_1,
    float NoV_2,
    float NoL_2,
    float NoH_2,
    float LoH_2,
    float specularIntensity
) {
    metal::float3 f0_6 = {};
    float roughness_5 = {};
    float NoV_3 = {};
    float NoL_3 = {};
    float NoH_3 = {};
    float LoH_3 = {};
    float specularIntensity_1 = {};
    float D = {};
    float V = {};
    metal::float3 F = {};
    f0_6 = f0_5;
    roughness_5 = roughness_4;
    NoV_3 = NoV_2;
    NoL_3 = NoL_2;
    NoH_3 = NoH_2;
    LoH_3 = LoH_2;
    specularIntensity_1 = specularIntensity;
    float _e15 = roughness_5;
    float _e16 = NoH_3;
    float _e17 = D_GGX(_e15, _e16, h_1);
    D = _e17;
    float _e19 = roughness_5;
    float _e20 = NoV_3;
    float _e21 = NoL_3;
    float _e22 = V_SmithGGXCorrelated(_e19, _e20, _e21);
    V = _e22;
    metal::float3 _e24 = f0_6;
    float _e25 = LoH_3;
    metal::float3 _e26 = fresnel(_e24, _e25);
    F = _e26;
    float _e28 = specularIntensity_1;
    float _e29 = D;
    float _e31 = V;
    metal::float3 _e33 = F;
    return ((_e28 * _e29) * _e31) * _e33;
}

float Fd_Burley(
    float roughness_6,
    float NoV_4,
    float NoL_4,
    float LoH_4
) {
    float roughness_7 = {};
    float NoV_5 = {};
    float NoL_5 = {};
    float LoH_5 = {};
    float f90_5 = {};
    float lightScatter = {};
    float viewScatter = {};
    roughness_7 = roughness_6;
    NoV_5 = NoV_4;
    NoL_5 = NoL_4;
    LoH_5 = LoH_4;
    float _e10 = roughness_7;
    float _e12 = LoH_5;
    float _e14 = LoH_5;
    f90_5 = 0.5 + (((2.0 * _e10) * _e12) * _e14);
    float _e19 = f90_5;
    float _e20 = NoL_5;
    float _e21 = F_Schlick_1(1.0, _e19, _e20);
    lightScatter = _e21;
    float _e24 = f90_5;
    float _e25 = NoV_5;
    float _e26 = F_Schlick_1(1.0, _e24, _e25);
    viewScatter = _e26;
    float _e28 = lightScatter;
    float _e29 = viewScatter;
    return (_e28 * _e29) * 0.31830987;
}

metal::float3 EnvBRDFApprox(
    metal::float3 f0_7,
    float perceptual_roughness,
    float NoV_6
) {
    metal::float3 f0_8 = {};
    float perceptual_roughness_1 = {};
    float NoV_7 = {};
    metal::float4 c0_ = metal::float4(-1.0, -0.0275, -0.572, 0.022);
    metal::float4 c1_ = metal::float4(1.0, 0.0425, 1.04, -0.04);
    metal::float4 r = {};
    float a004_ = {};
    metal::float2 AB = {};
    f0_8 = f0_7;
    perceptual_roughness_1 = perceptual_roughness;
    NoV_7 = NoV_6;
    float _e18 = perceptual_roughness_1;
    metal::float4 _e20 = c0_;
    metal::float4 _e22 = c1_;
    r = (metal::float4(_e18) * _e20) + _e22;
    metal::float4 _e25 = r;
    metal::float4 _e27 = r;
    float _e31 = NoV_7;
    metal::float4 _e35 = r;
    metal::float4 _e38 = r;
    a004_ = (metal::min(_e25.x * _e27.x, metal::exp2(-9.28 * _e31)) * _e35.x) + _e38.y;
    float _e45 = a004_;
    metal::float4 _e48 = r;
    AB = (metal::float2(-1.04, 1.04) * metal::float2(_e45)) + _e48.zw;
    metal::float3 _e52 = f0_8;
    metal::float2 _e53 = AB;
    metal::float2 _e57 = AB;
    return (_e52 * metal::float3(_e53.x)) + metal::float3(_e57.y);
}

float perceptualRoughnessToRoughness(
    float perceptualRoughness
) {
    float perceptualRoughness_1 = {};
    float clampedPerceptualRoughness = {};
    perceptualRoughness_1 = perceptualRoughness;
    float _e2 = perceptualRoughness_1;
    clampedPerceptualRoughness = metal::clamp(_e2, 0.089, 1.0);
    float _e7 = clampedPerceptualRoughness;
    float _e8 = clampedPerceptualRoughness;
    return _e7 * _e8;
}

float luminance(
    metal::float3 v_1
) {
    metal::float3 v_2 = {};
    v_2 = v_1;
    metal::float3 _e2 = v_2;
    return metal::dot(_e2, metal::float3(0.2126, 0.7152, 0.0722));
}

metal::float3 change_luminance(
    metal::float3 c_in,
    float l_out
) {
    metal::float3 c_in_1 = {};
    float l_out_1 = {};
    float l_in = {};
    c_in_1 = c_in;
    l_out_1 = l_out;
    metal::float3 _e4 = c_in_1;
    float _e5 = luminance(_e4);
    l_in = _e5;
    metal::float3 _e7 = c_in_1;
    float _e8 = l_out_1;
    float _e9 = l_in;
    return _e7 * (_e8 / _e9);
}

metal::float3 reinhard_luminance(
    metal::float3 color
) {
    metal::float3 color_1 = {};
    float l_old = {};
    float l_new = {};
    color_1 = color;
    metal::float3 _e2 = color_1;
    float _e3 = luminance(_e2);
    l_old = _e3;
    float _e5 = l_old;
    float _e7 = l_old;
    l_new = _e5 / (1.0 + _e7);
    metal::float3 _e11 = color_1;
    float _e12 = l_new;
    metal::float3 _e13 = change_luminance(_e11, _e12);
    return _e13;
}

metal::float3 point_light(
    PointLight light,
    float roughness_8,
    float NdotV,
    metal::float3 N,
    metal::float3 V_1,
    metal::float3 R,
    metal::float3 F0_,
    metal::float3 diffuseColor,
    thread metal::float3& v_WorldPosition_1
) {
    PointLight light_1 = {};
    float roughness_9 = {};
    float NdotV_1 = {};
    metal::float3 N_1 = {};
    metal::float3 V_2 = {};
    metal::float3 R_1 = {};
    metal::float3 F0_1 = {};
    metal::float3 diffuseColor_1 = {};
    metal::float3 light_to_frag = {};
    float distance_square = {};
    float rangeAttenuation = {};
    float a_1 = {};
    float radius = {};
    metal::float3 centerToRay = {};
    metal::float3 closestPoint = {};
    float LspecLengthInverse = {};
    float normalizationFactor = {};
    float specularIntensity_2 = {};
    metal::float3 L = {};
    metal::float3 H = {};
    float NoL_6 = {};
    float NoH_4 = {};
    float LoH_6 = {};
    metal::float3 specular_1 = {};
    metal::float3 diffuse = {};
    light_1 = light;
    roughness_9 = roughness_8;
    NdotV_1 = NdotV;
    N_1 = N;
    V_2 = V_1;
    R_1 = R;
    F0_1 = F0_;
    diffuseColor_1 = diffuseColor;
    PointLight _e17 = light_1;
    metal::float3 _e20 = v_WorldPosition_1;
    light_to_frag = _e17.pos.xyz - _e20.xyz;
    metal::float3 _e24 = light_to_frag;
    metal::float3 _e25 = light_to_frag;
    distance_square = metal::dot(_e24, _e25);
    float _e28 = distance_square;
    PointLight _e29 = light_1;
    float _e32 = getDistanceAttenuation(_e28, _e29.lightParams.x);
    rangeAttenuation = _e32;
    float _e34 = roughness_9;
    a_1 = _e34;
    PointLight _e36 = light_1;
    radius = _e36.lightParams.y;
    metal::float3 _e40 = light_to_frag;
    metal::float3 _e41 = R_1;
    metal::float3 _e43 = R_1;
    metal::float3 _e45 = light_to_frag;
    centerToRay = (metal::dot(_e40, _e41) * _e43) - _e45;
    metal::float3 _e48 = light_to_frag;
    metal::float3 _e49 = centerToRay;
    float _e50 = radius;
    metal::float3 _e51 = centerToRay;
    metal::float3 _e52 = centerToRay;
    closestPoint = _e48 + (_e49 * metal::clamp(_e50 * metal::rsqrt(metal::dot(_e51, _e52)), 0.0, 1.0));
    metal::float3 _e62 = closestPoint;
    metal::float3 _e63 = closestPoint;
    LspecLengthInverse = metal::rsqrt(metal::dot(_e62, _e63));
    float _e67 = a_1;
    float _e68 = a_1;
    float _e69 = radius;
    float _e72 = LspecLengthInverse;
    normalizationFactor = _e67 / metal::clamp(_e68 + ((_e69 * 0.5) * _e72), 0.0, 1.0);
    float _e80 = normalizationFactor;
    float _e81 = normalizationFactor;
    specularIntensity_2 = _e80 * _e81;
    metal::float3 _e84 = closestPoint;
    float _e85 = LspecLengthInverse;
    L = _e84 * _e85;
    metal::float3 _e88 = L;
    metal::float3 _e89 = V_2;
    H = metal::normalize(_e88 + _e89);
    metal::float3 _e93 = N_1;
    metal::float3 _e94 = L;
    NoL_6 = metal::clamp(metal::dot(_e93, _e94), 0.0, 1.0);
    metal::float3 _e100 = N_1;
    metal::float3 _e101 = H;
    NoH_4 = metal::clamp(metal::dot(_e100, _e101), 0.0, 1.0);
    metal::float3 _e107 = L;
    metal::float3 _e108 = H;
    LoH_6 = metal::clamp(metal::dot(_e107, _e108), 0.0, 1.0);
    metal::float3 _e114 = F0_1;
    float _e115 = roughness_9;
    metal::float3 _e116 = H;
    float _e117 = NdotV_1;
    float _e118 = NoL_6;
    float _e119 = NoH_4;
    float _e120 = LoH_6;
    float _e121 = specularIntensity_2;
    metal::float3 _e122 = specular(_e114, _e115, _e116, _e117, _e118, _e119, _e120, _e121);
    specular_1 = _e122;
    metal::float3 _e124 = light_to_frag;
    L = metal::normalize(_e124);
    metal::float3 _e126 = L;
    metal::float3 _e127 = V_2;
    H = metal::normalize(_e126 + _e127);
    metal::float3 _e130 = N_1;
    metal::float3 _e131 = L;
    NoL_6 = metal::clamp(metal::dot(_e130, _e131), 0.0, 1.0);
    metal::float3 _e136 = N_1;
    metal::float3 _e137 = H;
    NoH_4 = metal::clamp(metal::dot(_e136, _e137), 0.0, 1.0);
    metal::float3 _e142 = L;
    metal::float3 _e143 = H;
    LoH_6 = metal::clamp(metal::dot(_e142, _e143), 0.0, 1.0);
    metal::float3 _e148 = diffuseColor_1;
    float _e149 = roughness_9;
    float _e150 = NdotV_1;
    float _e151 = NoL_6;
    float _e152 = LoH_6;
    float _e153 = Fd_Burley(_e149, _e150, _e151, _e152);
    diffuse = _e148 * _e153;
    metal::float3 _e156 = diffuse;
    metal::float3 _e157 = specular_1;
    PointLight _e159 = light_1;
    float _e163 = rangeAttenuation;
    float _e164 = NoL_6;
    return ((_e156 + _e157) * _e159.color.xyz) * (_e163 * _e164);
}

metal::float3 dir_light(
    DirectionalLight light_2,
    float roughness_10,
    float NdotV_2,
    metal::float3 normal,
    metal::float3 view,
    metal::float3 R_2,
    metal::float3 F0_2,
    metal::float3 diffuseColor_2
) {
    DirectionalLight light_3 = {};
    float roughness_11 = {};
    float NdotV_3 = {};
    metal::float3 normal_1 = {};
    metal::float3 view_1 = {};
    metal::float3 R_3 = {};
    metal::float3 F0_3 = {};
    metal::float3 diffuseColor_3 = {};
    metal::float3 incident_light = {};
    metal::float3 half_vector = {};
    float NoL_7 = {};
    float NoH_5 = {};
    float LoH_7 = {};
    metal::float3 diffuse_1 = {};
    float specularIntensity_3 = 1.0;
    metal::float3 specular_2 = {};
    light_3 = light_2;
    roughness_11 = roughness_10;
    NdotV_3 = NdotV_2;
    normal_1 = normal;
    view_1 = view;
    R_3 = R_2;
    F0_3 = F0_2;
    diffuseColor_3 = diffuseColor_2;
    DirectionalLight _e16 = light_3;
    incident_light = _e16.direction.xyz;
    metal::float3 _e20 = incident_light;
    metal::float3 _e21 = view_1;
    half_vector = metal::normalize(_e20 + _e21);
    metal::float3 _e25 = normal_1;
    metal::float3 _e26 = incident_light;
    NoL_7 = metal::clamp(metal::dot(_e25, _e26), 0.0, 1.0);
    metal::float3 _e32 = normal_1;
    metal::float3 _e33 = half_vector;
    NoH_5 = metal::clamp(metal::dot(_e32, _e33), 0.0, 1.0);
    metal::float3 _e39 = incident_light;
    metal::float3 _e40 = half_vector;
    LoH_7 = metal::clamp(metal::dot(_e39, _e40), 0.0, 1.0);
    metal::float3 _e46 = diffuseColor_3;
    float _e47 = roughness_11;
    float _e48 = NdotV_3;
    float _e49 = NoL_7;
    float _e50 = LoH_7;
    float _e51 = Fd_Burley(_e47, _e48, _e49, _e50);
    diffuse_1 = _e46 * _e51;
    metal::float3 _e56 = F0_3;
    float _e57 = roughness_11;
    metal::float3 _e58 = half_vector;
    float _e59 = NdotV_3;
    float _e60 = NoL_7;
    float _e61 = NoH_5;
    float _e62 = LoH_7;
    float _e63 = specularIntensity_3;
    metal::float3 _e64 = specular(_e56, _e57, _e58, _e59, _e60, _e61, _e62, _e63);
    specular_2 = _e64;
    metal::float3 _e66 = specular_2;
    metal::float3 _e67 = diffuse_1;
    DirectionalLight _e69 = light_3;
    float _e73 = NoL_7;
    return ((_e66 + _e67) * _e69.color.xyz) * _e73;
}

void main_1(
    thread metal::float3& v_WorldPosition_1,
    thread metal::float3& v_WorldNormal_1,
    thread metal::float2& v_Uv_1,
    thread metal::float4& v_WorldTangent_1,
    thread metal::float4& o_Target,
    constant CameraPosition& global,
    constant Lights& global_1,
    constant StandardMaterial_base_color& global_2,
    metal::texture2d<float, metal::access::sample> StandardMaterial_base_color_texture,
    metal::sampler StandardMaterial_base_color_texture_sampler,
    constant StandardMaterial_roughness& global_3,
    constant StandardMaterial_metallic& global_4,
    metal::texture2d<float, metal::access::sample> StandardMaterial_metallic_roughness_texture,
    metal::sampler StandardMaterial_metallic_roughness_texture_sampler,
    constant StandardMaterial_reflectance& global_5,
    metal::texture2d<float, metal::access::sample> StandardMaterial_normal_map,
    metal::sampler StandardMaterial_normal_map_sampler,
    metal::texture2d<float, metal::access::sample> StandardMaterial_occlusion_texture,
    metal::sampler StandardMaterial_occlusion_texture_sampler,
    constant StandardMaterial_emissive& global_6,
    metal::texture2d<float, metal::access::sample> StandardMaterial_emissive_texture,
    metal::sampler StandardMaterial_emissive_texture_sampler,
    thread bool& gl_FrontFacing_1
) {
    metal::float4 output_color = {};
    metal::float4 metallic_roughness = {};
    float metallic = {};
    float perceptual_roughness_2 = {};
    float roughness_12 = {};
    metal::float3 N_2 = {};
    metal::float3 T = {};
    metal::float3 B = {};
    metal::float3 local = {};
    metal::float3 local_1 = {};
    metal::float3 local_2 = {};
    metal::float3x3 TBN = {};
    float occlusion = {};
    metal::float4 emissive = {};
    metal::float3 V_3 = {};
    float NdotV_4 = {};
    metal::float3 F0_4 = {};
    metal::float3 diffuseColor_4 = {};
    metal::float3 R_4 = {};
    metal::float3 light_accum = metal::float3(0.0);
    int i = 0;
    int i_1 = 0;
    metal::float3 diffuse_ambient = {};
    metal::float3 specular_ambient = {};
    metal::float4 _e37 = global_2.base_color;
    output_color = _e37;
    metal::float4 _e39 = output_color;
    metal::float2 _e40 = v_Uv_1;
    metal::float4 _e41 = StandardMaterial_base_color_texture.sample(StandardMaterial_base_color_texture_sampler, _e40);
    output_color = _e39 * _e41;
    metal::float2 _e43 = v_Uv_1;
    metal::float4 _e44 = StandardMaterial_metallic_roughness_texture.sample(StandardMaterial_metallic_roughness_texture_sampler, _e43);
    metallic_roughness = _e44;
    float _e46 = global_4.metallic;
    metal::float4 _e47 = metallic_roughness;
    metallic = _e46 * _e47.z;
    float _e51 = global_3.perceptual_roughness;
    metal::float4 _e52 = metallic_roughness;
    perceptual_roughness_2 = _e51 * _e52.y;
    float _e56 = perceptual_roughness_2;
    float _e57 = perceptualRoughnessToRoughness(_e56);
    roughness_12 = _e57;
    metal::float3 _e59 = v_WorldNormal_1;
    N_2 = metal::normalize(_e59);
    metal::float4 _e62 = v_WorldTangent_1;
    T = metal::normalize(_e62.xyz);
    metal::float3 _e66 = N_2;
    metal::float3 _e67 = T;
    metal::float4 _e69 = v_WorldTangent_1;
    B = metal::cross(_e66, _e67) * _e69.w;
    bool _e74 = gl_FrontFacing_1;
    if (_e74) {
        metal::float3 _e75 = N_2;
        local = _e75;
    } else {
        metal::float3 _e76 = N_2;
        local = -(_e76);
    }
    metal::float3 _e79 = local;
    N_2 = _e79;
    bool _e80 = gl_FrontFacing_1;
    if (_e80) {
        metal::float3 _e81 = T;
        local_1 = _e81;
    } else {
        metal::float3 _e82 = T;
        local_1 = -(_e82);
    }
    metal::float3 _e85 = local_1;
    T = _e85;
    bool _e86 = gl_FrontFacing_1;
    if (_e86) {
        metal::float3 _e87 = B;
        local_2 = _e87;
    } else {
        metal::float3 _e88 = B;
        local_2 = -(_e88);
    }
    metal::float3 _e91 = local_2;
    B = _e91;
    metal::float3 _e92 = T;
    metal::float3 _e93 = B;
    metal::float3 _e94 = N_2;
    TBN = metal::float3x3(metal::float3(_e92.x, _e92.y, _e92.z), metal::float3(_e93.x, _e93.y, _e93.z), metal::float3(_e94.x, _e94.y, _e94.z));
    metal::float3x3 _e109 = TBN;
    metal::float2 _e110 = v_Uv_1;
    metal::float4 _e111 = StandardMaterial_normal_map.sample(StandardMaterial_normal_map_sampler, _e110);
    N_2 = _e109 * metal::normalize((_e111.xyz * 2.0) - metal::float3(1.0));
    metal::float2 _e120 = v_Uv_1;
    metal::float4 _e121 = StandardMaterial_occlusion_texture.sample(StandardMaterial_occlusion_texture_sampler, _e120);
    occlusion = _e121.x;
    metal::float4 _e124 = global_6.emissive;
    emissive = _e124;
    metal::float4 _e126 = emissive;
    metal::float2 _e128 = v_Uv_1;
    metal::float4 _e129 = StandardMaterial_emissive_texture.sample(StandardMaterial_emissive_texture_sampler, _e128);
    metal::float3 _e131 = _e126.xyz * _e129.xyz;
    emissive.x = _e131.x;
    emissive.y = _e131.y;
    emissive.z = _e131.z;
    metal::float4 _e138 = global.CameraPos;
    metal::float3 _e140 = v_WorldPosition_1;
    V_3 = metal::normalize(_e138.xyz - _e140.xyz);
    metal::float3 _e145 = N_2;
    metal::float3 _e146 = V_3;
    NdotV_4 = metal::max(metal::dot(_e145, _e146), 0.001);
    float _e152 = global_5.reflectance;
    float _e154 = global_5.reflectance;
    float _e157 = metallic;
    metal::float4 _e161 = output_color;
    float _e163 = metallic;
    F0_4 = metal::float3(((0.16 * _e152) * _e154) * (1.0 - _e157)) + (_e161.xyz * metal::float3(_e163));
    metal::float4 _e168 = output_color;
    float _e171 = metallic;
    diffuseColor_4 = _e168.xyz * metal::float3(1.0 - _e171);
    metal::float3 _e176 = V_3;
    metal::float3 _e178 = N_2;
    R_4 = metal::reflect(-(_e176), _e178);
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e195 = i;
            i = as_type<int>(as_type<uint>(_e195) + as_type<uint>(1));
        }
        loop_init = false;
        int _e186 = i;
        metal::uint4 _e187 = global_1.NumLights;
        int _e191 = i;
        if (!(((_e186 < static_cast<int>(_e187.x)) && (_e191 < MAX_POINT_LIGHTS)))) {
            break;
        }
        {
            metal::float3 _e198 = light_accum;
            int _e199 = i;
            PointLight _e201 = global_1.PointLights.inner[_e199];
            float _e202 = roughness_12;
            float _e203 = NdotV_4;
            metal::float3 _e204 = N_2;
            metal::float3 _e205 = V_3;
            metal::float3 _e206 = R_4;
            metal::float3 _e207 = F0_4;
            metal::float3 _e208 = diffuseColor_4;
            metal::float3 _e209 = point_light(_e201, _e202, _e203, _e204, _e205, _e206, _e207, _e208, v_WorldPosition_1);
            light_accum = _e198 + _e209;
        }
    }
    uint2 loop_bound_1 = uint2(4294967295u);
    bool loop_init_1 = true;
    while(true) {
        if (metal::all(loop_bound_1 == uint2(0u))) { break; }
        loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
        if (!loop_init_1) {
            int _e222 = i_1;
            i_1 = as_type<int>(as_type<uint>(_e222) + as_type<uint>(1));
        }
        loop_init_1 = false;
        int _e213 = i_1;
        metal::uint4 _e214 = global_1.NumLights;
        int _e218 = i_1;
        if (!(((_e213 < static_cast<int>(_e214.y)) && (_e218 < MAX_DIRECTIONAL_LIGHTS)))) {
            break;
        }
        {
            metal::float3 _e225 = light_accum;
            int _e226 = i_1;
            DirectionalLight _e228 = global_1.DirectionalLights.inner[_e226];
            float _e229 = roughness_12;
            float _e230 = NdotV_4;
            metal::float3 _e231 = N_2;
            metal::float3 _e232 = V_3;
            metal::float3 _e233 = R_4;
            metal::float3 _e234 = F0_4;
            metal::float3 _e235 = diffuseColor_4;
            metal::float3 _e236 = dir_light(_e228, _e229, _e230, _e231, _e232, _e233, _e234, _e235);
            light_accum = _e225 + _e236;
        }
    }
    metal::float3 _e238 = diffuseColor_4;
    float _e240 = NdotV_4;
    metal::float3 _e241 = EnvBRDFApprox(_e238, 1.0, _e240);
    diffuse_ambient = _e241;
    metal::float3 _e243 = F0_4;
    float _e244 = perceptual_roughness_2;
    float _e245 = NdotV_4;
    metal::float3 _e246 = EnvBRDFApprox(_e243, _e244, _e245);
    specular_ambient = _e246;
    metal::float3 _e248 = light_accum;
    output_color.x = _e248.x;
    output_color.y = _e248.y;
    output_color.z = _e248.z;
    metal::float4 _e255 = output_color;
    metal::float3 _e257 = diffuse_ambient;
    metal::float3 _e258 = specular_ambient;
    metal::float4 _e260 = global_1.AmbientColor;
    float _e263 = occlusion;
    metal::float3 _e265 = _e255.xyz + (((_e257 + _e258) * _e260.xyz) * _e263);
    output_color.x = _e265.x;
    output_color.y = _e265.y;
    output_color.z = _e265.z;
    metal::float4 _e272 = output_color;
    metal::float4 _e274 = emissive;
    metal::float4 _e276 = output_color;
    metal::float3 _e279 = _e272.xyz + (_e274.xyz * _e276.w);
    output_color.x = _e279.x;
    output_color.y = _e279.y;
    output_color.z = _e279.z;
    metal::float4 _e286 = output_color;
    metal::float3 _e288 = reinhard_luminance(_e286.xyz);
    output_color.x = _e288.x;
    output_color.y = _e288.y;
    output_color.z = _e288.z;
    metal::float4 _e295 = output_color;
    o_Target = _e295;
    return;
}

struct main_Input {
    metal::float3 v_WorldPosition [[user(loc0), center_perspective]];
    metal::float3 v_WorldNormal [[user(loc1), center_perspective]];
    metal::float2 v_Uv [[user(loc2), center_perspective]];
    metal::float4 v_WorldTangent [[user(loc3), center_perspective]];
};
struct main_Output {
    metal::float4 o_Target [[color(0)]];
};
fragment main_Output main_(
  main_Input varyings [[stage_in]]
, bool gl_FrontFacing [[front_facing]]
, constant CameraPosition& global [[user(fake0)]]
, constant Lights& global_1 [[user(fake0)]]
, constant StandardMaterial_base_color& global_2 [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> StandardMaterial_base_color_texture [[user(fake0)]]
, metal::sampler StandardMaterial_base_color_texture_sampler [[user(fake0)]]
, constant StandardMaterial_roughness& global_3 [[user(fake0)]]
, constant StandardMaterial_metallic& global_4 [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> StandardMaterial_metallic_roughness_texture [[user(fake0)]]
, metal::sampler StandardMaterial_metallic_roughness_texture_sampler [[user(fake0)]]
, constant StandardMaterial_reflectance& global_5 [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> StandardMaterial_normal_map [[user(fake0)]]
, metal::sampler StandardMaterial_normal_map_sampler [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> StandardMaterial_occlusion_texture [[user(fake0)]]
, metal::sampler StandardMaterial_occlusion_texture_sampler [[user(fake0)]]
, constant StandardMaterial_emissive& global_6 [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> StandardMaterial_emissive_texture [[user(fake0)]]
, metal::sampler StandardMaterial_emissive_texture_sampler [[user(fake0)]]
) {
    metal::float3 v_WorldPosition_1 = {};
    metal::float3 v_WorldNormal_1 = {};
    metal::float2 v_Uv_1 = {};
    metal::float4 v_WorldTangent_1 = {};
    metal::float4 o_Target = {};
    bool gl_FrontFacing_1 = {};
    const auto v_WorldPosition = varyings.v_WorldPosition;
    const auto v_WorldNormal = varyings.v_WorldNormal;
    const auto v_Uv = varyings.v_Uv;
    const auto v_WorldTangent = varyings.v_WorldTangent;
    v_WorldPosition_1 = v_WorldPosition;
    v_WorldNormal_1 = v_WorldNormal;
    v_Uv_1 = v_Uv;
    v_WorldTangent_1 = v_WorldTangent;
    gl_FrontFacing_1 = gl_FrontFacing;
    main_1(v_WorldPosition_1, v_WorldNormal_1, v_Uv_1, v_WorldTangent_1, o_Target, global, global_1, global_2, StandardMaterial_base_color_texture, StandardMaterial_base_color_texture_sampler, global_3, global_4, StandardMaterial_metallic_roughness_texture, StandardMaterial_metallic_roughness_texture_sampler, global_5, StandardMaterial_normal_map, StandardMaterial_normal_map_sampler, StandardMaterial_occlusion_texture, StandardMaterial_occlusion_texture_sampler, global_6, StandardMaterial_emissive_texture, StandardMaterial_emissive_texture_sampler, gl_FrontFacing_1);
    metal::float4 _e11 = o_Target;
    const auto _tmp = FragmentOutput {_e11};
    return main_Output { _tmp.o_Target };
}
