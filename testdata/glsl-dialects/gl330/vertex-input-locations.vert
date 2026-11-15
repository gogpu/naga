#version 330 core

// GLSL 3.30 core declares vertex inputs with layout(location) (GLSL 3.30
// §4.3.4). The stage's outputs have no location in 3.30; the linker assigns
// them, and the frontend numbers them in declaration order.
layout(location = 0) in vec3 position;
layout(location = 1) in vec2 texCoord;

out vec2 vUV;

void main() {
    vUV = texCoord;
    gl_Position = vec4(position, 1.0);
}
