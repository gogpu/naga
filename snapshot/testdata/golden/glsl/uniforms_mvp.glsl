// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct Uniforms {
    mat4 model;
    mat4 view;
    mat4 projection;
};

struct VertexOutput {
    vec4 position;
    vec3 world_pos;
};

layout(std140, binding = 0) uniform _Uniforms_ubo {
    mat4 model;
    mat4 view;
    mat4 projection;
} uniforms;

layout(location = 0) in vec3 position;
layout(location = 0) out vec3 v_world_pos;

void main() {
    VertexOutput _out;
    _out.position = ((uniforms.projection * uniforms.view) * (uniforms.model * vec4(position, 1.0)));
    _out.world_pos = (uniforms.model * vec4(position, 1.0)).xyz;
    gl_Position = _out.position;
    v_world_pos = _out.world_pos;
}

// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_separate_shader_objects : enable

struct Uniforms {
    mat4 model;
    mat4 view;
    mat4 projection;
};

struct VertexOutput {
    vec4 position;
    vec3 world_pos;
};

layout(std140, binding = 0) uniform _Uniforms_ubo {
    mat4 model;
    mat4 view;
    mat4 projection;
} uniforms;

layout(location = 0) in vec3 world_pos;
layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = vec4(((normalize(world_pos) * 0.5) + 0.5), 1.0);
}
