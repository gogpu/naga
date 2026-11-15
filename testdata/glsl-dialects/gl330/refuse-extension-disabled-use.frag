#version 330 core

// disable turns a recognized extension off (design §15): float16_t is the
// type that extension provides, so using it after the disable is an error.
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : disable

layout(location = 0) out vec4 color;

void main() {
    float16_t h = float16_t(0.5);
    color = vec4(float(h));
}
