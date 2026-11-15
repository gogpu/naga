struct FragmentOutput {
    float4 o_color : SV_Target0;
};

static float2 tex_coord_1 = (float2)0;
static int index_1 = (int)0;
static float4 o_color = (float4)0;

struct FragmentInput_main {
    float2 tex_coord_2 : LOC0;
    nointerpolation int index_2 : LOC1;
};

void main_1()
{
    float2 _e3 = tex_coord_1;
    int _e5 = index_1;
    o_color = float4(_e3.x, _e3.y, 0.0, float(_e5));
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_color = arg0;
    return ret;
}

FragmentOutput main(FragmentInput_main fragmentinput_main)
{
    float2 tex_coord = fragmentinput_main.tex_coord_2;
    int index = fragmentinput_main.index_2;
    tex_coord_1 = tex_coord;
    index_1 = index;
    main_1();
    float4 _e5 = o_color;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e5);
    return fragmentoutput;
}
