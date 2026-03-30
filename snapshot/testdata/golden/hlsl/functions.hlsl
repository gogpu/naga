float add(float a, float b)
{
    float _e2 = (a + b);
    return _e2;
}

float2 multiply(float2 a, float2 b)
{
    float2 _e2 = (a * b);
    return _e2;
}

float2 test_fma()
{
    float _e0 = 2.0;
    float _e1 = 2.0;
    float2 a = float2(_e0, _e1);
    float _e3 = 0.5;
    float _e4 = 0.5;
    float2 b = float2(_e3, _e4);
    float _e6 = 0.5;
    float _e7 = 0.5;
    float2 c = float2(_e6, _e7);
    float2 _e9 = mad(a, b, c);
    return _e9;
}

[numthreads(1, 1, 1)]
void main()
{
    float _cr2 = add(1.0, 2.0);
    float _e0 = 1.0;
    float _e1 = 2.0;
    float2 _cr9 = multiply(float2(2.0, 3.0), float2(4.0, 5.0));
    float _e3 = 2.0;
    float _e4 = 3.0;
    float2 _e5 = float2(_e3, _e4);
    float _e6 = 4.0;
    float _e7 = 5.0;
    float2 _e8 = float2(_e6, _e7);
    float2 _cr10 = test_fma();
    return;
}
