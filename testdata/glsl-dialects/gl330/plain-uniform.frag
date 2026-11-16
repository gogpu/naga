#version 330 core

// GL 3.30 core lets a shader declare uniforms outside any block, and the IR has
// no resource for one: the frontend collects them into one generated default
// uniform block (ADR-013, gl_DefaultUniformBlock), laid out std140. The members
// below cover the layout's edges: a vec3 and an array and a matrix column are
// aligned to 16 bytes, and the vec4 after them is pushed past the vec3's slot.
// snapshot/glsl_plain_uniform_test.go cross-checks every offset against
// glslangValidator.
uniform float scale;
uniform vec3 tint;
uniform float weights[4];
uniform mat2x3 skew;
uniform vec4 tail;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(scale) + vec4(tint, 1.0) + vec4(weights[0]) + vec4(skew[0], 1.0) + tail;
}
