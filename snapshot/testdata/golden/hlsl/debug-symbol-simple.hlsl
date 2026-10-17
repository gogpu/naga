struct VertexInput {
    float3 position : LOC0;
    float3 color : LOC1;
};

struct VertexOutput {
    float4 clip_position : SV_Position;
    float3 color : LOC0;
};

struct VertexOutput_vs_main {
    float3 color_1 : LOC0;
    float4 clip_position : SV_Position;
};

struct FragmentInput_fs_main {
    float3 color_2 : LOC0;
    float4 clip_position_1 : SV_Position;
};

VertexOutput_vs_main vs_main(VertexInput model)
{
    VertexOutput out_ = (VertexOutput)0;

    out_.color = model.color;
    out_.clip_position = float4(model.position, 1.0);
    VertexOutput _e8 = out_;
    const VertexOutput vertexoutput = _e8;
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.color, vertexoutput.clip_position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    VertexOutput in_ = { fragmentinput_fs_main.clip_position_1, fragmentinput_fs_main.color_2 };
    float3 color = (float3)0;
    int i = int(0);
    float ii = (float)0;

    color = in_.color;
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e23 = i;
            i = asint(asuint(_e23) + asuint(int(1)));
        }
        loop_init = false;
        int _e5 = i;
        if ((_e5 < int(10))) {
        } else {
            break;
        }
        {
            int _e8 = i;
            ii = float(_e8);
            float _e12 = color.x;
            float _e13 = ii;
            color.x = (_e12 + (_e13 * 0.001));
            float _e18 = color.y;
            float _e19 = ii;
            color.y = (_e18 + (_e19 * 0.002));
        }
    }
    float3 _e26 = color;
    return float4(_e26, 1.0);
}
