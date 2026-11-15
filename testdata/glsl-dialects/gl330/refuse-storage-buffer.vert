#version 330 core

// Storage buffers (the `buffer` block) are GLSL 4.30 / ES 3.10 (design §15);
// 3.30 core does not even have the keyword.
buffer Data {
    vec4 items[];
};

layout(location = 0) in vec3 position;

void main() {
    gl_Position = items[0] + vec4(position, 1.0);
}
