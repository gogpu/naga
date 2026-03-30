struct QEFResult {
    float a;
    int _pad1_0;
    int _pad1_1;
    int _pad1_2;
    float3 b;
    int _end_pad_0;
};

QEFResult ConstructQEFResult(float arg0, float3 arg1) {
    QEFResult ret = (QEFResult)0;
    ret.a = arg0;
    ret.b = arg1;
    return ret;
}

QEFResult foobar(float3 normals[12], uint count)
{
    uint i = 0u;
    float3 n0_ = (float3)0;
    uint j = 0u;
    float3 n1_ = (float3)0;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            uint _e10 = i;
            i = (_e10 + 1u);
        }
        loop_init = false;
        uint _e4 = i;
        if ((_e4 < count)) {
        } else {
            break;
        }
        {
            uint _e6 = i;
            n0_ = normals[min(uint(_e6), 11u)];
        }
    }
    uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
    bool loop_init_1 = true;
    while(true) {
        if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
        loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
        if (!loop_init_1) {
            uint _e20 = j;
            j = (_e20 + 1u);
        }
        loop_init_1 = false;
        uint _e14 = j;
        if ((_e14 < count)) {
        } else {
            break;
        }
        {
            uint _e16 = j;
            n1_ = normals[min(uint(_e16), 11u)];
        }
    }
    const QEFResult qefresult = ConstructQEFResult(0.0, (0.0).xxx);
    return qefresult;
}

void main()
{
    float3 arr[12] = (float3[12])0;

    float3 _e1[12] = arr;
    const QEFResult _e3 = foobar(_e1, 1u);
    return;
}
