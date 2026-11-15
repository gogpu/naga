// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct type_1 {
    float inner[2];
};
constant type_1 array = type_1 {{1.0, 2.0}};

void main_1(
) {
    type_1 local = array;
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
