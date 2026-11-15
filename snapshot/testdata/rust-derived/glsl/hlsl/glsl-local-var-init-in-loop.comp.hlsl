void main_1()
{
    float4 sum = (0.0).xxxx;
    int i = int(0);
    float4 a = (float4)0;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e9 = i;
            i = asint(asuint(_e9) + asuint(int(1)));
        }
        loop_init = false;
        int _e5 = i;
        if (!((_e5 < int(4)))) {
            break;
        }
        {
            a = (1.0).xxxx;
            float4 _e15 = sum;
            float4 _e16 = a;
            sum = (_e15 + _e16);
        }
    }
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    main_1();
    return;
}
