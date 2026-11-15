#version 330 core

// layout(push_constant) is Vulkan GLSL only (design §15 "Push constants").
layout(push_constant) uniform Push {
    vec4 tint;
} pc;

layout(location = 0) out vec4 color;

void main() {
    color = pc.tint;
}
