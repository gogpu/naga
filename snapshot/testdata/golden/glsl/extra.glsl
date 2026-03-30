#version 330 core
struct ImmediateData {
    uint index;
    vec2 double_;
};
struct FragmentIn {
    vec4 color;
    uint primitive_index;
};
uniform ImmediateData _immediates_binding_fs;

smooth in vec4 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    FragmentIn in_ = FragmentIn(_vs2fs_location0, gl_PrimitiveID);
    uint _e4 = _immediates_binding_fs.index;
    if ((in_.primitive_index == _e4)) {
        _fs2p_location0 = in_.color;
        return;
    } else {
        _fs2p_location0 = vec4((vec3(1.0) - in_.color.xyz), in_.color.w);
        return;
    }
}

