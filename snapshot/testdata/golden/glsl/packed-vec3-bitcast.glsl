#version 330 core
struct VertexInput {
    ivec3 chunk;
    uint texture_index;
};
struct VertexOutput {
    vec4 clip_position;
};
layout(location = 0) in ivec3 _p2vs_location0;
layout(location = 1) in uint _p2vs_location1;

void main() {
    VertexInput in_ = VertexInput(_p2vs_location0, _p2vs_location1);
    VertexOutput out_1 = VertexOutput(vec4(0.0));
    vec3 position = vec3((in_.chunk - ivec3(5)));
    VertexOutput _e7 = out_1;
    gl_Position = _e7.clip_position;
    return;
}

