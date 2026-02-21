// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable


void main() {
    gl_Position = vec4((float(uint(gl_VertexID)) / 3.0), 0.0, 0.0, 1.0);
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

layout(location = 0) in vec4 color;
layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = color;
}
