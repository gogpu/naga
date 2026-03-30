#version 330 core
struct VertexOutput {
    vec4 position;
    float clip_distances[1];
};
out float gl_ClipDistance[1];

void main() {
    VertexOutput out_1 = VertexOutput(vec4(0.0), float[1](0.0));
    out_1.clip_distances[0] = 0.5;
    VertexOutput _e4 = out_1;
    gl_Position = _e4.position;
    gl_ClipDistance = _e4.clip_distances;
    return;
}

