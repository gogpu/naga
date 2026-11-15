struct FragmentOutput {
    float4 o_Target : SV_Target0;
};

static float4 o_Target = (float4)0;

void main_1()
{
    o_Target = (0.0).xxxx;
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_Target = arg0;
    return ret;
}

FragmentOutput main()
{
    main_1();
    float4 _e1 = o_Target;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e1);
    return fragmentoutput;
}
