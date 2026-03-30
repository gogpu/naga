#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer DynamicArray_block_0Compute {
    uint arr[];
} _group_0_binding_0_cs;


void f() {
    mat2x2 v = mat2x2(0.0);
    v[0] = vec2(10.0);
    return;
}

void index_unsized(int i, uint v_1) {
    uint val = _group_0_binding_0_cs.arr[i];
    _group_0_binding_0_cs.arr[i] = (val + v_1);
    return;
}

void index_dynamic_array(int i_1, uint v_2) {
    uint val_1 = _group_0_binding_0_cs.arr[i_1];
    _group_0_binding_0_cs.arr[i_1] = (val_1 + v_2);
    return;
}

void main() {
    f();
    index_unsized(1, 1u);
    index_dynamic_array(1, 1u);
    return;
}

