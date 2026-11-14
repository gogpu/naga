#version 310 es

precision highp float;
precision highp int;

struct Data {
    mat4x4 m;
};
layout(std140) uniform Data_block_0Fragment { Data _group_0_binding_0_fs; };

vec4 color = vec4(0.0);

layout(location = 0) out vec4 _fs2p_location0;

void main_1() {
    mat4x4 m = mat4x4(0.0);
    mat4x4 _e4 = _group_0_binding_0_fs.m;
    m = transpose(_e4);
    vec4 _e7 = m[1];
    color = _e7;
    return;
}

void main() {
    main_1();
    vec4 _e1 = color;
    _fs2p_location0 = _e1;
    return;
}

