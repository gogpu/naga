// === Entry Point: vs_main (vertex) ===
#version 330 core

void main() {
    uint in_vertex_index = uint(gl_VertexID);
    float x = float((int(in_vertex_index) - 1));
    float y = float(((int((in_vertex_index & 1u)) * 2) - 1));
    gl_Position = vec4(x, y, 0.0, 1.0);
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    _fs2p_location0 = vec4(1.0, 0.0, 0.0, 1.0);
    return;
}

