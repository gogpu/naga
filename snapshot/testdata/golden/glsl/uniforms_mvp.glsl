// === Entry Point: vs_main (vertex) ===
#version 330 core
struct Uniforms {
    mat4x4 model;
    mat4x4 view;
    mat4x4 projection;
};
struct VertexOutput {
    vec4 position;
    vec3 world_pos;
};
layout(std140) uniform Uniforms_block_0Vertex { Uniforms _group_0_binding_0_vs; };

layout(location = 0) in vec3 _p2vs_location0;
smooth out vec3 _vs2fs_location0;

void main() {
    vec3 position = _p2vs_location0;
    VertexOutput out_ = VertexOutput(vec4(0.0), vec3(0.0));
    mat4x4 _e3 = _group_0_binding_0_vs.model;
    vec4 world = (_e3 * vec4(position, 1.0));
    mat4x4 _e9 = _group_0_binding_0_vs.projection;
    mat4x4 _e12 = _group_0_binding_0_vs.view;
    vec4 clip = ((_e9 * _e12) * world);
    out_.position = clip;
    out_.world_pos = world.xyz;
    VertexOutput _e19 = out_;
    gl_Position = _e19.position;
    _vs2fs_location0 = _e19.world_pos;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct Uniforms {
    mat4x4 model;
    mat4x4 view;
    mat4x4 projection;
};
struct VertexOutput {
    vec4 position;
    vec3 world_pos;
};
smooth in vec3 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec3 world_pos = _vs2fs_location0;
    vec3 color = ((normalize(world_pos) * 0.5) + vec3(0.5));
    _fs2p_location0 = vec4(color, 1.0);
    return;
}

