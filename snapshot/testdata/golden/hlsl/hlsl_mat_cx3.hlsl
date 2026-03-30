struct StructWithMat {
    row_major float3x3 m;
    int _end_pad_0;
};

struct StructWithArrayOfStructOfMat {
    StructWithMat a[4];
};

RWStructuredBuffer<float3x3> s_m : register(u0);
cbuffer u_m : register(b1) { row_major float3x3 u_m; }
RWStructuredBuffer<StructWithMat> s_sm : register(u0, space1);
cbuffer u_sm : register(b1, space1) { StructWithMat u_sm; }
RWStructuredBuffer<StructWithArrayOfStructOfMat> s_sasm : register(u0, space2);
cbuffer u_sasm : register(b1, space2) { StructWithArrayOfStructOfMat u_sasm; }

void access_m()
{
    int idx = int(1);

    int _e3 = idx;
    idx = (_e3 - int(1));
    float3x3 l_s_m = s_m;
    float3 l_s_c_c = s_m[0];
    int _e11 = idx;
    float l_s_e_cc = s_m[0].x;
    int _e20 = idx;
    float l_s_e_cv = s_m[0][min(uint(_e20), 2u)];
    int _e24 = idx;
    int _e29 = idx;
    int _e31 = idx;
    float3x3 l_u_m = u_m;
    float3 l_u_c_c = u_m[0];
    int _e40 = idx;
    float l_u_e_cc = u_m[0].x;
    int _e49 = idx;
    float l_u_e_cv = u_m[0][min(uint(_e49), 2u)];
    int _e53 = idx;
    int _e58 = idx;
    int _e60 = idx;
    s_m = l_u_m;
    s_m[0] = l_u_c_c;
    int _e67 = idx;
    s_m[min(uint(_e67), 2u)] = u_m[min(uint(_e40), 2u)];
    s_m[0].x = l_u_e_cc;
    int _e74 = idx;
    s_m[0][min(uint(_e74), 2u)] = l_u_e_cv;
    int _e77 = idx;
    s_m[min(uint(_e77), 2u)][0] = u_m[min(uint(_e53), 2u)][0];
    int _e81 = idx;
    int _e83 = idx;
    s_m[min(uint(_e81), 2u)][_e83] = u_m[min(uint(_e58), 2u)][_e60];
    return;
}

void access_sm()
{
    int idx_1 = int(1);

    int _e3 = idx_1;
    idx_1 = (_e3 - int(1));
    StructWithMat l_s_s = s_sm;
    float3x3 l_s_m_1 = s_sm.m;
    float3 l_s_c_c_1 = s_sm.m[0];
    int _e16 = idx_1;
    float3 l_s_c_v = s_sm.m[min(uint(_e16), 2u)];
    float l_s_e_cc_1 = s_sm.m[0].x;
    int _e27 = idx_1;
    float l_s_e_cv_1 = s_sm.m[0][min(uint(_e27), 2u)];
    int _e32 = idx_1;
    float l_s_e_vc = s_sm.m[min(uint(_e32), 2u)].x;
    int _e38 = idx_1;
    int _e40 = idx_1;
    float l_s_e_vv = s_sm.m[min(uint(_e38), 2u)][min(uint(_e40), 2u)];
    StructWithMat l_u_s = u_sm;
    float3x3 l_u_m_1 = u_sm.m;
    float3 l_u_c_c_1 = u_sm.m[0];
    int _e54 = idx_1;
    float3 l_u_c_v = u_sm.m[min(uint(_e54), 2u)];
    float l_u_e_cc_1 = u_sm.m[0].x;
    int _e65 = idx_1;
    float l_u_e_cv_1 = u_sm.m[0][min(uint(_e65), 2u)];
    int _e70 = idx_1;
    float l_u_e_vc = u_sm.m[min(uint(_e70), 2u)].x;
    int _e76 = idx_1;
    int _e78 = idx_1;
    float l_u_e_vv = u_sm.m[min(uint(_e76), 2u)][min(uint(_e78), 2u)];
    s_sm = l_u_s;
    s_sm.m = l_u_m_1;
    s_sm.m[0] = l_u_c_c_1;
    int _e89 = idx_1;
    s_sm.m[min(uint(_e89), 2u)] = l_u_c_v;
    s_sm.m[0].x = l_u_e_cc_1;
    int _e98 = idx_1;
    s_sm.m[0][min(uint(_e98), 2u)] = l_u_e_cv_1;
    int _e102 = idx_1;
    s_sm.m[min(uint(_e102), 2u)].x = l_u_e_vc;
    int _e107 = idx_1;
    int _e109 = idx_1;
    s_sm.m[min(uint(_e107), 2u)][min(uint(_e109), 2u)] = l_u_e_vv;
    return;
}

void access_sasm()
{
    int idx_2 = int(1);

    int _e3 = idx_2;
    idx_2 = (_e3 - int(1));
    StructWithArrayOfStructOfMat l_s_s_1 = s_sasm;
    StructWithMat l_s_a[4] = s_sasm.a;
    float3x3 l_s_m_c = s_sasm.a[0].m;
    int _e17 = idx_2;
    float3x3 l_s_m_v = s_sasm.a[min(uint(_e17), 3u)].m;
    float3 l_s_c_cc = s_sasm.a[0].m[0];
    int _e31 = idx_2;
    float3 l_s_c_cv = s_sasm.a[0].m[min(uint(_e31), 2u)];
    int _e36 = idx_2;
    float3 l_s_c_vc = s_sasm.a[min(uint(_e36), 3u)].m[0];
    int _e43 = idx_2;
    int _e46 = idx_2;
    float3 l_s_c_vv = s_sasm.a[min(uint(_e43), 3u)].m[min(uint(_e46), 2u)];
    float l_s_e_ccc = s_sasm.a[0].m[0].x;
    int _e61 = idx_2;
    float l_s_e_ccv = s_sasm.a[0].m[0][min(uint(_e61), 2u)];
    int _e68 = idx_2;
    float l_s_e_cvc = s_sasm.a[0].m[min(uint(_e68), 2u)].x;
    int _e76 = idx_2;
    int _e78 = idx_2;
    float l_s_e_cvv = s_sasm.a[0].m[min(uint(_e76), 2u)][min(uint(_e78), 2u)];
    int _e83 = idx_2;
    float l_s_e_vcc = s_sasm.a[min(uint(_e83), 3u)].m[0].x;
    int _e91 = idx_2;
    int _e95 = idx_2;
    float l_s_e_vcv = s_sasm.a[min(uint(_e91), 3u)].m[0][min(uint(_e95), 2u)];
    int _e100 = idx_2;
    int _e103 = idx_2;
    float l_s_e_vvc = s_sasm.a[min(uint(_e100), 3u)].m[min(uint(_e103), 2u)].x;
    int _e109 = idx_2;
    int _e112 = idx_2;
    int _e114 = idx_2;
    float l_s_e_vvv = s_sasm.a[min(uint(_e109), 3u)].m[min(uint(_e112), 2u)][min(uint(_e114), 2u)];
    StructWithArrayOfStructOfMat l_u_s_1 = u_sasm;
    StructWithMat l_u_a[4] = u_sasm.a;
    float3x3 l_u_m_c = u_sasm.a[0].m;
    int _e129 = idx_2;
    float3x3 l_u_m_v = u_sasm.a[min(uint(_e129), 3u)].m;
    float3 l_u_c_cc = u_sasm.a[0].m[0];
    int _e143 = idx_2;
    float3 l_u_c_cv = u_sasm.a[0].m[min(uint(_e143), 2u)];
    int _e148 = idx_2;
    float3 l_u_c_vc = u_sasm.a[min(uint(_e148), 3u)].m[0];
    int _e155 = idx_2;
    int _e158 = idx_2;
    float3 l_u_c_vv = u_sasm.a[min(uint(_e155), 3u)].m[min(uint(_e158), 2u)];
    float l_u_e_ccc = u_sasm.a[0].m[0].x;
    int _e173 = idx_2;
    float l_u_e_ccv = u_sasm.a[0].m[0][min(uint(_e173), 2u)];
    int _e180 = idx_2;
    float l_u_e_cvc = u_sasm.a[0].m[min(uint(_e180), 2u)].x;
    int _e188 = idx_2;
    int _e190 = idx_2;
    float l_u_e_cvv = u_sasm.a[0].m[min(uint(_e188), 2u)][min(uint(_e190), 2u)];
    int _e195 = idx_2;
    float l_u_e_vcc = u_sasm.a[min(uint(_e195), 3u)].m[0].x;
    int _e203 = idx_2;
    int _e207 = idx_2;
    float l_u_e_vcv = u_sasm.a[min(uint(_e203), 3u)].m[0][min(uint(_e207), 2u)];
    int _e212 = idx_2;
    int _e215 = idx_2;
    float l_u_e_vvc = u_sasm.a[min(uint(_e212), 3u)].m[min(uint(_e215), 2u)].x;
    int _e221 = idx_2;
    int _e224 = idx_2;
    int _e226 = idx_2;
    float l_u_e_vvv = u_sasm.a[min(uint(_e221), 3u)].m[min(uint(_e224), 2u)][min(uint(_e226), 2u)];
    s_sasm = l_u_s_1;
    s_sasm.a = l_u_a;
    s_sasm.a[0].m = l_u_m_c;
    int _e238 = idx_2;
    s_sasm.a[min(uint(_e238), 3u)].m = l_u_m_v;
    s_sasm.a[0].m[0] = l_u_c_cc;
    int _e250 = idx_2;
    s_sasm.a[0].m[min(uint(_e250), 2u)] = l_u_c_cv;
    int _e254 = idx_2;
    s_sasm.a[min(uint(_e254), 3u)].m[0] = l_u_c_vc;
    int _e260 = idx_2;
    int _e263 = idx_2;
    s_sasm.a[min(uint(_e260), 3u)].m[min(uint(_e263), 2u)] = l_u_c_vv;
    s_sasm.a[0].m[0].x = l_u_e_ccc;
    int _e276 = idx_2;
    s_sasm.a[0].m[0][min(uint(_e276), 2u)] = l_u_e_ccv;
    int _e282 = idx_2;
    s_sasm.a[0].m[min(uint(_e282), 2u)].x = l_u_e_cvc;
    int _e289 = idx_2;
    int _e291 = idx_2;
    s_sasm.a[0].m[min(uint(_e289), 2u)][min(uint(_e291), 2u)] = l_u_e_cvv;
    int _e295 = idx_2;
    s_sasm.a[min(uint(_e295), 3u)].m[0].x = l_u_e_vcc;
    int _e302 = idx_2;
    int _e306 = idx_2;
    s_sasm.a[min(uint(_e302), 3u)].m[0][min(uint(_e306), 2u)] = l_u_e_vcv;
    int _e310 = idx_2;
    int _e313 = idx_2;
    s_sasm.a[min(uint(_e310), 3u)].m[min(uint(_e313), 2u)].x = l_u_e_vvc;
    int _e318 = idx_2;
    int _e321 = idx_2;
    int _e323 = idx_2;
    s_sasm.a[min(uint(_e318), 3u)].m[min(uint(_e321), 2u)][min(uint(_e323), 2u)] = l_u_e_vvv;
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    access_m();
    access_sm();
    access_sasm();
    return;
}
