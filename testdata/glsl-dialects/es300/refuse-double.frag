#version 300 es

// ES 3.00 has no double type (design §15 "double").
precision highp float;

out vec4 color;

void main() {
    double d = double(1.0);
    color = vec4(float(d));
}
