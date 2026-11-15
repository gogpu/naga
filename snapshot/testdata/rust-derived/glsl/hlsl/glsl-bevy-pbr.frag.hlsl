struct PointLight {
    float4 pos;
    float4 color;
    float4 lightParams;
};

struct DirectionalLight {
    float4 direction;
    float4 color;
};

struct CameraPosition {
    float4 CameraPos;
};

struct Lights {
    float4 AmbientColor;
    uint4 NumLights;
    PointLight PointLights[10];
    DirectionalLight DirectionalLights[1];
};

struct StandardMaterial_base_color {
    float4 base_color;
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
    float4 emissive;
};

struct FragmentOutput {
    float4 o_Target : SV_Target0;
};

static const int MAX_POINT_LIGHTS = int(10);
static const int MAX_DIRECTIONAL_LIGHTS = int(1);
static const float PI = 3.1415927;

static float3 v_WorldPosition_1 = (float3)0;
static float3 v_WorldNormal_1 = (float3)0;
static float2 v_Uv_1 = (float2)0;
static float4 v_WorldTangent_1 = (float4)0;
static float4 o_Target = (float4)0;
cbuffer global : register(b1) { CameraPosition global; }
cbuffer global_1 : register(b0, space1) { Lights global_1; }
cbuffer global_2 : register(b0, space3) { StandardMaterial_base_color global_2; }
Texture2D<float4> StandardMaterial_base_color_texture : register(t1, space3);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup3SamplerIndexArray : register(t3, space255);
static const SamplerState StandardMaterial_base_color_texture_sampler = nagaSamplerHeap[nagaGroup3SamplerIndexArray[2]];
cbuffer global_3 : register(b3, space3) { StandardMaterial_roughness global_3; }
cbuffer global_4 : register(b4, space3) { StandardMaterial_metallic global_4; }
Texture2D<float4> StandardMaterial_metallic_roughness_texture : register(t5, space3);
static const SamplerState StandardMaterial_metallic_roughness_texture_sampler = nagaSamplerHeap[nagaGroup3SamplerIndexArray[6]];
cbuffer global_5 : register(b7, space3) { StandardMaterial_reflectance global_5; }
Texture2D<float4> StandardMaterial_normal_map : register(t8, space3);
static const SamplerState StandardMaterial_normal_map_sampler = nagaSamplerHeap[nagaGroup3SamplerIndexArray[9]];
Texture2D<float4> StandardMaterial_occlusion_texture : register(t10, space3);
static const SamplerState StandardMaterial_occlusion_texture_sampler = nagaSamplerHeap[nagaGroup3SamplerIndexArray[11]];
cbuffer global_6 : register(b12, space3) { StandardMaterial_emissive global_6; }
Texture2D<float4> StandardMaterial_emissive_texture : register(t13, space3);
static const SamplerState StandardMaterial_emissive_texture_sampler = nagaSamplerHeap[nagaGroup3SamplerIndexArray[14]];
static bool gl_FrontFacing_1 = (bool)0;

struct FragmentInput_main {
    float3 v_WorldPosition_2 : LOC0;
    float3 v_WorldNormal_2 : LOC1;
    float2 v_Uv_2 : LOC2;
    float4 v_WorldTangent_2 : LOC3;
    bool gl_FrontFacing_2 : SV_IsFrontFace;
};

float pow5_(float x)
{
    float x_1 = (float)0;
    float x2_ = (float)0;

    x_1 = x;
    float _e2 = x_1;
    float _e3 = x_1;
    x2_ = (_e2 * _e3);
    float _e6 = x2_;
    float _e7 = x2_;
    float _e9 = x_1;
    return ((_e6 * _e7) * _e9);
}

float getDistanceAttenuation(float distanceSquare, float inverseRangeSquared)
{
    float distanceSquare_1 = (float)0;
    float inverseRangeSquared_1 = (float)0;
    float factor = (float)0;
    float smoothFactor = (float)0;
    float attenuation = (float)0;

    distanceSquare_1 = distanceSquare;
    inverseRangeSquared_1 = inverseRangeSquared;
    float _e4 = distanceSquare_1;
    float _e5 = inverseRangeSquared_1;
    factor = (_e4 * _e5);
    float _e9 = factor;
    float _e10 = factor;
    smoothFactor = clamp((1.0 - (_e9 * _e10)), 0.0, 1.0);
    float _e17 = smoothFactor;
    float _e18 = smoothFactor;
    attenuation = (_e17 * _e18);
    float _e21 = attenuation;
    float _e24 = distanceSquare_1;
    return ((_e21 * 1.0) / max(_e24, 0.001));
}

float D_GGX(float roughness, float NoH, float3 h)
{
    float roughness_1 = (float)0;
    float NoH_1 = (float)0;
    float oneMinusNoHSquared = (float)0;
    float a = (float)0;
    float k = (float)0;
    float d = (float)0;

    roughness_1 = roughness;
    NoH_1 = NoH;
    float _e5 = NoH_1;
    float _e6 = NoH_1;
    oneMinusNoHSquared = (1.0 - (_e5 * _e6));
    float _e10 = NoH_1;
    float _e11 = roughness_1;
    a = (_e10 * _e11);
    float _e14 = roughness_1;
    float _e15 = oneMinusNoHSquared;
    float _e16 = a;
    float _e17 = a;
    k = (_e14 / (_e15 + (_e16 * _e17)));
    float _e22 = k;
    float _e23 = k;
    d = ((_e22 * _e23) * 0.31830987);
    float _e28 = d;
    return _e28;
}

float V_SmithGGXCorrelated(float roughness_2, float NoV, float NoL)
{
    float roughness_3 = (float)0;
    float NoV_1 = (float)0;
    float NoL_1 = (float)0;
    float a2_ = (float)0;
    float lambdaV = (float)0;
    float lambdaL = (float)0;
    float v = (float)0;

    roughness_3 = roughness_2;
    NoV_1 = NoV;
    NoL_1 = NoL;
    float _e6 = roughness_3;
    float _e7 = roughness_3;
    a2_ = (_e6 * _e7);
    float _e10 = NoL_1;
    float _e11 = NoV_1;
    float _e12 = a2_;
    float _e13 = NoV_1;
    float _e16 = NoV_1;
    float _e18 = a2_;
    lambdaV = (_e10 * sqrt((((_e11 - (_e12 * _e13)) * _e16) + _e18)));
    float _e23 = NoV_1;
    float _e24 = NoL_1;
    float _e25 = a2_;
    float _e26 = NoL_1;
    float _e29 = NoL_1;
    float _e31 = a2_;
    lambdaL = (_e23 * sqrt((((_e24 - (_e25 * _e26)) * _e29) + _e31)));
    float _e37 = lambdaV;
    float _e38 = lambdaL;
    v = (0.5 / (_e37 + _e38));
    float _e42 = v;
    return _e42;
}

float3 F_Schlick(float3 f0_, float f90_, float VoH)
{
    float f90_1 = (float)0;
    float VoH_1 = (float)0;

    f90_1 = f90_;
    VoH_1 = VoH;
    float _e5 = f90_1;
    float _e9 = VoH_1;
    const float _e11 = pow5_((1.0 - _e9));
    return (f0_ + (((_e5).xxx - f0_) * _e11));
}

float F_Schlick_1(float f0_1, float f90_2, float VoH_2)
{
    float f0_2 = (float)0;
    float f90_3 = (float)0;
    float VoH_3 = (float)0;

    f0_2 = f0_1;
    f90_3 = f90_2;
    VoH_3 = VoH_2;
    float _e6 = f0_2;
    float _e7 = f90_3;
    float _e8 = f0_2;
    float _e11 = VoH_3;
    const float _e13 = pow5_((1.0 - _e11));
    return (_e6 + ((_e7 - _e8) * _e13));
}

float3 fresnel(float3 f0_3, float LoH)
{
    float3 f0_4 = (float3)0;
    float LoH_1 = (float)0;
    float f90_4 = (float)0;

    f0_4 = f0_3;
    LoH_1 = LoH;
    float3 _e4 = f0_4;
    f90_4 = clamp(dot(_e4, (16.5).xxx), 0.0, 1.0);
    float3 _e12 = f0_4;
    float _e13 = f90_4;
    float _e14 = LoH_1;
    const float3 _e15 = F_Schlick(_e12, _e13, _e14);
    return _e15;
}

float3 specular(float3 f0_5, float roughness_4, float3 h_1, float NoV_2, float NoL_2, float NoH_2, float LoH_2, float specularIntensity)
{
    float3 f0_6 = (float3)0;
    float roughness_5 = (float)0;
    float NoV_3 = (float)0;
    float NoL_3 = (float)0;
    float NoH_3 = (float)0;
    float LoH_3 = (float)0;
    float specularIntensity_1 = (float)0;
    float D = (float)0;
    float V = (float)0;
    float3 F = (float3)0;

    f0_6 = f0_5;
    roughness_5 = roughness_4;
    NoV_3 = NoV_2;
    NoL_3 = NoL_2;
    NoH_3 = NoH_2;
    LoH_3 = LoH_2;
    specularIntensity_1 = specularIntensity;
    float _e15 = roughness_5;
    float _e16 = NoH_3;
    const float _e17 = D_GGX(_e15, _e16, h_1);
    D = _e17;
    float _e19 = roughness_5;
    float _e20 = NoV_3;
    float _e21 = NoL_3;
    const float _e22 = V_SmithGGXCorrelated(_e19, _e20, _e21);
    V = _e22;
    float3 _e24 = f0_6;
    float _e25 = LoH_3;
    const float3 _e26 = fresnel(_e24, _e25);
    F = _e26;
    float _e28 = specularIntensity_1;
    float _e29 = D;
    float _e31 = V;
    float3 _e33 = F;
    return (((_e28 * _e29) * _e31) * _e33);
}

float Fd_Burley(float roughness_6, float NoV_4, float NoL_4, float LoH_4)
{
    float roughness_7 = (float)0;
    float NoV_5 = (float)0;
    float NoL_5 = (float)0;
    float LoH_5 = (float)0;
    float f90_5 = (float)0;
    float lightScatter = (float)0;
    float viewScatter = (float)0;

    roughness_7 = roughness_6;
    NoV_5 = NoV_4;
    NoL_5 = NoL_4;
    LoH_5 = LoH_4;
    float _e10 = roughness_7;
    float _e12 = LoH_5;
    float _e14 = LoH_5;
    f90_5 = (0.5 + (((2.0 * _e10) * _e12) * _e14));
    float _e19 = f90_5;
    float _e20 = NoL_5;
    const float _e21 = F_Schlick_1(1.0, _e19, _e20);
    lightScatter = _e21;
    float _e24 = f90_5;
    float _e25 = NoV_5;
    const float _e26 = F_Schlick_1(1.0, _e24, _e25);
    viewScatter = _e26;
    float _e28 = lightScatter;
    float _e29 = viewScatter;
    return ((_e28 * _e29) * 0.31830987);
}

float3 EnvBRDFApprox(float3 f0_7, float perceptual_roughness, float NoV_6)
{
    float3 f0_8 = (float3)0;
    float perceptual_roughness_1 = (float)0;
    float NoV_7 = (float)0;
    float4 c0_ = float4(-1.0, -0.0275, -0.572, 0.022);
    float4 c1_ = float4(1.0, 0.0425, 1.04, -0.04);
    float4 r = (float4)0;
    float a004_ = (float)0;
    float2 AB = (float2)0;

    f0_8 = f0_7;
    perceptual_roughness_1 = perceptual_roughness;
    NoV_7 = NoV_6;
    float _e18 = perceptual_roughness_1;
    float4 _e20 = c0_;
    float4 _e22 = c1_;
    r = (((_e18).xxxx * _e20) + _e22);
    float4 _e25 = r;
    float4 _e27 = r;
    float _e31 = NoV_7;
    float4 _e35 = r;
    float4 _e38 = r;
    a004_ = ((min((_e25.x * _e27.x), exp2((-9.28 * _e31))) * _e35.x) + _e38.y);
    float _e45 = a004_;
    float4 _e48 = r;
    AB = ((float2(-1.04, 1.04) * (_e45).xx) + _e48.zw);
    float3 _e52 = f0_8;
    float2 _e53 = AB;
    float2 _e57 = AB;
    return ((_e52 * (_e53.x).xxx) + (_e57.y).xxx);
}

float perceptualRoughnessToRoughness(float perceptualRoughness)
{
    float perceptualRoughness_1 = (float)0;
    float clampedPerceptualRoughness = (float)0;

    perceptualRoughness_1 = perceptualRoughness;
    float _e2 = perceptualRoughness_1;
    clampedPerceptualRoughness = clamp(_e2, 0.089, 1.0);
    float _e7 = clampedPerceptualRoughness;
    float _e8 = clampedPerceptualRoughness;
    return (_e7 * _e8);
}

float luminance(float3 v_1)
{
    float3 v_2 = (float3)0;

    v_2 = v_1;
    float3 _e2 = v_2;
    return dot(_e2, float3(0.2126, 0.7152, 0.0722));
}

float3 change_luminance(float3 c_in, float l_out)
{
    float3 c_in_1 = (float3)0;
    float l_out_1 = (float)0;
    float l_in = (float)0;

    c_in_1 = c_in;
    l_out_1 = l_out;
    float3 _e4 = c_in_1;
    const float _e5 = luminance(_e4);
    l_in = _e5;
    float3 _e7 = c_in_1;
    float _e8 = l_out_1;
    float _e9 = l_in;
    return (_e7 * (_e8 / _e9));
}

float3 reinhard_luminance(float3 color)
{
    float3 color_1 = (float3)0;
    float l_old = (float)0;
    float l_new = (float)0;

    color_1 = color;
    float3 _e2 = color_1;
    const float _e3 = luminance(_e2);
    l_old = _e3;
    float _e5 = l_old;
    float _e7 = l_old;
    l_new = (_e5 / (1.0 + _e7));
    float3 _e11 = color_1;
    float _e12 = l_new;
    const float3 _e13 = change_luminance(_e11, _e12);
    return _e13;
}

float3 point_light(PointLight light, float roughness_8, float NdotV, float3 N, float3 V_1, float3 R, float3 F0_, float3 diffuseColor)
{
    PointLight light_1 = (PointLight)0;
    float roughness_9 = (float)0;
    float NdotV_1 = (float)0;
    float3 N_1 = (float3)0;
    float3 V_2 = (float3)0;
    float3 R_1 = (float3)0;
    float3 F0_1 = (float3)0;
    float3 diffuseColor_1 = (float3)0;
    float3 light_to_frag = (float3)0;
    float distance_square = (float)0;
    float rangeAttenuation = (float)0;
    float a_1 = (float)0;
    float radius = (float)0;
    float3 centerToRay = (float3)0;
    float3 closestPoint = (float3)0;
    float LspecLengthInverse = (float)0;
    float normalizationFactor = (float)0;
    float specularIntensity_2 = (float)0;
    float3 L = (float3)0;
    float3 H = (float3)0;
    float NoL_6 = (float)0;
    float NoH_4 = (float)0;
    float LoH_6 = (float)0;
    float3 specular_1 = (float3)0;
    float3 diffuse = (float3)0;

    light_1 = light;
    roughness_9 = roughness_8;
    NdotV_1 = NdotV;
    N_1 = N;
    V_2 = V_1;
    R_1 = R;
    F0_1 = F0_;
    diffuseColor_1 = diffuseColor;
    PointLight _e17 = light_1;
    float3 _e20 = v_WorldPosition_1;
    light_to_frag = (_e17.pos.xyz - _e20.xyz);
    float3 _e24 = light_to_frag;
    float3 _e25 = light_to_frag;
    distance_square = dot(_e24, _e25);
    float _e28 = distance_square;
    PointLight _e29 = light_1;
    const float _e32 = getDistanceAttenuation(_e28, _e29.lightParams.x);
    rangeAttenuation = _e32;
    float _e34 = roughness_9;
    a_1 = _e34;
    PointLight _e36 = light_1;
    radius = _e36.lightParams.y;
    float3 _e40 = light_to_frag;
    float3 _e41 = R_1;
    float3 _e43 = R_1;
    float3 _e45 = light_to_frag;
    centerToRay = ((dot(_e40, _e41) * _e43) - _e45);
    float3 _e48 = light_to_frag;
    float3 _e49 = centerToRay;
    float _e50 = radius;
    float3 _e51 = centerToRay;
    float3 _e52 = centerToRay;
    closestPoint = (_e48 + (_e49 * clamp((_e50 * rsqrt(dot(_e51, _e52))), 0.0, 1.0)));
    float3 _e62 = closestPoint;
    float3 _e63 = closestPoint;
    LspecLengthInverse = rsqrt(dot(_e62, _e63));
    float _e67 = a_1;
    float _e68 = a_1;
    float _e69 = radius;
    float _e72 = LspecLengthInverse;
    normalizationFactor = (_e67 / clamp((_e68 + ((_e69 * 0.5) * _e72)), 0.0, 1.0));
    float _e80 = normalizationFactor;
    float _e81 = normalizationFactor;
    specularIntensity_2 = (_e80 * _e81);
    float3 _e84 = closestPoint;
    float _e85 = LspecLengthInverse;
    L = (_e84 * _e85);
    float3 _e88 = L;
    float3 _e89 = V_2;
    H = normalize((_e88 + _e89));
    float3 _e93 = N_1;
    float3 _e94 = L;
    NoL_6 = clamp(dot(_e93, _e94), 0.0, 1.0);
    float3 _e100 = N_1;
    float3 _e101 = H;
    NoH_4 = clamp(dot(_e100, _e101), 0.0, 1.0);
    float3 _e107 = L;
    float3 _e108 = H;
    LoH_6 = clamp(dot(_e107, _e108), 0.0, 1.0);
    float3 _e114 = F0_1;
    float _e115 = roughness_9;
    float3 _e116 = H;
    float _e117 = NdotV_1;
    float _e118 = NoL_6;
    float _e119 = NoH_4;
    float _e120 = LoH_6;
    float _e121 = specularIntensity_2;
    const float3 _e122 = specular(_e114, _e115, _e116, _e117, _e118, _e119, _e120, _e121);
    specular_1 = _e122;
    float3 _e124 = light_to_frag;
    L = normalize(_e124);
    float3 _e126 = L;
    float3 _e127 = V_2;
    H = normalize((_e126 + _e127));
    float3 _e130 = N_1;
    float3 _e131 = L;
    NoL_6 = clamp(dot(_e130, _e131), 0.0, 1.0);
    float3 _e136 = N_1;
    float3 _e137 = H;
    NoH_4 = clamp(dot(_e136, _e137), 0.0, 1.0);
    float3 _e142 = L;
    float3 _e143 = H;
    LoH_6 = clamp(dot(_e142, _e143), 0.0, 1.0);
    float3 _e148 = diffuseColor_1;
    float _e149 = roughness_9;
    float _e150 = NdotV_1;
    float _e151 = NoL_6;
    float _e152 = LoH_6;
    const float _e153 = Fd_Burley(_e149, _e150, _e151, _e152);
    diffuse = (_e148 * _e153);
    float3 _e156 = diffuse;
    float3 _e157 = specular_1;
    PointLight _e159 = light_1;
    float _e163 = rangeAttenuation;
    float _e164 = NoL_6;
    return (((_e156 + _e157) * _e159.color.xyz) * (_e163 * _e164));
}

float3 dir_light(DirectionalLight light_2, float roughness_10, float NdotV_2, float3 normal, float3 view, float3 R_2, float3 F0_2, float3 diffuseColor_2)
{
    DirectionalLight light_3 = (DirectionalLight)0;
    float roughness_11 = (float)0;
    float NdotV_3 = (float)0;
    float3 normal_1 = (float3)0;
    float3 view_1 = (float3)0;
    float3 R_3 = (float3)0;
    float3 F0_3 = (float3)0;
    float3 diffuseColor_3 = (float3)0;
    float3 incident_light = (float3)0;
    float3 half_vector = (float3)0;
    float NoL_7 = (float)0;
    float NoH_5 = (float)0;
    float LoH_7 = (float)0;
    float3 diffuse_1 = (float3)0;
    float specularIntensity_3 = 1.0;
    float3 specular_2 = (float3)0;

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
    float3 _e20 = incident_light;
    float3 _e21 = view_1;
    half_vector = normalize((_e20 + _e21));
    float3 _e25 = normal_1;
    float3 _e26 = incident_light;
    NoL_7 = clamp(dot(_e25, _e26), 0.0, 1.0);
    float3 _e32 = normal_1;
    float3 _e33 = half_vector;
    NoH_5 = clamp(dot(_e32, _e33), 0.0, 1.0);
    float3 _e39 = incident_light;
    float3 _e40 = half_vector;
    LoH_7 = clamp(dot(_e39, _e40), 0.0, 1.0);
    float3 _e46 = diffuseColor_3;
    float _e47 = roughness_11;
    float _e48 = NdotV_3;
    float _e49 = NoL_7;
    float _e50 = LoH_7;
    const float _e51 = Fd_Burley(_e47, _e48, _e49, _e50);
    diffuse_1 = (_e46 * _e51);
    float3 _e56 = F0_3;
    float _e57 = roughness_11;
    float3 _e58 = half_vector;
    float _e59 = NdotV_3;
    float _e60 = NoL_7;
    float _e61 = NoH_5;
    float _e62 = LoH_7;
    float _e63 = specularIntensity_3;
    const float3 _e64 = specular(_e56, _e57, _e58, _e59, _e60, _e61, _e62, _e63);
    specular_2 = _e64;
    float3 _e66 = specular_2;
    float3 _e67 = diffuse_1;
    DirectionalLight _e69 = light_3;
    float _e73 = NoL_7;
    return (((_e66 + _e67) * _e69.color.xyz) * _e73);
}

void main_1()
{
    float4 output_color = (float4)0;
    float4 metallic_roughness = (float4)0;
    float metallic = (float)0;
    float perceptual_roughness_2 = (float)0;
    float roughness_12 = (float)0;
    float3 N_2 = (float3)0;
    float3 T = (float3)0;
    float3 B = (float3)0;
    float3 local = (float3)0;
    float3 local_1 = (float3)0;
    float3 local_2 = (float3)0;
    float3x3 TBN = (float3x3)0;
    float occlusion = (float)0;
    float4 emissive = (float4)0;
    float3 V_3 = (float3)0;
    float NdotV_4 = (float)0;
    float3 F0_4 = (float3)0;
    float3 diffuseColor_4 = (float3)0;
    float3 R_4 = (float3)0;
    float3 light_accum = (0.0).xxx;
    int i = int(0);
    int i_1 = int(0);
    float3 diffuse_ambient = (float3)0;
    float3 specular_ambient = (float3)0;

    float4 _e37 = global_2.base_color;
    output_color = _e37;
    float4 _e39 = output_color;
    float2 _e40 = v_Uv_1;
    float4 _e41 = StandardMaterial_base_color_texture.Sample(StandardMaterial_base_color_texture_sampler, _e40);
    output_color = (_e39 * _e41);
    float2 _e43 = v_Uv_1;
    float4 _e44 = StandardMaterial_metallic_roughness_texture.Sample(StandardMaterial_metallic_roughness_texture_sampler, _e43);
    metallic_roughness = _e44;
    float _e46 = global_4.metallic;
    float4 _e47 = metallic_roughness;
    metallic = (_e46 * _e47.z);
    float _e51 = global_3.perceptual_roughness;
    float4 _e52 = metallic_roughness;
    perceptual_roughness_2 = (_e51 * _e52.y);
    float _e56 = perceptual_roughness_2;
    const float _e57 = perceptualRoughnessToRoughness(_e56);
    roughness_12 = _e57;
    float3 _e59 = v_WorldNormal_1;
    N_2 = normalize(_e59);
    float4 _e62 = v_WorldTangent_1;
    T = normalize(_e62.xyz);
    float3 _e66 = N_2;
    float3 _e67 = T;
    float4 _e69 = v_WorldTangent_1;
    B = (cross(_e66, _e67) * _e69.w);
    bool _e74 = gl_FrontFacing_1;
    if (_e74) {
        float3 _e75 = N_2;
        local = _e75;
    } else {
        float3 _e76 = N_2;
        local = -(_e76);
    }
    float3 _e79 = local;
    N_2 = _e79;
    bool _e80 = gl_FrontFacing_1;
    if (_e80) {
        float3 _e81 = T;
        local_1 = _e81;
    } else {
        float3 _e82 = T;
        local_1 = -(_e82);
    }
    float3 _e85 = local_1;
    T = _e85;
    bool _e86 = gl_FrontFacing_1;
    if (_e86) {
        float3 _e87 = B;
        local_2 = _e87;
    } else {
        float3 _e88 = B;
        local_2 = -(_e88);
    }
    float3 _e91 = local_2;
    B = _e91;
    float3 _e92 = T;
    float3 _e93 = B;
    float3 _e94 = N_2;
    TBN = float3x3(float3(_e92.x, _e92.y, _e92.z), float3(_e93.x, _e93.y, _e93.z), float3(_e94.x, _e94.y, _e94.z));
    float3x3 _e109 = TBN;
    float2 _e110 = v_Uv_1;
    float4 _e111 = StandardMaterial_normal_map.Sample(StandardMaterial_normal_map_sampler, _e110);
    N_2 = mul(normalize(((_e111.xyz * 2.0) - (1.0).xxx)), _e109);
    float2 _e120 = v_Uv_1;
    float4 _e121 = StandardMaterial_occlusion_texture.Sample(StandardMaterial_occlusion_texture_sampler, _e120);
    occlusion = _e121.x;
    float4 _e124 = global_6.emissive;
    emissive = _e124;
    float4 _e126 = emissive;
    float2 _e128 = v_Uv_1;
    float4 _e129 = StandardMaterial_emissive_texture.Sample(StandardMaterial_emissive_texture_sampler, _e128);
    float3 _e131 = (_e126.xyz * _e129.xyz);
    emissive.x = _e131.x;
    emissive.y = _e131.y;
    emissive.z = _e131.z;
    float4 _e138 = global.CameraPos;
    float3 _e140 = v_WorldPosition_1;
    V_3 = normalize((_e138.xyz - _e140.xyz));
    float3 _e145 = N_2;
    float3 _e146 = V_3;
    NdotV_4 = max(dot(_e145, _e146), 0.001);
    float _e152 = global_5.reflectance;
    float _e154 = global_5.reflectance;
    float _e157 = metallic;
    float4 _e161 = output_color;
    float _e163 = metallic;
    F0_4 = (((((0.16 * _e152) * _e154) * (1.0 - _e157))).xxx + (_e161.xyz * (_e163).xxx));
    float4 _e168 = output_color;
    float _e171 = metallic;
    diffuseColor_4 = (_e168.xyz * ((1.0 - _e171)).xxx);
    float3 _e176 = V_3;
    float3 _e178 = N_2;
    R_4 = reflect(-(_e176), _e178);
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e195 = i;
            i = asint(asuint(_e195) + asuint(int(1)));
        }
        loop_init = false;
        int _e186 = i;
        uint4 _e187 = global_1.NumLights;
        int _e191 = i;
        if (!(((_e186 < int(_e187.x)) && (_e191 < MAX_POINT_LIGHTS)))) {
            break;
        }
        {
            float3 _e198 = light_accum;
            int _e199 = i;
            PointLight _e201 = global_1.PointLights[_e199];
            float _e202 = roughness_12;
            float _e203 = NdotV_4;
            float3 _e204 = N_2;
            float3 _e205 = V_3;
            float3 _e206 = R_4;
            float3 _e207 = F0_4;
            float3 _e208 = diffuseColor_4;
            const float3 _e209 = point_light(_e201, _e202, _e203, _e204, _e205, _e206, _e207, _e208);
            light_accum = (_e198 + _e209);
        }
    }
    uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
    bool loop_init_1 = true;
    while(true) {
        if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
        loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
        if (!loop_init_1) {
            int _e222 = i_1;
            i_1 = asint(asuint(_e222) + asuint(int(1)));
        }
        loop_init_1 = false;
        int _e213 = i_1;
        uint4 _e214 = global_1.NumLights;
        int _e218 = i_1;
        if (!(((_e213 < int(_e214.y)) && (_e218 < MAX_DIRECTIONAL_LIGHTS)))) {
            break;
        }
        {
            float3 _e225 = light_accum;
            int _e226 = i_1;
            DirectionalLight _e228 = global_1.DirectionalLights[_e226];
            float _e229 = roughness_12;
            float _e230 = NdotV_4;
            float3 _e231 = N_2;
            float3 _e232 = V_3;
            float3 _e233 = R_4;
            float3 _e234 = F0_4;
            float3 _e235 = diffuseColor_4;
            const float3 _e236 = dir_light(_e228, _e229, _e230, _e231, _e232, _e233, _e234, _e235);
            light_accum = (_e225 + _e236);
        }
    }
    float3 _e238 = diffuseColor_4;
    float _e240 = NdotV_4;
    const float3 _e241 = EnvBRDFApprox(_e238, 1.0, _e240);
    diffuse_ambient = _e241;
    float3 _e243 = F0_4;
    float _e244 = perceptual_roughness_2;
    float _e245 = NdotV_4;
    const float3 _e246 = EnvBRDFApprox(_e243, _e244, _e245);
    specular_ambient = _e246;
    float3 _e248 = light_accum;
    output_color.x = _e248.x;
    output_color.y = _e248.y;
    output_color.z = _e248.z;
    float4 _e255 = output_color;
    float3 _e257 = diffuse_ambient;
    float3 _e258 = specular_ambient;
    float4 _e260 = global_1.AmbientColor;
    float _e263 = occlusion;
    float3 _e265 = (_e255.xyz + (((_e257 + _e258) * _e260.xyz) * _e263));
    output_color.x = _e265.x;
    output_color.y = _e265.y;
    output_color.z = _e265.z;
    float4 _e272 = output_color;
    float4 _e274 = emissive;
    float4 _e276 = output_color;
    float3 _e279 = (_e272.xyz + (_e274.xyz * _e276.w));
    output_color.x = _e279.x;
    output_color.y = _e279.y;
    output_color.z = _e279.z;
    float4 _e286 = output_color;
    const float3 _e288 = reinhard_luminance(_e286.xyz);
    output_color.x = _e288.x;
    output_color.y = _e288.y;
    output_color.z = _e288.z;
    float4 _e295 = output_color;
    o_Target = _e295;
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_Target = arg0;
    return ret;
}

FragmentOutput main(FragmentInput_main fragmentinput_main)
{
    float3 v_WorldPosition = fragmentinput_main.v_WorldPosition_2;
    float3 v_WorldNormal = fragmentinput_main.v_WorldNormal_2;
    float2 v_Uv = fragmentinput_main.v_Uv_2;
    float4 v_WorldTangent = fragmentinput_main.v_WorldTangent_2;
    bool gl_FrontFacing = fragmentinput_main.gl_FrontFacing_2;
    v_WorldPosition_1 = v_WorldPosition;
    v_WorldNormal_1 = v_WorldNormal;
    v_Uv_1 = v_Uv;
    v_WorldTangent_1 = v_WorldTangent;
    gl_FrontFacing_1 = gl_FrontFacing;
    main_1();
    float4 _e11 = o_Target;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e11);
    return fragmentoutput;
}
