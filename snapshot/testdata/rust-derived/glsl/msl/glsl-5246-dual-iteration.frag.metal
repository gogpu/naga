// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    int x = 0;
    int y = {};
    int z = {};
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e6 = x;
            x = as_type<int>(as_type<uint>(_e6) + as_type<uint>(1));
        }
        loop_init = false;
        int _e2 = x;
        if (!((_e2 < 10))) {
            break;
        }
        {
            y = 0;
            uint2 loop_bound_1 = uint2(4294967295u);
            bool loop_init_1 = true;
            while(true) {
                if (metal::all(loop_bound_1 == uint2(0u))) { break; }
                loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
                if (!loop_init_1) {
                    int _e15 = y;
                    y = as_type<int>(as_type<uint>(_e15) + as_type<uint>(1));
                }
                loop_init_1 = false;
                int _e11 = y;
                if (!((_e11 < 10))) {
                    break;
                }
                {
                    z = 0;
                    uint2 loop_bound_2 = uint2(4294967295u);
                    bool loop_init_2 = true;
                    while(true) {
                        if (metal::all(loop_bound_2 == uint2(0u))) { break; }
                        loop_bound_2 -= uint2(loop_bound_2.y == 0u, 1u);
                        if (!loop_init_2) {
                            int _e24 = z;
                            z = as_type<int>(as_type<uint>(_e24) + as_type<uint>(1));
                        }
                        loop_init_2 = false;
                        int _e20 = z;
                        if (!((_e20 < 10))) {
                            break;
                        }
                    }
                }
            }
        }
    }
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
