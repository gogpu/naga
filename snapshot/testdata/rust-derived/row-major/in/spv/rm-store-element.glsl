#version 450
layout(location = 0) out vec4 color;
layout(set = 0, binding = 0, std430) buffer Data {
    layout(row_major) mat4 m;
} data;
void main() {
    data.m[1][2] = 1.0;
    color = vec4(data.m[1][2]);
}
