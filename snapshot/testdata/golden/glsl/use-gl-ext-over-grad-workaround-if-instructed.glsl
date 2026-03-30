#version 330 core
uniform sampler2DArrayShadow _group_0_binding_0_fs;

layout(location = 0) out float _fs2p_location0;

void main() {
    vec2 pos = vec2(0.0);
    float _e6 = textureGrad(_group_0_binding_0_fs, vec4(pos, 0, 0.0), vec2(0.0), vec2(0.0));
    _fs2p_location0 = _e6;
    return;
}

