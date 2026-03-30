RWByteAddressBuffer dynamic_array : register(u0);

void f()
{
    float2x2 v = (float2x2)0;

    v[0] = (10.0).xx;
    return;
}

void index_unsized(int i, uint v_1)
{
    uint val = asuint(dynamic_array.Load(i*4+0));
    dynamic_array.Store(i*4+0, asuint((val + v_1)));
    return;
}

void index_dynamic_array(int i_1, uint v_2)
{
    uint val_1 = asuint(dynamic_array.Load(i_1*4+0));
    dynamic_array.Store(i_1*4+0, asuint((val_1 + v_2)));
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    f();
    index_unsized(int(1), 1u);
    index_dynamic_array(int(1), 1u);
    return;
}
