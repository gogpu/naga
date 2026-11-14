#version 450
layout(location = 0) out vec4 color;
layout(set = 0, binding = 0, std140) uniform Data {
    layout(row_major) mat4 m[2];
} data;
void main() {
    mat4 m = data.m[1];
    color = m[0];
}
