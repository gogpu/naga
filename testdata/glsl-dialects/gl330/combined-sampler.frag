#version 330 core

// A combined sampler with no binding qualifier: the caller's ResourceBindings
// places the image and SamplerBindings places the sampler it is split into.
uniform sampler2D tex;

in vec2 vUV;

layout(location = 0) out vec4 color;

void main() {
    color = texture(tex, vUV);
}
