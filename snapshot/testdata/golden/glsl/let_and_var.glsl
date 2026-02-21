#version 430 core

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    float a = 10.0;
    uint counter = 0u;
    int temp = 20;
    float uninit_f;
    uvec2 uninit_v;
    a = (a + 1.0);
    a = (a * 2.0);
    counter = (counter + 1u);
    counter = (counter + 1u);
    {
        temp = (temp + 1);
    }
    uninit_f = 5.0;
    uninit_v = uvec2(1u, 2u);
}
