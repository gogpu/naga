#version 300 es

// A default precision statement names a sampler type as well as int and
// float (GLSL ES 3.00 §4.5.4).
precision highp float;
precision lowp sampler2D;

uniform sampler2D tex;

in vec2 vUV;

out vec4 color;

void main() {
    color = texture(tex, vUV);
}
