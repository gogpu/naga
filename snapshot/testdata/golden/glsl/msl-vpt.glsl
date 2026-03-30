#version 330 core
struct VertexOutput {
    vec4 position;
    vec4 color;
    vec2 texcoord;
};
struct VertexInput {
    vec4 position;
    vec3 normal;
    vec2 texcoord;
};
layout(std140) uniform type_3_block_0Vertex { mat4x4 _group_0_binding_0_vs; };

layout(location = 0) in vec4 _p2vs_location0;
layout(location = 1) in vec3 _p2vs_location1;
layout(location = 2) in vec2 _p2vs_location2;
smooth out vec4 _vs2fs_location0;
smooth out vec2 _vs2fs_location1;

vec4 do_lighting(vec4 position, vec3 normal) {
    return vec4(0.0);
}

void main() {
    VertexInput v_in = VertexInput(_p2vs_location0, _p2vs_location1, _p2vs_location2);
    uint v_existing_id = uint(gl_VertexID);
    VertexOutput v_out_1 = VertexOutput(vec4(0.0), vec4(0.0), vec2(0.0));
    mat4x4 _e6 = _group_0_binding_0_vs;
    v_out_1.position = (v_in.position * _e6);
    vec4 _e11 = do_lighting(v_in.position, v_in.normal);
    v_out_1.color = _e11;
    v_out_1.texcoord = v_in.texcoord;
    VertexOutput _e14 = v_out_1;
    gl_Position = _e14.position;
    _vs2fs_location0 = _e14.color;
    _vs2fs_location1 = _e14.texcoord;
    return;
}

