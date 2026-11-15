// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    metal::float4x4 a = metal::float4x4(metal::float4(1.0, 0.0, 0.0, 0.0), metal::float4(0.0, 1.0, 0.0, 0.0), metal::float4(0.0, 0.0, 1.0, 0.0), metal::float4(0.0, 0.0, 0.0, 1.0));
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
