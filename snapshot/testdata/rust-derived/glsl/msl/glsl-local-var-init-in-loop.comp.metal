// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    metal::float4 sum = metal::float4(0.0);
    int i = 0;
    metal::float4 a = {};
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e9 = i;
            i = as_type<int>(as_type<uint>(_e9) + as_type<uint>(1));
        }
        loop_init = false;
        int _e5 = i;
        if (!((_e5 < 4))) {
            break;
        }
        {
            a = metal::float4(1.0);
            metal::float4 _e15 = sum;
            metal::float4 _e16 = a;
            sum = _e15 + _e16;
        }
    }
    return;
}

kernel void main_(
) {
    main_1();
    return;
}
