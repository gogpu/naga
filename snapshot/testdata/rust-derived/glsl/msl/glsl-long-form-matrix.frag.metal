// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    metal::float2x2 splat = metal::float2x2(metal::float2(1.0, 0.0), metal::float2(0.0, 1.0));
    metal::float2x2 normal = metal::float2x2(metal::float2(1.0, 1.0), metal::float2(2.0, 2.0));
    metal::float2x4 from_matrix = metal::float2x4(metal::float4(1.0, 0.0, 0.0, 0.0), metal::float4(0.0, 1.0, 0.0, 0.0));
    metal::float2x2 a = metal::float2x2(metal::float2(1.0, 2.0), metal::float2(3.0, 4.0));
    metal::float2x2 b = metal::float2x2(metal::float2(1.0, 2.0), metal::float2(3.0, 4.0));
    metal::float3x3 c = metal::float3x3(metal::float3(1.0, 2.0, 3.0), metal::float3(1.0, 1.0, 1.0), metal::float3(1.0, 1.0, 1.0));
    metal::float3x3 d = metal::float3x3(metal::float3(2.0, 2.0, 1.0), metal::float3(1.0, 1.0, 1.0), metal::float3(1.0, 1.0, 1.0));
    metal::float4x4 e = metal::float4x4(metal::float4(2.0, 2.0, 1.0, 1.0), metal::float4(1.0, 1.0, 2.0, 2.0), metal::float4(1.0, 1.0, 1.0, 1.0), metal::float4(1.0, 1.0, 1.0, 1.0));
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
