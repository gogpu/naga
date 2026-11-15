#version 300 es

// ES 3.00 has no storage buffers (`buffer` blocks are ES 3.10, design §15).
buffer Data {
    vec4 items[];
};

layout(location = 0) in vec3 position;

void main() {
    gl_Position = items[0] + vec4(position, 1.0);
}
