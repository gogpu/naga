struct VertexData {
    float2 position;
    float2 a;
};

struct FragmentData {
    float2 position;
    float2 a;
};

struct TestStruct {
    float a;
    float b;
};

struct LightScatteringParams {
    float BetaRay;
    int _pad1_0;
    int _pad1_1;
    int _pad1_2;
    float BetaMie[3];
    int _pad2_0;
    int _pad2_1;
    int _pad2_2;
    float HGg;
    int _pad3_0;
    int _pad3_1;
    int _pad3_2;
    float DistanceMul[4];
    int _pad4_0;
    int _pad4_1;
    int _pad4_2;
    float BlendCoeff;
    int _pad5_0;
    int _pad5_1;
    int _pad5_2;
    float3 SunDirection;
    int _pad6_0;
    float3 SunColor;
    int _end_pad_0;
};

struct FragmentOutput {
    float2 position : SV_Target0;
    float2 a : SV_Target1;
    float4 out_array : SV_Target2;
    float4 out_array_1 : SV_Target3;
};

static VertexData vert = (VertexData)0;
static FragmentData frag = (FragmentData)0;
static float4 in_array_2[2] = (float4[2])0;
static float4 out_array[2] = (float4[2])0;
static float array_2d[2][2] = (float[2][2])0;
static float array_toomanyd[2][2][2][2][2][2][2] = (float[2][2][2][2][2][2][2])0;

struct FragmentInput_main {
    float2 position_1 : LOC0;
    float2 a_2 : LOC1;
    float4 in_array_3 : LOC2;
    float4 in_array_4 : LOC3;
};

typedef float3 ret_Constructarray2_float3_[2];
ret_Constructarray2_float3_ Constructarray2_float3_(float3 arg0, float3 arg1) {
    float3 ret[2] = { arg0, arg1 };
    return ret;
}

TestStruct ConstructTestStruct(float arg0, float arg1) {
    TestStruct ret = (TestStruct)0;
    ret.a = arg0;
    ret.b = arg1;
    return ret;
}

void main_1()
{
    float3 positions[2] = Constructarray2_float3_(float3(-1.0, 1.0, 0.0), float3(-1.0, -1.0, 0.0));
    TestStruct strct = ConstructTestStruct(1.0, 2.0);
    float4 from_input_array = (float4)0;
    float a_1 = (float)0;
    float b = (float)0;
    LightScatteringParams light_scattering_params = (LightScatteringParams)0;

    float4 _e17 = in_array_2[1];
    from_input_array = _e17;
    float _e21 = array_2d[0][0];
    a_1 = _e21;
    float _e30 = array_toomanyd[0][0][0][0][0][0][0];
    b = _e30;
    out_array[int(0)] = (2.0).xxxx;
    return;
}

FragmentOutput ConstructFragmentOutput(float2 arg0, float2 arg1, float4 arg2, float4 arg3) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.position = arg0;
    ret.a = arg1;
    ret.out_array = arg2;
    ret.out_array_1 = arg3;
    return ret;
}

FragmentOutput main(FragmentInput_main fragmentinput_main)
{
    float2 position = fragmentinput_main.position_1;
    float2 a = fragmentinput_main.a_2;
    float4 in_array = fragmentinput_main.in_array_3;
    float4 in_array_1 = fragmentinput_main.in_array_4;
    vert.position = position;
    vert.a = a;
    in_array_2[0] = in_array;
    in_array_2[1] = in_array_1;
    main_1();
    float2 _e12 = frag.position;
    float2 _e14 = frag.a;
    float4 _e17 = out_array[0];
    float4 _e19 = out_array[1];
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e12, _e14, _e17, _e19);
    return fragmentoutput;
}
