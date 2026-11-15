struct Data {
    float4 vecs[42];
};

static const int NUM_VECS = int(42);

cbuffer global : register(b0, space1) { Data global; }

float4 function()
{
    float4 sum = (0.0).xxxx;
    int i = int(0);

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e11 = i;
            i = asint(asuint(_e11) + asuint(int(1)));
        }
        loop_init = false;
        int _e8 = i;
        if (!((_e8 < NUM_VECS))) {
            break;
        }
        {
            float4 _e14 = sum;
            int _e15 = i;
            float4 _e17 = global.vecs[_e15];
            sum = (_e14 + _e17);
        }
    }
    float4 _e19 = sum;
    return _e19;
}

void main_1()
{
    const float4 _e0 = function();
    return;
}

void main()
{
    main_1();
    return;
}
