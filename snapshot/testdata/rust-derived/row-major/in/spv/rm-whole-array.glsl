#version 450
layout(location = 0) out vec4 color;
layout(set = 0, binding = 0, std140) uniform Data {
    layout(row_major) mat4 m[2];
} data;
float first(mat4 m[2]) {
    return m[1][0][0];
}
void main() {
    color = vec4(first(data.m));
}
