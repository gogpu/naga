typedef float ret_Constructarray2_float_[2];
ret_Constructarray2_float_ Constructarray2_float_(float arg0, float arg1) {
    float ret[2] = { arg0, arg1 };
    return ret;
}

typedef int ret_Constructarray2_int_[2];
ret_Constructarray2_int_ Constructarray2_int_(int arg0, int arg1) {
    int ret[2] = { arg0, arg1 };
    return ret;
}

typedef int3 ret_Constructarray1_int3_[1];
ret_Constructarray1_int3_ Constructarray1_int3_(int3 arg0) {
    int3 ret[1] = { arg0 };
    return ret;
}

typedef float3 ret_Constructarray1_float3_[1];
ret_Constructarray1_float3_ Constructarray1_float3_(float3 arg0) {
    float3 ret[1] = { arg0 };
    return ret;
}

void all_constant_arguments()
{
    int2 xvipaiai = int2(int(42), int(43));
    uint2 xvupaiai = uint2(44u, 45u);
    float2 xvfpaiai = float2(46.0, 47.0);
    float2 xvfpafaf = float2(48.0, 49.0);
    float2 xvfpaiaf = float2(48.0, 49.0);
    uint2 xvupuai = uint2(42u, 43u);
    uint2 xvupaiu = uint2(42u, 43u);
    uint2 xvuuai = uint2(42u, 43u);
    uint2 xvuaiu = uint2(42u, 43u);
    int2 xvip = int2(int(0), int(0));
    uint2 xvup = uint2(0u, 0u);
    float2 xvfp = float2(0.0, 0.0);
    float2x2 xmfp = float2x2(float2(0.0, 0.0), float2(0.0, 0.0));
    float2x2 xmfpaiaiaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpafaiaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpaiafaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpaiaiafai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpaiaiaiaf = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfp_faiaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpai_faiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpaiai_fai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    float2x2 xmfpaiaiai_f = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    int2 xvispai = (int(1)).xx;
    float2 xvfspaf = (1.0).xx;
    int2 xvis_ai = (int(1)).xx;
    uint2 xvus_ai = (1u).xx;
    float2 xvfs_ai = (1.0).xx;
    float2 xvfs_af = (1.0).xx;
    float xafafaf[2] = Constructarray2_float_(1.0, 2.0);
    float xaf_faf[2] = Constructarray2_float_(1.0, 2.0);
    float xafaf_f[2] = Constructarray2_float_(1.0, 2.0);
    float xafaiai[2] = Constructarray2_float_(1.0, 2.0);
    int xai_iai[2] = Constructarray2_int_(int(1), int(2));
    int xaiai_i[2] = Constructarray2_int_(int(1), int(2));
    int xaipaiai[2] = Constructarray2_int_(int(1), int(2));
    float xafpaiai[2] = Constructarray2_float_(1.0, 2.0);
    float xafpaiaf[2] = Constructarray2_float_(1.0, 2.0);
    float xafpafai[2] = Constructarray2_float_(1.0, 2.0);
    float xafpafaf[2] = Constructarray2_float_(1.0, 2.0);
    int3 xavipai[1] = Constructarray1_int3_((int(1)).xxx);
    float3 xavfpai[1] = Constructarray1_float3_((1.0).xxx);
    float3 xavfpaf[1] = Constructarray1_float3_((1.0).xxx);
    int2 xvisai = (int(1)).xx;
    uint2 xvusai = (1u).xx;
    float2 xvfsai = (1.0).xx;
    float2 xvfsaf = (1.0).xx;
    int iaipaiai[2] = Constructarray2_int_(int(1), int(2));
    float iafpaiaf[2] = Constructarray2_float_(1.0, 2.0);
    float iafpafai[2] = Constructarray2_float_(1.0, 2.0);
    float iafpafaf[2] = Constructarray2_float_(1.0, 2.0);
    return;
}

void mixed_constant_and_runtime_arguments()
{
    uint u = (uint)0;
    int i = (int)0;
    float f = (float)0;

    uint _e3 = u;
    uint2 xvupuai_1 = uint2(_e3, 43u);
    uint _e6 = u;
    uint2 xvupaiu_1 = uint2(42u, _e6);
    float _e9 = f;
    float2 xvfpfai = float2(_e9, 47.0);
    float _e12 = f;
    float2 xvfpfaf = float2(_e12, 49.0);
    uint _e15 = u;
    uint2 xvuuai_1 = uint2(_e15, 43u);
    uint _e18 = u;
    uint2 xvuaiu_1 = uint2(42u, _e18);
    float _e21 = f;
    float2x2 xmfp_faiaiai_1 = float2x2(float2(_e21, 2.0), float2(3.0, 4.0));
    float _e28 = f;
    float2x2 xmfpai_faiai_1 = float2x2(float2(1.0, _e28), float2(3.0, 4.0));
    float _e35 = f;
    float2x2 xmfpaiai_fai_1 = float2x2(float2(1.0, 2.0), float2(_e35, 4.0));
    float _e42 = f;
    float2x2 xmfpaiaiai_f_1 = float2x2(float2(1.0, 2.0), float2(3.0, _e42));
    float _e49 = f;
    float xaf_faf_1[2] = Constructarray2_float_(_e49, 2.0);
    float _e52 = f;
    float xafaf_f_1[2] = Constructarray2_float_(1.0, _e52);
    float _e55 = f;
    float xaf_fai[2] = Constructarray2_float_(_e55, 2.0);
    float _e58 = f;
    float xafai_f[2] = Constructarray2_float_(1.0, _e58);
    int _e61 = i;
    int xai_iai_1[2] = Constructarray2_int_(_e61, int(2));
    int _e64 = i;
    int xaiai_i_1[2] = Constructarray2_int_(int(1), _e64);
    float _e67 = f;
    float xafp_faf[2] = Constructarray2_float_(_e67, 2.0);
    float _e70 = f;
    float xafpaf_f[2] = Constructarray2_float_(1.0, _e70);
    float _e73 = f;
    float xafp_fai[2] = Constructarray2_float_(_e73, 2.0);
    float _e76 = f;
    float xafpai_f[2] = Constructarray2_float_(1.0, _e76);
    int _e79 = i;
    int xaip_iai[2] = Constructarray2_int_(_e79, int(2));
    int _e82 = i;
    int xaipai_i[2] = Constructarray2_int_(int(1), _e82);
    int _e85 = i;
    int2 xvisi = (_e85).xx;
    uint _e87 = u;
    uint2 xvusu = (_e87).xx;
    float _e89 = f;
    float2 xvfsf = (_e89).xx;
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    all_constant_arguments();
    mixed_constant_and_runtime_arguments();
    return;
}
