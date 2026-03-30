Texture1D<float4> image_1d : register(t0);
Texture2D<float4> image_2d : register(t1);
Texture2DArray<float4> image_2d_array : register(t2);
Texture3D<float4> image_3d : register(t3);
Texture2DMS<float4> image_multisampled_2d : register(t4);
RWTexture1D<unorm float4> image_storage_1d : register(u5);
RWTexture2D<unorm float4> image_storage_2d : register(u6);
RWTexture2DArray<unorm float4> image_storage_2d_array : register(u7);
RWTexture3D<unorm float4> image_storage_3d : register(u8);

float4 test_textureLoad_1d(int coords, int level)
{
    float4 _e3 = image_1d.Load(int2(coords, level));
    return _e3;
}

float4 test_textureLoad_2d(int2 coords_1, int level_1)
{
    float4 _e3 = image_2d.Load(int3(coords_1, level_1));
    return _e3;
}

float4 test_textureLoad_2d_array_u(int2 coords_2, uint index, int level_2)
{
    float4 _e4 = image_2d_array.Load(int4(coords_2, index, level_2));
    return _e4;
}

float4 test_textureLoad_2d_array_s(int2 coords_3, int index_1, int level_3)
{
    float4 _e4 = image_2d_array.Load(int4(coords_3, index_1, level_3));
    return _e4;
}

float4 test_textureLoad_3d(int3 coords_4, int level_4)
{
    float4 _e3 = image_3d.Load(int4(coords_4, level_4));
    return _e3;
}

float4 test_textureLoad_multisampled_2d(int2 coords_5, int _sample)
{
    float4 _e3 = image_multisampled_2d.Load(coords_5, _sample);
    return _e3;
}

void test_textureStore_1d(int coords_6, float4 value)
{
    image_storage_1d[coords_6] = value;
    return;
}

void test_textureStore_2d(int2 coords_7, float4 value_1)
{
    image_storage_2d[coords_7] = value_1;
    return;
}

void test_textureStore_2d_array_u(int2 coords_8, uint array_index, float4 value_2)
{
    image_storage_2d_array[coords_8, array_index] = value_2;
    return;
}

void test_textureStore_2d_array_s(int2 coords_9, int array_index_1, float4 value_3)
{
    image_storage_2d_array[coords_9, array_index_1] = value_3;
    return;
}

void test_textureStore_3d(int3 coords_10, float4 value_4)
{
    image_storage_3d[coords_10] = value_4;
    return;
}

int2 ZeroValueint2() {
    return (int2)0;
}

int3 ZeroValueint3() {
    return (int3)0;
}

float4 ZeroValuefloat4() {
    return (float4)0;
}

float4 fragment_shader() : SV_Target0
{
    const float4 _e2 = test_textureLoad_1d(int(0), int(0));
    const float4 _e5 = test_textureLoad_2d(ZeroValueint2(), int(0));
    const float4 _e9 = test_textureLoad_2d_array_u(ZeroValueint2(), 0u, int(0));
    const float4 _e13 = test_textureLoad_2d_array_s(ZeroValueint2(), int(0), int(0));
    const float4 _e16 = test_textureLoad_3d(ZeroValueint3(), int(0));
    const float4 _e19 = test_textureLoad_multisampled_2d(ZeroValueint2(), int(0));
    test_textureStore_1d(int(0), ZeroValuefloat4());
    test_textureStore_2d(ZeroValueint2(), ZeroValuefloat4());
    test_textureStore_2d_array_u(ZeroValueint2(), 0u, ZeroValuefloat4());
    test_textureStore_2d_array_s(ZeroValueint2(), int(0), ZeroValuefloat4());
    test_textureStore_3d(ZeroValueint3(), ZeroValuefloat4());
    return float4(0.0, 0.0, 0.0, 0.0);
}
