// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    float a = 1.0;
    float b = 0.25;
    float c = 1.5;
    int i = 20;
    i = 0;
    float _e9 = c;
    c = _e9 - 1.0;
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e16 = i;
            i = as_type<int>(as_type<uint>(_e16) + as_type<uint>(1));
            float _e19 = b;
            b = _e19 + 0.01;
        }
        loop_init = false;
        int _e12 = i;
        if (!((_e12 < 25))) {
            break;
        }
        {
            float _e22 = a;
            a = _e22 - 0.02;
        }
    }
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
