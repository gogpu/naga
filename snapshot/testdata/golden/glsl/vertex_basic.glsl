#version 330 core

void main() {
    uint idx = uint(gl_VertexID);
    gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
    gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
    return;
}

