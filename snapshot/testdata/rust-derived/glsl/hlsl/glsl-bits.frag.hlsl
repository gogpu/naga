int naga_insertBits(
    int e,
    int newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
int2 naga_insertBits(
    int2 e,
    int2 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
int3 naga_insertBits(
    int3 e,
    int3 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
int4 naga_insertBits(
    int4 e,
    int4 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
uint naga_insertBits(
    uint e,
    uint newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
uint2 naga_insertBits(
    uint2 e,
    uint2 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
uint3 naga_insertBits(
    uint3 e,
    uint3 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
uint4 naga_insertBits(
    uint4 e,
    uint4 newbits,
    uint offset,
    uint count
) {
    uint w = 32u;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    uint mask = ((4294967295u >> (32u - c)) << o);
    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));
}
int naga_extractBits(
    int e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
int2 naga_extractBits(
    int2 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
int3 naga_extractBits(
    int3 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
int4 naga_extractBits(
    int4 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
uint naga_extractBits(
    uint e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
uint2 naga_extractBits(
    uint2 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
uint3 naga_extractBits(
    uint3 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
uint4 naga_extractBits(
    uint4 e,
    uint offset,
    uint count
) {
    uint w = 32;
    uint o = min(offset, w);
    uint c = min(count, w - o);
    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));
}
void main_1()
{
    int i = int(0);
    int2 i2_ = (int(0)).xx;
    int3 i3_ = (int(0)).xxx;
    int4 i4_ = (int(0)).xxxx;
    uint u = 0u;
    uint2 u2_ = (0u).xx;
    uint3 u3_ = (0u).xxx;
    uint4 u4_ = (0u).xxxx;
    float2 f2_ = (0.0).xx;
    float4 f4_ = (0.0).xxxx;

    float4 _e28 = f4_;
    u = uint((int(round(clamp(_e28[0], -1.0, 1.0) * 127.0)) & 0xFF) | ((int(round(clamp(_e28[1], -1.0, 1.0) * 127.0)) & 0xFF) << 8) | ((int(round(clamp(_e28[2], -1.0, 1.0) * 127.0)) & 0xFF) << 16) | ((int(round(clamp(_e28[3], -1.0, 1.0) * 127.0)) & 0xFF) << 24));
    float4 _e30 = f4_;
    u = (uint(round(clamp(_e30[0], 0.0, 1.0) * 255.0)) | uint(round(clamp(_e30[1], 0.0, 1.0) * 255.0)) << 8 | uint(round(clamp(_e30[2], 0.0, 1.0) * 255.0)) << 16 | uint(round(clamp(_e30[3], 0.0, 1.0) * 255.0)) << 24);
    float2 _e32 = f2_;
    u = uint((int(round(clamp(_e32[0], -1.0, 1.0) * 32767.0)) & 0xFFFF) | ((int(round(clamp(_e32[1], -1.0, 1.0) * 32767.0)) & 0xFFFF) << 16));
    float2 _e34 = f2_;
    u = (uint(round(clamp(_e34[0], 0.0, 1.0) * 65535.0)) | uint(round(clamp(_e34[1], 0.0, 1.0) * 65535.0)) << 16);
    float2 _e36 = f2_;
    u = (f32tof16(_e36[0]) | f32tof16(_e36[1]) << 16);
    uint _e38 = u;
    f4_ = (float4(int4(_e38 << 24, _e38 << 16, _e38 << 8, _e38) >> 24) / 127.0);
    uint _e40 = u;
    f4_ = (float4(_e40 & 0xFF, _e40 >> 8 & 0xFF, _e40 >> 16 & 0xFF, _e40 >> 24) / 255.0);
    uint _e42 = u;
    f2_ = (float2(int2(_e42 << 16, _e42) >> 16) / 32767.0);
    uint _e44 = u;
    f2_ = (float2(_e44 & 0xFFFF, _e44 >> 16) / 65535.0);
    uint _e46 = u;
    f2_ = float2(f16tof32(_e46), f16tof32((_e46) >> 16));
    int _e48 = i;
    int _e49 = i;
    i = naga_insertBits(_e48, _e49, 5u, 10u);
    int2 _e53 = i2_;
    int2 _e54 = i2_;
    i2_ = naga_insertBits(_e53, _e54, 5u, 10u);
    int3 _e58 = i3_;
    int3 _e59 = i3_;
    i3_ = naga_insertBits(_e58, _e59, 5u, 10u);
    int4 _e63 = i4_;
    int4 _e64 = i4_;
    i4_ = naga_insertBits(_e63, _e64, 5u, 10u);
    uint _e68 = u;
    uint _e69 = u;
    u = naga_insertBits(_e68, _e69, 5u, 10u);
    uint2 _e73 = u2_;
    uint2 _e74 = u2_;
    u2_ = naga_insertBits(_e73, _e74, 5u, 10u);
    uint3 _e78 = u3_;
    uint3 _e79 = u3_;
    u3_ = naga_insertBits(_e78, _e79, 5u, 10u);
    uint4 _e83 = u4_;
    uint4 _e84 = u4_;
    u4_ = naga_insertBits(_e83, _e84, 5u, 10u);
    int _e88 = i;
    i = naga_extractBits(_e88, 5u, 10u);
    int2 _e92 = i2_;
    i2_ = naga_extractBits(_e92, 5u, 10u);
    int3 _e96 = i3_;
    i3_ = naga_extractBits(_e96, 5u, 10u);
    int4 _e100 = i4_;
    i4_ = naga_extractBits(_e100, 5u, 10u);
    uint _e104 = u;
    u = naga_extractBits(_e104, 5u, 10u);
    uint2 _e108 = u2_;
    u2_ = naga_extractBits(_e108, 5u, 10u);
    uint3 _e112 = u3_;
    u3_ = naga_extractBits(_e112, 5u, 10u);
    uint4 _e116 = u4_;
    u4_ = naga_extractBits(_e116, 5u, 10u);
    int _e120 = i;
    i = asint(firstbitlow(_e120));
    int2 _e122 = i2_;
    i2_ = asint(firstbitlow(_e122));
    int3 _e124 = i3_;
    i3_ = asint(firstbitlow(_e124));
    int4 _e126 = i4_;
    i4_ = asint(firstbitlow(_e126));
    uint _e128 = u;
    i = int(firstbitlow(_e128));
    uint2 _e131 = u2_;
    i2_ = int2(firstbitlow(_e131));
    uint3 _e134 = u3_;
    i3_ = int3(firstbitlow(_e134));
    uint4 _e137 = u4_;
    i4_ = int4(firstbitlow(_e137));
    int _e140 = i;
    i = asint(firstbithigh(_e140));
    int2 _e142 = i2_;
    i2_ = asint(firstbithigh(_e142));
    int3 _e144 = i3_;
    i3_ = asint(firstbithigh(_e144));
    int4 _e146 = i4_;
    i4_ = asint(firstbithigh(_e146));
    uint _e148 = u;
    i = int(firstbithigh(_e148));
    uint2 _e151 = u2_;
    i2_ = int2(firstbithigh(_e151));
    uint3 _e154 = u3_;
    i3_ = int3(firstbithigh(_e154));
    uint4 _e157 = u4_;
    i4_ = int4(firstbithigh(_e157));
    return;
}

void main()
{
    main_1();
    return;
}
