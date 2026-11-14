#version 310 es

precision highp float;
precision highp int;

struct Data {
    mat4x4 m;
};
vec4 color = vec4(0.0);

layout(std140) uniform Data_block_0Fragment { Data _group_0_binding_0_fs; };

layout(location = 0) out vec4 _fs2p_location0;

void main_1() {
    mat4x4 _e3 = _group_0_binding_0_fs.m;
    color = transpose(_e3)[3];
    return;
}

void main() {
    main_1();
    vec4 _e1 = color;
    _fs2p_location0 = _e1;
    return;
}

