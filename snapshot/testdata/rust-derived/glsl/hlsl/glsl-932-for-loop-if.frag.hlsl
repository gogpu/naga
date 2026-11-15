void main_1()
{
    int i = int(0);

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e6 = i;
            i = asint(asuint(_e6) + asuint(int(1)));
        }
        loop_init = false;
        int _e2 = i;
        if (!((_e2 < int(1)))) {
            break;
        }
        {
        }
    }
    return;
}

void main()
{
    main_1();
    return;
}
