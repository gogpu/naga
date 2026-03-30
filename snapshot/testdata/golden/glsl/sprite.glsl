#version 330 core
uniform sampler2D _group_0_binding_0_fs;

smooth in vec2 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 uv = _vs2fs_location0;
    vec4 _e3 = texture(_group_0_binding_0_fs, vec2(uv));
    _fs2p_location0 = _e3;
    return;
}

