struct S {
    float f;
    int i;
    uint u;
};

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

typedef uint ret_Constructarray2_uint_[2];
ret_Constructarray2_uint_ Constructarray2_uint_(uint arg0, uint arg1) {
    uint ret[2] = { arg0, arg1 };
    return ret;
}

S ConstructS(float arg0, int arg1, uint arg2) {
    S ret = (S)0;
    ret.f = arg0;
    ret.i = arg1;
    ret.u = arg2;
    return ret;
}

static const int2 xvipaiai = int2(int(42), int(43));
static const uint2 xvupaiai = uint2(44u, 45u);
static const float2 xvfpaiai = float2(46.0, 47.0);
static const float2 xvfpafaf = float2(48.0, 49.0);
static const float2 xvfpaiaf = float2(48.0, 49.0);
static const uint2 xvupuai = uint2(42u, 43u);
static const uint2 xvupaiu = uint2(42u, 43u);
static const uint2 xvuuai = uint2(42u, 43u);
static const uint2 xvuaiu = uint2(42u, 43u);
static const int2 xvip = int2(int(0), int(0));
static const uint2 xvup = uint2(0u, 0u);
static const float2 xvfp = float2(0.0, 0.0);
static const float2x2 xmfp = float2x2(float2(0.0, 0.0), float2(0.0, 0.0));
static const float2x2 xmfpaiaiaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
static const float2x2 xmfpafaiaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
static const float2x2 xmfpaiafaiai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
static const float2x2 xmfpaiaiafai = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
static const float2x2 xmfpaiaiaiaf = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
static const int2 ivis_ai = (int(1)).xx;
static const uint2 ivus_ai = (1u).xx;
static const float2 ivfs_ai = (1.0).xx;
static const float2 ivfs_af = (1.0).xx;
static const float iafafaf[2] = Constructarray2_float_(1.0, 2.0);
static const float iafaiai[2] = Constructarray2_float_(1.0, 2.0);
static const int xaipaiai[2] = Constructarray2_int_(int(1), int(2));
static const uint xaupaiai[2] = Constructarray2_uint_(1u, 2u);
static const float xafpaiaf[2] = Constructarray2_float_(1.0, 2.0);
static const float xafpafai[2] = Constructarray2_float_(1.0, 2.0);
static const float xafpafaf[2] = Constructarray2_float_(1.0, 2.0);
static const S s_f_i_u = ConstructS(1.0, int(1), 1u);
static const S s_f_iai = ConstructS(1.0, int(1), 1u);
static const S s_fai_u = ConstructS(1.0, int(1), 1u);
static const S s_faiai = ConstructS(1.0, int(1), 1u);
static const S saf_i_u = ConstructS(1.0, int(1), 1u);
static const S saf_iai = ConstructS(1.0, int(1), 1u);
static const S safai_u = ConstructS(1.0, int(1), 1u);
static const S safaiai = ConstructS(1.0, int(1), 1u);
static const int2 xvisai = (int(1)).xx;
static const uint2 xvusai = (1u).xx;
static const float2 xvfsai = (1.0).xx;
static const float2 xvfsaf = (1.0).xx;
static const float3 ivfr_f_f = float3(float2(1.0, 2.0), 3.0);
static const float3 ivfr_f_af = float3(float2(1.0, 2.0), 3.0);
static const float3 ivfraf_f = float3(float2(1.0, 2.0), 3.0);
static const float3 ivfraf_af = float3(float2(1.0, 2.0), 3.0);
static const float3 ivf_fr_f = float3(1.0, float2(2.0, 3.0));
static const float3 ivf_fraf = float3(1.0, float2(2.0, 3.0));
static const float3 ivf_afr_f = float3(1.0, float2(2.0, 3.0));
static const float3 ivf_afraf = float3(1.0, float2(2.0, 3.0));
static const float3 ivfr_f_ai = float3(float2(1.0, 2.0), 3.0);
static const float3 ivfrai_f = float3(float2(1.0, 2.0), 3.0);
static const float3 ivfrai_ai = float3(float2(1.0, 2.0), 3.0);
static const float3 ivf_frai = float3(1.0, float2(2.0, 3.0));
static const float3 ivf_air_f = float3(1.0, float2(2.0, 3.0));
static const float3 ivf_airai = float3(1.0, float2(2.0, 3.0));
