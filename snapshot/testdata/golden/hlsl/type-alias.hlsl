int2 ZeroValueint2() {
    return (int2)0;
}

float2x2 ZeroValuefloat2x2() {
    return (float2x2)0;
}

int3 naga_f2i32(float3 value) {
    return int3(clamp(value, -2147483600.0, 2147483500.0));
}

[numthreads(1, 1, 1)]
void main()
{
    float3 a = float3(0.0, 0.0, 0.0);
    float3 c = (0.0).xxx;
    float3 b = float3((0.0).xx, 0.0);
    float3 d = float3((0.0).xx, 0.0);
    int3 e = naga_f2i32(d);
    float2x2 f = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float3x3 g = float3x3(a, a, a);
    return;
}
