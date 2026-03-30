#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer type_1_block_0Compute { int _group_0_binding_0_cs[2]; };


void main() {
    int tmp_1[2] = int[2](1, 2);
    int i_1 = 0;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e16 = i_1;
            i_1 = (_e16 + 1);
        }
        loop_init = false;
        int _e6 = i_1;
        if ((_e6 < 2)) {
        } else {
            break;
        }
        {
            int _e10 = i_1;
            int _e12 = i_1;
            int _e14 = tmp_1[_e12];
            _group_0_binding_0_cs[_e10] = _e14;
        }
    }
    return;
}

