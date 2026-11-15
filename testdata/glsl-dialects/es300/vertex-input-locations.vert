#version 300 es

// ES 3.00 declares vertex inputs with layout(location) (GLSL ES 3.00 §4.4.1).
layout(location = 0) in vec3 position;
layout(location = 1) in vec2 texCoord;

out vec2 vUV;

void main() {
    vUV = texCoord;
    gl_Position = vec4(position, 1.0);
}
