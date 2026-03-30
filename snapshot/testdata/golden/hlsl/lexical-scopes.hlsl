void blockLexicalScope(bool a)
{
    {
        {
            return;
        }
    }
}

void ifLexicalScope(bool a_1)
{
    if (a_1) {
        return;
    } else {
        return;
    }
}

void loopLexicalScope(bool a_2)
{
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
    }
    return;
}

void forLexicalScope(float a_3)
{
    int a_4 = int(0);

    uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
        loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
        if (!loop_init) {
            int _e8 = a_4;
            a_4 = asint(asuint(_e8) + asuint(int(1)));
        }
        loop_init = false;
        int _e3 = a_4;
        if ((_e3 < int(1))) {
        } else {
            break;
        }
        {
        }
    }
    return;
}

void whileLexicalScope(int a_5)
{
    uint2 loop_bound_2 = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound_2 == uint2(0u, 0u))) { break; }
        loop_bound_2 -= uint2(loop_bound_2.y == 0u, 1u);
        if ((a_5 > int(2))) {
        } else {
            break;
        }
        {
        }
    }
    return;
}

void switchLexicalScope(int a_6)
{
    switch(a_6) {
        case 0: {
            break;
        }
        case 1: {
            break;
        }
        default: {
            break;
        }
    }
    bool test = (a_6 == int(2));
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    blockLexicalScope(false);
    ifLexicalScope(true);
    loopLexicalScope(false);
    forLexicalScope(1.0);
    whileLexicalScope(int(1));
    switchLexicalScope(int(1));
    return;
}
