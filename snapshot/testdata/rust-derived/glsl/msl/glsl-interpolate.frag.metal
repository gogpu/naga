// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;

struct FragmentOutput {
    metal::float4 o_color;
};

void main_1(
    thread metal::float2& tex_coord_1,
    thread int& index_1,
    thread metal::float4& o_color
) {
    metal::float2 _e3 = tex_coord_1;
    int _e5 = index_1;
    o_color = metal::float4(_e3.x, _e3.y, 0.0, static_cast<float>(_e5));
    return;
}

struct main_Input {
    metal::float2 tex_coord [[user(loc0), center_perspective]];
    int index [[user(loc1), flat]];
};
struct main_Output {
    metal::float4 o_color [[color(0)]];
};
fragment main_Output main_(
  main_Input varyings [[stage_in]]
) {
    metal::float2 tex_coord_1 = {};
    int index_1 = {};
    metal::float4 o_color = {};
    const auto tex_coord = varyings.tex_coord;
    const auto index = varyings.index;
    tex_coord_1 = tex_coord;
    index_1 = index;
    main_1(tex_coord_1, index_1, o_color);
    metal::float4 _e5 = o_color;
    const auto _tmp = FragmentOutput {_e5};
    return main_Output { _tmp.o_color };
}
