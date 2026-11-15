struct FragmentOutput {
    float4 o_color : SV_Target0;
};

static float4 o_color = (float4)0;

float TevPerCompGT(float a, float b)
{
    float a_1 = (float)0;
    float b_1 = (float)0;

    a_1 = a;
    b_1 = b;
    float _e4 = a_1;
    float _e5 = b_1;
    return ((_e4 > _e5) ? 1.0 : 0.0);
}

float3 TevPerCompGT_1(float3 a_2, float3 b_2)
{
    float3 a_3 = (float3)0;
    float3 b_3 = (float3)0;

    a_3 = a_2;
    b_3 = b_2;
    float3 _e4 = a_3;
    float3 _e5 = b_3;
    return ((_e4 > _e5) ? (1.0).xxx : (0.0).xxx);
}

void main_1()
{
    const float3 _e5 = TevPerCompGT_1((3.0).xxx, (5.0).xxx);
    o_color.x = _e5.x;
    o_color.y = _e5.y;
    o_color.z = _e5.z;
    const float _e15 = TevPerCompGT(3.0, 5.0);
    o_color.w = _e15;
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
