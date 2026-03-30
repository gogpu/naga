#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct StructWithMat {
    mat3x3 m;
};
struct StructWithArrayOfStructOfMat {
    StructWithMat a[4];
};
layout(std430) buffer Mat_block_0Compute { mat3x3 _group_0_binding_0_cs; };

layout(std140) uniform Mat_block_1Compute { mat3x3 _group_0_binding_1_cs; };

layout(std430) buffer StructWithMat_block_2Compute { StructWithMat _group_1_binding_0_cs; };

layout(std140) uniform StructWithMat_block_3Compute { StructWithMat _group_1_binding_1_cs; };

layout(std430) buffer StructWithArrayOfStructOfMat_block_4Compute { StructWithArrayOfStructOfMat _group_2_binding_0_cs; };

layout(std140) uniform StructWithArrayOfStructOfMat_block_5Compute { StructWithArrayOfStructOfMat _group_2_binding_1_cs; };


void access_m() {
    int idx = 1;
    int _e3 = idx;
    idx = (_e3 - 1);
    mat3x3 l_s_m = _group_0_binding_0_cs;
    vec3 l_s_c_c = _group_0_binding_0_cs[0];
    int _e11 = idx;
    vec3 l_s_c_v = _group_0_binding_0_cs[_e11];
    float l_s_e_cc = _group_0_binding_0_cs[0][0];
    int _e20 = idx;
    float l_s_e_cv = _group_0_binding_0_cs[0][_e20];
    int _e24 = idx;
    float l_s_e_vc = _group_0_binding_0_cs[_e24][0];
    int _e29 = idx;
    int _e31 = idx;
    float l_s_e_vv = _group_0_binding_0_cs[_e29][_e31];
    mat3x3 l_u_m = _group_0_binding_1_cs;
    vec3 l_u_c_c = _group_0_binding_1_cs[0];
    int _e40 = idx;
    vec3 l_u_c_v = _group_0_binding_1_cs[_e40];
    float l_u_e_cc = _group_0_binding_1_cs[0][0];
    int _e49 = idx;
    float l_u_e_cv = _group_0_binding_1_cs[0][_e49];
    int _e53 = idx;
    float l_u_e_vc = _group_0_binding_1_cs[_e53][0];
    int _e58 = idx;
    int _e60 = idx;
    float l_u_e_vv = _group_0_binding_1_cs[_e58][_e60];
    _group_0_binding_0_cs = l_u_m;
    _group_0_binding_0_cs[0] = l_u_c_c;
    int _e67 = idx;
    _group_0_binding_0_cs[_e67] = l_u_c_v;
    _group_0_binding_0_cs[0][0] = l_u_e_cc;
    int _e74 = idx;
    _group_0_binding_0_cs[0][_e74] = l_u_e_cv;
    int _e77 = idx;
    _group_0_binding_0_cs[_e77][0] = l_u_e_vc;
    int _e81 = idx;
    int _e83 = idx;
    _group_0_binding_0_cs[_e81][_e83] = l_u_e_vv;
    return;
}

void access_sm() {
    int idx_1 = 1;
    int _e3 = idx_1;
    idx_1 = (_e3 - 1);
    StructWithMat l_s_s = _group_1_binding_0_cs;
    mat3x3 l_s_m_1 = _group_1_binding_0_cs.m;
    vec3 l_s_c_c_1 = _group_1_binding_0_cs.m[0];
    int _e16 = idx_1;
    vec3 l_s_c_v_1 = _group_1_binding_0_cs.m[_e16];
    float l_s_e_cc_1 = _group_1_binding_0_cs.m[0][0];
    int _e27 = idx_1;
    float l_s_e_cv_1 = _group_1_binding_0_cs.m[0][_e27];
    int _e32 = idx_1;
    float l_s_e_vc_1 = _group_1_binding_0_cs.m[_e32][0];
    int _e38 = idx_1;
    int _e40 = idx_1;
    float l_s_e_vv_1 = _group_1_binding_0_cs.m[_e38][_e40];
    StructWithMat l_u_s = _group_1_binding_1_cs;
    mat3x3 l_u_m_1 = _group_1_binding_1_cs.m;
    vec3 l_u_c_c_1 = _group_1_binding_1_cs.m[0];
    int _e54 = idx_1;
    vec3 l_u_c_v_1 = _group_1_binding_1_cs.m[_e54];
    float l_u_e_cc_1 = _group_1_binding_1_cs.m[0][0];
    int _e65 = idx_1;
    float l_u_e_cv_1 = _group_1_binding_1_cs.m[0][_e65];
    int _e70 = idx_1;
    float l_u_e_vc_1 = _group_1_binding_1_cs.m[_e70][0];
    int _e76 = idx_1;
    int _e78 = idx_1;
    float l_u_e_vv_1 = _group_1_binding_1_cs.m[_e76][_e78];
    _group_1_binding_0_cs = l_u_s;
    _group_1_binding_0_cs.m = l_u_m_1;
    _group_1_binding_0_cs.m[0] = l_u_c_c_1;
    int _e89 = idx_1;
    _group_1_binding_0_cs.m[_e89] = l_u_c_v_1;
    _group_1_binding_0_cs.m[0][0] = l_u_e_cc_1;
    int _e98 = idx_1;
    _group_1_binding_0_cs.m[0][_e98] = l_u_e_cv_1;
    int _e102 = idx_1;
    _group_1_binding_0_cs.m[_e102][0] = l_u_e_vc_1;
    int _e107 = idx_1;
    int _e109 = idx_1;
    _group_1_binding_0_cs.m[_e107][_e109] = l_u_e_vv_1;
    return;
}

void access_sasm() {
    int idx_2 = 1;
    int _e3 = idx_2;
    idx_2 = (_e3 - 1);
    StructWithArrayOfStructOfMat l_s_s_1 = _group_2_binding_0_cs;
    StructWithMat l_s_a[4] = _group_2_binding_0_cs.a;
    mat3x3 l_s_m_c = _group_2_binding_0_cs.a[0].m;
    int _e17 = idx_2;
    mat3x3 l_s_m_v = _group_2_binding_0_cs.a[_e17].m;
    vec3 l_s_c_cc = _group_2_binding_0_cs.a[0].m[0];
    int _e31 = idx_2;
    vec3 l_s_c_cv = _group_2_binding_0_cs.a[0].m[_e31];
    int _e36 = idx_2;
    vec3 l_s_c_vc = _group_2_binding_0_cs.a[_e36].m[0];
    int _e43 = idx_2;
    int _e46 = idx_2;
    vec3 l_s_c_vv = _group_2_binding_0_cs.a[_e43].m[_e46];
    float l_s_e_ccc = _group_2_binding_0_cs.a[0].m[0][0];
    int _e61 = idx_2;
    float l_s_e_ccv = _group_2_binding_0_cs.a[0].m[0][_e61];
    int _e68 = idx_2;
    float l_s_e_cvc = _group_2_binding_0_cs.a[0].m[_e68][0];
    int _e76 = idx_2;
    int _e78 = idx_2;
    float l_s_e_cvv = _group_2_binding_0_cs.a[0].m[_e76][_e78];
    int _e83 = idx_2;
    float l_s_e_vcc = _group_2_binding_0_cs.a[_e83].m[0][0];
    int _e91 = idx_2;
    int _e95 = idx_2;
    float l_s_e_vcv = _group_2_binding_0_cs.a[_e91].m[0][_e95];
    int _e100 = idx_2;
    int _e103 = idx_2;
    float l_s_e_vvc = _group_2_binding_0_cs.a[_e100].m[_e103][0];
    int _e109 = idx_2;
    int _e112 = idx_2;
    int _e114 = idx_2;
    float l_s_e_vvv = _group_2_binding_0_cs.a[_e109].m[_e112][_e114];
    StructWithArrayOfStructOfMat l_u_s_1 = _group_2_binding_1_cs;
    StructWithMat l_u_a[4] = _group_2_binding_1_cs.a;
    mat3x3 l_u_m_c = _group_2_binding_1_cs.a[0].m;
    int _e129 = idx_2;
    mat3x3 l_u_m_v = _group_2_binding_1_cs.a[_e129].m;
    vec3 l_u_c_cc = _group_2_binding_1_cs.a[0].m[0];
    int _e143 = idx_2;
    vec3 l_u_c_cv = _group_2_binding_1_cs.a[0].m[_e143];
    int _e148 = idx_2;
    vec3 l_u_c_vc = _group_2_binding_1_cs.a[_e148].m[0];
    int _e155 = idx_2;
    int _e158 = idx_2;
    vec3 l_u_c_vv = _group_2_binding_1_cs.a[_e155].m[_e158];
    float l_u_e_ccc = _group_2_binding_1_cs.a[0].m[0][0];
    int _e173 = idx_2;
    float l_u_e_ccv = _group_2_binding_1_cs.a[0].m[0][_e173];
    int _e180 = idx_2;
    float l_u_e_cvc = _group_2_binding_1_cs.a[0].m[_e180][0];
    int _e188 = idx_2;
    int _e190 = idx_2;
    float l_u_e_cvv = _group_2_binding_1_cs.a[0].m[_e188][_e190];
    int _e195 = idx_2;
    float l_u_e_vcc = _group_2_binding_1_cs.a[_e195].m[0][0];
    int _e203 = idx_2;
    int _e207 = idx_2;
    float l_u_e_vcv = _group_2_binding_1_cs.a[_e203].m[0][_e207];
    int _e212 = idx_2;
    int _e215 = idx_2;
    float l_u_e_vvc = _group_2_binding_1_cs.a[_e212].m[_e215][0];
    int _e221 = idx_2;
    int _e224 = idx_2;
    int _e226 = idx_2;
    float l_u_e_vvv = _group_2_binding_1_cs.a[_e221].m[_e224][_e226];
    _group_2_binding_0_cs = l_u_s_1;
    _group_2_binding_0_cs.a = l_u_a;
    _group_2_binding_0_cs.a[0].m = l_u_m_c;
    int _e238 = idx_2;
    _group_2_binding_0_cs.a[_e238].m = l_u_m_v;
    _group_2_binding_0_cs.a[0].m[0] = l_u_c_cc;
    int _e250 = idx_2;
    _group_2_binding_0_cs.a[0].m[_e250] = l_u_c_cv;
    int _e254 = idx_2;
    _group_2_binding_0_cs.a[_e254].m[0] = l_u_c_vc;
    int _e260 = idx_2;
    int _e263 = idx_2;
    _group_2_binding_0_cs.a[_e260].m[_e263] = l_u_c_vv;
    _group_2_binding_0_cs.a[0].m[0][0] = l_u_e_ccc;
    int _e276 = idx_2;
    _group_2_binding_0_cs.a[0].m[0][_e276] = l_u_e_ccv;
    int _e282 = idx_2;
    _group_2_binding_0_cs.a[0].m[_e282][0] = l_u_e_cvc;
    int _e289 = idx_2;
    int _e291 = idx_2;
    _group_2_binding_0_cs.a[0].m[_e289][_e291] = l_u_e_cvv;
    int _e295 = idx_2;
    _group_2_binding_0_cs.a[_e295].m[0][0] = l_u_e_vcc;
    int _e302 = idx_2;
    int _e306 = idx_2;
    _group_2_binding_0_cs.a[_e302].m[0][_e306] = l_u_e_vcv;
    int _e310 = idx_2;
    int _e313 = idx_2;
    _group_2_binding_0_cs.a[_e310].m[_e313][0] = l_u_e_vvc;
    int _e318 = idx_2;
    int _e321 = idx_2;
    int _e323 = idx_2;
    _group_2_binding_0_cs.a[_e318].m[_e321][_e323] = l_u_e_vvv;
    return;
}

void main() {
    access_m();
    access_sm();
    access_sasm();
    return;
}

