[numthreads(1, 1, 1)]
void main()
{
    float a = 10.0;
    uint counter = 0u;
    int temp = (int)0;
    float uninit_f = (float)0;
    uint2 uninit_v = (uint2)0;

    float z = (1.0 + 2.0);
    float _e5 = a;
    a = (_e5 + 1.0);
    float _e8 = a;
    a = (_e8 * 2.0);
    float3 v = float3(1.0, 2.0, 3.0);
    uint _e20 = counter;
    counter = (_e20 + 1u);
    uint _e23 = counter;
    counter = (_e23 + 1u);
    {
        temp = int(20);
        int _e29 = temp;
        temp = asint(asuint(_e29) + asuint(int(1)));
    }
    uninit_f = 5.0;
    uninit_v = uint2(1u, 2u);
    return;
}
