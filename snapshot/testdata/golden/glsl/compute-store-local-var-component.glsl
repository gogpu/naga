#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer type_1_block_0Compute { vec2 _group_0_binding_0_cs[]; };


void main() {
    vec2 v_1 = vec2(0.0, 0.0);
    v_1.x = 1.0;
    v_1.y = 2.0;
    vec2 _e10 = v_1;
    _group_0_binding_0_cs[0] = _e10;
    return;
}

