#version 330 core

// textureGather is GLSL 4.00 (design §15); the frontend learned it for the
// Vulkan dialect in Phase 3, and the GL dialect gate must refuse it here.
uniform sampler2D tex;

layout(location = 0) out vec4 color;

void main() {
    color = textureGather(tex, vec2(0.5), 0);
}
