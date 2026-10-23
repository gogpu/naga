// === Entry Point: vs_main (vertex) ===
#version 330 core
struct gen_gl_Light {
    vec4 gen_gl_color;
    float gen_gl_intensity;
};
struct gen_gl_VertexOut {
    vec4 gen_gl_pos;
    vec2 gen_gl_uv;
};
const float gen_gl_SCALE = 2.0;

layout(location = 0) in vec2 _p2vs_location0;
smooth out vec2 _vs2fs_location0;

vec4 gen_gl_shade(gen_gl_Light gen_gl_l, float gen__group_0_binding_0_fs) {
    vec4 gen_gl_acc = vec4(0.0);
    vec4 gen_gl_c = ((gen_gl_l.gen_gl_color * gen_gl_l.gen_gl_intensity) * gen_gl_SCALE);
    gen_gl_acc = (gen_gl_c * gen__group_0_binding_0_fs);
    vec4 _e9 = gen_gl_acc;
    return _e9;
}

void main() {
    vec2 gen_gl_in = _p2vs_location0;
    gen_gl_VertexOut gen_gl_o = gen_gl_VertexOut(vec4(0.0), vec2(0.0));
    gen_gl_o.gen_gl_pos = vec4(gen_gl_in, 0.0, 1.0);
    gen_gl_o.gen_gl_uv = gen_gl_in;
    gen_gl_VertexOut _e7 = gen_gl_o;
    gl_Position = _e7.gen_gl_pos;
    _vs2fs_location0 = _e7.gen_gl_uv;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct gen_gl_Light {
    vec4 gen_gl_color;
    float gen_gl_intensity;
};
struct gen_gl_VertexOut {
    vec4 gen_gl_pos;
    vec2 gen_gl_uv;
};
const float gen_gl_SCALE = 2.0;

layout(std140) uniform gen_gl_Light_block_0Fragment { gen_gl_Light _group_0_binding_0_fs; };

int gen_gl_counter = 0;

smooth in vec2 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

vec4 gen_gl_shade(gen_gl_Light gen_gl_l, float gen__group_0_binding_0_fs) {
    vec4 gen_gl_acc = vec4(0.0);
    vec4 gen_gl_c = ((gen_gl_l.gen_gl_color * gen_gl_l.gen_gl_intensity) * gen_gl_SCALE);
    gen_gl_acc = (gen_gl_c * gen__group_0_binding_0_fs);
    vec4 _e9 = gen_gl_acc;
    return _e9;
}

void main() {
    gen_gl_VertexOut gen_gl_v = gen_gl_VertexOut(gl_FragCoord, _vs2fs_location0);
    int _e2 = gen_gl_counter;
    gen_gl_counter = (_e2 + 1);
    int _e6 = gen_gl_counter;
    float gen__immediates_binding_fs = float(_e6);
    gen_gl_Light _e9 = _group_0_binding_0_fs;
    vec4 _e10 = gen_gl_shade(_e9, gen__immediates_binding_fs);
    _fs2p_location0 = (_e10 + vec4(gen_gl_v.gen_gl_uv, 0.0, 0.0));
    return;
}

