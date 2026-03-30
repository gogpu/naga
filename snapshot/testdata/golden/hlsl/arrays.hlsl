static const uint DATA_SIZE = 8u;

float sum_array(float arr[4])
{
    float total = 0.0;
    uint i = 0u;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
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
            total = (_e8 + arr[min(uint(_e9), 3u)]);
        }
    }
    float _e15 = total;
    return _e15;
}

typedef float ret_Constructarray4_float_[4];
ret_Constructarray4_float_ Constructarray4_float_(float arg0, float arg1, float arg2, float arg3) {
    float ret[4] = { arg0, arg1, arg2, arg3 };
    return ret;
}

typedef int ret_Constructarray3_int_[3];
ret_Constructarray3_int_ Constructarray3_int_(int arg0, int arg1, int arg2) {
    int ret[3] = { arg0, arg1, arg2 };
    return ret;
}

typedef float2 ret_Constructarray2_float2_[2];
ret_Constructarray2_float2_ Constructarray2_float2_(float2 arg0, float2 arg1) {
    float2 ret[2] = { arg0, arg1 };
    return ret;
}

[numthreads(1, 1, 1)]
void main()
{
    uint data[4] = (uint[4])0;
    uint idx = 2u;

    float arr1_[4] = Constructarray4_float_(1.0, 2.0, 3.0, 4.0);
    int arr2_[3] = Constructarray3_int_(int(10), int(20), int(30));
    float2 arr3_[2] = Constructarray2_float2_(float2(1.0, 2.0), float2(3.0, 4.0));
    float first = arr1_[0];
    float last = arr1_[3];
    float vec_elem = arr3_[1].y;
    data[0] = 100u;
    data[1] = 200u;
    uint _e27 = data[0];
    uint _e29 = data[1];
    data[2] = (_e27 + _e29);
    uint _e33 = data[2];
    data[3] = (_e33 * 2u);
    uint _e38 = idx;
    uint dynamic_val = data[min(uint(_e38), 3u)];
    const float _e41 = sum_array(arr1_);
    return;
}
