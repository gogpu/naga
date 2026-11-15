// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_Target;
};

void main_1(
    thread metal::float4& o_Target
) {
    o_Target = metal::float4(0.0);
    return;
}

struct main_Output {
    metal::float4 o_Target [[color(0)]];
};
fragment main_Output main_(
) {
    metal::float4 o_Target = {};
    main_1(o_Target);
    metal::float4 _e1 = o_Target;
    const auto _tmp = FragmentOutput {_e1};
    return main_Output { _tmp.o_Target };
}
