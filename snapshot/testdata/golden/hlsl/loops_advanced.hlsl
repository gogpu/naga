int nested_loops()
{
    int total = int(0);
    int i = int(0);
    int j = (int)0;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e22 = i;
            i = asint(asuint(_e22) + asuint(int(1)));
        }
        loop_init = false;
        int _e4 = i;
        if ((_e4 < int(3))) {
        } else {
            break;
        }
        {
            j = int(0);
            uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
            bool loop_init_1 = true;
            while(true) {
                if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
                loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
                if (!loop_init_1) {
                    int _e19 = j;
                    j = asint(asuint(_e19) + asuint(int(1)));
                }
                loop_init_1 = false;
                int _e9 = j;
                if ((_e9 < int(3))) {
                } else {
                    break;
                }
                {
                    int _e12 = total;
                    int _e13 = i;
                    int _e17 = j;
                    total = asint(asuint(asint(asuint(_e12) + asuint(asint(asuint(_e13) * asuint(int(3)))))) + asuint(_e17));
                }
            }
        }
    }
    int _e25 = total;
    return _e25;
}

int loop_with_break()
{
    int sum = int(0);
    int i_1 = int(0);

    uint2 loop_bound_2 = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound_2 == uint2(0u, 0u))) { break; }
        loop_bound_2 -= uint2(loop_bound_2.y == 0u, 1u);
        int _e4 = i_1;
        if ((_e4 >= int(10))) {
            break;
        }
        int _e7 = i_1;
        if ((_e7 == int(5))) {
            int _e10 = i_1;
            i_1 = asint(asuint(_e10) + asuint(int(1)));
            continue;
        }
        int _e13 = sum;
        int _e14 = i_1;
        sum = asint(asuint(_e13) + asuint(_e14));
        int _e16 = i_1;
        i_1 = asint(asuint(_e16) + asuint(int(1)));
    }
    int _e19 = sum;
    return _e19;
}

int loop_with_continuing()
{
    int sum_1 = int(0);
    int i_2 = int(0);

    uint2 loop_bound_3 = uint2(4294967295u, 4294967295u);
    bool loop_init_2 = true;
    while(true) {
        if (all(loop_bound_3 == uint2(0u, 0u))) { break; }
        loop_bound_3 -= uint2(loop_bound_3.y == 0u, 1u);
        if (!loop_init_2) {
            int _e10 = i_2;
            i_2 = asint(asuint(_e10) + asuint(int(1)));
        }
        loop_init_2 = false;
        int _e4 = i_2;
        if ((_e4 >= int(5))) {
            break;
        }
        int _e7 = sum_1;
        int _e8 = i_2;
        sum_1 = asint(asuint(_e7) + asuint(_e8));
    }
    int _e13 = sum_1;
    return _e13;
}

int naga_div(int lhs, int rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}

int while_loop()
{
    int n = int(100);

    uint2 loop_bound_4 = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound_4 == uint2(0u, 0u))) { break; }
        loop_bound_4 -= uint2(loop_bound_4.y == 0u, 1u);
        int _e2 = n;
        if ((_e2 > int(1))) {
        } else {
            break;
        }
        {
            int _e5 = n;
            n = naga_div(_e5, int(2));
        }
    }
    int _e8 = n;
    return _e8;
}

[numthreads(1, 1, 1)]
void main()
{
    const int _e0 = nested_loops();
    const int _e1 = loop_with_break();
    const int _e2 = loop_with_continuing();
    const int _e3 = while_loop();
    return;
}
