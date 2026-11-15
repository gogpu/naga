Texture1D<float4> tex1D_ : register(t0, space1);
Texture2D<float4> tex2D_ : register(t2, space1);
Texture2DArray<float4> tex2DArray : register(t3, space1);
TextureCube<float4> texCube : register(t4, space1);
TextureCubeArray<float4> texCubeArray : register(t5, space1);
Texture3D<float4> tex3D_ : register(t6, space1);
Texture2D<uint4> utex2D : register(t7, space1);
Texture2D<int4> itex2D : register(t8, space1);
SamplerState nagaSamplerHeap[2048]: register(s0, space0);
SamplerComparisonState nagaComparisonSamplerHeap[2048]: register(s0, space1);
StructuredBuffer<uint> nagaGroup2SamplerIndexArray : register(t2, space255);
static const SamplerState samp = nagaSamplerHeap[nagaGroup2SamplerIndexArray[0]];
Texture2D<float> tex2DShadow : register(t12, space1);
Texture2DArray<float> tex2DArrayShadow : register(t13, space1);
TextureCube<float> texCubeShadow : register(t14, space1);
TextureCubeArray<float> texCubeArrayShadow : register(t15, space1);
StructuredBuffer<uint> nagaGroup1SamplerIndexArray : register(t1, space255);
static const SamplerComparisonState sampShadow = nagaComparisonSamplerHeap[nagaGroup1SamplerIndexArray[17]];
Texture2DMS<float4> tex2DMS : register(t18);

int naga_f2i32(float value) {
    return int(clamp(value, -2147483600.0, 2147483500.0));
}

uint NagaMipDimensions1D(Texture1D<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y);
    return ret.x;
}

uint NagaNumLevels1D(Texture1D<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y);
    return ret.y;
}

void testTex1D(float coord)
{
    float coord_1 = (float)0;
    int size1D = (int)0;
    int levels = (int)0;
    float4 c = (float4)0;

    coord_1 = coord;
    size1D = int(NagaMipDimensions1D(tex1D_, int(0)));
    levels = int(NagaNumLevels1D(tex1D_));
    float _e12 = coord_1;
    float4 _e13 = tex1D_.Sample(samp, _e12);
    c = _e13;
    float _e14 = coord_1;
    float4 _e17 = tex1D_.SampleGrad(samp, _e14, 4.0, 4.0);
    c = _e17;
    float _e18 = coord_1;
    float4 _e22 = tex1D_.SampleGrad(samp, _e18, 4.0, 4.0, int(int(5)));
    c = _e22;
    float _e23 = coord_1;
    float4 _e25 = tex1D_.SampleLevel(samp, _e23, 3.0);
    c = _e25;
    float _e26 = coord_1;
    float4 _e29 = tex1D_.SampleLevel(samp, _e26, 3.0, int(int(5)));
    c = _e29;
    float _e30 = coord_1;
    float4 _e32 = tex1D_.Sample(samp, _e30, int(int(5)));
    c = _e32;
    float _e33 = coord_1;
    float2 _e35 = float2(_e33, 6.0);
    float4 _e39 = tex1D_.Sample(samp, (_e35.x / _e35.y));
    c = _e39;
    float _e40 = coord_1;
    float4 _e44 = float4(_e40, 0.0, 0.0, 6.0);
    float4 _e50 = tex1D_.Sample(samp, (_e44.xyz / (_e44.w).xxx).x);
    c = _e50;
    float _e51 = coord_1;
    float2 _e53 = float2(_e51, 6.0);
    float4 _e59 = tex1D_.SampleGrad(samp, (_e53.x / _e53.y), 4.0, 4.0);
    c = _e59;
    float _e60 = coord_1;
    float4 _e64 = float4(_e60, 0.0, 0.0, 6.0);
    float4 _e72 = tex1D_.SampleGrad(samp, (_e64.xyz / (_e64.w).xxx).x, 4.0, 4.0);
    c = _e72;
    float _e73 = coord_1;
    float2 _e75 = float2(_e73, 6.0);
    float4 _e82 = tex1D_.SampleGrad(samp, (_e75.x / _e75.y), 4.0, 4.0, int(int(5)));
    c = _e82;
    float _e83 = coord_1;
    float4 _e87 = float4(_e83, 0.0, 0.0, 6.0);
    float4 _e96 = tex1D_.SampleGrad(samp, (_e87.xyz / (_e87.w).xxx).x, 4.0, 4.0, int(int(5)));
    c = _e96;
    float _e97 = coord_1;
    float2 _e99 = float2(_e97, 6.0);
    float4 _e104 = tex1D_.SampleLevel(samp, (_e99.x / _e99.y), 3.0);
    c = _e104;
    float _e105 = coord_1;
    float4 _e109 = float4(_e105, 0.0, 0.0, 6.0);
    float4 _e116 = tex1D_.SampleLevel(samp, (_e109.xyz / (_e109.w).xxx).x, 3.0);
    c = _e116;
    float _e117 = coord_1;
    float2 _e119 = float2(_e117, 6.0);
    float4 _e125 = tex1D_.SampleLevel(samp, (_e119.x / _e119.y), 3.0, int(int(5)));
    c = _e125;
    float _e126 = coord_1;
    float4 _e130 = float4(_e126, 0.0, 0.0, 6.0);
    float4 _e138 = tex1D_.SampleLevel(samp, (_e130.xyz / (_e130.w).xxx).x, 3.0, int(int(5)));
    c = _e138;
    float _e139 = coord_1;
    float2 _e141 = float2(_e139, 6.0);
    float4 _e146 = tex1D_.Sample(samp, (_e141.x / _e141.y), int(int(5)));
    c = _e146;
    float _e147 = coord_1;
    float4 _e151 = float4(_e147, 0.0, 0.0, 6.0);
    float4 _e158 = tex1D_.Sample(samp, (_e151.xyz / (_e151.w).xxx).x, int(int(5)));
    c = _e158;
    float _e159 = coord_1;
    float4 _e162 = tex1D_.Load(int2(naga_f2i32(_e159), int(3)));
    c = _e162;
    float _e163 = coord_1;
    float4 _e166 = tex1D_.Load(int2(naga_f2i32(_e163), int(3)));
    c = _e166;
    return;
}

int2 naga_f2i32(float2 value) {
    return int2(clamp(value, -2147483600.0, 2147483500.0));
}

uint2 NagaMipDimensions2D(Texture2D<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

uint NagaNumLevels2D(Texture2D<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z);
    return ret.z;
}

void testTex2D(float2 coord_2)
{
    float2 coord_3 = (float2)0;
    int2 size2D = (int2)0;
    int levels_1 = (int)0;
    float4 c_1 = (float4)0;

    coord_3 = coord_2;
    size2D = int2(NagaMipDimensions2D(tex2D_, int(0)));
    levels_1 = int(NagaNumLevels2D(tex2D_));
    float2 _e14 = coord_3;
    float4 _e15 = tex2D_.Sample(samp, _e14);
    c_1 = _e15;
    float2 _e16 = coord_3;
    float4 _e18 = tex2D_.SampleBias(samp, _e16, 2.0);
    c_1 = _e18;
    float2 _e19 = coord_3;
    float4 _e24 = tex2D_.SampleGrad(samp, _e19, (4.0).xx, (4.0).xx);
    c_1 = _e24;
    float2 _e25 = coord_3;
    float4 _e32 = tex2D_.SampleGrad(samp, _e25, (4.0).xx, (4.0).xx, int2((int(5)).xx));
    c_1 = _e32;
    float2 _e33 = coord_3;
    float4 _e35 = tex2D_.SampleLevel(samp, _e33, 3.0);
    c_1 = _e35;
    float2 _e36 = coord_3;
    float4 _e40 = tex2D_.SampleLevel(samp, _e36, 3.0, int2((int(5)).xx));
    c_1 = _e40;
    float2 _e41 = coord_3;
    float4 _e44 = tex2D_.Sample(samp, _e41, int2((int(5)).xx));
    c_1 = _e44;
    float2 _e45 = coord_3;
    float4 _e49 = tex2D_.SampleBias(samp, _e45, 2.0, int2((int(5)).xx));
    c_1 = _e49;
    float2 _e50 = coord_3;
    float3 _e54 = float3(_e50.x, _e50.y, 6.0);
    float4 _e59 = tex2D_.Sample(samp, (_e54.xy / (_e54.z).xx));
    c_1 = _e59;
    float2 _e60 = coord_3;
    float4 _e65 = float4(_e60.x, _e60.y, 0.0, 6.0);
    float4 _e71 = tex2D_.Sample(samp, (_e65.xyz / (_e65.w).xxx).xy);
    c_1 = _e71;
    float2 _e72 = coord_3;
    float3 _e76 = float3(_e72.x, _e72.y, 6.0);
    float4 _e82 = tex2D_.SampleBias(samp, (_e76.xy / (_e76.z).xx), 2.0);
    c_1 = _e82;
    float2 _e83 = coord_3;
    float4 _e88 = float4(_e83.x, _e83.y, 0.0, 6.0);
    float4 _e95 = tex2D_.SampleBias(samp, (_e88.xyz / (_e88.w).xxx).xy, 2.0);
    c_1 = _e95;
    float2 _e96 = coord_3;
    float3 _e100 = float3(_e96.x, _e96.y, 6.0);
    float4 _e109 = tex2D_.SampleGrad(samp, (_e100.xy / (_e100.z).xx), (4.0).xx, (4.0).xx);
    c_1 = _e109;
    float2 _e110 = coord_3;
    float4 _e115 = float4(_e110.x, _e110.y, 0.0, 6.0);
    float4 _e125 = tex2D_.SampleGrad(samp, (_e115.xyz / (_e115.w).xxx).xy, (4.0).xx, (4.0).xx);
    c_1 = _e125;
    float2 _e126 = coord_3;
    float3 _e130 = float3(_e126.x, _e126.y, 6.0);
    float4 _e141 = tex2D_.SampleGrad(samp, (_e130.xy / (_e130.z).xx), (4.0).xx, (4.0).xx, int2((int(5)).xx));
    c_1 = _e141;
    float2 _e142 = coord_3;
    float4 _e147 = float4(_e142.x, _e142.y, 0.0, 6.0);
    float4 _e159 = tex2D_.SampleGrad(samp, (_e147.xyz / (_e147.w).xxx).xy, (4.0).xx, (4.0).xx, int2((int(5)).xx));
    c_1 = _e159;
    float2 _e160 = coord_3;
    float3 _e164 = float3(_e160.x, _e160.y, 6.0);
    float4 _e170 = tex2D_.SampleLevel(samp, (_e164.xy / (_e164.z).xx), 3.0);
    c_1 = _e170;
    float2 _e171 = coord_3;
    float4 _e176 = float4(_e171.x, _e171.y, 0.0, 6.0);
    float4 _e183 = tex2D_.SampleLevel(samp, (_e176.xyz / (_e176.w).xxx).xy, 3.0);
    c_1 = _e183;
    float2 _e184 = coord_3;
    float3 _e188 = float3(_e184.x, _e184.y, 6.0);
    float4 _e196 = tex2D_.SampleLevel(samp, (_e188.xy / (_e188.z).xx), 3.0, int2((int(5)).xx));
    c_1 = _e196;
    float2 _e197 = coord_3;
    float4 _e202 = float4(_e197.x, _e197.y, 0.0, 6.0);
    float4 _e211 = tex2D_.SampleLevel(samp, (_e202.xyz / (_e202.w).xxx).xy, 3.0, int2((int(5)).xx));
    c_1 = _e211;
    float2 _e212 = coord_3;
    float3 _e216 = float3(_e212.x, _e212.y, 6.0);
    float4 _e223 = tex2D_.Sample(samp, (_e216.xy / (_e216.z).xx), int2((int(5)).xx));
    c_1 = _e223;
    float2 _e224 = coord_3;
    float4 _e229 = float4(_e224.x, _e224.y, 0.0, 6.0);
    float4 _e237 = tex2D_.Sample(samp, (_e229.xyz / (_e229.w).xxx).xy, int2((int(5)).xx));
    c_1 = _e237;
    float2 _e238 = coord_3;
    float3 _e242 = float3(_e238.x, _e238.y, 6.0);
    float4 _e250 = tex2D_.SampleBias(samp, (_e242.xy / (_e242.z).xx), 2.0, int2((int(5)).xx));
    c_1 = _e250;
    float2 _e251 = coord_3;
    float4 _e256 = float4(_e251.x, _e251.y, 0.0, 6.0);
    float4 _e265 = tex2D_.SampleBias(samp, (_e256.xyz / (_e256.w).xxx).xy, 2.0, int2((int(5)).xx));
    c_1 = _e265;
    float2 _e266 = coord_3;
    float4 _e269 = tex2D_.Load(int3(naga_f2i32(_e266), int(3)));
    c_1 = _e269;
    float2 _e270 = coord_3;
    uint4 _e273 = utex2D.Load(int3(naga_f2i32(_e270), int(3)));
    c_1 = float4(_e273);
    float2 _e275 = coord_3;
    int4 _e278 = itex2D.Load(int3(naga_f2i32(_e275), int(3)));
    c_1 = float4(_e278);
    float2 _e280 = coord_3;
    float4 _e283 = tex2D_.Load(int3(naga_f2i32(_e280), int(3)));
    c_1 = _e283;
    float2 _e284 = coord_3;
    uint4 _e287 = utex2D.Load(int3(naga_f2i32(_e284), int(3)));
    c_1 = float4(_e287);
    float2 _e289 = coord_3;
    int4 _e292 = itex2D.Load(int3(naga_f2i32(_e289), int(3)));
    c_1 = float4(_e292);
    return;
}

uint2 NagaDepthMipDimensions2D(Texture2D<float> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

uint NagaDepthNumLevels2D(Texture2D<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z);
    return ret.z;
}

void testTex2DShadow(float2 coord_4)
{
    float2 coord_5 = (float2)0;
    int2 size2DShadow = (int2)0;
    int levels_2 = (int)0;
    float d = (float)0;

    coord_5 = coord_4;
    size2DShadow = int2(NagaDepthMipDimensions2D(tex2DShadow, int(0)));
    levels_2 = int(NagaDepthNumLevels2D(tex2DShadow));
    float2 _e12 = coord_5;
    float3 _e16 = float3(_e12.x, _e12.y, 1.0);
    float _e19 = tex2DShadow.SampleCmp(sampShadow, _e16.xy, _e16.z);
    d = _e19;
    float2 _e20 = coord_5;
    float3 _e24 = float3(_e20.x, _e20.y, 1.0);
    float _e27 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e24.xy, _e24.z);
    d = _e27;
    float2 _e28 = coord_5;
    float3 _e32 = float3(_e28.x, _e28.y, 1.0);
    float _e37 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e32.xy, _e32.z, int2((int(5)).xx));
    d = _e37;
    float2 _e38 = coord_5;
    float3 _e42 = float3(_e38.x, _e38.y, 1.0);
    float _e45 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e42.xy, _e42.z);
    d = _e45;
    float2 _e46 = coord_5;
    float3 _e50 = float3(_e46.x, _e46.y, 1.0);
    float _e55 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e50.xy, _e50.z, int2((int(5)).xx));
    d = _e55;
    float2 _e56 = coord_5;
    float3 _e60 = float3(_e56.x, _e56.y, 1.0);
    float _e65 = tex2DShadow.SampleCmp(sampShadow, _e60.xy, _e60.z, int2((int(5)).xx));
    d = _e65;
    float2 _e66 = coord_5;
    float4 _e71 = float4(_e66.x, _e66.y, 1.0, 6.0);
    float3 _e75 = (_e71.xyz / (_e71.w).xxx);
    float _e78 = tex2DShadow.SampleCmp(sampShadow, _e75.xy, _e75.z);
    d = _e78;
    float2 _e79 = coord_5;
    float4 _e84 = float4(_e79.x, _e79.y, 1.0, 6.0);
    float3 _e88 = (_e84.xyz / (_e84.w).xxx);
    float _e91 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e88.xy, _e88.z);
    d = _e91;
    float2 _e92 = coord_5;
    float4 _e97 = float4(_e92.x, _e92.y, 1.0, 6.0);
    float3 _e103 = (_e97.xyz / (_e97.w).xxx);
    float _e106 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e103.xy, _e103.z, int2((int(5)).xx));
    d = _e106;
    float2 _e107 = coord_5;
    float4 _e112 = float4(_e107.x, _e107.y, 1.0, 6.0);
    float3 _e116 = (_e112.xyz / (_e112.w).xxx);
    float _e119 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e116.xy, _e116.z);
    d = _e119;
    float2 _e120 = coord_5;
    float4 _e125 = float4(_e120.x, _e120.y, 1.0, 6.0);
    float3 _e131 = (_e125.xyz / (_e125.w).xxx);
    float _e134 = tex2DShadow.SampleCmpLevelZero(sampShadow, _e131.xy, _e131.z, int2((int(5)).xx));
    d = _e134;
    float2 _e135 = coord_5;
    float4 _e140 = float4(_e135.x, _e135.y, 1.0, 6.0);
    float3 _e146 = (_e140.xyz / (_e140.w).xxx);
    float _e149 = tex2DShadow.SampleCmp(sampShadow, _e146.xy, _e146.z, int2((int(5)).xx));
    d = _e149;
    return;
}

int3 naga_f2i32(float3 value) {
    return int3(clamp(value, -2147483600.0, 2147483500.0));
}

uint2 NagaMipDimensions2DArray(Texture2DArray<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z, ret.w);
    return ret.xy;
}

uint NagaNumLayers2DArray(Texture2DArray<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.z;
}

uint NagaNumLevels2DArray(Texture2DArray<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.w;
}

void testTex2DArray(float3 coord_6)
{
    float3 coord_7 = (float3)0;
    int3 size2DArray = (int3)0;
    int levels_3 = (int)0;
    float4 c_2 = (float4)0;

    coord_7 = coord_6;
    uint2 _e5 = NagaMipDimensions2DArray(tex2DArray, int(0));
    size2DArray = int3(uint3(_e5.x, _e5.y, NagaNumLayers2DArray(tex2DArray)));
    levels_3 = int(NagaNumLevels2DArray(tex2DArray));
    float3 _e16 = coord_7;
    float4 _e20 = tex2DArray.Sample(samp, float3(_e16.xy, naga_f2i32(_e16.z)));
    c_2 = _e20;
    float3 _e21 = coord_7;
    float4 _e26 = tex2DArray.SampleBias(samp, float3(_e21.xy, naga_f2i32(_e21.z)), 2.0);
    c_2 = _e26;
    float3 _e27 = coord_7;
    float4 _e35 = tex2DArray.SampleGrad(samp, float3(_e27.xy, naga_f2i32(_e27.z)), (4.0).xx, (4.0).xx);
    c_2 = _e35;
    float3 _e36 = coord_7;
    float4 _e46 = tex2DArray.SampleGrad(samp, float3(_e36.xy, naga_f2i32(_e36.z)), (4.0).xx, (4.0).xx, int2((int(5)).xx));
    c_2 = _e46;
    float3 _e47 = coord_7;
    float4 _e52 = tex2DArray.SampleLevel(samp, float3(_e47.xy, naga_f2i32(_e47.z)), 3.0);
    c_2 = _e52;
    float3 _e53 = coord_7;
    float4 _e60 = tex2DArray.SampleLevel(samp, float3(_e53.xy, naga_f2i32(_e53.z)), 3.0, int2((int(5)).xx));
    c_2 = _e60;
    float3 _e61 = coord_7;
    float4 _e67 = tex2DArray.Sample(samp, float3(_e61.xy, naga_f2i32(_e61.z)), int2((int(5)).xx));
    c_2 = _e67;
    float3 _e68 = coord_7;
    float4 _e75 = tex2DArray.SampleBias(samp, float3(_e68.xy, naga_f2i32(_e68.z)), 2.0, int2((int(5)).xx));
    c_2 = _e75;
    float3 _e76 = coord_7;
    int3 _e77 = naga_f2i32(_e76);
    float4 _e81 = tex2DArray.Load(int4(_e77.xy, _e77.z, int(3)));
    c_2 = _e81;
    float3 _e82 = coord_7;
    int3 _e83 = naga_f2i32(_e82);
    float4 _e87 = tex2DArray.Load(int4(_e83.xy, _e83.z, int(3)));
    c_2 = _e87;
    return;
}

uint2 NagaDepthMipDimensions2DArray(Texture2DArray<float> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z, ret.w);
    return ret.xy;
}

uint NagaDepthNumLayers2DArray(Texture2DArray<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.z;
}

uint NagaDepthNumLevels2DArray(Texture2DArray<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.w;
}

void testTex2DArrayShadow(float3 coord_8)
{
    float3 coord_9 = (float3)0;
    int3 size2DArrayShadow = (int3)0;
    int levels_4 = (int)0;
    float d_1 = (float)0;

    coord_9 = coord_8;
    uint2 _e5 = NagaDepthMipDimensions2DArray(tex2DArrayShadow, int(0));
    size2DArrayShadow = int3(uint3(_e5.x, _e5.y, NagaDepthNumLayers2DArray(tex2DArrayShadow)));
    levels_4 = int(NagaDepthNumLevels2DArray(tex2DArrayShadow));
    float3 _e16 = coord_9;
    float4 _e21 = float4(_e16.x, _e16.y, _e16.z, 1.0);
    float _e26 = tex2DArrayShadow.SampleCmp(sampShadow, float3(_e21.xy, naga_f2i32(_e21.z)), _e21.w);
    d_1 = _e26;
    float3 _e27 = coord_9;
    float4 _e32 = float4(_e27.x, _e27.y, _e27.z, 1.0);
    float _e37 = tex2DArrayShadow.SampleCmpLevelZero(sampShadow, float3(_e32.xy, naga_f2i32(_e32.z)), _e32.w);
    d_1 = _e37;
    float3 _e38 = coord_9;
    float4 _e43 = float4(_e38.x, _e38.y, _e38.z, 1.0);
    float _e50 = tex2DArrayShadow.SampleCmpLevelZero(sampShadow, float3(_e43.xy, naga_f2i32(_e43.z)), _e43.w, int2((int(5)).xx));
    d_1 = _e50;
    float3 _e51 = coord_9;
    float4 _e56 = float4(_e51.x, _e51.y, _e51.z, 1.0);
    float _e63 = tex2DArrayShadow.SampleCmp(sampShadow, float3(_e56.xy, naga_f2i32(_e56.z)), _e56.w, int2((int(5)).xx));
    d_1 = _e63;
    return;
}

uint2 NagaMipDimensionsCube(TextureCube<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

uint NagaNumLevelsCube(TextureCube<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z);
    return ret.z;
}

void testTexCube(float3 coord_10)
{
    float3 coord_11 = (float3)0;
    int2 sizeCube = (int2)0;
    int levels_5 = (int)0;
    float4 c_3 = (float4)0;

    coord_11 = coord_10;
    sizeCube = int2(NagaMipDimensionsCube(texCube, int(0)));
    levels_5 = int(NagaNumLevelsCube(texCube));
    float3 _e12 = coord_11;
    float4 _e13 = texCube.Sample(samp, _e12);
    c_3 = _e13;
    float3 _e14 = coord_11;
    float4 _e16 = texCube.SampleBias(samp, _e14, 2.0);
    c_3 = _e16;
    float3 _e17 = coord_11;
    float4 _e22 = texCube.SampleGrad(samp, _e17, (4.0).xxx, (4.0).xxx);
    c_3 = _e22;
    float3 _e23 = coord_11;
    float4 _e25 = texCube.SampleLevel(samp, _e23, 3.0);
    c_3 = _e25;
    return;
}

uint2 NagaDepthMipDimensionsCube(TextureCube<float> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

uint NagaDepthNumLevelsCube(TextureCube<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z);
    return ret.z;
}

void testTexCubeShadow(float3 coord_12)
{
    float3 coord_13 = (float3)0;
    int2 sizeCubeShadow = (int2)0;
    int levels_6 = (int)0;
    float d_2 = (float)0;

    coord_13 = coord_12;
    sizeCubeShadow = int2(NagaDepthMipDimensionsCube(texCubeShadow, int(0)));
    levels_6 = int(NagaDepthNumLevelsCube(texCubeShadow));
    float3 _e12 = coord_13;
    float4 _e17 = float4(_e12.x, _e12.y, _e12.z, 1.0);
    float _e20 = texCubeShadow.SampleCmp(sampShadow, _e17.xyz, _e17.w);
    d_2 = _e20;
    float3 _e21 = coord_13;
    float4 _e26 = float4(_e21.x, _e21.y, _e21.z, 1.0);
    float _e29 = texCubeShadow.SampleCmpLevelZero(sampShadow, _e26.xyz, _e26.w);
    d_2 = _e29;
    return;
}

uint2 NagaMipDimensionsCubeArray(TextureCubeArray<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z, ret.w);
    return ret.xy;
}

uint NagaNumLayersCubeArray(TextureCubeArray<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.z;
}

uint NagaNumLevelsCubeArray(TextureCubeArray<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.w;
}

void testTexCubeArray(float4 coord_14)
{
    float4 coord_15 = (float4)0;
    int3 sizeCubeArray = (int3)0;
    int levels_7 = (int)0;
    float4 c_4 = (float4)0;

    coord_15 = coord_14;
    uint2 _e5 = NagaMipDimensionsCubeArray(texCubeArray, int(0));
    sizeCubeArray = int3(uint3(_e5.x, _e5.y, NagaNumLayersCubeArray(texCubeArray)));
    levels_7 = int(NagaNumLevelsCubeArray(texCubeArray));
    float4 _e16 = coord_15;
    float4 _e20 = texCubeArray.Sample(samp, float4(_e16.xyz, naga_f2i32(_e16.w)));
    c_4 = _e20;
    float4 _e21 = coord_15;
    float4 _e26 = texCubeArray.SampleBias(samp, float4(_e21.xyz, naga_f2i32(_e21.w)), 2.0);
    c_4 = _e26;
    float4 _e27 = coord_15;
    float4 _e35 = texCubeArray.SampleGrad(samp, float4(_e27.xyz, naga_f2i32(_e27.w)), (4.0).xxx, (4.0).xxx);
    c_4 = _e35;
    float4 _e36 = coord_15;
    float4 _e41 = texCubeArray.SampleLevel(samp, float4(_e36.xyz, naga_f2i32(_e36.w)), 3.0);
    c_4 = _e41;
    return;
}

uint2 NagaDepthMipDimensionsCubeArray(TextureCubeArray<float> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z, ret.w);
    return ret.xy;
}

uint NagaDepthNumLayersCubeArray(TextureCubeArray<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.z;
}

uint NagaDepthNumLevelsCubeArray(TextureCubeArray<float> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.w;
}

void testTexCubeArrayShadow(float4 coord_16)
{
    float4 coord_17 = (float4)0;
    int3 sizeCubeArrayShadow = (int3)0;
    int levels_8 = (int)0;
    float d_3 = (float)0;

    coord_17 = coord_16;
    uint2 _e5 = NagaDepthMipDimensionsCubeArray(texCubeArrayShadow, int(0));
    sizeCubeArrayShadow = int3(uint3(_e5.x, _e5.y, NagaDepthNumLayersCubeArray(texCubeArrayShadow)));
    levels_8 = int(NagaDepthNumLevelsCubeArray(texCubeArrayShadow));
    float4 _e16 = coord_17;
    float _e21 = texCubeArrayShadow.SampleCmp(sampShadow, float4(_e16.xyz, naga_f2i32(_e16.w)), 1.0);
    d_3 = _e21;
    return;
}

uint3 NagaMipDimensions3D(Texture3D<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z, ret.w);
    return ret.xyz;
}

uint NagaNumLevels3D(Texture3D<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(0, ret.x, ret.y, ret.z, ret.w);
    return ret.w;
}

void testTex3D(float3 coord_18)
{
    float3 coord_19 = (float3)0;
    int3 size3D = (int3)0;
    int levels_9 = (int)0;
    float4 c_5 = (float4)0;

    coord_19 = coord_18;
    size3D = int3(NagaMipDimensions3D(tex3D_, int(0)));
    levels_9 = int(NagaNumLevels3D(tex3D_));
    float3 _e12 = coord_19;
    float4 _e13 = tex3D_.Sample(samp, _e12);
    c_5 = _e13;
    float3 _e14 = coord_19;
    float4 _e16 = tex3D_.SampleBias(samp, _e14, 2.0);
    c_5 = _e16;
    float3 _e17 = coord_19;
    float4 _e22 = float4(_e17.x, _e17.y, _e17.z, 6.0);
    float4 _e27 = tex3D_.Sample(samp, (_e22.xyz / (_e22.w).xxx));
    c_5 = _e27;
    float3 _e28 = coord_19;
    float4 _e33 = float4(_e28.x, _e28.y, _e28.z, 6.0);
    float4 _e39 = tex3D_.SampleBias(samp, (_e33.xyz / (_e33.w).xxx), 2.0);
    c_5 = _e39;
    float3 _e40 = coord_19;
    float4 _e45 = float4(_e40.x, _e40.y, _e40.z, 6.0);
    float4 _e52 = tex3D_.Sample(samp, (_e45.xyz / (_e45.w).xxx), int3((int(5)).xxx));
    c_5 = _e52;
    float3 _e53 = coord_19;
    float4 _e58 = float4(_e53.x, _e53.y, _e53.z, 6.0);
    float4 _e66 = tex3D_.SampleBias(samp, (_e58.xyz / (_e58.w).xxx), 2.0, int3((int(5)).xxx));
    c_5 = _e66;
    float3 _e67 = coord_19;
    float4 _e72 = float4(_e67.x, _e67.y, _e67.z, 6.0);
    float4 _e78 = tex3D_.SampleLevel(samp, (_e72.xyz / (_e72.w).xxx), 3.0);
    c_5 = _e78;
    float3 _e79 = coord_19;
    float4 _e84 = float4(_e79.x, _e79.y, _e79.z, 6.0);
    float4 _e92 = tex3D_.SampleLevel(samp, (_e84.xyz / (_e84.w).xxx), 3.0, int3((int(5)).xxx));
    c_5 = _e92;
    float3 _e93 = coord_19;
    float4 _e98 = float4(_e93.x, _e93.y, _e93.z, 6.0);
    float4 _e107 = tex3D_.SampleGrad(samp, (_e98.xyz / (_e98.w).xxx), (4.0).xxx, (4.0).xxx);
    c_5 = _e107;
    float3 _e108 = coord_19;
    float4 _e113 = float4(_e108.x, _e108.y, _e108.z, 6.0);
    float4 _e124 = tex3D_.SampleGrad(samp, (_e113.xyz / (_e113.w).xxx), (4.0).xxx, (4.0).xxx, int3((int(5)).xxx));
    c_5 = _e124;
    float3 _e125 = coord_19;
    float4 _e130 = tex3D_.SampleGrad(samp, _e125, (4.0).xxx, (4.0).xxx);
    c_5 = _e130;
    float3 _e131 = coord_19;
    float4 _e138 = tex3D_.SampleGrad(samp, _e131, (4.0).xxx, (4.0).xxx, int3((int(5)).xxx));
    c_5 = _e138;
    float3 _e139 = coord_19;
    float4 _e141 = tex3D_.SampleLevel(samp, _e139, 3.0);
    c_5 = _e141;
    float3 _e142 = coord_19;
    float4 _e146 = tex3D_.SampleLevel(samp, _e142, 3.0, int3((int(5)).xxx));
    c_5 = _e146;
    float3 _e147 = coord_19;
    float4 _e150 = tex3D_.Sample(samp, _e147, int3((int(5)).xxx));
    c_5 = _e150;
    float3 _e151 = coord_19;
    float4 _e155 = tex3D_.SampleBias(samp, _e151, 2.0, int3((int(5)).xxx));
    c_5 = _e155;
    float3 _e156 = coord_19;
    float4 _e159 = tex3D_.Load(int4(naga_f2i32(_e156), int(3)));
    c_5 = _e159;
    float3 _e160 = coord_19;
    float4 _e163 = tex3D_.Load(int4(naga_f2i32(_e160), int(3)));
    c_5 = _e163;
    return;
}

uint2 NagaMSDimensions2D(Texture2DMS<float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y, ret.z);
    return ret.xy;
}

void testTex2DMS(float2 coord_20)
{
    float2 coord_21 = (float2)0;
    int2 size2DMS = (int2)0;
    float4 c_6 = (float4)0;

    coord_21 = coord_20;
    size2DMS = int2(NagaMSDimensions2D(tex2DMS));
    float2 _e7 = coord_21;
    float4 _e10 = tex2DMS.Load(naga_f2i32(_e7), int(3));
    c_6 = _e10;
    return;
}

void main_1()
{
    testTex1D(1.0);
    testTex2D((1.0).xx);
    testTex2DShadow((1.0).xx);
    testTex2DArray((1.0).xxx);
    testTex2DArrayShadow((1.0).xxx);
    testTexCube((1.0).xxx);
    testTexCubeShadow((1.0).xxx);
    testTexCubeArray((1.0).xxxx);
    testTexCubeArrayShadow((1.0).xxxx);
    testTex3D((1.0).xxx);
    testTex2DMS((1.0).xx);
    return;
}

void main()
{
    main_1();
    return;
}
