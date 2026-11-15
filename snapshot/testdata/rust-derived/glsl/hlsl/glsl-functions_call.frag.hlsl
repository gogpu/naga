void swizzleCallee(inout float2 a)
{
    return;
}

void swizzleCaller(float3 a_1)
{
    float3 a_2 = (float3)0;
    float2 local = (float2)0;

    a_2 = a_1;
    float3 _e2 = a_2;
    local = _e2.xz;
    swizzleCallee(local);
    float _e9 = local.x;
    a_2.x = _e9;
    float _e10 = local.y;
    a_2.z = _e10;
    return;
}

void outImplicitCastCallee(inout uint a_3)
{
    return;
}

void outImplicitCastCaller(float a_4)
{
    float a_5 = (float)0;
    uint local_1 = (uint)0;

    a_5 = a_4;
    outImplicitCastCallee(local_1);
    uint _e3 = local_1;
    a_5 = float(_e3);
    return;
}

void swizzleImplicitCastCallee(inout uint2 a_6)
{
    return;
}

void swizzleImplicitCastCaller(float3 a_7)
{
    float3 a_8 = (float3)0;
    uint2 local_2 = (uint2)0;

    a_8 = a_7;
    swizzleImplicitCastCallee(local_2);
    uint _e7 = local_2.x;
    a_8.x = float(_e7);
    uint _e9 = local_2.y;
    a_8.z = float(_e9);
    return;
}

void main_1()
{
    uint a_9 = (uint)0;
    uint2 b = (uint2)0;

    swizzleCaller((0.0).xxx);
    outImplicitCastCallee(a_9);
    outImplicitCastCaller(1.0);
    swizzleImplicitCastCallee(b);
    swizzleImplicitCastCaller((0.0).xxx);
    return;
}

void main()
{
    main_1();
    return;
}
