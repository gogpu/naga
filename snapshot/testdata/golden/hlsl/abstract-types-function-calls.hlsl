void func_f(float a)
{
    return;
}

void func_i(int a_1)
{
    return;
}

void func_u(uint a_2)
{
    return;
}

void func_vf(float2 a_3)
{
    return;
}

void func_vi(int2 a_4)
{
    return;
}

void func_vu(uint2 a_5)
{
    return;
}

void func_mf(float2x2 a_6)
{
    return;
}

void func_af(float a_7[2])
{
    return;
}

void func_ai(int a_8[2])
{
    return;
}

void func_au(uint a_9[2])
{
    return;
}

void func_f_i(float a_10, int b)
{
    return;
}

typedef float ret_Constructarray2_float_[2];
ret_Constructarray2_float_ Constructarray2_float_(float arg0, float arg1) {
    float ret[2] = { arg0, arg1 };
    return ret;
}

typedef int ret_Constructarray2_int_[2];
ret_Constructarray2_int_ Constructarray2_int_(int arg0, int arg1) {
    int ret[2] = { arg0, arg1 };
    return ret;
}

typedef uint ret_Constructarray2_uint_[2];
ret_Constructarray2_uint_ Constructarray2_uint_(uint arg0, uint arg1) {
    uint ret[2] = { arg0, arg1 };
    return ret;
}

[numthreads(1, 1, 1)]
void main()
{
    func_f(0.0);
    func_f(0.0);
    func_i(int(0));
    func_u(0u);
    func_f(0.0);
    func_f(0.0);
    func_i(int(0));
    func_u(0u);
    func_vf((0.0).xx);
    func_vf((0.0).xx);
    func_vi((int(0)).xx);
    func_vu((0u).xx);
    func_vf((0.0).xx);
    func_vf((0.0).xx);
    func_vi((int(0)).xx);
    func_vu((0u).xx);
    func_mf(float2x2((0.0).xx, (0.0).xx));
    func_mf(float2x2((0.0).xx, (0.0).xx));
    func_mf(float2x2((0.0).xx, (0.0).xx));
    func_af(Constructarray2_float_(0.0, 0.0));
    func_af(Constructarray2_float_(0.0, 0.0));
    func_ai(Constructarray2_int_(int(0), int(0)));
    func_au(Constructarray2_uint_(0u, 0u));
    func_af(Constructarray2_float_(0.0, 0.0));
    func_af(Constructarray2_float_(0.0, 0.0));
    func_ai(Constructarray2_int_(int(0), int(0)));
    func_au(Constructarray2_uint_(0u, 0u));
    func_f_i(0.0, int(0));
    func_f_i(0.0, int(0));
    func_f_i(0.0, int(0));
    func_f_i(0.0, int(0));
    return;
}
