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

    float _e9 = poly.x;
    int _e10 = j;
    int _e13 = k;
    poly.x = (_e9 + (ZeroValuearray2_float3_()[min(uint(_e10), 1u)].y * ZeroValuearray2_float3_()[min(uint(_e13), 1u)].z));
    return;
}
