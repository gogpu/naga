#version 330 core

// Disabling a recognized extension leaves the shader valid (design §15); the
// disabled name is what refuse-extension-disabled-use.frag covers.
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : disable

layout(location = 0) out vec4 color;

void main() {
    color = vec4(1.0);
}
