#version 330 core

// double is GLSL 4.00 (design §15 "double"); 3.30 core has no such type.
layout(location = 0) out vec4 color;

void main() {
    double d = double(1.0);
    color = vec4(float(d));
}
