// === Entry Point: vs_main (vertex) ===
#version 330 core
struct VertexInput {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 position;
    vec3 color;
};
layout(location = 0) in vec3 _p2vs_location0;
layout(location = 1) in vec3 _p2vs_location1;
smooth out vec3 _vs2fs_location0;

void main() {
    VertexInput in_ = VertexInput(_p2vs_location0, _p2vs_location1);
    VertexOutput out_ = VertexOutput(vec4(0.0), vec3(0.0));
    out_.position = vec4(in_.position, 1.0);
    out_.color = in_.color;
    VertexOutput _e8 = out_;
    gl_Position = _e8.position;
    _vs2fs_location0 = _e8.color;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct VertexInput {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 position;
    vec3 color;
};
smooth in vec3 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec3 color = _vs2fs_location0;
    _fs2p_location0 = vec4(color, 1.0);
    return;
}

