struct FragmentOutput {
    float4 output0_ : SV_Target0;
    float4 output1_ : SV_Target1;
};

static float4 output0_ = (float4)0;
static float4 output1_ = (float4)0;

void main_1()
{
    output0_ = float4(1.0, 0.0, 1.0, 0.0);
    output1_ = float4(0.0, 1.0, 0.0, 1.0);
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0, float4 arg1) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.output0_ = arg0;
    ret.output1_ = arg1;
    return ret;
}

FragmentOutput main()
{
    main_1();
    float4 _e1 = output0_;
    float4 _e3 = output1_;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e1, _e3);
    return fragmentoutput;
}
