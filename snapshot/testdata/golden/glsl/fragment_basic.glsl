#version 330 core
smooth in vec4 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec4 color = _vs2fs_location0;
    _fs2p_location0 = color;
    return;
}

