#version 450
layout(location = 0) out vec4 color;
layout(set = 0, binding = 0, std430) buffer Data {
    layout(row_major) mat4 m;
} data;
void main() {
    data.m = mat4(1.0);
    color = vec4(0.0);
}
