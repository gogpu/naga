#version 330 core

// layout(push_constant) is Vulkan GLSL only, and the qualifier takes no value
// at all: `push_constant = 1` is Rust naga's "Qualifier doesn't expect a value".
layout(push_constant = 1) uniform Push {
    vec4 tint;
} pc;

void main() {
    gl_Position = pc.tint;
}
