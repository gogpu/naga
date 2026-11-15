void main_1()
{
    int x = int(0);
    int y = (int)0;
    int z = (int)0;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e6 = x;
            x = asint(asuint(_e6) + asuint(int(1)));
        }
        loop_init = false;
        int _e2 = x;
        if (!((_e2 < int(10)))) {
            break;
        }
        {
            y = int(0);
            uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
            bool loop_init_1 = true;
            while(true) {
                if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
                loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
                if (!loop_init_1) {
                    int _e15 = y;
                    y = asint(asuint(_e15) + asuint(int(1)));
                }
                loop_init_1 = false;
                int _e11 = y;
                if (!((_e11 < int(10)))) {
                    break;
                }
                {
                    z = int(0);
                    uint2 loop_bound_2 = uint2(4294967295u, 4294967295u);
                    bool loop_init_2 = true;
                    while(true) {
                        if (all(loop_bound_2 == uint2(0u, 0u))) { break; }
                        loop_bound_2 -= uint2(loop_bound_2.y == 0u, 1u);
                        if (!loop_init_2) {
                            int _e24 = z;
                            z = asint(asuint(_e24) + asuint(int(1)));
                        }
                        loop_init_2 = false;
                        int _e20 = z;
                        if (!((_e20 < int(10)))) {
                            break;
                        }
                        {
                        }
                    }
                }
            }
        }
    }
    return;
}

void main()
{
    main_1();
    return;
}
