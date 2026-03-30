struct TaskPayload {
    float4 colorMask;
    bool visible;
    int _end_pad_0;
    int _end_pad_1;
    int _end_pad_2;
};

struct VertexOutput {
    float4 position : SV_Position;
    float4 color : LOC0;
};

struct PrimitiveOutput {
    uint3 indices_ : SV_Position;
    bool cull : SV_Position;
    float4 colorMask : LOC1;
};

struct PrimitiveInput {
    float4 colorMask : LOC1;
};

struct MeshOutput {
    VertexOutput vertices_[3] : SV_Position;
    PrimitiveOutput primitives_[1] : SV_Position;
    uint vertex_count : SV_Position;
    uint primitive_count : SV_Position;
};

TaskPayload taskPayload;
groupshared float workgroupData;
groupshared MeshOutput mesh_output;

struct FragmentInput_fs_main {
    float4 color : LOC0;
    float4 colorMask : LOC1;
    float4 position : SV_Position;
};

uint3 ts_main() : SV_Position
{
    workgroupData = 1.0;
    taskPayload.colorMask = float4(1.0, 1.0, 0.0, 1.0);
    taskPayload.visible = true;
    return uint3(1u, 1u, 1u);
}

void ms_main(uint index : SV_GroupIndex, uint3 id : SV_DispatchThreadID)
{
    mesh_output.vertex_count = 3u;
    mesh_output.primitive_count = 1u;
    workgroupData = 2.0;
    mesh_output.vertices_[0].position = 0.0;
    float4 _e22 = taskPayload.colorMask;
    mesh_output.vertices_[0].color = (0.0 * _e22);
    mesh_output.vertices_[1].position = 1.0;
    float4 _e36 = taskPayload.colorMask;
    mesh_output.vertices_[1].color = (1.0 * _e36);
    mesh_output.vertices_[2].position = 0.0;
    float4 _e50 = taskPayload.colorMask;
    mesh_output.vertices_[2].color = (0.0 * _e50);
    mesh_output.primitives_[0].indices_ = uint3(0u, 1u, 2u);
    bool _e66 = taskPayload.visible;
    mesh_output.primitives_[0].cull = !(_e66);
    mesh_output.primitives_[0].colorMask = float4(1.0, 0.0, 1.0, 1.0);
    return;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    VertexOutput vertex = { fragmentinput_fs_main.position, fragmentinput_fs_main.color };
    PrimitiveInput primitive = { fragmentinput_fs_main.colorMask };
    return (vertex.color * primitive.colorMask);
}
