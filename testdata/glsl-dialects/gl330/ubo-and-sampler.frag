#version 330 core

// One counter per group over every bound resource (design §11): the block and
// the texture take 0 and 1, and the split sampler follows at 2.
uniform Globals {
    vec4 tint;
};
uniform sampler2D tex;

in vec2 vUV;

layout(location = 0) out vec4 color;

void main() {
    color = texture(tex, vUV) * tint;
}
