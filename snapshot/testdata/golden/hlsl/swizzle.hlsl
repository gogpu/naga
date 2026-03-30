[numthreads(1, 1, 1)]
void main()
{
    float4 v4_ = float4(1.0, 2.0, 3.0, 4.0);
    float x = v4_.x;
    float y = v4_.y;
    float z = v4_.z;
    float w = v4_.w;
    float2 xy = v4_.xy;
    float3 xyz = v4_.xyz;
    float4 xyzw = v4_.xyzw;
    float2 zw = v4_.zw;
    float2 xx = v4_.xx;
    float3 yyy = v4_.yyy;
    float4 wzyx = v4_.wzyx;
    float2 yx = v4_.yx;
    float3 zxy = v4_.zxy;
    float2 v2_ = float2(1.0, 2.0);
    float2 v2_yx = v2_.yx;
    float2 v2_xx = v2_.xx;
    float3 v3_ = float3(1.0, 2.0, 3.0);
    float2 v3_zx = v3_.zx;
    float3 v3_yxz = v3_.yxz;
    return;
}
