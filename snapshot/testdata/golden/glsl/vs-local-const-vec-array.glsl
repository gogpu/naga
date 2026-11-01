#version 330 core

void main() {
    uint idx = uint(gl_VertexID);
    vec2 positions[3] = vec2[3](vec2(0.0, 0.5), vec2(-0.5, -0.5), vec2(0.5, -0.5));
    vec2 _e13 = positions[idx];
    gl_Position = vec4(_e13, 0.0, 1.0);
    return;
}

