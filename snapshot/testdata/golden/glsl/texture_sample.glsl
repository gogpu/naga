// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexOutput {
    vec4 position;
    vec2 uv;
};

layout(binding = 0) uniform sampler2D t_diffuse_s_diffuse;

layout(location = 0) out vec2 v_uv;

void main() {
    gl_Position = vec4(vec2(((float((int(uint(gl_VertexID)) / 2)) * 4.0) - 1.0), ((float(_naga_mod(int(uint(gl_VertexID)), 2)) * 4.0) - 1.0)), 0.0, 1.0);
    v_uv = ((vec2(((float((int(uint(gl_VertexID)) / 2)) * 4.0) - 1.0), ((float(_naga_mod(int(uint(gl_VertexID)), 2)) * 4.0) - 1.0)) + 1.0) * 0.5);
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct VertexOutput {
    vec4 position;
    vec2 uv;
};

layout(binding = 0) uniform sampler2D t_diffuse_s_diffuse;

layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = texture(t_diffuse_s_diffuse, uv);
}
