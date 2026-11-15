struct FragmentOutput {
    float4 o_color : SV_Target0;
};

static const bool SPEC_CONST_BOOL = true;
static const int SPEC_CONST_INT = int(42);
static const uint SPEC_CONST_UINT = 10u;
static const float SPEC_CONST_FLOAT = 3.14;

static float4 o_color = (float4)0;

void main_1()
{
    float result = 0.0;

    if (SPEC_CONST_BOOL) {
        {
            float _e7 = result;
            result = (_e7 + 42.0);
        }
    }
    float _e11 = result;
    result = (_e11 + 31.400002);
    float _e17 = result;
    o_color = float4(_e17, 0.0, 0.0, 1.0);
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_color = arg0;
    return ret;
}

FragmentOutput main()
{
    main_1();
    float4 _e1 = o_color;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e1);
    return fragmentoutput;
}
