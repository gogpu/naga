#version 330 core

// GLSL 3.30 core lets a fragment shader name its output locations (GLSL 3.30
// §4.3.8.3), which dual-target blending needs.
layout(location = 0) out vec4 color;
layout(location = 1) out vec4 bloom;

void main() {
    color = vec4(1.0);
    bloom = vec4(0.5);
}
