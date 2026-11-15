// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_color;
};
constant bool SPEC_CONST_BOOL = true;
constant int SPEC_CONST_INT = 42;
constant uint SPEC_CONST_UINT = 10u;
constant float SPEC_CONST_FLOAT = 3.14;

void main_1(
    thread metal::float4& o_color
) {
    float result = 0.0;
    if (SPEC_CONST_BOOL) {
        {
            float _e7 = result;
            result = _e7 + 42.0;
        }
    }
    float _e11 = result;
    result = _e11 + 31.400002;
    float _e17 = result;
    o_color = metal::float4(_e17, 0.0, 0.0, 1.0);
    return;
}

struct main_Output {
    metal::float4 o_color [[color(0)]];
};
fragment main_Output main_(
) {
    metal::float4 o_color = {};
    main_1(o_color);
    metal::float4 _e1 = o_color;
    const auto _tmp = FragmentOutput {_e1};
    return main_Output { _tmp.o_color };
}
