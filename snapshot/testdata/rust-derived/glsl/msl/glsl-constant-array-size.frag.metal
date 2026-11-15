// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct type_2 {
    metal::float4 inner[42];
};
struct Data {
    type_2 vecs;
};
constant int NUM_VECS = 42;

metal::float4 function(
    constant Data& global
) {
    metal::float4 sum = metal::float4(0.0);
    int i = 0;
    uint2 loop_bound = uint2(4294967295u);
    bool loop_init = true;
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e11 = i;
            i = as_type<int>(as_type<uint>(_e11) + as_type<uint>(1));
        }
        loop_init = false;
        int _e8 = i;
        if (!((_e8 < NUM_VECS))) {
            break;
        }
        {
            metal::float4 _e14 = sum;
            int _e15 = i;
            metal::float4 _e17 = global.vecs.inner[_e15];
            sum = _e14 + _e17;
        }
    }
    metal::float4 _e19 = sum;
    return _e19;
}

void main_1(
    constant Data& global
) {
    metal::float4 _e0 = function(global);
    return;
}

fragment void main_(
  constant Data& global [[user(fake0)]]
) {
    main_1(global);
    return;
}
