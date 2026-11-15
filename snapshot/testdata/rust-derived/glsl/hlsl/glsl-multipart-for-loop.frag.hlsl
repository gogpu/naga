void main_1()
{
    float a = 1.0;
    float b = 0.25;
    float c = 1.5;
    int i = int(20);

    i = int(0);
    float _e9 = c;
    c = (_e9 - 1.0);
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e16 = i;
            i = asint(asuint(_e16) + asuint(int(1)));
            float _e19 = b;
            b = (_e19 + 0.01);
        }
        loop_init = false;
        int _e12 = i;
        if (!((_e12 < int(25)))) {
            break;
        }
        {
            float _e22 = a;
            a = (_e22 - 0.02);
        }
    }
    return;
}

void main()
{
    main_1();
    return;
}
