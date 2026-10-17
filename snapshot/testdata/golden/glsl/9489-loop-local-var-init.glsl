#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) readonly buffer type_1_block_0Compute { float _group_0_binding_0_cs[64]; };

layout(std430) buffer type_2_block_1Compute { float _group_0_binding_1_cs[8]; };


void main() {
    uint t_1 = 0u;
    vec4 acc_noinit_1 = vec4(0.0);
    vec4 acc_init_1 = vec4(0.0);
    uint d_1 = 0u;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e46 = t_1;
            t_1 = (_e46 + 1u);
        }
        loop_init = false;
        uint _e2 = t_1;
        if ((_e2 < 4u)) {
        } else {
            break;
        }
        {
            acc_init_1 = vec4(0.0);
            d_1 = 0u;
            bool loop_init_1 = true;
            while(true) {
                if (!loop_init_1) {
                    uint _e27 = d_1;
                    d_1 = (_e27 + 1u);
                }
                loop_init_1 = false;
                uint _e10 = d_1;
                if ((_e10 < 16u)) {
                } else {
                    break;
                }
                {
                    uint _e14 = t_1;
                    uint _e17 = d_1;
                    float _e20 = _group_0_binding_0_cs[((_e14 * 16u) + _e17)];
                    vec4 v = vec4(_e20);
                    vec4 _e22 = acc_noinit_1;
                    acc_noinit_1 = (_e22 + v);
                    vec4 _e24 = acc_init_1;
                    acc_init_1 = (_e24 + v);
                }
            }
            uint _e30 = t_1;
            float _e35 = acc_noinit_1.x;
            _group_0_binding_1_cs[(_e30 * 2u)] = _e35;
            uint _e37 = t_1;
            float _e44 = acc_init_1.x;
            _group_0_binding_1_cs[((_e37 * 2u) + 1u)] = _e44;
        }
    }
    return;
}

