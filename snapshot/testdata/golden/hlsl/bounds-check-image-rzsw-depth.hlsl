Texture2D<float> image_depth_2d : register(t0);
Texture2DArray<float> image_depth_2d_array : register(t1);
Texture2DMS<float> image_depth_multisampled_2d : register(t2);

float test_textureLoad_depth_2d(int2 coords, int level)
{
    float _e3 = image_depth_2d.Load(int3(coords, level)).x;
    return _e3;
}

float test_textureLoad_depth_2d_array_u(int2 coords_1, uint index, int level_1)
{
    float _e4 = image_depth_2d_array.Load(int4(coords_1, index, level_1)).x;
    return _e4;
}

float test_textureLoad_depth_2d_array_s(int2 coords_2, int index_1, int level_2)
{
    float _e4 = image_depth_2d_array.Load(int4(coords_2, index_1, level_2)).x;
    return _e4;
}

float test_textureLoad_depth_multisampled_2d(int2 coords_3, int _sample)
{
    float _e3 = image_depth_multisampled_2d.Load(coords_3, _sample).x;
    return _e3;
}

int2 ZeroValueint2() {
    return (int2)0;
}

float4 fragment_shader() : SV_Target0
{
    const float _e2 = test_textureLoad_depth_2d(ZeroValueint2(), int(0));
    const float _e6 = test_textureLoad_depth_2d_array_u(ZeroValueint2(), 0u, int(0));
    const float _e10 = test_textureLoad_depth_2d_array_s(ZeroValueint2(), int(0), int(0));
    const float _e13 = test_textureLoad_depth_multisampled_2d(ZeroValueint2(), int(0));
    return float4(0.0, 0.0, 0.0, 0.0);
}
