struct ColorMaterial_color {
    float4 Color;
};

struct FragmentOutput {
    float4 o_Target : SV_Target0;
};

static float2 v_Uv_1 = (float2)0;
static float4 o_Target = (float4)0;
cbuffer global : register(b0, space1) { ColorMaterial_color global; }

struct FragmentInput_main {
    float2 v_Uv_2 : LOC0;
};

void main_1()
{
    float4 color = (float4)0;

    float4 _e3 = global.Color;
    color = _e3;
    float4 _e5 = color;
    o_Target = _e5;
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
