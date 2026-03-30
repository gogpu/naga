#version 330 core
struct FragmentIn {
    float value;
    float value2_;
    vec4 position;
};
smooth in float _vs2fs_location1;
smooth in float _vs2fs_location3;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    FragmentIn v_out = FragmentIn(_vs2fs_location1, _vs2fs_location3, gl_FragCoord);
    _fs2p_location0 = vec4(v_out.value, v_out.value, v_out.value2_, v_out.value2_);
    return;
}

