#version 450
layout(location = 0) out vec4 color;
layout(set = 0, binding = 0, std140) uniform Data {
    layout(row_major) mat4 m;
} data;
void main() {
    color = data.m[3];
}
