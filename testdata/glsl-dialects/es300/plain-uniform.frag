#version 300 es

// GLSL ES 3.00 has the same default uniform block, laid out std140: the plain
// uniforms below become members of the generated block (ADR-013), and their
// offsets are cross-checked against glslangValidator in
// snapshot/glsl_plain_uniform_test.go.
precision highp float;

uniform float scale;
uniform vec2 offset;
uniform float weights[4];

out vec4 color;

void main() {
    color = vec4(scale, offset, 1.0) + vec4(weights[0]) + vec4(weights[1]);
}
