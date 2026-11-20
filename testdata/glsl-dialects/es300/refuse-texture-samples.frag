#version 300 es

// textureSamples is ES 3.10, and the multisampled sampler types are reserved
// words in ES 3.00 (design §15).
precision highp float;

uniform sampler2DMS tex;

out ivec4 count;

void main() {
    count = ivec4(textureSamples(tex), 0, 0, 0);
}
