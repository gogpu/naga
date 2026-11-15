void switchEmpty(int a)
{
    int a_1 = (int)0;

    a_1 = a;
    int _e2 = a_1;
    do {
    } while(false);
    return;
}

void switchNoDefault(int a_2)
{
    int a_3 = (int)0;

    a_3 = a_2;
    int _e2 = a_3;
    switch(_e2) {
        case 0: {
            break;
        }
        default: {
            break;
        }
    }
    return;
}

void switchCaseImplConv(uint a_4)
{
    uint a_5 = (uint)0;

    a_5 = a_4;
    uint _e2 = a_5;
    switch(_e2) {
        case 0u: {
            break;
        }
        default: {
            break;
        }
    }
    return;
}

void switchNoLastBreak(int a_6)
{
    int a_7 = (int)0;
    int b = (int)0;

    a_7 = a_6;
    int _e2 = a_7;
    do {
        int _e3 = a_7;
        b = _e3;
    } while(false);
    return;
}

void main_1()
{
    switchEmpty(int(1));
    switchNoDefault(int(2));
    switchCaseImplConv(3u);
    switchNoLastBreak(int(4));
    return;
}

void main()
{
    main_1();
    return;
}
