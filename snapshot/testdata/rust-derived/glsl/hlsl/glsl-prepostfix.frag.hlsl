void main_1()
{
    int scalar_target = (int)0;
    int scalar = int(1);
    uint2 vec_target = (uint2)0;
    uint2 vec = (1u).xx;
    float4x3 mat_target = (float4x3)0;
    float4x3 mat = float4x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0), float3(0.0, 0.0, 0.0));

    int _e3 = scalar;
    scalar = asint(asuint(_e3) + asuint(int(1)));
    scalar_target = _e3;
    int _e6 = scalar;
    int _e8 = asint(asuint(_e6) - asuint(int(1)));
    scalar = _e8;
    scalar_target = _e8;
    uint2 _e13 = vec;
    vec = (_e13 - (1u).xx);
    vec_target = _e13;
    uint2 _e17 = vec;
    uint2 _e20 = (_e17 + (1u).xx);
    vec = _e20;
    vec_target = _e20;
    float4x3 _e30 = mat;
    float3 _e32 = (1.0).xxx;
    mat = (_e30 + float4x3(_e32, _e32, _e32, _e32));
    mat_target = _e30;
    float4x3 _e35 = mat;
    float3 _e37 = (1.0).xxx;
    float4x3 _e39 = (_e35 - float4x3(_e37, _e37, _e37, _e37));
    mat = _e39;
    mat_target = _e39;
    return;
}

void main()
{
    main_1();
    return;
}
