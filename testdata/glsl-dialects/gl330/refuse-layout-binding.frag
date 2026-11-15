#version 330 core

// layout(binding=) is GLSL 4.20 (design §15's first row: 3.30 core has no
// binding qualifier); the dialect's binding model is ResourceBindings.
layout(binding = 0) uniform sampler2D tex;

layout(location = 0) out vec4 color;

void main() {
    color = texture(tex, vec2(0.5));
}
