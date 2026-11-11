struct TaskPayload {
    uint dummy;
};

struct VertexOutput {
    float4 position : SV_Position;
};

struct PrimitiveOutput {
    uint3 indices_;
};

struct MeshOutput {
    VertexOutput vertices_[3];
    PrimitiveOutput primitives_[1];
    uint vertex_count;
    uint primitive_count;
};

TaskPayload taskPayload;
groupshared MeshOutput mesh_output;

uint3 ts_main()
{
    return uint3(1u, 1u, 1u);
}

void ms_main()
{
    return;
}
