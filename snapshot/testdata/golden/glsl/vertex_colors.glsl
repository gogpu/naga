// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexInput {
    vec3 position;
    vec3 color;
};

struct VertexOutput {
    vec4 position;
    vec3 color;
};

layout(location = 0) in vec3 position;
layout(location = 1) in vec3 color;
layout(location = 0) out vec3 v_color;

void main() {
    VertexOutput _out;
    _out.position = vec4(position, 1.0);
    _out.color = color;
    gl_Position = _out.position;
    v_color = _out.color;
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexInput {
    vec3 position;
    vec3 color;
};

struct VertexOutput {
    vec4 position;
    vec3 color;
};

layout(location = 0) in vec3 color;
layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = vec4(color, 1.0);
}
