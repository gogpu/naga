struct TaskPayload {
    uint dummy;
};

struct VertexOutput {
    float4 position : SV_Position;
};

struct PrimitiveOutput {
    uint2 indices_ : SV_Position;
};

struct MeshOutput {
    VertexOutput vertices_[2] : SV_Position;
    PrimitiveOutput primitives_[1] : SV_Position;
    uint vertex_count : SV_Position;
    uint primitive_count : SV_Position;
};

TaskPayload taskPayload;
groupshared MeshOutput mesh_output;

uint3 ts_main() : SV_Position
{
    return uint3(1u, 1u, 1u);
}

void ms_main()
{
    return;
}
