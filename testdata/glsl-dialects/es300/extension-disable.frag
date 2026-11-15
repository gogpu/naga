#version 300 es

// disable is accepted in ES 3.00 whatever the extension (design §15: disable
// turns a supported extension off; it is never an error).
#extension GL_EXT_not_a_real_extension : disable

precision highp float;

out vec4 color;

void main() {
    color = vec4(1.0);
}
