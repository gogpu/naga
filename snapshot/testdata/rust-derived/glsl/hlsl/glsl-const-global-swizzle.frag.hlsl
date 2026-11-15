struct FragmentOutput {
    float4 o_Target : SV_Target0;
};

static const float2 blank = float2(0.0, 1.0);

static float2 v_Uv_1 = (float2)0;
static float4 o_Target = (float4)0;

struct FragmentInput_main {
    float2 v_Uv_2 : LOC0;
};

void main_1()
{
    float2 col = (float2)0;

    float2 _e3 = v_Uv_1;
    col = (_e3.xy * blank);
    float2 _e7 = col;
    o_Target = float4(_e7.x, _e7.y, 0.0, 1.0);
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_Target = arg0;
    return ret;
}

FragmentOutput main(FragmentInput_main fragmentinput_main)
{
    float2 v_Uv = fragmentinput_main.v_Uv_2;
    v_Uv_1 = v_Uv;
    main_1();
    float4 _e3 = o_Target;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e3);
    return fragmentoutput;
}
