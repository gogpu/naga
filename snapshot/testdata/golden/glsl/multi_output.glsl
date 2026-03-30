// === Entry Point: vs_main (vertex) ===
#version 330 core
struct FragmentOutput {
    vec4 color;
    vec4 normal;
};

void main() {
    uint vi = uint(gl_VertexID);
    gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct FragmentOutput {
    vec4 color;
    vec4 normal;
};
layout(location = 0) out vec4 _fs2p_location0;
layout(location = 1) out vec4 _fs2p_location1;

void main() {
    vec4 frag_coord = gl_FragCoord;
    FragmentOutput out_ = FragmentOutput(vec4(0.0), vec4(0.0));
    out_.color = vec4(1.0, 0.0, 0.0, 1.0);
    out_.normal = vec4(0.0, 0.0, 1.0, 1.0);
    FragmentOutput _e14 = out_;
    _fs2p_location0 = _e14.color;
    _fs2p_location1 = _e14.normal;
    return;
}

