// === Entry Point: ts_main (task) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct TaskPayload {
    uint dummy;
};
struct VertexOutput {
    vec4 position;
};
struct PrimitiveOutput {
    uvec2 indices;
};
struct MeshOutput {
    VertexOutput vertices[2];
    PrimitiveOutput primitives[1];
    uint vertex_count;
    uint primitive_count;
};

void main() {
    gl_UNKNOWN = uvec3(1u, 1u, 1u);
    return;
}


// === Entry Point: ms_main (mesh) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct TaskPayload {
    uint dummy;
};
struct VertexOutput {
    vec4 position;
};
struct PrimitiveOutput {
    uvec2 indices;
};
struct MeshOutput {
    VertexOutput vertices[2];
    PrimitiveOutput primitives[1];
    uint vertex_count;
    uint primitive_count;
};

void main() {
    return;
}

