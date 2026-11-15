struct FragmentOutput {
    float4 o_color : SV_Target0;
};

static float2 v_uv_1 = (float2)0;
static float4 o_color = (float4)0;

struct FragmentInput_main {
    float2 v_uv_2 : LOC0;
};

void main_1()
{
    o_color = float4(1.0, 1.0, 1.0, 1.0);
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_color = arg0;
    return ret;
}

FragmentOutput main(FragmentInput_main fragmentinput_main)
{
    float2 v_uv = fragmentinput_main.v_uv_2;
    v_uv_1 = v_uv;
    main_1();
    float4 _e3 = o_color;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e3);
    return fragmentoutput;
}
