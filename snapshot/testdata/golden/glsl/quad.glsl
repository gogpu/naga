// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexOutput {
    vec4 position;
    vec2 uv;
};

layout(location = 0) out vec2 v_uv;

void main() {
    vec2 pos;
    vec2 uv;
    pos = vec2(((float((int(uint(gl_VertexID)) / 2)) * 4.0) - 1.0), ((float(_naga_mod(int(uint(gl_VertexID)), 2)) * 4.0) - 1.0));
    uv = ((pos + 1.0) * 0.5);
    gl_Position = vec4(pos, 0.0, 1.0);
    v_uv = uv;
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexOutput {
    vec4 position;
    vec2 uv;
};

layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = vec4(uv, 0.0, 1.0);
}
