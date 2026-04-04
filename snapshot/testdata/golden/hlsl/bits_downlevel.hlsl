void main()
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

    int4 _e23 = i4_;
    u = uint((_e23[0] & 0xFF) | ((_e23[1] & 0xFF) << 8) | ((_e23[2] & 0xFF) << 16) | ((_e23[3] & 0xFF) << 24));
    uint4 _e25 = u4_;
    u = (_e25[0] & 0xFF) | ((_e25[1] & 0xFF) << 8) | ((_e25[2] & 0xFF) << 16) | ((_e25[3] & 0xFF) << 24);
    uint _e27 = u;
    f4_ = unpack_s8(_e27);
    uint _e29 = u;
    f4_ = unpack_u8(_e29);
    uint _e31 = u;
    f2_ = unpack_snorm2x16(_e31);
    uint _e33 = u;
    f2_ = unpack_unorm2x16(_e33);
    return;
}
