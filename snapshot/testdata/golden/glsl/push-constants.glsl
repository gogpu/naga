// === Entry Point: vert_main (vertex) ===
#version 330 core
uniform uint naga_vs_first_instance;

struct ImmediateData {
    float multiplier;
};
struct FragmentIn {
    vec4 color;
};
uniform ImmediateData _immediates_binding_vs;

layout(location = 0) in vec2 _p2vs_location0;

void main() {
    vec2 pos = _p2vs_location0;
    uint ii = (uint(gl_InstanceID) + naga_vs_first_instance);
    uint vi = uint(gl_VertexID);
    float _e8 = _immediates_binding_vs.multiplier;
    gl_Position = vec4((((float(ii) * float(vi)) * _e8) * pos), 0.0, 1.0);
    return;
}


// === Entry Point: main (fragment) ===
#version 330 core
struct ImmediateData {
    float multiplier;
};
struct FragmentIn {
    vec4 color;
};
uniform ImmediateData _immediates_binding_fs;

smooth in vec4 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    FragmentIn in_ = FragmentIn(_vs2fs_location0);
    float _e4 = _immediates_binding_fs.multiplier;
    _fs2p_location0 = (in_.color * _e4);
    return;
}

