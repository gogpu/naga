void derivatives()
{
    float x = ddx(0.0);
    float y = ddy(0.0);
    float width = fwidth(0.0);
    return;
}

void barriers()
{
    DeviceMemoryBarrierWithGroupSync();
    GroupMemoryBarrierWithGroupSync();
    DeviceMemoryBarrierWithGroupSync();
    return;
}

float4 ZeroValuefloat4() {
    return (float4)0;
}

float4 fragment() : SV_Target0
{
    derivatives();
    return ZeroValuefloat4();
}

[numthreads(1, 1, 1)]
void compute()
{
    barriers();
    return;
}
