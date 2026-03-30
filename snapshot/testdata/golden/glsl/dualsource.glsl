#version 330 core
struct FragmentOutput {
    vec4 output0_;
    vec4 output1_;
};
layout(location = 0, index = 0) out vec4 _fs2p_location0;
layout(location = 0, index = 1) out vec4 _fs2p_location1;

void main() {
    FragmentOutput _tmp_return = FragmentOutput(vec4(0.4, 0.3, 0.2, 0.1), vec4(0.9, 0.8, 0.7, 0.6));
    _fs2p_location0 = _tmp_return.output0_;
    _fs2p_location1 = _tmp_return.output1_;
    return;
}

