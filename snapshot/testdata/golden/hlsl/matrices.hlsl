[numthreads(1, 1, 1)]
void main()
{
    float4x4 identity = float4x4(float4(1.0, 0.0, 0.0, 0.0), float4(0.0, 1.0, 0.0, 0.0), float4(0.0, 0.0, 1.0, 0.0), float4(0.0, 0.0, 0.0, 1.0));
    float2x2 m2_ = float2x2(float2(1.0, 0.0), float2(0.0, 1.0));
    float3x3 m3_ = float3x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0));
    float4x3 m4x3_ = float4x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0), float3(0.0, 0.0, 0.0));
    float4 v = float4(1.0, 2.0, 3.0, 1.0);
    float4 transformed = mul(v, identity);
    float4x4 combined = mul(identity, identity);
    float2x2 t2_ = transpose(m2_);
    float4 col0_ = identity[0];
    float4 col1_ = identity[1];
    float element = identity[2].w;
    float2x2 scaled = mul(2.0, m2_);
    return;
}
