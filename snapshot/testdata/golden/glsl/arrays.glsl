#version 430 core

const uint DATA_SIZE = 8u;

float sum_array(float[4] arr) {
    float total = 0.0;
    uint i = 0u;
    for (;;) {
        if (!((i < 4u))) {
            break;
        }
        total = (total + arr[i]);
        i = (i + 1u);
    }
    return total;
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    uint data[4];
    uint idx = 2u;
    data[0] = 100u;
    data[1] = 200u;
    data[2] = (data[0] + data[1]);
    data[3] = (data[2] * 2u);
    float _fc46 = sum_array(float[4](1.0, 2.0, 3.0, 4.0));
}
