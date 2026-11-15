#version 300 es

// ES 3.00 has no layout(binding=) (design §15's first row).
precision highp float;

layout(binding = 0) uniform sampler2D tex;

out vec4 color;

void main() {
    color = texture(tex, vec2(0.5));
}
