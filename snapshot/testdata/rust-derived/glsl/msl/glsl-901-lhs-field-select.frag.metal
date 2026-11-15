// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    metal::float4 a = metal::float4(1.0);
    a.x = 2.0;
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
