RWByteAddressBuffer globals : register(u0);

float index_array(int i)
{
    float _e4 = asfloat(globals.Load(i*4+0));
    return _e4;
}

float index_dynamic_array(int i_1)
{
    float _e4 = asfloat(globals.Load(i_1*4+112));
    return _e4;
}

float index_vector(int i_2)
{
    float _e4 = asfloat(globals.Load(i_2*4+48));
    return _e4;
}

float index_vector_by_value(float4 v, int i_3)
{
    return v[min(uint(i_3), 3u)];
}

float4 index_matrix(int i_4)
{
    float4 _e4 = asfloat(globals.Load4(i_4*16+64));
    return _e4;
}

float index_twice(int i_5, int j)
{
    float _e6 = asfloat(globals.Load(j*4+i_5*16+64));
    return _e6;
}

int naga_f2i32(float value) {
    return int(clamp(value, -2147483600.0, 2147483500.0));
}

float index_expensive(int i_6)
{
    float _e11 = asfloat(globals.Load(naga_f2i32((sin((float(i_6) / 100.0)) * 100.0))*4+0));
    return _e11;
}

float index_in_bounds()
{
    float _e3 = asfloat(globals.Load(36+0));
    float _e7 = asfloat(globals.Load(12+48));
    float _e13 = asfloat(globals.Load(12+32+64));
    return ((_e3 + _e7) + _e13);
}

void set_array(int i_7, float v_1)
{
    globals.Store(i_7*4+0, asuint(v_1));
    return;
}

void set_dynamic_array(int i_8, float v_2)
{
    globals.Store(i_8*4+112, asuint(v_2));
    return;
}

void set_vector(int i_9, float v_3)
{
    globals.Store(i_9*4+48, asuint(v_3));
    return;
}

void set_matrix(int i_10, float4 v_4)
{
    globals.Store4(i_10*16+64, asuint(v_4));
    return;
}

void set_index_twice(int i_11, int j_1, float v_5)
{
    globals.Store(j_1*4+i_11*16+64, asuint(v_5));
    return;
}

void set_expensive(int i_12, float v_6)
{
    globals.Store(naga_f2i32((sin((float(i_12) / 100.0)) * 100.0))*4+0, asuint(v_6));
    return;
}

void set_in_bounds(float v_7)
{
    globals.Store(36+0, asuint(v_7));
    globals.Store(12+48, asuint(v_7));
    globals.Store(12+32+64, asuint(v_7));
    return;
}

float index_dynamic_array_constant_index()
{
    float _e3 = asfloat(globals.Load(4000+112));
    return _e3;
}

void set_dynamic_array_constant_index(float v_8)
{
    globals.Store(4000+112, asuint(v_8));
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    const float _e1 = index_array(int(1));
    const float _e3 = index_dynamic_array(int(1));
    const float _e5 = index_vector(int(1));
    const float _e12 = index_vector_by_value(float4(2.0, 3.0, 4.0, 5.0), int(6));
    const float4 _e14 = index_matrix(int(1));
    const float _e17 = index_twice(int(1), int(2));
    const float _e19 = index_expensive(int(1));
    const float _e20 = index_in_bounds();
    set_array(int(1), 2.0);
    set_dynamic_array(int(1), 2.0);
    set_vector(int(1), 2.0);
    set_matrix(int(1), float4(2.0, 3.0, 4.0, 5.0));
    set_index_twice(int(1), int(2), 1.0);
    set_expensive(int(1), 1.0);
    set_in_bounds(1.0);
    const float _e39 = index_dynamic_array_constant_index();
    set_dynamic_array_constant_index(1.0);
    return;
}
