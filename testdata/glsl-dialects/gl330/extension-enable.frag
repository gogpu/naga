#version 330 core

// An extension the frontend does not recognize in "enable" behavior: a
// warning, not an error (design §15's #extension semantics).
#extension GL_EXT_not_a_real_extension : enable

layout(location = 0) out vec4 color;

void main() {
    color = vec4(1.0);
}
