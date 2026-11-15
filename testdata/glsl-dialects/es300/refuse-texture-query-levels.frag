#version 300 es

// textureQueryLevels is ES 3.10 (design §15).
precision highp float;

uniform sampler2D tex;

out vec4 color;

void main() {
    color = vec4(float(textureQueryLevels(tex)));
}
