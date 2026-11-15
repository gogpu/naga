// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void swizzleCallee(
    thread metal::float2& a
) {
    return;
}

void swizzleCaller(
    metal::float3 a_1
) {
    metal::float3 a_2 = {};
    metal::float2 local = {};
    a_2 = a_1;
    metal::float3 _e2 = a_2;
    local = _e2.xz;
    swizzleCallee(local);
    float _e9 = local.x;
    a_2.x = _e9;
    float _e10 = local.y;
    a_2.z = _e10;
    return;
}

void outImplicitCastCallee(
    thread uint& a_3
) {
    return;
}

void outImplicitCastCaller(
    float a_4
) {
    float a_5 = {};
    uint local_1 = {};
    a_5 = a_4;
    outImplicitCastCallee(local_1);
    uint _e3 = local_1;
    a_5 = static_cast<float>(_e3);
    return;
}

void swizzleImplicitCastCallee(
    thread metal::uint2& a_6
) {
    return;
}

void swizzleImplicitCastCaller(
    metal::float3 a_7
) {
    metal::float3 a_8 = {};
    metal::uint2 local_2 = {};
    a_8 = a_7;
    swizzleImplicitCastCallee(local_2);
    uint _e7 = local_2.x;
    a_8.x = static_cast<float>(_e7);
    uint _e9 = local_2.y;
    a_8.z = static_cast<float>(_e9);
    return;
}

void main_1(
) {
    uint a_9 = {};
    metal::uint2 b = {};
    swizzleCaller(metal::float3(0.0));
    outImplicitCastCallee(a_9);
    outImplicitCastCaller(1.0);
    swizzleImplicitCastCallee(b);
    swizzleImplicitCastCaller(metal::float3(0.0));
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
