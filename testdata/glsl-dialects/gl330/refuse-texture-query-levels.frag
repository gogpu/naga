#version 330 core

// textureQueryLevels is GLSL 4.30 (design §15).
uniform sampler2D tex;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(float(textureQueryLevels(tex)));
}
