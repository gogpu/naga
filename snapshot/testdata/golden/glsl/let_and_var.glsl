#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void main() {
    float a = 10.0;
    uint counter = 0u;
    int temp = 0;
    float uninit_f = 0.0;
    uvec2 uninit_v = uvec2(0u);
    float z = (1.0 + 2.0);
    float _e5 = a;
    a = (_e5 + 1.0);
    float _e8 = a;
    a = (_e8 * 2.0);
    vec3 v = vec3(1.0, 2.0, 3.0);
    uint _e20 = counter;
    counter = (_e20 + 1u);
    uint _e23 = counter;
    counter = (_e23 + 1u);
    {
        temp = 20;
        int _e29 = temp;
        temp = (_e29 + 1);
    }
    uninit_f = 5.0;
    uninit_v = uvec2(1u, 2u);
    return;
}

