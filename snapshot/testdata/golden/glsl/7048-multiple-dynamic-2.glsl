#version 330 core
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    int index_0_1 = 0;
    vec2 my_array[2] = vec2[2](vec2(0.0, 0.0), vec2(0.0, 0.0));
    int _e9 = index_0_1;
    vec2 val_0_ = my_array[_e9];
    int _e11 = index_0_1;
    vec2 val_1_ = my_array[_e11];
    _fs2p_location0 = (val_0_ * val_1_).xxyy;
    return;
}

