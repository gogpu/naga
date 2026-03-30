#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

const float16_t MIN_F16_ = 3.18727e-319LF;
const float16_t MAX_F16_ = 1.5683e-319LF;
const float MIN_F32_ = -3.4028235e38;
const float MAX_F32_ = 3.4028235e38;
const double MIN_F64_ = -1.7976931348623157e+308LF;
const double MAX_F64_ = 1.7976931348623157e+308LF;


void test_const_eval() {
    int min_f16_to_i32_ = -65504;
    int max_f16_to_i32_ = 65504;
    uint min_f16_to_u32_ = 0u;
    uint max_f16_to_u32_ = 65504u;
    int64_t min_f16_to_i64_ = -65504L;
    int64_t max_f16_to_i64_ = 65504L;
    uint64_t min_f16_to_u64_ = 0uL;
    uint64_t max_f16_to_u64_ = 65504uL;
    int min_f32_to_i32_ = -2147483648;
    int max_f32_to_i32_ = 2147483520;
    uint min_f32_to_u32_ = 0u;
    uint max_f32_to_u32_ = 4294967040u;
    int64_t min_f32_to_i64_ = -9223372036854775808L;
    int64_t max_f32_to_i64_ = 9223371487098961920L;
    uint64_t min_f32_to_u64_ = 0uL;
    uint64_t max_f32_to_u64_ = 18446742974197923840uL;
    int64_t min_f64_to_i64_ = -9223372036854775808L;
    int64_t max_f64_to_i64_ = 9223372036854774784L;
    uint64_t min_f64_to_u64_ = 0uL;
    uint64_t max_f64_to_u64_ = 18446744073709549568uL;
    int min_abstract_float_to_i32_ = -2147483648;
    int max_abstract_float_to_i32_ = 2147483647;
    uint min_abstract_float_to_u32_ = 0u;
    uint max_abstract_float_to_u32_ = 4294967295u;
    int64_t min_abstract_float_to_i64_ = -9223372036854775808L;
    int64_t max_abstract_float_to_i64_ = 9223372036854774784L;
    uint64_t min_abstract_float_to_u64_ = 0uL;
    uint64_t max_abstract_float_to_u64_ = 18446744073709549568uL;
    return;
}

int test_f16_to_i32_(float16_t f) {
    return int(f);
}

uint test_f16_to_u32_(float16_t f_1) {
    return uint(f_1);
}

int64_t test_f16_to_i64_(float16_t f_2) {
    return int(f_2);
}

uint64_t test_f16_to_u64_(float16_t f_3) {
    return uint(f_3);
}

int test_f32_to_i32_(float f_4) {
    return int(f_4);
}

uint test_f32_to_u32_(float f_5) {
    return uint(f_5);
}

int64_t test_f32_to_i64_(float f_6) {
    return int(f_6);
}

uint64_t test_f32_to_u64_(float f_7) {
    return uint(f_7);
}

int test_f64_to_i32_(double f_8) {
    return int(f_8);
}

uint test_f64_to_u32_(double f_9) {
    return uint(f_9);
}

int64_t test_f64_to_i64_(double f_10) {
    return int(f_10);
}

uint64_t test_f64_to_u64_(double f_11) {
    return uint(f_11);
}

ivec2 test_f16_to_i32_vec(vec2 f_12) {
    return ivec2(f_12);
}

uvec2 test_f16_to_u32_vec(vec2 f_13) {
    return uvec2(f_13);
}

ivec2 test_f16_to_i64_vec(vec2 f_14) {
    return ivec2(f_14);
}

uvec2 test_f16_to_u64_vec(vec2 f_15) {
    return uvec2(f_15);
}

ivec2 test_f32_to_i32_vec(vec2 f_16) {
    return ivec2(f_16);
}

uvec2 test_f32_to_u32_vec(vec2 f_17) {
    return uvec2(f_17);
}

ivec2 test_f32_to_i64_vec(vec2 f_18) {
    return ivec2(f_18);
}

uvec2 test_f32_to_u64_vec(vec2 f_19) {
    return uvec2(f_19);
}

ivec2 test_f64_to_i32_vec(dvec2 f_20) {
    return ivec2(f_20);
}

uvec2 test_f64_to_u32_vec(dvec2 f_21) {
    return uvec2(f_21);
}

ivec2 test_f64_to_i64_vec(dvec2 f_22) {
    return ivec2(f_22);
}

uvec2 test_f64_to_u64_vec(dvec2 f_23) {
    return uvec2(f_23);
}

void main() {
    test_const_eval();
    int _e1 = test_f16_to_i32_(0);
    uint _e3 = test_f16_to_u32_(0);
    int64_t _e5 = test_f16_to_i64_(0);
    uint64_t _e7 = test_f16_to_u64_(0);
    int _e9 = test_f32_to_i32_(1.0);
    uint _e11 = test_f32_to_u32_(1.0);
    int64_t _e13 = test_f32_to_i64_(1.0);
    uint64_t _e15 = test_f32_to_u64_(1.0);
    int _e17 = test_f64_to_i32_(1.0LF);
    uint _e19 = test_f64_to_u32_(1.0LF);
    int64_t _e21 = test_f64_to_i64_(1.0LF);
    uint64_t _e23 = test_f64_to_u64_(1.0LF);
    ivec2 _e27 = test_f16_to_i32_vec(vec2(0, 0));
    uvec2 _e31 = test_f16_to_u32_vec(vec2(0, 0));
    ivec2 _e35 = test_f16_to_i64_vec(vec2(0, 0));
    uvec2 _e39 = test_f16_to_u64_vec(vec2(0, 0));
    ivec2 _e43 = test_f32_to_i32_vec(vec2(1.0, 2.0));
    uvec2 _e47 = test_f32_to_u32_vec(vec2(1.0, 2.0));
    ivec2 _e51 = test_f32_to_i64_vec(vec2(1.0, 2.0));
    uvec2 _e55 = test_f32_to_u64_vec(vec2(1.0, 2.0));
    ivec2 _e59 = test_f64_to_i32_vec(dvec2(1.0LF, 2.0LF));
    uvec2 _e63 = test_f64_to_u32_vec(dvec2(1.0LF, 2.0LF));
    ivec2 _e67 = test_f64_to_i64_vec(dvec2(1.0LF, 2.0LF));
    uvec2 _e71 = test_f64_to_u64_vec(dvec2(1.0LF, 2.0LF));
    return;
}

