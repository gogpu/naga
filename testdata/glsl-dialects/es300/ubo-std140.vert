#version 300 es

// ES 3.00 has uniform blocks with the std140 layout and no layout(binding=):
// the caller's ResourceBindings binds the block.
uniform Globals {
    mat4 mvp;
    vec4 tint;
};

layout(location = 0) in vec3 position;

out vec4 vColor;

void main() {
    vColor = tint;
    gl_Position = mvp * vec4(position, 1.0);
}
