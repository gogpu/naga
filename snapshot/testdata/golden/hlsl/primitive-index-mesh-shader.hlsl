struct VertexOutput {
    float4 position : SV_Position;
};

struct PrimitiveOutput {
    uint3 indices_ : SV_Position;
    uint index : SV_PrimitiveID;
};

struct MeshOutput {
    VertexOutput vertices_[3] : SV_Position;
    PrimitiveOutput primitives_[1] : SV_Position;
    uint vertex_count : SV_Position;
    uint primitive_count : SV_Position;
};

groupshared MeshOutput mesh_output;

struct FragmentInput_fs_main {
    uint index_1 : SV_PrimitiveID;
};

void ms_main()
{
    mesh_output.vertex_count = 3u;
    mesh_output.primitive_count = 1u;
    mesh_output.vertices_[0].position = float4(0.0, 1.0, 0.0, 1.0);
    mesh_output.vertices_[1].position = float4(-1.0, -1.0, 0.0, 1.0);
    mesh_output.vertices_[2].position = float4(1.0, -1.0, 0.0, 1.0);
    mesh_output.primitives_[0].indices_ = uint3(0u, 1u, 2u);
    mesh_output.primitives_[0].index = 7u;
    return;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    uint index = fragmentinput_fs_main.index_1;
    return float4(float(index), 1.0, 1.0, 1.0);
}
