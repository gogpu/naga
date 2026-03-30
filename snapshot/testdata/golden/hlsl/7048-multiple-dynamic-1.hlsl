typedef float3 ret_ZeroValuearray2_float3_[2];
ret_ZeroValuearray2_float3_ ZeroValuearray2_float3_() {
    return (float3[2])0;
}

[numthreads(1, 1, 1)]
void f()
{
    float4 poly = (0.0).xxxx;
    int k = int(0);
    int j = int(0);

    int _e9 = j;
    int _e12 = k;
    float _e16 = poly.x;
    poly.x = (_e16 + (ZeroValuearray2_float3_()[min(uint(_e9), 1u)].y * ZeroValuearray2_float3_()[min(uint(_e12), 1u)].z));
    return;
}
