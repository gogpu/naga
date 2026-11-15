#version 300 es

// An extension the frontend does not recognize in "warn" behavior: a warning,
// not an error (design §15's #extension semantics).
#extension GL_EXT_not_a_real_extension : warn

precision highp float;

out vec4 color;

void main() {
    color = vec4(1.0);
}
