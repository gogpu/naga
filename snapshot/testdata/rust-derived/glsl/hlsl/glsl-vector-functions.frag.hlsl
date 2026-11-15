void ftest(float4 a, float4 b)
{
    float4 a_1 = (float4)0;
    float4 b_1 = (float4)0;
    bool4 c = (bool4)0;
    bool4 d = (bool4)0;
    bool4 e = (bool4)0;
    bool4 f = (bool4)0;
    bool4 g = (bool4)0;
    bool4 h = (bool4)0;

    a_1 = a;
    b_1 = b;
    float4 _e4 = a_1;
    float4 _e5 = b_1;
    c = (_e4 < _e5);
    float4 _e8 = a_1;
    float4 _e9 = b_1;
    d = (_e8 <= _e9);
    float4 _e12 = a_1;
    float4 _e13 = b_1;
    e = (_e12 > _e13);
    float4 _e16 = a_1;
    float4 _e17 = b_1;
    f = (_e16 >= _e17);
    float4 _e20 = a_1;
    float4 _e21 = b_1;
    g = (_e20 == _e21);
    float4 _e24 = a_1;
    float4 _e25 = b_1;
    h = (_e24 != _e25);
    return;
}

void dtest(double4 a_2, double4 b_2)
{
    double4 a_3 = (double4)0;
    double4 b_3 = (double4)0;
    bool4 c_1 = (bool4)0;
    bool4 d_1 = (bool4)0;
    bool4 e_1 = (bool4)0;
    bool4 f_1 = (bool4)0;
    bool4 g_1 = (bool4)0;
    bool4 h_1 = (bool4)0;

    a_3 = a_2;
    b_3 = b_2;
    double4 _e4 = a_3;
    double4 _e5 = b_3;
    c_1 = (_e4 < _e5);
    double4 _e8 = a_3;
    double4 _e9 = b_3;
    d_1 = (_e8 <= _e9);
    double4 _e12 = a_3;
    double4 _e13 = b_3;
    e_1 = (_e12 > _e13);
    double4 _e16 = a_3;
    double4 _e17 = b_3;
    f_1 = (_e16 >= _e17);
    double4 _e20 = a_3;
    double4 _e21 = b_3;
    g_1 = (_e20 == _e21);
    double4 _e24 = a_3;
    double4 _e25 = b_3;
    h_1 = (_e24 != _e25);
    return;
}

void itest(int4 a_4, int4 b_4)
{
    int4 a_5 = (int4)0;
    int4 b_5 = (int4)0;
    bool4 c_2 = (bool4)0;
    bool4 d_2 = (bool4)0;
    bool4 e_2 = (bool4)0;
    bool4 f_2 = (bool4)0;
    bool4 g_2 = (bool4)0;
    bool4 h_2 = (bool4)0;

    a_5 = a_4;
    b_5 = b_4;
    int4 _e4 = a_5;
    int4 _e5 = b_5;
    c_2 = (_e4 < _e5);
    int4 _e8 = a_5;
    int4 _e9 = b_5;
    d_2 = (_e8 <= _e9);
    int4 _e12 = a_5;
    int4 _e13 = b_5;
    e_2 = (_e12 > _e13);
    int4 _e16 = a_5;
    int4 _e17 = b_5;
    f_2 = (_e16 >= _e17);
    int4 _e20 = a_5;
    int4 _e21 = b_5;
    g_2 = (_e20 == _e21);
    int4 _e24 = a_5;
    int4 _e25 = b_5;
    h_2 = (_e24 != _e25);
    return;
}

void utest(uint4 a_6, uint4 b_6)
{
    uint4 a_7 = (uint4)0;
    uint4 b_7 = (uint4)0;
    bool4 c_3 = (bool4)0;
    bool4 d_3 = (bool4)0;
    bool4 e_3 = (bool4)0;
    bool4 f_3 = (bool4)0;
    bool4 g_3 = (bool4)0;
    bool4 h_3 = (bool4)0;

    a_7 = a_6;
    b_7 = b_6;
    uint4 _e4 = a_7;
    uint4 _e5 = b_7;
    c_3 = (_e4 < _e5);
    uint4 _e8 = a_7;
    uint4 _e9 = b_7;
    d_3 = (_e8 <= _e9);
    uint4 _e12 = a_7;
    uint4 _e13 = b_7;
    e_3 = (_e12 > _e13);
    uint4 _e16 = a_7;
    uint4 _e17 = b_7;
    f_3 = (_e16 >= _e17);
    uint4 _e20 = a_7;
    uint4 _e21 = b_7;
    g_3 = (_e20 == _e21);
    uint4 _e24 = a_7;
    uint4 _e25 = b_7;
    h_3 = (_e24 != _e25);
    return;
}

void btest(bool4 a_8, bool4 b_8)
{
    bool4 a_9 = (bool4)0;
    bool4 b_9 = (bool4)0;
    bool4 c_4 = (bool4)0;
    bool4 d_4 = (bool4)0;
    bool e_4 = (bool)0;
    bool f_4 = (bool)0;
    bool4 g_4 = (bool4)0;

    a_9 = a_8;
    b_9 = b_8;
    bool4 _e4 = a_9;
    bool4 _e5 = b_9;
    c_4 = (_e4 == _e5);
    bool4 _e8 = a_9;
    bool4 _e9 = b_9;
    d_4 = (_e8 != _e9);
    bool4 _e12 = a_9;
    e_4 = any(_e12);
    bool4 _e15 = a_9;
    f_4 = all(_e15);
    bool4 _e18 = a_9;
    g_4 = !(_e18);
    return;
}

void main_1()
{
    ftest((0.0).xxxx, (0.0).xxxx);
    dtest((0.0L).xxxx, (0.0L).xxxx);
    itest((int(0)).xxxx, (int(0)).xxxx);
    utest((0u).xxxx, (0u).xxxx);
    btest((false).xxxx, (false).xxxx);
    return;
}

void main()
{
    main_1();
    return;
}
