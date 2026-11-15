#version 300 es

// ES 3.00 binds fragment outputs sequentially (design §15: no layout
// qualifier, the frontend numbers them in declaration order).
precision highp float;

in vec2 vUV;

out vec4 color;

void main() {
    color = vec4(vUV, 0.0, 1.0);
}
