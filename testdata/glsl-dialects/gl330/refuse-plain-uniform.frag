#version 330 core

// A plain default-block uniform: GLSL has one, the IR has no default-block
// member model, so the frontend refuses it instead of synthesizing a
// one-member uniform buffer (card P1-GLSLIN-021; Rust naga rejects it too).
uniform float scale;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(scale);
}
