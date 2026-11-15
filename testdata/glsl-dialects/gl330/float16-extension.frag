#version 330 core

// float16_t arrives with an extension in this dialect (design §15);
// GL_EXT_shader_explicit_arithmetic_types_float16 is on the frontend's fixed
// extension list, so the directive is accepted and the type is usable.
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : enable

layout(location = 0) out vec4 color;

void main() {
    float16_t h = float16_t(0.5);
    color = vec4(float(h));
}
