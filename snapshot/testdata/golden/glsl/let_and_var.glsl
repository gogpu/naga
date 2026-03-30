#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    float a_1 = 10.0;
    uint counter_1 = 0u;
    int temp_1 = 0;
    float uninit_f_1 = 0.0;
    uvec2 uninit_v_1 = uvec2(0u);
    float z = (1.0 + 2.0);
    float _e5 = a_1;
    a_1 = (_e5 + 1.0);
    float _e8 = a_1;
    a_1 = (_e8 * 2.0);
    vec3 v = vec3(1.0, 2.0, 3.0);
    uint _e20 = counter_1;
    counter_1 = (_e20 + 1u);
    uint _e23 = counter_1;
    counter_1 = (_e23 + 1u);
    {
        temp_1 = 20;
        int _e29 = temp_1;
        temp_1 = (_e29 + 1);
    }
    uninit_f_1 = 5.0;
    uninit_v_1 = uvec2(1u, 2u);
    return;
}

