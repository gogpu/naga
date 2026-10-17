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

bool helper_reader()
{
    bool _e2 = taskPayload.visible;
    return _e2;
}

void helper_writer(bool value)
{
    taskPayload.visible = value;
    return;
}

uint3 ts_main() : SV_Position
{
    workgroupData = 1.0;
    taskPayload.colorMask = float4(1.0, 1.0, 0.0, 1.0);
    helper_writer(true);
    const bool _e12 = helper_reader();
    taskPayload.visible = _e12;
    return uint3(1u, 1u, 1u);
}

uint3 ts_divergent(uint3 thread_id : SV_GroupThreadID) : SV_Position
{
    if ((thread_id.x == 0u)) {
        taskPayload.colorMask = float4(1.0, 1.0, 0.0, 1.0);
        taskPayload.visible = true;
        return uint3(1u, 1u, 1u);
    }
    return uint3(2u, 2u, 2u);
}

void ms_main()
{
    mesh_output.vertex_count = 3u;
    mesh_output.primitive_count = 1u;
    workgroupData = 2.0;
    mesh_output.vertices_[0].position = float4(0.0, 1.0, 0.0, 1.0);
    float4 _e23 = taskPayload.colorMask;
    mesh_output.vertices_[0].color = (float4(0.0, 1.0, 0.0, 1.0) * _e23);
    mesh_output.vertices_[1].position = float4(-1.0, -1.0, 0.0, 1.0);
    float4 _e45 = taskPayload.colorMask;
    mesh_output.vertices_[1].color = (float4(0.0, 0.0, 1.0, 1.0) * _e45);
    mesh_output.vertices_[2].position = float4(1.0, -1.0, 0.0, 1.0);
    float4 _e67 = taskPayload.colorMask;
    mesh_output.vertices_[2].color = (float4(1.0, 0.0, 0.0, 1.0) * _e67);
    mesh_output.primitives_[0].indices_ = uint3(0u, 1u, 2u);
    const bool _e86 = helper_reader();
    mesh_output.primitives_[0].cull = !(_e86);
    mesh_output.primitives_[0].colorMask = float4(1.0, 0.0, 1.0, 1.0);
    return;
}

void ms_no_ts()
{
    mesh_output.vertex_count = 3u;
    mesh_output.primitive_count = 1u;
    workgroupData = 2.0;
    mesh_output.vertices_[0].position = float4(0.0, 1.0, 0.0, 1.0);
    mesh_output.vertices_[0].color = float4(0.0, 1.0, 0.0, 1.0);
    mesh_output.vertices_[1].position = float4(-1.0, -1.0, 0.0, 1.0);
    mesh_output.vertices_[1].color = float4(0.0, 0.0, 1.0, 1.0);
    mesh_output.vertices_[2].position = float4(1.0, -1.0, 0.0, 1.0);
    mesh_output.vertices_[2].color = float4(1.0, 0.0, 0.0, 1.0);
    mesh_output.primitives_[0].indices_ = uint3(0u, 1u, 2u);
    mesh_output.primitives_[0].cull = false;
    mesh_output.primitives_[0].colorMask = float4(1.0, 0.0, 1.0, 1.0);
    return;
}

void ms_divergent(uint3 thread_id_1 : SV_GroupThreadID)
{
    if ((thread_id_1.x == 0u)) {
        mesh_output.vertex_count = 3u;
        mesh_output.primitive_count = 1u;
        workgroupData = 2.0;
        mesh_output.vertices_[0].position = float4(0.0, 1.0, 0.0, 1.0);
        mesh_output.vertices_[0].color = float4(0.0, 1.0, 0.0, 1.0);
        mesh_output.vertices_[1].position = float4(-1.0, -1.0, 0.0, 1.0);
        mesh_output.vertices_[1].color = float4(0.0, 0.0, 1.0, 1.0);
        mesh_output.vertices_[2].position = float4(1.0, -1.0, 0.0, 1.0);
        mesh_output.vertices_[2].color = float4(1.0, 0.0, 0.0, 1.0);
        mesh_output.primitives_[0].indices_ = uint3(0u, 1u, 2u);
        mesh_output.primitives_[0].cull = false;
        mesh_output.primitives_[0].colorMask = float4(1.0, 0.0, 1.0, 1.0);
        return;
    } else {
        return;
    }
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    VertexOutput vertex = { fragmentinput_fs_main.position, fragmentinput_fs_main.color };
    PrimitiveInput primitive = { fragmentinput_fs_main.colorMask };
    return (vertex.color * primitive.colorMask);
}
