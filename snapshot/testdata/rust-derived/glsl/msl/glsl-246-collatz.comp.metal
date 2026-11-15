// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct _mslBufferSizes {
    uint size0;
};

typedef uint type_1[1];
struct PrimeIndices {
    type_1 indices;
};

uint naga_div(uint lhs, uint rhs) {
    return lhs / metal::select(rhs, 1u, rhs == 0u);
}

uint collatz_iterations(
    uint n
) {
    uint n_1 = {};
    uint i = 0u;
    n_1 = n;
    uint2 loop_bound = uint2(4294967295u);
    while(true) {
        if (metal::all(loop_bound == uint2(0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        uint _e4 = n_1;
        if (!((_e4 != 1u))) {
            break;
        }
        {
            uint _e8 = n_1;
            float _e9 = static_cast<float>(_e8);
            if ((_e9 - (metal::floor(_e9 / 2.0) * 2.0)) == 0.0) {
                {
                    uint _e17 = n_1;
                    n_1 = naga_div(_e17, 2u);
                }
            } else {
                {
                    uint _e20 = n_1;
                    n_1 = (3u * _e20) + 1u;
                }
            }
            uint _e25 = i;
            i = _e25 + 1u;
        }
    }
    uint _e28 = i;
    return _e28;
}

void main_1(
    device PrimeIndices& global,
    thread metal::uint3& gl_GlobalInvocationID_1,
    constant _mslBufferSizes& _buffer_sizes
) {
    uint index = {};
    metal::uint3 _e3 = gl_GlobalInvocationID_1;
    index = _e3.x;
    uint _e6 = index;
    uint _e8 = index;
    uint _e10 = global.indices[_e8];
    uint _e11 = collatz_iterations(_e10);
    global.indices[_e6] = _e11;
    return;
}

struct main_Input {
};
kernel void main_(
  metal::uint3 gl_GlobalInvocationID [[thread_position_in_grid]]
, device PrimeIndices& global [[user(fake0)]]
, constant _mslBufferSizes& _buffer_sizes [[user(fake0)]]
) {
    metal::uint3 gl_GlobalInvocationID_1 = {};
    gl_GlobalInvocationID_1 = gl_GlobalInvocationID;
    main_1(global, gl_GlobalInvocationID_1, _buffer_sizes);
    return;
}
