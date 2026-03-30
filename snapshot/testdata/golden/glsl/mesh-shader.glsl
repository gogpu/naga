// === Entry Point: ts_main (task) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct TaskPayload {
    vec4 colorMask;
    bool visible;
};
struct VertexOutput {
    vec4 position;
    vec4 color;
};
struct PrimitiveOutput {
    uvec3 indices;
    bool cull;
    vec4 colorMask;
};
struct PrimitiveInput {
    vec4 colorMask;
};
struct MeshOutput {
    VertexOutput vertices[3];
    PrimitiveOutput primitives[1];
    uint vertex_count;
    uint primitive_count;
};
TaskPayload taskPayload;

shared float workgroupData;


void main() {
    workgroupData = 1.0;
    taskPayload.colorMask = vec4(1.0, 1.0, 0.0, 1.0);
    taskPayload.visible = true;
    gl_UNKNOWN = uvec3(1u, 1u, 1u);
    return;
}


// === Entry Point: ms_main (mesh) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct TaskPayload {
    vec4 colorMask;
    bool visible;
};
struct VertexOutput {
    vec4 position;
    vec4 color;
};
struct PrimitiveOutput {
    uvec3 indices;
    bool cull;
    vec4 colorMask;
};
struct PrimitiveInput {
    vec4 colorMask;
};
struct MeshOutput {
    VertexOutput vertices[3];
    PrimitiveOutput primitives[1];
    uint vertex_count;
    uint primitive_count;
};
TaskPayload taskPayload;

shared float workgroupData;

shared MeshOutput mesh_output;


void main() {
    uint index = gl_LocalInvocationIndex;
    uvec3 id = gl_GlobalInvocationID;
    mesh_output.vertex_count = 3u;
    mesh_output.primitive_count = 1u;
    workgroupData = 2.0;
    mesh_output.vertices[0].position = 0.0;
    vec4 _e22 = taskPayload.colorMask;
    mesh_output.vertices[0].color = (0.0 * _e22);
    mesh_output.vertices[1].position = 1.0;
    vec4 _e36 = taskPayload.colorMask;
    mesh_output.vertices[1].color = (1.0 * _e36);
    mesh_output.vertices[2].position = 0.0;
    vec4 _e50 = taskPayload.colorMask;
    mesh_output.vertices[2].color = (0.0 * _e50);
    mesh_output.primitives[0].indices = uvec3(0u, 1u, 2u);
    bool _e66 = taskPayload.visible;
    mesh_output.primitives[0].cull = !(_e66);
    mesh_output.primitives[0].colorMask = vec4(1.0, 0.0, 1.0, 1.0);
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct TaskPayload {
    vec4 colorMask;
    bool visible;
};
struct VertexOutput {
    vec4 position;
    vec4 color;
};
struct PrimitiveOutput {
    uvec3 indices;
    bool cull;
    vec4 colorMask;
};
struct PrimitiveInput {
    vec4 colorMask;
};
struct MeshOutput {
    VertexOutput vertices[3];
    PrimitiveOutput primitives[1];
    uint vertex_count;
    uint primitive_count;
};
smooth in vec4 _vs2fs_location0;
smooth in vec4 _vs2fs_location1;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    VertexOutput vertex = VertexOutput(gl_FragCoord, _vs2fs_location0);
    PrimitiveInput primitive = PrimitiveInput(_vs2fs_location1);
    _fs2p_location0 = (vertex.color * primitive.colorMask);
    return;
}

