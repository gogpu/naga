#version 300 es

// require of an extension the frontend does not know: accepted as a no-op with
// a warning (deliberate deviation from GLSL 4.60 §3.3, card P2-GLSLIN-022; the
// frontend cannot enumerate the Khronos extension registry).
#extension GL_EXT_not_a_real_extension : require

precision highp float;

out vec4 color;

void main() {
    color = vec4(1.0);
}
