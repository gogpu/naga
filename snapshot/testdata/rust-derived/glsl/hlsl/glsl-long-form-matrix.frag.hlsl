void main_1()
{
    float2x2 splat = float2x2(float2(1.0, 0.0), float2(0.0, 1.0));
    float2x2 normal = float2x2(float2(1.0, 1.0), float2(2.0, 2.0));
    float2x4 from_matrix = float2x4(float4(1.0, 0.0, 0.0, 0.0), float4(0.0, 1.0, 0.0, 0.0));
    float2x2 a = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 b = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float3x3 c = float3x3(float3(1.0, 2.0, 3.0), float3(1.0, 1.0, 1.0), float3(1.0, 1.0, 1.0));
    float3x3 d = float3x3(float3(2.0, 2.0, 1.0), float3(1.0, 1.0, 1.0), float3(1.0, 1.0, 1.0));
    float4x4 e = float4x4(float4(2.0, 2.0, 1.0, 1.0), float4(1.0, 1.0, 2.0, 2.0), float4(1.0, 1.0, 1.0, 1.0), float4(1.0, 1.0, 1.0, 1.0));

    return;
}

void main()
{
    main_1();
    return;
}
