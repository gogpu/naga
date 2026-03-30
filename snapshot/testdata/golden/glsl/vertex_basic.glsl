#version 330 core

void main() {
    uint idx = uint(gl_VertexID);
    gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
    return;
}

