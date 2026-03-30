#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer Globals_block_0Compute {
    float a[10];
    vec4 v;
    mat3x4 m;
    float d[];
} _group_0_binding_0_cs;


float index_array(int i) {
    float _e4 = _group_0_binding_0_cs.a[i];
    return _e4;
}

float index_dynamic_array(int i_1) {
    float _e4 = _group_0_binding_0_cs.d[i_1];
    return _e4;
}

float index_vector(int i_2) {
    float _e4 = _group_0_binding_0_cs.v[i_2];
    return _e4;
}

float index_vector_by_value(vec4 v, int i_3) {
    return v[i_3];
}

vec4 index_matrix(int i_4) {
    vec4 _e4 = _group_0_binding_0_cs.m[i_4];
    return _e4;
}

float index_twice(int i_5, int j) {
    float _e6 = _group_0_binding_0_cs.m[i_5][j];
    return _e6;
}

float index_expensive(int i_6) {
    float _e11 = _group_0_binding_0_cs.a[int((sin((float(i_6) / 100.0)) * 100.0))];
    return _e11;
}

float index_in_bounds() {
    float _e3 = _group_0_binding_0_cs.a[9];
    float _e7 = _group_0_binding_0_cs.v.w;
    float _e13 = _group_0_binding_0_cs.m[2][3];
    return ((_e3 + _e7) + _e13);
}

void set_array(int i_7, float v_1) {
    _group_0_binding_0_cs.a[i_7] = v_1;
    return;
}

void set_dynamic_array(int i_8, float v_2) {
    _group_0_binding_0_cs.d[i_8] = v_2;
    return;
}

void set_vector(int i_9, float v_3) {
    _group_0_binding_0_cs.v[i_9] = v_3;
    return;
}

void set_matrix(int i_10, vec4 v_4) {
    _group_0_binding_0_cs.m[i_10] = v_4;
    return;
}

void set_index_twice(int i_11, int j_1, float v_5) {
    _group_0_binding_0_cs.m[i_11][j_1] = v_5;
    return;
}

void set_expensive(int i_12, float v_6) {
    _group_0_binding_0_cs.a[int((sin((float(i_12) / 100.0)) * 100.0))] = v_6;
    return;
}

void set_in_bounds(float v_7) {
    _group_0_binding_0_cs.a[9] = v_7;
    _group_0_binding_0_cs.v.w = v_7;
    _group_0_binding_0_cs.m[2][3] = v_7;
    return;
}

float index_dynamic_array_constant_index() {
    float _e3 = _group_0_binding_0_cs.d[1000];
    return _e3;
}

void set_dynamic_array_constant_index(float v_8) {
    _group_0_binding_0_cs.d[1000] = v_8;
    return;
}

void main() {
    float _e1 = index_array(1);
    float _e3 = index_dynamic_array(1);
    float _e5 = index_vector(1);
    float _e12 = index_vector_by_value(vec4(2.0, 3.0, 4.0, 5.0), 6);
    vec4 _e14 = index_matrix(1);
    float _e17 = index_twice(1, 2);
    float _e19 = index_expensive(1);
    float _e20 = index_in_bounds();
    set_array(1, 2.0);
    set_dynamic_array(1, 2.0);
    set_vector(1, 2.0);
    set_matrix(1, vec4(2.0, 3.0, 4.0, 5.0));
    set_index_twice(1, 2, 1.0);
    set_expensive(1, 1.0);
    set_in_bounds(1.0);
    float _e39 = index_dynamic_array_constant_index();
    set_dynamic_array_constant_index(1.0);
    return;
}

