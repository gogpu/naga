struct FragmentOutput {
    float4 color : SV_Target0;
    float4 normal : SV_Target1;
};

struct FragmentInput_fs_main {
    float4 frag_coord_1 : SV_Position;
};

float4 vs_main(uint vi : SV_VertexID) : SV_Position
{
    return float4(0.0, 0.0, 0.0, 1.0);
}

FragmentOutput fs_main(FragmentInput_fs_main fragmentinput_fs_main)
{
    float4 frag_coord = fragmentinput_fs_main.frag_coord_1;
    FragmentOutput out_ = (FragmentOutput)0;

    out_.color = float4(1.0, 0.0, 0.0, 1.0);
    out_.normal = float4(0.0, 0.0, 1.0, 1.0);
    FragmentOutput _e14 = out_;
    const FragmentOutput fragmentoutput = _e14;
    return fragmentoutput;
}
