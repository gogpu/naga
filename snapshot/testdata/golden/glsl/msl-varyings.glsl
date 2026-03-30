// === Entry Point: vs_main (vertex) ===
#version 330 core
struct Vertex {
    vec2 position;
};
struct NoteInstance {
    vec2 position;
};
struct VertexOutput {
    vec4 position;
};
layout(location = 0) in vec2 _p2vs_location0;
layout(location = 1) in vec2 _p2vs_location1;

void main() {
    Vertex vertex = Vertex(_p2vs_location0);
    Vertex note = Vertex(_p2vs_location1);
    VertexOutput out_ = VertexOutput(vec4(0.0));
    VertexOutput _e3 = out_;
    gl_Position = _e3.position;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct Vertex {
    vec2 position;
};
struct NoteInstance {
    vec2 position;
};
struct VertexOutput {
    vec4 position;
};
smooth in vec2 _vs2fs_location1;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    VertexOutput in_ = VertexOutput(gl_FragCoord);
    Vertex note_1 = Vertex(_vs2fs_location1);
    vec3 position = vec3(1.0);
    _fs2p_location0 = in_.position;
    return;
}

