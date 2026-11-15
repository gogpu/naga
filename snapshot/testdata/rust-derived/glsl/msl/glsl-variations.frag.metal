// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
    metal::texturecube<float, metal::access::sample> texCube
) {
    metal::int2 sizeCube = {};
    float a = 1.0;
    sizeCube = static_cast<metal::int2>(metal::uint2(texCube.get_width(0)));
    return;
}

fragment void main_(
  metal::texturecube<float, metal::access::sample> texCube [[user(fake0)]]
) {
    main_1(texCube);
    return;
}
