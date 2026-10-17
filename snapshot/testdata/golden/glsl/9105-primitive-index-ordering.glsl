#version 330 core
smooth in float _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    float input_location = _vs2fs_location0;
    vec4 arbitrary_position = gl_FragCoord;
    uint index = gl_PrimitiveID;
    _fs2p_location0 = vec4(arbitrary_position.xy, input_location, float(index));
    return;
}

