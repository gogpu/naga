#version 300 es

// The ES fragment stage needs a default float precision (GLSL ES 3.00 §4.5.4).
// The block-free sampler is bound by the caller's ResourceBindings and the
// split sampler by SamplerBindings.
precision highp float;

uniform sampler2D tex;

in vec2 vUV;

out vec4 color;

void main() {
    color = texture(tex, vUV);
}
