#version 310 es

precision highp float;
precision highp int;

struct Data {
    mat4x4 m[2];
};
vec4 color = vec4(0.0);

layout(std140) uniform Data_block_0Fragment { Data _group_0_binding_0_fs; };

layout(location = 0) out vec4 _fs2p_location0;

float first_u0028_mf44_u005b_2_u005d_u003b(inout mat4x4 m[2]) {
    float _e6 = m[1][0][0u];
    return _e6;
}

void main_1() {
    mat4x4 param[2] = mat4x4[2](mat4x4(0.0), mat4x4(0.0));
    mat4x4 _e6[2] = _group_0_binding_0_fs.m;
    param[0] = _e6[0];
    param[1] = _e6[1];
    float _e11 = first_u0028_mf44_u005b_2_u005d_u003b(param);
    color = vec4(_e11);
    return;
}

void main() {
    main_1();
    vec4 _e1 = color;
    _fs2p_location0 = _e1;
    return;
}

