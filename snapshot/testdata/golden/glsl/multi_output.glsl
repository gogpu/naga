// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct FragmentOutput {
    vec4 color;
    vec4 normal;
};


void main() {
    gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct FragmentOutput {
    vec4 color;
    vec4 normal;
};

layout(location = 0) out vec4 color;
layout(location = 1) out vec4 normal;

void main() {
    FragmentOutput _out;
    _out.color = vec4(1.0, 0.0, 0.0, 1.0);
    _out.normal = vec4(0.0, 0.0, 1.0, 1.0);
    color = _out.color;
    normal = _out.normal;
}
