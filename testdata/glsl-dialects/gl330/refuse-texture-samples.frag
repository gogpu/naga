#version 330 core

// textureSamples is GLSL 4.00 (design §15).
uniform sampler2DMS tex;

layout(location = 0) out ivec4 count;

void main() {
    count = ivec4(textureSamples(tex), 0, 0, 0);
}
