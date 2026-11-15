// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void exact(
    int a
) {
    int a_1 = {};
    a_1 = a;
    return;
}

void implicit(
    float a_2
) {
    float a_3 = {};
    a_3 = a_2;
    return;
}

void implicit_dims(
    metal::float3 v
) {
    metal::float3 v_1 = {};
    v_1 = v;
    return;
}

void main_1(
) {
    exact(1);
    implicit(1.0);
    implicit_dims(metal::float3(1.0));
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
