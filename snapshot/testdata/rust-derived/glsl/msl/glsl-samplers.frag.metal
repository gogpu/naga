// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


int naga_f2i32(float value) {
    return static_cast<int>(metal::clamp(value, -2147483600.0, 2147483500.0));
}

void testTex1D(
    float coord,
    metal::texture1d<float, metal::access::sample> tex1D,
    metal::sampler samp
) {
    float coord_1 = {};
    int size1D = {};
    int levels = {};
    metal::float4 c = {};
    coord_1 = coord;
    size1D = static_cast<int>(tex1D.get_width());
    levels = static_cast<int>(tex1D.get_num_mip_levels());
    float _e12 = coord_1;
    metal::float4 _e13 = tex1D.sample(samp, _e12);
    c = _e13;
    float _e14 = coord_1;
    metal::float4 _e17 = tex1D.sample(samp, _e14);
    c = _e17;
    float _e18 = coord_1;
    metal::float4 _e22 = tex1D.sample(samp, _e18, 5);
    c = _e22;
    float _e23 = coord_1;
    metal::float4 _e25 = tex1D.sample(samp, _e23);
    c = _e25;
    float _e26 = coord_1;
    metal::float4 _e29 = tex1D.sample(samp, _e26, 5);
    c = _e29;
    float _e30 = coord_1;
    metal::float4 _e32 = tex1D.sample(samp, _e30, 5);
    c = _e32;
    float _e33 = coord_1;
    metal::float2 _e35 = metal::float2(_e33, 6.0);
    metal::float4 _e39 = tex1D.sample(samp, _e35.x / _e35.y);
    c = _e39;
    float _e40 = coord_1;
    metal::float4 _e44 = metal::float4(_e40, 0.0, 0.0, 6.0);
    metal::float4 _e50 = tex1D.sample(samp, (_e44.xyz / metal::float3(_e44.w)).x);
    c = _e50;
    float _e51 = coord_1;
    metal::float2 _e53 = metal::float2(_e51, 6.0);
    metal::float4 _e59 = tex1D.sample(samp, _e53.x / _e53.y);
    c = _e59;
    float _e60 = coord_1;
    metal::float4 _e64 = metal::float4(_e60, 0.0, 0.0, 6.0);
    metal::float4 _e72 = tex1D.sample(samp, (_e64.xyz / metal::float3(_e64.w)).x);
    c = _e72;
    float _e73 = coord_1;
    metal::float2 _e75 = metal::float2(_e73, 6.0);
    metal::float4 _e82 = tex1D.sample(samp, _e75.x / _e75.y, 5);
    c = _e82;
    float _e83 = coord_1;
    metal::float4 _e87 = metal::float4(_e83, 0.0, 0.0, 6.0);
    metal::float4 _e96 = tex1D.sample(samp, (_e87.xyz / metal::float3(_e87.w)).x, 5);
    c = _e96;
    float _e97 = coord_1;
    metal::float2 _e99 = metal::float2(_e97, 6.0);
    metal::float4 _e104 = tex1D.sample(samp, _e99.x / _e99.y);
    c = _e104;
    float _e105 = coord_1;
    metal::float4 _e109 = metal::float4(_e105, 0.0, 0.0, 6.0);
    metal::float4 _e116 = tex1D.sample(samp, (_e109.xyz / metal::float3(_e109.w)).x);
    c = _e116;
    float _e117 = coord_1;
    metal::float2 _e119 = metal::float2(_e117, 6.0);
    metal::float4 _e125 = tex1D.sample(samp, _e119.x / _e119.y, 5);
    c = _e125;
    float _e126 = coord_1;
    metal::float4 _e130 = metal::float4(_e126, 0.0, 0.0, 6.0);
    metal::float4 _e138 = tex1D.sample(samp, (_e130.xyz / metal::float3(_e130.w)).x, 5);
    c = _e138;
    float _e139 = coord_1;
    metal::float2 _e141 = metal::float2(_e139, 6.0);
    metal::float4 _e146 = tex1D.sample(samp, _e141.x / _e141.y, 5);
    c = _e146;
    float _e147 = coord_1;
    metal::float4 _e151 = metal::float4(_e147, 0.0, 0.0, 6.0);
    metal::float4 _e158 = tex1D.sample(samp, (_e151.xyz / metal::float3(_e151.w)).x, 5);
    c = _e158;
    float _e159 = coord_1;
    metal::float4 _e162 = tex1D.read(uint(naga_f2i32(_e159)));
    c = _e162;
    float _e163 = coord_1;
    metal::float4 _e166 = tex1D.read(uint(naga_f2i32(_e163)));
    c = _e166;
    return;
}

metal::int2 naga_f2i32(metal::float2 value) {
    return static_cast<metal::int2>(metal::clamp(value, -2147483600.0, 2147483500.0));
}

void testTex2D(
    metal::float2 coord_2,
    metal::texture2d<float, metal::access::sample> tex2D,
    metal::texture2d<uint, metal::access::sample> utex2D,
    metal::texture2d<int, metal::access::sample> itex2D,
    metal::sampler samp
) {
    metal::float2 coord_3 = {};
    metal::int2 size2D = {};
    int levels_1 = {};
    metal::float4 c_1 = {};
    coord_3 = coord_2;
    size2D = static_cast<metal::int2>(metal::uint2(tex2D.get_width(0), tex2D.get_height(0)));
    levels_1 = static_cast<int>(tex2D.get_num_mip_levels());
    metal::float2 _e14 = coord_3;
    metal::float4 _e15 = tex2D.sample(samp, _e14);
    c_1 = _e15;
    metal::float2 _e16 = coord_3;
    metal::float4 _e18 = tex2D.sample(samp, _e16, metal::bias(2.0));
    c_1 = _e18;
    metal::float2 _e19 = coord_3;
    metal::float4 _e24 = tex2D.sample(samp, _e19, metal::gradient2d(metal::float2(4.0), metal::float2(4.0)));
    c_1 = _e24;
    metal::float2 _e25 = coord_3;
    metal::float4 _e32 = tex2D.sample(samp, _e25, metal::gradient2d(metal::float2(4.0), metal::float2(4.0)), metal::int2(5));
    c_1 = _e32;
    metal::float2 _e33 = coord_3;
    metal::float4 _e35 = tex2D.sample(samp, _e33, metal::level(3.0));
    c_1 = _e35;
    metal::float2 _e36 = coord_3;
    metal::float4 _e40 = tex2D.sample(samp, _e36, metal::level(3.0), metal::int2(5));
    c_1 = _e40;
    metal::float2 _e41 = coord_3;
    metal::float4 _e44 = tex2D.sample(samp, _e41, metal::int2(5));
    c_1 = _e44;
    metal::float2 _e45 = coord_3;
    metal::float4 _e49 = tex2D.sample(samp, _e45, metal::bias(2.0), metal::int2(5));
    c_1 = _e49;
    metal::float2 _e50 = coord_3;
    metal::float3 _e54 = metal::float3(_e50.x, _e50.y, 6.0);
    metal::float4 _e59 = tex2D.sample(samp, _e54.xy / metal::float2(_e54.z));
    c_1 = _e59;
    metal::float2 _e60 = coord_3;
    metal::float4 _e65 = metal::float4(_e60.x, _e60.y, 0.0, 6.0);
    metal::float4 _e71 = tex2D.sample(samp, (_e65.xyz / metal::float3(_e65.w)).xy);
    c_1 = _e71;
    metal::float2 _e72 = coord_3;
    metal::float3 _e76 = metal::float3(_e72.x, _e72.y, 6.0);
    metal::float4 _e82 = tex2D.sample(samp, _e76.xy / metal::float2(_e76.z), metal::bias(2.0));
    c_1 = _e82;
    metal::float2 _e83 = coord_3;
    metal::float4 _e88 = metal::float4(_e83.x, _e83.y, 0.0, 6.0);
    metal::float4 _e95 = tex2D.sample(samp, (_e88.xyz / metal::float3(_e88.w)).xy, metal::bias(2.0));
    c_1 = _e95;
    metal::float2 _e96 = coord_3;
    metal::float3 _e100 = metal::float3(_e96.x, _e96.y, 6.0);
    metal::float4 _e109 = tex2D.sample(samp, _e100.xy / metal::float2(_e100.z), metal::gradient2d(metal::float2(4.0), metal::float2(4.0)));
    c_1 = _e109;
    metal::float2 _e110 = coord_3;
    metal::float4 _e115 = metal::float4(_e110.x, _e110.y, 0.0, 6.0);
    metal::float4 _e125 = tex2D.sample(samp, (_e115.xyz / metal::float3(_e115.w)).xy, metal::gradient2d(metal::float2(4.0), metal::float2(4.0)));
    c_1 = _e125;
    metal::float2 _e126 = coord_3;
    metal::float3 _e130 = metal::float3(_e126.x, _e126.y, 6.0);
    metal::float4 _e141 = tex2D.sample(samp, _e130.xy / metal::float2(_e130.z), metal::gradient2d(metal::float2(4.0), metal::float2(4.0)), metal::int2(5));
    c_1 = _e141;
    metal::float2 _e142 = coord_3;
    metal::float4 _e147 = metal::float4(_e142.x, _e142.y, 0.0, 6.0);
    metal::float4 _e159 = tex2D.sample(samp, (_e147.xyz / metal::float3(_e147.w)).xy, metal::gradient2d(metal::float2(4.0), metal::float2(4.0)), metal::int2(5));
    c_1 = _e159;
    metal::float2 _e160 = coord_3;
    metal::float3 _e164 = metal::float3(_e160.x, _e160.y, 6.0);
    metal::float4 _e170 = tex2D.sample(samp, _e164.xy / metal::float2(_e164.z), metal::level(3.0));
    c_1 = _e170;
    metal::float2 _e171 = coord_3;
    metal::float4 _e176 = metal::float4(_e171.x, _e171.y, 0.0, 6.0);
    metal::float4 _e183 = tex2D.sample(samp, (_e176.xyz / metal::float3(_e176.w)).xy, metal::level(3.0));
    c_1 = _e183;
    metal::float2 _e184 = coord_3;
    metal::float3 _e188 = metal::float3(_e184.x, _e184.y, 6.0);
    metal::float4 _e196 = tex2D.sample(samp, _e188.xy / metal::float2(_e188.z), metal::level(3.0), metal::int2(5));
    c_1 = _e196;
    metal::float2 _e197 = coord_3;
    metal::float4 _e202 = metal::float4(_e197.x, _e197.y, 0.0, 6.0);
    metal::float4 _e211 = tex2D.sample(samp, (_e202.xyz / metal::float3(_e202.w)).xy, metal::level(3.0), metal::int2(5));
    c_1 = _e211;
    metal::float2 _e212 = coord_3;
    metal::float3 _e216 = metal::float3(_e212.x, _e212.y, 6.0);
    metal::float4 _e223 = tex2D.sample(samp, _e216.xy / metal::float2(_e216.z), metal::int2(5));
    c_1 = _e223;
    metal::float2 _e224 = coord_3;
    metal::float4 _e229 = metal::float4(_e224.x, _e224.y, 0.0, 6.0);
    metal::float4 _e237 = tex2D.sample(samp, (_e229.xyz / metal::float3(_e229.w)).xy, metal::int2(5));
    c_1 = _e237;
    metal::float2 _e238 = coord_3;
    metal::float3 _e242 = metal::float3(_e238.x, _e238.y, 6.0);
    metal::float4 _e250 = tex2D.sample(samp, _e242.xy / metal::float2(_e242.z), metal::bias(2.0), metal::int2(5));
    c_1 = _e250;
    metal::float2 _e251 = coord_3;
    metal::float4 _e256 = metal::float4(_e251.x, _e251.y, 0.0, 6.0);
    metal::float4 _e265 = tex2D.sample(samp, (_e256.xyz / metal::float3(_e256.w)).xy, metal::bias(2.0), metal::int2(5));
    c_1 = _e265;
    metal::float2 _e266 = coord_3;
    metal::float4 _e269 = tex2D.read(metal::uint2(naga_f2i32(_e266)), 3);
    c_1 = _e269;
    metal::float2 _e270 = coord_3;
    metal::uint4 _e273 = utex2D.read(metal::uint2(naga_f2i32(_e270)), 3);
    c_1 = static_cast<metal::float4>(_e273);
    metal::float2 _e275 = coord_3;
    metal::int4 _e278 = itex2D.read(metal::uint2(naga_f2i32(_e275)), 3);
    c_1 = static_cast<metal::float4>(_e278);
    metal::float2 _e280 = coord_3;
    metal::float4 _e283 = tex2D.read(metal::uint2(naga_f2i32(_e280)), 3);
    c_1 = _e283;
    metal::float2 _e284 = coord_3;
    metal::uint4 _e287 = utex2D.read(metal::uint2(naga_f2i32(_e284)), 3);
    c_1 = static_cast<metal::float4>(_e287);
    metal::float2 _e289 = coord_3;
    metal::int4 _e292 = itex2D.read(metal::uint2(naga_f2i32(_e289)), 3);
    c_1 = static_cast<metal::float4>(_e292);
    return;
}

void testTex2DShadow(
    metal::float2 coord_4,
    metal::depth2d<float, metal::access::sample> tex2DShadow,
    metal::sampler sampShadow
) {
    metal::float2 coord_5 = {};
    metal::int2 size2DShadow = {};
    int levels_2 = {};
    float d = {};
    coord_5 = coord_4;
    size2DShadow = static_cast<metal::int2>(metal::uint2(tex2DShadow.get_width(0), tex2DShadow.get_height(0)));
    levels_2 = static_cast<int>(tex2DShadow.get_num_mip_levels());
    metal::float2 _e12 = coord_5;
    metal::float3 _e16 = metal::float3(_e12.x, _e12.y, 1.0);
    float _e19 = tex2DShadow.sample_compare(sampShadow, _e16.xy, _e16.z);
    d = _e19;
    metal::float2 _e20 = coord_5;
    metal::float3 _e24 = metal::float3(_e20.x, _e20.y, 1.0);
    float _e27 = tex2DShadow.sample_compare(sampShadow, _e24.xy, _e24.z);
    d = _e27;
    metal::float2 _e28 = coord_5;
    metal::float3 _e32 = metal::float3(_e28.x, _e28.y, 1.0);
    float _e37 = tex2DShadow.sample_compare(sampShadow, _e32.xy, _e32.z, metal::int2(5));
    d = _e37;
    metal::float2 _e38 = coord_5;
    metal::float3 _e42 = metal::float3(_e38.x, _e38.y, 1.0);
    float _e45 = tex2DShadow.sample_compare(sampShadow, _e42.xy, _e42.z);
    d = _e45;
    metal::float2 _e46 = coord_5;
    metal::float3 _e50 = metal::float3(_e46.x, _e46.y, 1.0);
    float _e55 = tex2DShadow.sample_compare(sampShadow, _e50.xy, _e50.z, metal::int2(5));
    d = _e55;
    metal::float2 _e56 = coord_5;
    metal::float3 _e60 = metal::float3(_e56.x, _e56.y, 1.0);
    float _e65 = tex2DShadow.sample_compare(sampShadow, _e60.xy, _e60.z, metal::int2(5));
    d = _e65;
    metal::float2 _e66 = coord_5;
    metal::float4 _e71 = metal::float4(_e66.x, _e66.y, 1.0, 6.0);
    metal::float3 _e75 = _e71.xyz / metal::float3(_e71.w);
    float _e78 = tex2DShadow.sample_compare(sampShadow, _e75.xy, _e75.z);
    d = _e78;
    metal::float2 _e79 = coord_5;
    metal::float4 _e84 = metal::float4(_e79.x, _e79.y, 1.0, 6.0);
    metal::float3 _e88 = _e84.xyz / metal::float3(_e84.w);
    float _e91 = tex2DShadow.sample_compare(sampShadow, _e88.xy, _e88.z);
    d = _e91;
    metal::float2 _e92 = coord_5;
    metal::float4 _e97 = metal::float4(_e92.x, _e92.y, 1.0, 6.0);
    metal::float3 _e103 = _e97.xyz / metal::float3(_e97.w);
    float _e106 = tex2DShadow.sample_compare(sampShadow, _e103.xy, _e103.z, metal::int2(5));
    d = _e106;
    metal::float2 _e107 = coord_5;
    metal::float4 _e112 = metal::float4(_e107.x, _e107.y, 1.0, 6.0);
    metal::float3 _e116 = _e112.xyz / metal::float3(_e112.w);
    float _e119 = tex2DShadow.sample_compare(sampShadow, _e116.xy, _e116.z);
    d = _e119;
    metal::float2 _e120 = coord_5;
    metal::float4 _e125 = metal::float4(_e120.x, _e120.y, 1.0, 6.0);
    metal::float3 _e131 = _e125.xyz / metal::float3(_e125.w);
    float _e134 = tex2DShadow.sample_compare(sampShadow, _e131.xy, _e131.z, metal::int2(5));
    d = _e134;
    metal::float2 _e135 = coord_5;
    metal::float4 _e140 = metal::float4(_e135.x, _e135.y, 1.0, 6.0);
    metal::float3 _e146 = _e140.xyz / metal::float3(_e140.w);
    float _e149 = tex2DShadow.sample_compare(sampShadow, _e146.xy, _e146.z, metal::int2(5));
    d = _e149;
    return;
}

metal::int3 naga_f2i32(metal::float3 value) {
    return static_cast<metal::int3>(metal::clamp(value, -2147483600.0, 2147483500.0));
}

void testTex2DArray(
    metal::float3 coord_6,
    metal::texture2d_array<float, metal::access::sample> tex2DArray,
    metal::sampler samp
) {
    metal::float3 coord_7 = {};
    metal::int3 size2DArray = {};
    int levels_3 = {};
    metal::float4 c_2 = {};
    coord_7 = coord_6;
    metal::uint2 _e5 = metal::uint2(tex2DArray.get_width(0), tex2DArray.get_height(0));
    size2DArray = static_cast<metal::int3>(metal::uint3(_e5.x, _e5.y, tex2DArray.get_array_size()));
    levels_3 = static_cast<int>(tex2DArray.get_num_mip_levels());
    metal::float3 _e16 = coord_7;
    metal::float4 _e20 = tex2DArray.sample(samp, _e16.xy, naga_f2i32(_e16.z));
    c_2 = _e20;
    metal::float3 _e21 = coord_7;
    metal::float4 _e26 = tex2DArray.sample(samp, _e21.xy, naga_f2i32(_e21.z), metal::bias(2.0));
    c_2 = _e26;
    metal::float3 _e27 = coord_7;
    metal::float4 _e35 = tex2DArray.sample(samp, _e27.xy, naga_f2i32(_e27.z), metal::gradient2d(metal::float2(4.0), metal::float2(4.0)));
    c_2 = _e35;
    metal::float3 _e36 = coord_7;
    metal::float4 _e46 = tex2DArray.sample(samp, _e36.xy, naga_f2i32(_e36.z), metal::gradient2d(metal::float2(4.0), metal::float2(4.0)), metal::int2(5));
    c_2 = _e46;
    metal::float3 _e47 = coord_7;
    metal::float4 _e52 = tex2DArray.sample(samp, _e47.xy, naga_f2i32(_e47.z), metal::level(3.0));
    c_2 = _e52;
    metal::float3 _e53 = coord_7;
    metal::float4 _e60 = tex2DArray.sample(samp, _e53.xy, naga_f2i32(_e53.z), metal::level(3.0), metal::int2(5));
    c_2 = _e60;
    metal::float3 _e61 = coord_7;
    metal::float4 _e67 = tex2DArray.sample(samp, _e61.xy, naga_f2i32(_e61.z), metal::int2(5));
    c_2 = _e67;
    metal::float3 _e68 = coord_7;
    metal::float4 _e75 = tex2DArray.sample(samp, _e68.xy, naga_f2i32(_e68.z), metal::bias(2.0), metal::int2(5));
    c_2 = _e75;
    metal::float3 _e76 = coord_7;
    metal::int3 _e77 = naga_f2i32(_e76);
    metal::float4 _e81 = tex2DArray.read(metal::uint2(_e77.xy), _e77.z, 3);
    c_2 = _e81;
    metal::float3 _e82 = coord_7;
    metal::int3 _e83 = naga_f2i32(_e82);
    metal::float4 _e87 = tex2DArray.read(metal::uint2(_e83.xy), _e83.z, 3);
    c_2 = _e87;
    return;
}

void testTex2DArrayShadow(
    metal::float3 coord_8,
    metal::depth2d_array<float, metal::access::sample> tex2DArrayShadow,
    metal::sampler sampShadow
) {
    metal::float3 coord_9 = {};
    metal::int3 size2DArrayShadow = {};
    int levels_4 = {};
    float d_1 = {};
    coord_9 = coord_8;
    metal::uint2 _e5 = metal::uint2(tex2DArrayShadow.get_width(0), tex2DArrayShadow.get_height(0));
    size2DArrayShadow = static_cast<metal::int3>(metal::uint3(_e5.x, _e5.y, tex2DArrayShadow.get_array_size()));
    levels_4 = static_cast<int>(tex2DArrayShadow.get_num_mip_levels());
    metal::float3 _e16 = coord_9;
    metal::float4 _e21 = metal::float4(_e16.x, _e16.y, _e16.z, 1.0);
    float _e26 = tex2DArrayShadow.sample_compare(sampShadow, _e21.xy, naga_f2i32(_e21.z), _e21.w);
    d_1 = _e26;
    metal::float3 _e27 = coord_9;
    metal::float4 _e32 = metal::float4(_e27.x, _e27.y, _e27.z, 1.0);
    float _e37 = tex2DArrayShadow.sample_compare(sampShadow, _e32.xy, naga_f2i32(_e32.z), _e32.w);
    d_1 = _e37;
    metal::float3 _e38 = coord_9;
    metal::float4 _e43 = metal::float4(_e38.x, _e38.y, _e38.z, 1.0);
    float _e50 = tex2DArrayShadow.sample_compare(sampShadow, _e43.xy, naga_f2i32(_e43.z), _e43.w, metal::int2(5));
    d_1 = _e50;
    metal::float3 _e51 = coord_9;
    metal::float4 _e56 = metal::float4(_e51.x, _e51.y, _e51.z, 1.0);
    float _e63 = tex2DArrayShadow.sample_compare(sampShadow, _e56.xy, naga_f2i32(_e56.z), _e56.w, metal::int2(5));
    d_1 = _e63;
    return;
}

void testTexCube(
    metal::float3 coord_10,
    metal::texturecube<float, metal::access::sample> texCube,
    metal::sampler samp
) {
    metal::float3 coord_11 = {};
    metal::int2 sizeCube = {};
    int levels_5 = {};
    metal::float4 c_3 = {};
    coord_11 = coord_10;
    sizeCube = static_cast<metal::int2>(metal::uint2(texCube.get_width(0)));
    levels_5 = static_cast<int>(texCube.get_num_mip_levels());
    metal::float3 _e12 = coord_11;
    metal::float4 _e13 = texCube.sample(samp, _e12);
    c_3 = _e13;
    metal::float3 _e14 = coord_11;
    metal::float4 _e16 = texCube.sample(samp, _e14, metal::bias(2.0));
    c_3 = _e16;
    metal::float3 _e17 = coord_11;
    metal::float4 _e22 = texCube.sample(samp, _e17, metal::gradient2d(metal::float3(4.0), metal::float3(4.0)));
    c_3 = _e22;
    metal::float3 _e23 = coord_11;
    metal::float4 _e25 = texCube.sample(samp, _e23, metal::level(3.0));
    c_3 = _e25;
    return;
}

void testTexCubeShadow(
    metal::float3 coord_12,
    metal::depthcube<float, metal::access::sample> texCubeShadow,
    metal::sampler sampShadow
) {
    metal::float3 coord_13 = {};
    metal::int2 sizeCubeShadow = {};
    int levels_6 = {};
    float d_2 = {};
    coord_13 = coord_12;
    sizeCubeShadow = static_cast<metal::int2>(metal::uint2(texCubeShadow.get_width(0)));
    levels_6 = static_cast<int>(texCubeShadow.get_num_mip_levels());
    metal::float3 _e12 = coord_13;
    metal::float4 _e17 = metal::float4(_e12.x, _e12.y, _e12.z, 1.0);
    float _e20 = texCubeShadow.sample_compare(sampShadow, _e17.xyz, _e17.w);
    d_2 = _e20;
    metal::float3 _e21 = coord_13;
    metal::float4 _e26 = metal::float4(_e21.x, _e21.y, _e21.z, 1.0);
    float _e29 = texCubeShadow.sample_compare(sampShadow, _e26.xyz, _e26.w);
    d_2 = _e29;
    return;
}

void testTexCubeArray(
    metal::float4 coord_14,
    metal::texturecube_array<float, metal::access::sample> texCubeArray,
    metal::sampler samp
) {
    metal::float4 coord_15 = {};
    metal::int3 sizeCubeArray = {};
    int levels_7 = {};
    metal::float4 c_4 = {};
    coord_15 = coord_14;
    metal::uint2 _e5 = metal::uint2(texCubeArray.get_width(0));
    sizeCubeArray = static_cast<metal::int3>(metal::uint3(_e5.x, _e5.y, texCubeArray.get_array_size()));
    levels_7 = static_cast<int>(texCubeArray.get_num_mip_levels());
    metal::float4 _e16 = coord_15;
    metal::float4 _e20 = texCubeArray.sample(samp, _e16.xyz, naga_f2i32(_e16.w));
    c_4 = _e20;
    metal::float4 _e21 = coord_15;
    metal::float4 _e26 = texCubeArray.sample(samp, _e21.xyz, naga_f2i32(_e21.w), metal::bias(2.0));
    c_4 = _e26;
    metal::float4 _e27 = coord_15;
    metal::float4 _e35 = texCubeArray.sample(samp, _e27.xyz, naga_f2i32(_e27.w), metal::gradient2d(metal::float3(4.0), metal::float3(4.0)));
    c_4 = _e35;
    metal::float4 _e36 = coord_15;
    metal::float4 _e41 = texCubeArray.sample(samp, _e36.xyz, naga_f2i32(_e36.w), metal::level(3.0));
    c_4 = _e41;
    return;
}

void testTexCubeArrayShadow(
    metal::float4 coord_16,
    metal::depthcube_array<float, metal::access::sample> texCubeArrayShadow,
    metal::sampler sampShadow
) {
    metal::float4 coord_17 = {};
    metal::int3 sizeCubeArrayShadow = {};
    int levels_8 = {};
    float d_3 = {};
    coord_17 = coord_16;
    metal::uint2 _e5 = metal::uint2(texCubeArrayShadow.get_width(0));
    sizeCubeArrayShadow = static_cast<metal::int3>(metal::uint3(_e5.x, _e5.y, texCubeArrayShadow.get_array_size()));
    levels_8 = static_cast<int>(texCubeArrayShadow.get_num_mip_levels());
    metal::float4 _e16 = coord_17;
    float _e21 = texCubeArrayShadow.sample_compare(sampShadow, _e16.xyz, naga_f2i32(_e16.w), 1.0);
    d_3 = _e21;
    return;
}

void testTex3D(
    metal::float3 coord_18,
    metal::texture3d<float, metal::access::sample> tex3D,
    metal::sampler samp
) {
    metal::float3 coord_19 = {};
    metal::int3 size3D = {};
    int levels_9 = {};
    metal::float4 c_5 = {};
    coord_19 = coord_18;
    size3D = static_cast<metal::int3>(metal::uint3(tex3D.get_width(0), tex3D.get_height(0), tex3D.get_depth(0)));
    levels_9 = static_cast<int>(tex3D.get_num_mip_levels());
    metal::float3 _e12 = coord_19;
    metal::float4 _e13 = tex3D.sample(samp, _e12);
    c_5 = _e13;
    metal::float3 _e14 = coord_19;
    metal::float4 _e16 = tex3D.sample(samp, _e14, metal::bias(2.0));
    c_5 = _e16;
    metal::float3 _e17 = coord_19;
    metal::float4 _e22 = metal::float4(_e17.x, _e17.y, _e17.z, 6.0);
    metal::float4 _e27 = tex3D.sample(samp, _e22.xyz / metal::float3(_e22.w));
    c_5 = _e27;
    metal::float3 _e28 = coord_19;
    metal::float4 _e33 = metal::float4(_e28.x, _e28.y, _e28.z, 6.0);
    metal::float4 _e39 = tex3D.sample(samp, _e33.xyz / metal::float3(_e33.w), metal::bias(2.0));
    c_5 = _e39;
    metal::float3 _e40 = coord_19;
    metal::float4 _e45 = metal::float4(_e40.x, _e40.y, _e40.z, 6.0);
    metal::float4 _e52 = tex3D.sample(samp, _e45.xyz / metal::float3(_e45.w), metal::int3(5));
    c_5 = _e52;
    metal::float3 _e53 = coord_19;
    metal::float4 _e58 = metal::float4(_e53.x, _e53.y, _e53.z, 6.0);
    metal::float4 _e66 = tex3D.sample(samp, _e58.xyz / metal::float3(_e58.w), metal::bias(2.0), metal::int3(5));
    c_5 = _e66;
    metal::float3 _e67 = coord_19;
    metal::float4 _e72 = metal::float4(_e67.x, _e67.y, _e67.z, 6.0);
    metal::float4 _e78 = tex3D.sample(samp, _e72.xyz / metal::float3(_e72.w), metal::level(3.0));
    c_5 = _e78;
    metal::float3 _e79 = coord_19;
    metal::float4 _e84 = metal::float4(_e79.x, _e79.y, _e79.z, 6.0);
    metal::float4 _e92 = tex3D.sample(samp, _e84.xyz / metal::float3(_e84.w), metal::level(3.0), metal::int3(5));
    c_5 = _e92;
    metal::float3 _e93 = coord_19;
    metal::float4 _e98 = metal::float4(_e93.x, _e93.y, _e93.z, 6.0);
    metal::float4 _e107 = tex3D.sample(samp, _e98.xyz / metal::float3(_e98.w), metal::gradient2d(metal::float3(4.0), metal::float3(4.0)));
    c_5 = _e107;
    metal::float3 _e108 = coord_19;
    metal::float4 _e113 = metal::float4(_e108.x, _e108.y, _e108.z, 6.0);
    metal::float4 _e124 = tex3D.sample(samp, _e113.xyz / metal::float3(_e113.w), metal::gradient2d(metal::float3(4.0), metal::float3(4.0)), metal::int3(5));
    c_5 = _e124;
    metal::float3 _e125 = coord_19;
    metal::float4 _e130 = tex3D.sample(samp, _e125, metal::gradient2d(metal::float3(4.0), metal::float3(4.0)));
    c_5 = _e130;
    metal::float3 _e131 = coord_19;
    metal::float4 _e138 = tex3D.sample(samp, _e131, metal::gradient2d(metal::float3(4.0), metal::float3(4.0)), metal::int3(5));
    c_5 = _e138;
    metal::float3 _e139 = coord_19;
    metal::float4 _e141 = tex3D.sample(samp, _e139, metal::level(3.0));
    c_5 = _e141;
    metal::float3 _e142 = coord_19;
    metal::float4 _e146 = tex3D.sample(samp, _e142, metal::level(3.0), metal::int3(5));
    c_5 = _e146;
    metal::float3 _e147 = coord_19;
    metal::float4 _e150 = tex3D.sample(samp, _e147, metal::int3(5));
    c_5 = _e150;
    metal::float3 _e151 = coord_19;
    metal::float4 _e155 = tex3D.sample(samp, _e151, metal::bias(2.0), metal::int3(5));
    c_5 = _e155;
    metal::float3 _e156 = coord_19;
    metal::float4 _e159 = tex3D.read(metal::uint3(naga_f2i32(_e156)), 3);
    c_5 = _e159;
    metal::float3 _e160 = coord_19;
    metal::float4 _e163 = tex3D.read(metal::uint3(naga_f2i32(_e160)), 3);
    c_5 = _e163;
    return;
}

void testTex2DMS(
    metal::float2 coord_20,
    metal::texture2d_ms<float, metal::access::read> tex2DMS
) {
    metal::float2 coord_21 = {};
    metal::int2 size2DMS = {};
    metal::float4 c_6 = {};
    coord_21 = coord_20;
    size2DMS = static_cast<metal::int2>(metal::uint2(tex2DMS.get_width(), tex2DMS.get_height()));
    metal::float2 _e7 = coord_21;
    metal::float4 _e10 = tex2DMS.read(metal::uint2(naga_f2i32(_e7)), 3);
    c_6 = _e10;
    return;
}

void main_1(
    metal::texture1d<float, metal::access::sample> tex1D,
    metal::texture2d<float, metal::access::sample> tex2D,
    metal::texture2d_array<float, metal::access::sample> tex2DArray,
    metal::texturecube<float, metal::access::sample> texCube,
    metal::texturecube_array<float, metal::access::sample> texCubeArray,
    metal::texture3d<float, metal::access::sample> tex3D,
    metal::texture2d<uint, metal::access::sample> utex2D,
    metal::texture2d<int, metal::access::sample> itex2D,
    metal::sampler samp,
    metal::depth2d<float, metal::access::sample> tex2DShadow,
    metal::depth2d_array<float, metal::access::sample> tex2DArrayShadow,
    metal::depthcube<float, metal::access::sample> texCubeShadow,
    metal::depthcube_array<float, metal::access::sample> texCubeArrayShadow,
    metal::sampler sampShadow,
    metal::texture2d_ms<float, metal::access::read> tex2DMS
) {
    testTex1D(1.0, tex1D, samp);
    testTex2D(metal::float2(1.0), tex2D, utex2D, itex2D, samp);
    testTex2DShadow(metal::float2(1.0), tex2DShadow, sampShadow);
    testTex2DArray(metal::float3(1.0), tex2DArray, samp);
    testTex2DArrayShadow(metal::float3(1.0), tex2DArrayShadow, sampShadow);
    testTexCube(metal::float3(1.0), texCube, samp);
    testTexCubeShadow(metal::float3(1.0), texCubeShadow, sampShadow);
    testTexCubeArray(metal::float4(1.0), texCubeArray, samp);
    testTexCubeArrayShadow(metal::float4(1.0), texCubeArrayShadow, sampShadow);
    testTex3D(metal::float3(1.0), tex3D, samp);
    testTex2DMS(metal::float2(1.0), tex2DMS);
    return;
}

fragment void main_(
  metal::texture1d<float, metal::access::sample> tex1D [[user(fake0)]]
, metal::texture2d<float, metal::access::sample> tex2D [[user(fake0)]]
, metal::texture2d_array<float, metal::access::sample> tex2DArray [[user(fake0)]]
, metal::texturecube<float, metal::access::sample> texCube [[user(fake0)]]
, metal::texturecube_array<float, metal::access::sample> texCubeArray [[user(fake0)]]
, metal::texture3d<float, metal::access::sample> tex3D [[user(fake0)]]
, metal::texture2d<uint, metal::access::sample> utex2D [[user(fake0)]]
, metal::texture2d<int, metal::access::sample> itex2D [[user(fake0)]]
, metal::sampler samp [[user(fake0)]]
, metal::depth2d<float, metal::access::sample> tex2DShadow [[user(fake0)]]
, metal::depth2d_array<float, metal::access::sample> tex2DArrayShadow [[user(fake0)]]
, metal::depthcube<float, metal::access::sample> texCubeShadow [[user(fake0)]]
, metal::depthcube_array<float, metal::access::sample> texCubeArrayShadow [[user(fake0)]]
, metal::sampler sampShadow [[user(fake0)]]
, metal::texture2d_ms<float, metal::access::read> tex2DMS [[user(fake0)]]
) {
    main_1(tex1D, tex2D, tex2DArray, texCube, texCubeArray, tex3D, utex2D, itex2D, samp, tex2DShadow, tex2DArrayShadow, texCubeShadow, texCubeArrayShadow, sampShadow, tex2DMS);
    return;
}
