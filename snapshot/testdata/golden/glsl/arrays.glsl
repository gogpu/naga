#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

const uint DATA_SIZE = 8u;


float sum_array(float arr[4]) {
    float total = 0.0;
    uint i = 0u;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e12 = i;
            i = (_e12 + 1u);
        }
        loop_init = false;
        uint _e5 = i;
        if ((_e5 < 4u)) {
        } else {
            break;
        }
        {
            float _e8 = total;
            uint _e9 = i;
            total = (_e8 + arr[_e9]);
        }
    }
    float _e15 = total;
    return _e15;
}

void main() {
    uint data_1[4] = uint[4](0u, 0u, 0u, 0u);
    uint idx_1 = 2u;
    float arr1_[4] = float[4](1.0, 2.0, 3.0, 4.0);
    int arr2_[3] = int[3](10, 20, 30);
    vec2 arr3_[2] = vec2[2](vec2(1.0, 2.0), vec2(3.0, 4.0));
    float first = arr1_[0];
    float last = arr1_[3];
    float vec_elem = arr3_[1].y;
    data_1[0] = 100u;
    data_1[1] = 200u;
    uint _e27 = data_1[0];
    uint _e29 = data_1[1];
    data_1[2] = (_e27 + _e29);
    uint _e33 = data_1[2];
    data_1[3] = (_e33 * 2u);
    uint _e38 = idx_1;
    uint dynamic_val = data_1[_e38];
    float _e41 = sum_array(arr1_);
    return;
}

