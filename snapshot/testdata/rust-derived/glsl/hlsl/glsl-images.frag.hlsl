RWTexture1D<unorm float4> img1D : register(u0);
RWTexture2D<unorm float4> img2D : register(u1);
RWTexture3D<unorm float4> img3D : register(u2);
RWTexture2DArray<unorm float4> img2DArray : register(u5);
RWTexture2D<unorm float4> imgReadOnly : register(u7);
RWTexture2D<unorm float4> imgWriteOnly : register(u8);
RWTexture2D<unorm float4> imgWriteReadOnly : register(u9);

uint NagaRWDimensions1D(RWTexture1D<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x);
    return ret.x;
}

void testImg1D(int coord)
{
    int coord_1 = (int)0;
    int size = (int)0;
    float4 c = (float4)0;

    coord_1 = coord;
    size = int(NagaRWDimensions1D(img1D));
    int _e6 = coord_1;
    img1D[_e6] = (2.0).xxxx;
    int _e9 = coord_1;
    float4 _e10 = img1D.Load(_e9);
    c = _e10;
    return;
}

uint2 NagaRWDimensions2D(RWTexture2D<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y);
    return ret.xy;
}

void testImg2D(int2 coord_2)
{
    int2 coord_3 = (int2)0;
    float2 size_1 = (float2)0;
    float4 c_1 = (float4)0;

    coord_3 = coord_2;
    size_1 = float2(int2(NagaRWDimensions2D(img2D)));
    int2 _e7 = coord_3;
    float4 _e8 = img2D.Load(_e7);
    c_1 = _e8;
    int2 _e10 = coord_3;
    img2D[_e10] = (2.0).xxxx;
    return;
}

uint2 NagaRWDimensions2DArray(RWTexture2DArray<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y, ret.z);
    return ret.xy;
}

uint NagaRWNumLayers2DArray(RWTexture2DArray<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y, ret.z);
    return ret.z;
}

void testImg2DArray(int3 coord_4)
{
    int3 coord_5 = (int3)0;
    float3 size_2 = (float3)0;
    float4 c_2 = (float4)0;

    coord_5 = coord_4;
    uint2 _e3 = NagaRWDimensions2DArray(img2DArray);
    size_2 = float3(int3(uint3(_e3.x, _e3.y, NagaRWNumLayers2DArray(img2DArray))));
    int3 _e11 = coord_5;
    float4 _e14 = img2DArray.Load(int3(_e11.xy, _e11.z));
    c_2 = _e14;
    int3 _e16 = coord_5;
    img2DArray[int3(_e16.xy, _e16.z)] = (2.0).xxxx;
    return;
}

uint3 NagaRWDimensions3D(RWTexture3D<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y, ret.z);
    return ret.xyz;
}

void testImg3D(int3 coord_6)
{
    int3 coord_7 = (int3)0;
    float3 size_3 = (float3)0;
    float4 c_3 = (float4)0;

    coord_7 = coord_6;
    size_3 = float3(int3(NagaRWDimensions3D(img3D)));
    int3 _e7 = coord_7;
    float4 _e8 = img3D.Load(_e7);
    c_3 = _e8;
    int3 _e10 = coord_7;
    img3D[_e10] = (2.0).xxxx;
    return;
}

void testImgReadOnly(int2 coord_8)
{
    int2 coord_9 = (int2)0;
    float2 size_4 = (float2)0;
    float4 c_4 = (float4)0;

    coord_9 = coord_8;
    size_4 = float2(int2(NagaRWDimensions2D(img2D)));
    int2 _e8 = coord_9;
    float4 _e9 = imgReadOnly.Load(_e8);
    c_4 = _e9;
    return;
}

void testImgWriteOnly(int2 coord_10)
{
    int2 coord_11 = (int2)0;
    float2 size_5 = (float2)0;

    coord_11 = coord_10;
    size_5 = float2(int2(NagaRWDimensions2D(img2D)));
    int2 _e8 = coord_11;
    imgWriteOnly[_e8] = (2.0).xxxx;
    return;
}

uint2 NagaRWDimensions2D(RWTexture2D<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y);
    return ret.xy;
}

void testImgWriteReadOnly(int2 coord_12)
{
    int2 coord_13 = (int2)0;
    float2 size_6 = (float2)0;

    coord_13 = coord_12;
    size_6 = float2(int2(NagaRWDimensions2D(imgWriteReadOnly)));
    return;
}

void main_1()
{
    testImg1D(int(1));
    testImg2D((int(0)).xx);
    testImg2DArray((int(0)).xxx);
    testImg3D((int(0)).xxx);
    testImgReadOnly((int(0)).xx);
    testImgWriteOnly((int(0)).xx);
    testImgWriteReadOnly((int(0)).xx);
    return;
}

void main()
{
    main_1();
    return;
}
