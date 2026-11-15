struct Mat4x3_ {
    float4 mx;
    float4 my;
    float4 mz;
};

struct FragmentOutput {
    float4 o_color : SV_Target0;
};

static float4 o_color = (float4)0;

void Fma(inout Mat4x3_ d, Mat4x3_ m, float s)
{
    Mat4x3_ m_1 = (Mat4x3_)0;
    float s_1 = (float)0;

    m_1 = m;
    s_1 = s;
    Mat4x3_ _e6 = d;
    Mat4x3_ _e8 = m_1;
    float _e10 = s_1;
    d.mx = (_e6.mx + (_e8.mx * _e10));
    Mat4x3_ _e14 = d;
    Mat4x3_ _e16 = m_1;
    float _e18 = s_1;
    d.my = (_e14.my + (_e16.my * _e18));
    Mat4x3_ _e22 = d;
    Mat4x3_ _e24 = m_1;
    float _e26 = s_1;
    d.mz = (_e22.mz + (_e24.mz * _e26));
    return;
}

Mat4x3_ ConstructMat4x3_(float4 arg0, float4 arg1, float4 arg2) {
    Mat4x3_ ret = (Mat4x3_)0;
    ret.mx = arg0;
    ret.my = arg1;
    ret.mz = arg2;
    return ret;
}

void main_1()
{
    Mat4x3_ m1_ = ConstructMat4x3_((0.0).xxxx, (1.0).xxxx, (2.0).xxxx);
    Mat4x3_ m2_ = ConstructMat4x3_((0.0).xxxx, (1.0).xxxx, (2.0).xxxx);

    Mat4x3_ _e17 = m2_;
    Fma(m1_, _e17, 2.0);
    o_color.x = 1.0;
    o_color.y = 1.0;
    o_color.z = 1.0;
    o_color.w = 1.0;
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
