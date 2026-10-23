#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct StructWithMat {
    mat2x2 m;
};
struct StructWithArrayOfStructOfMat {
    StructWithMat a[4];
};
struct mat2x2_std140_ {
    vec2 col0;
    vec2 col1;
};
struct StructWithMat_std140_ {
    vec2 m_0;
    vec2 m_1;
};
struct StructWithArrayOfStructOfMat_std140_ {
    StructWithMat_std140_ a[4];
};
mat2x2 mat2x2_from_std140_(mat2x2_std140_ v) {
    return mat2x2(v.col0, v.col1);
}

StructWithMat StructWithMat_from_std140_(StructWithMat_std140_ v) {
    return StructWithMat(mat2x2(v.m_0, v.m_1));
}

mat2x2[4] array_from_std140_(mat2x2_std140_ v[4]) {
    mat2x2 r[4];
    for (int i = 0; i < 4; i++) {
        r[i] = mat2x2_from_std140_(v[i]);
    }
    return r;
}

StructWithMat[4] array_from_std140_1(StructWithMat_std140_ v[4]) {
    StructWithMat r[4];
    for (int i = 0; i < 4; i++) {
        r[i] = StructWithMat_from_std140_(v[i]);
    }
    return r;
}

StructWithArrayOfStructOfMat StructWithArrayOfStructOfMat_from_std140_(StructWithArrayOfStructOfMat_std140_ v) {
    return StructWithArrayOfStructOfMat(array_from_std140_1(v.a));
}

layout(std430) buffer Mat_block_0Compute { mat2x2 _group_0_binding_0_cs; };

layout(std140) uniform Mat_block_1Compute { mat2x2_std140_ _group_0_binding_1_cs; };

layout(std430) buffer StructWithMat_block_2Compute { StructWithMat _group_1_binding_0_cs; };

layout(std140) uniform StructWithMat_block_3Compute { StructWithMat_std140_ _group_1_binding_1_cs; };

layout(std430) buffer type_1_block_4Compute { mat2x2 _group_2_binding_0_cs[4]; };

layout(std140) uniform type_1_block_5Compute { mat2x2_std140_ _group_2_binding_1_cs[4]; };

layout(std430) buffer StructWithArrayOfStructOfMat_block_6Compute { StructWithArrayOfStructOfMat _group_3_binding_0_cs; };

layout(std140) uniform StructWithArrayOfStructOfMat_block_7Compute { StructWithArrayOfStructOfMat_std140_ _group_3_binding_1_cs; };


void access_m() {
    int idx = 1;
    int _e3 = idx;
    idx = (_e3 - 1);
    mat2x2 l_s_m = _group_0_binding_0_cs;
    vec2 l_s_c_c = _group_0_binding_0_cs[0];
    int _e11 = idx;
    vec2 l_s_c_v = _group_0_binding_0_cs[_e11];
    float l_s_e_cc = _group_0_binding_0_cs[0][0];
    int _e20 = idx;
    float l_s_e_cv = _group_0_binding_0_cs[0][_e20];
    int _e24 = idx;
    float l_s_e_vc = _group_0_binding_0_cs[_e24][0];
    int _e29 = idx;
    int _e31 = idx;
    float l_s_e_vv = _group_0_binding_0_cs[_e29][_e31];
    mat2x2 l_u_m = mat2x2_from_std140_(_group_0_binding_1_cs);
    vec2 l_u_c_c = mat2x2_from_std140_(_group_0_binding_1_cs)[0];
    int _e40 = idx;
    vec2 l_u_c_v = mat2x2_from_std140_(_group_0_binding_1_cs)[_e40];
    float l_u_e_cc = mat2x2_from_std140_(_group_0_binding_1_cs)[0][0];
    int _e49 = idx;
    float l_u_e_cv = mat2x2_from_std140_(_group_0_binding_1_cs)[0][_e49];
    int _e53 = idx;
    float l_u_e_vc = mat2x2_from_std140_(_group_0_binding_1_cs)[_e53][0];
    int _e58 = idx;
    int _e60 = idx;
    float l_u_e_vv = mat2x2_from_std140_(_group_0_binding_1_cs)[_e58][_e60];
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
    mat2x2 l_s_m_1 = _group_1_binding_0_cs.m;
    vec2 l_s_c_c_1 = _group_1_binding_0_cs.m[0];
    int _e16 = idx_1;
    vec2 l_s_c_v_1 = _group_1_binding_0_cs.m[_e16];
    float l_s_e_cc_1 = _group_1_binding_0_cs.m[0][0];
    int _e27 = idx_1;
    float l_s_e_cv_1 = _group_1_binding_0_cs.m[0][_e27];
    int _e32 = idx_1;
    float l_s_e_vc_1 = _group_1_binding_0_cs.m[_e32][0];
    int _e38 = idx_1;
    int _e40 = idx_1;
    float l_s_e_vv_1 = _group_1_binding_0_cs.m[_e38][_e40];
    StructWithMat l_u_s = StructWithMat_from_std140_(_group_1_binding_1_cs);
    mat2x2 l_u_m_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1);
    vec2 l_u_c_c_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[0];
    int _e54 = idx_1;
    vec2 l_u_c_v_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[_e54];
    float l_u_e_cc_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[0][0];
    int _e65 = idx_1;
    float l_u_e_cv_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[0][_e65];
    int _e70 = idx_1;
    float l_u_e_vc_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[_e70][0];
    int _e76 = idx_1;
    int _e78 = idx_1;
    float l_u_e_vv_1 = mat2x2(_group_1_binding_1_cs.m_0, _group_1_binding_1_cs.m_1)[_e76][_e78];
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

void access_am() {
    int idx_2 = 1;
    int _e3 = idx_2;
    idx_2 = (_e3 - 1);
    mat2x2 l_s_a[4] = _group_2_binding_0_cs;
    mat2x2 l_s_m_c = _group_2_binding_0_cs[0];
    int _e11 = idx_2;
    mat2x2 l_s_m_v = _group_2_binding_0_cs[_e11];
    vec2 l_s_c_cc = _group_2_binding_0_cs[0][0];
    int _e20 = idx_2;
    vec2 l_s_c_cv = _group_2_binding_0_cs[0][_e20];
    int _e24 = idx_2;
    vec2 l_s_c_vc = _group_2_binding_0_cs[_e24][0];
    int _e29 = idx_2;
    int _e31 = idx_2;
    vec2 l_s_c_vv = _group_2_binding_0_cs[_e29][_e31];
    float l_s_e_ccc = _group_2_binding_0_cs[0][0][0];
    int _e42 = idx_2;
    float l_s_e_ccv = _group_2_binding_0_cs[0][0][_e42];
    int _e47 = idx_2;
    float l_s_e_cvc = _group_2_binding_0_cs[0][_e47][0];
    int _e53 = idx_2;
    int _e55 = idx_2;
    float l_s_e_cvv = _group_2_binding_0_cs[0][_e53][_e55];
    int _e59 = idx_2;
    float l_s_e_vcc = _group_2_binding_0_cs[_e59][0][0];
    int _e65 = idx_2;
    int _e68 = idx_2;
    float l_s_e_vcv = _group_2_binding_0_cs[_e65][0][_e68];
    int _e72 = idx_2;
    int _e74 = idx_2;
    float l_s_e_vvc = _group_2_binding_0_cs[_e72][_e74][0];
    int _e79 = idx_2;
    int _e81 = idx_2;
    int _e83 = idx_2;
    float l_s_e_vvv = _group_2_binding_0_cs[_e79][_e81][_e83];
    mat2x2 l_u_a[4] = array_from_std140_(_group_2_binding_1_cs);
    mat2x2 l_u_m_c = mat2x2_from_std140_(_group_2_binding_1_cs[0]);
    int _e92 = idx_2;
    mat2x2 l_u_m_v = mat2x2_from_std140_(_group_2_binding_1_cs[_e92]);
    vec2 l_u_c_cc = mat2x2_from_std140_(_group_2_binding_1_cs[0])[0];
    int _e101 = idx_2;
    vec2 l_u_c_cv = mat2x2_from_std140_(_group_2_binding_1_cs[0])[_e101];
    int _e105 = idx_2;
    vec2 l_u_c_vc = mat2x2_from_std140_(_group_2_binding_1_cs[_e105])[0];
    int _e110 = idx_2;
    int _e112 = idx_2;
    vec2 l_u_c_vv = mat2x2_from_std140_(_group_2_binding_1_cs[_e110])[_e112];
    float l_u_e_ccc = mat2x2_from_std140_(_group_2_binding_1_cs[0])[0][0];
    int _e123 = idx_2;
    float l_u_e_ccv = mat2x2_from_std140_(_group_2_binding_1_cs[0])[0][_e123];
    int _e128 = idx_2;
    float l_u_e_cvc = mat2x2_from_std140_(_group_2_binding_1_cs[0])[_e128][0];
    int _e134 = idx_2;
    int _e136 = idx_2;
    float l_u_e_cvv = mat2x2_from_std140_(_group_2_binding_1_cs[0])[_e134][_e136];
    int _e140 = idx_2;
    float l_u_e_vcc = mat2x2_from_std140_(_group_2_binding_1_cs[_e140])[0][0];
    int _e146 = idx_2;
    int _e149 = idx_2;
    float l_u_e_vcv = mat2x2_from_std140_(_group_2_binding_1_cs[_e146])[0][_e149];
    int _e153 = idx_2;
    int _e155 = idx_2;
    float l_u_e_vvc = mat2x2_from_std140_(_group_2_binding_1_cs[_e153])[_e155][0];
    int _e160 = idx_2;
    int _e162 = idx_2;
    int _e164 = idx_2;
    float l_u_e_vvv = mat2x2_from_std140_(_group_2_binding_1_cs[_e160])[_e162][_e164];
    _group_2_binding_0_cs = l_u_a;
    _group_2_binding_0_cs[0] = l_u_m_c;
    int _e171 = idx_2;
    _group_2_binding_0_cs[_e171] = l_u_m_v;
    _group_2_binding_0_cs[0][0] = l_u_c_cc;
    int _e178 = idx_2;
    _group_2_binding_0_cs[0][_e178] = l_u_c_cv;
    int _e181 = idx_2;
    _group_2_binding_0_cs[_e181][0] = l_u_c_vc;
    int _e185 = idx_2;
    int _e187 = idx_2;
    _group_2_binding_0_cs[_e185][_e187] = l_u_c_vv;
    _group_2_binding_0_cs[0][0][0] = l_u_e_ccc;
    int _e196 = idx_2;
    _group_2_binding_0_cs[0][0][_e196] = l_u_e_ccv;
    int _e200 = idx_2;
    _group_2_binding_0_cs[0][_e200][0] = l_u_e_cvc;
    int _e205 = idx_2;
    int _e207 = idx_2;
    _group_2_binding_0_cs[0][_e205][_e207] = l_u_e_cvv;
    int _e210 = idx_2;
    _group_2_binding_0_cs[_e210][0][0] = l_u_e_vcc;
    int _e215 = idx_2;
    int _e218 = idx_2;
    _group_2_binding_0_cs[_e215][0][_e218] = l_u_e_vcv;
    int _e221 = idx_2;
    int _e223 = idx_2;
    _group_2_binding_0_cs[_e221][_e223][0] = l_u_e_vvc;
    int _e227 = idx_2;
    int _e229 = idx_2;
    int _e231 = idx_2;
    _group_2_binding_0_cs[_e227][_e229][_e231] = l_u_e_vvv;
    return;
}

void access_sasm() {
    int idx_3 = 1;
    int _e3 = idx_3;
    idx_3 = (_e3 - 1);
    StructWithArrayOfStructOfMat l_s_s_1 = _group_3_binding_0_cs;
    StructWithMat l_s_a_1[4] = _group_3_binding_0_cs.a;
    mat2x2 l_s_m_c_1 = _group_3_binding_0_cs.a[0].m;
    int _e17 = idx_3;
    mat2x2 l_s_m_v_1 = _group_3_binding_0_cs.a[_e17].m;
    vec2 l_s_c_cc_1 = _group_3_binding_0_cs.a[0].m[0];
    int _e31 = idx_3;
    vec2 l_s_c_cv_1 = _group_3_binding_0_cs.a[0].m[_e31];
    int _e36 = idx_3;
    vec2 l_s_c_vc_1 = _group_3_binding_0_cs.a[_e36].m[0];
    int _e43 = idx_3;
    int _e46 = idx_3;
    vec2 l_s_c_vv_1 = _group_3_binding_0_cs.a[_e43].m[_e46];
    float l_s_e_ccc_1 = _group_3_binding_0_cs.a[0].m[0][0];
    int _e61 = idx_3;
    float l_s_e_ccv_1 = _group_3_binding_0_cs.a[0].m[0][_e61];
    int _e68 = idx_3;
    float l_s_e_cvc_1 = _group_3_binding_0_cs.a[0].m[_e68][0];
    int _e76 = idx_3;
    int _e78 = idx_3;
    float l_s_e_cvv_1 = _group_3_binding_0_cs.a[0].m[_e76][_e78];
    int _e83 = idx_3;
    float l_s_e_vcc_1 = _group_3_binding_0_cs.a[_e83].m[0][0];
    int _e91 = idx_3;
    int _e95 = idx_3;
    float l_s_e_vcv_1 = _group_3_binding_0_cs.a[_e91].m[0][_e95];
    int _e100 = idx_3;
    int _e103 = idx_3;
    float l_s_e_vvc_1 = _group_3_binding_0_cs.a[_e100].m[_e103][0];
    int _e109 = idx_3;
    int _e112 = idx_3;
    int _e114 = idx_3;
    float l_s_e_vvv_1 = _group_3_binding_0_cs.a[_e109].m[_e112][_e114];
    StructWithArrayOfStructOfMat l_u_s_1 = StructWithArrayOfStructOfMat_from_std140_(_group_3_binding_1_cs);
    StructWithMat l_u_a_1[4] = array_from_std140_1(_group_3_binding_1_cs.a);
    mat2x2 l_u_m_c_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1);
    int _e129 = idx_3;
    mat2x2 l_u_m_v_1 = mat2x2(_group_3_binding_1_cs.a[_e129].m_0, _group_3_binding_1_cs.a[_e129].m_1);
    vec2 l_u_c_cc_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[0];
    int _e143 = idx_3;
    vec2 l_u_c_cv_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[_e143];
    int _e148 = idx_3;
    vec2 l_u_c_vc_1 = mat2x2(_group_3_binding_1_cs.a[_e148].m_0, _group_3_binding_1_cs.a[_e148].m_1)[0];
    int _e155 = idx_3;
    int _e158 = idx_3;
    vec2 l_u_c_vv_1 = mat2x2(_group_3_binding_1_cs.a[_e155].m_0, _group_3_binding_1_cs.a[_e155].m_1)[_e158];
    float l_u_e_ccc_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[0][0];
    int _e173 = idx_3;
    float l_u_e_ccv_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[0][_e173];
    int _e180 = idx_3;
    float l_u_e_cvc_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[_e180][0];
    int _e188 = idx_3;
    int _e190 = idx_3;
    float l_u_e_cvv_1 = mat2x2(_group_3_binding_1_cs.a[0].m_0, _group_3_binding_1_cs.a[0].m_1)[_e188][_e190];
    int _e195 = idx_3;
    float l_u_e_vcc_1 = mat2x2(_group_3_binding_1_cs.a[_e195].m_0, _group_3_binding_1_cs.a[_e195].m_1)[0][0];
    int _e203 = idx_3;
    int _e207 = idx_3;
    float l_u_e_vcv_1 = mat2x2(_group_3_binding_1_cs.a[_e203].m_0, _group_3_binding_1_cs.a[_e203].m_1)[0][_e207];
    int _e212 = idx_3;
    int _e215 = idx_3;
    float l_u_e_vvc_1 = mat2x2(_group_3_binding_1_cs.a[_e212].m_0, _group_3_binding_1_cs.a[_e212].m_1)[_e215][0];
    int _e221 = idx_3;
    int _e224 = idx_3;
    int _e226 = idx_3;
    float l_u_e_vvv_1 = mat2x2(_group_3_binding_1_cs.a[_e221].m_0, _group_3_binding_1_cs.a[_e221].m_1)[_e224][_e226];
    _group_3_binding_0_cs = l_u_s_1;
    _group_3_binding_0_cs.a = l_u_a_1;
    _group_3_binding_0_cs.a[0].m = l_u_m_c_1;
    int _e238 = idx_3;
    _group_3_binding_0_cs.a[_e238].m = l_u_m_v_1;
    _group_3_binding_0_cs.a[0].m[0] = l_u_c_cc_1;
    int _e250 = idx_3;
    _group_3_binding_0_cs.a[0].m[_e250] = l_u_c_cv_1;
    int _e254 = idx_3;
    _group_3_binding_0_cs.a[_e254].m[0] = l_u_c_vc_1;
    int _e260 = idx_3;
    int _e263 = idx_3;
    _group_3_binding_0_cs.a[_e260].m[_e263] = l_u_c_vv_1;
    _group_3_binding_0_cs.a[0].m[0][0] = l_u_e_ccc_1;
    int _e276 = idx_3;
    _group_3_binding_0_cs.a[0].m[0][_e276] = l_u_e_ccv_1;
    int _e282 = idx_3;
    _group_3_binding_0_cs.a[0].m[_e282][0] = l_u_e_cvc_1;
    int _e289 = idx_3;
    int _e291 = idx_3;
    _group_3_binding_0_cs.a[0].m[_e289][_e291] = l_u_e_cvv_1;
    int _e295 = idx_3;
    _group_3_binding_0_cs.a[_e295].m[0][0] = l_u_e_vcc_1;
    int _e302 = idx_3;
    int _e306 = idx_3;
    _group_3_binding_0_cs.a[_e302].m[0][_e306] = l_u_e_vcv_1;
    int _e310 = idx_3;
    int _e313 = idx_3;
    _group_3_binding_0_cs.a[_e310].m[_e313][0] = l_u_e_vvc_1;
    int _e318 = idx_3;
    int _e321 = idx_3;
    int _e323 = idx_3;
    _group_3_binding_0_cs.a[_e318].m[_e321][_e323] = l_u_e_vvv_1;
    return;
}

void main() {
    access_m();
    access_sm();
    access_am();
    access_sasm();
    return;
}

