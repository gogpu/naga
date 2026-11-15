#version 300 es

// Push constants are Vulkan GLSL only (design §15 "Push constants").
precision highp float;

layout(push_constant) uniform Push {
    vec4 tint;
} pc;

out vec4 color;

void main() {
    color = pc.tint;
}
