#version 300 es

// textureGather is ES 3.10 (design §15); the frontend learned it for the
// Vulkan dialect in Phase 3, and the ES dialect gate must refuse it here.
precision highp float;

uniform sampler2D tex;

out vec4 color;

void main() {
    color = textureGather(tex, vec2(0.5), 0);
}
