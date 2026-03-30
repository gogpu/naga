int test_if(int x)
{
    if ((x > int(0))) {
        return int(1);
    } else {
        if ((x < int(0))) {
            return int(-1);
        } else {
            return int(0);
        }
    }
}

int test_loop()
{
    int sum = int(0);
    int i = int(0);

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        int _e4 = i;
        if ((_e4 >= int(10))) {
            break;
        }
        int _e7 = sum;
        int _e8 = i;
        sum = asint(asuint(_e7) + asuint(_e8));
        int _e10 = i;
        i = asint(asuint(_e10) + asuint(int(1)));
    }
    int _e13 = sum;
    return _e13;
}

int test_for()
{
    int sum_1 = int(0);
    int i_1 = int(0);

    uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
        loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
        if (!loop_init) {
            int _e13 = i_1;
            i_1 = asint(asuint(_e13) + asuint(int(1)));
        }
        loop_init = false;
        int _e4 = i_1;
        if ((_e4 < int(5))) {
        } else {
            break;
        }
        {
            int _e7 = i_1;
            if ((_e7 == int(3))) {
                continue;
            }
            int _e10 = sum_1;
            int _e11 = i_1;
            sum_1 = asint(asuint(_e10) + asuint(_e11));
        }
    }
    int _e16 = sum_1;
    return _e16;
}

int test_while()
{
    int n = int(10);

    uint2 loop_bound_2 = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound_2 == uint2(0u, 0u))) { break; }
        loop_bound_2 -= uint2(loop_bound_2.y == 0u, 1u);
        int _e2 = n;
        if ((_e2 > int(0))) {
        } else {
            break;
        }
        {
            int _e5 = n;
            n = asint(asuint(_e5) - asuint(int(1)));
        }
    }
    int _e8 = n;
    return _e8;
}

int test_switch(int x_1)
{
    int result = (int)0;

    switch(x_1) {
        case 1: {
            result = int(10);
            break;
        }
        case 2:
        case 3: {
            result = int(20);
            break;
        }
        default: {
            result = int(0);
            break;
        }
    }
    int _e5 = result;
    return _e5;
}

[numthreads(1, 1, 1)]
void main()
{
    const int _e1 = test_if(int(1));
    const int _e2 = test_loop();
    const int _e3 = test_for();
    const int _e4 = test_while();
    const int _e6 = test_switch(int(2));
    return;
}
