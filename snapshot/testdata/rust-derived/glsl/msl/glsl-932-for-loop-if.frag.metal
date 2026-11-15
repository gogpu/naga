// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    int i = 0;
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e6 = i;
            i = as_type<int>(as_type<uint>(_e6) + as_type<uint>(1));
        }
        loop_init = false;
        int _e2 = i;
        if (!((_e2 < 1))) {
            break;
        }
    }
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
