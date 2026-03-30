RWByteAddressBuffer globals : register(u0);

uint fetch_add_atomic()
{
    uint _e3; globals.InterlockedAdd(0, 1u, _e3);
    return _e3;
}

uint fetch_add_atomic_static_sized_array(int i)
{
    uint _e5; globals.InterlockedAdd(i*4+4, 1u, _e5);
    return _e5;
}

uint fetch_add_atomic_dynamic_sized_array(int i_1)
{
    uint _e5; globals.InterlockedAdd(i_1*4+44, 1u, _e5);
    return _e5;
}

uint exchange_atomic()
{
    uint _e3; globals.InterlockedExchange(0, 1u, _e3);
    return _e3;
}

uint exchange_atomic_static_sized_array(int i_2)
{
    uint _e5; globals.InterlockedExchange(i_2*4+4, 1u, _e5);
    return _e5;
}

uint exchange_atomic_dynamic_sized_array(int i_3)
{
    uint _e5; globals.InterlockedExchange(i_3*4+44, 1u, _e5);
    return _e5;
}

uint fetch_add_atomic_dynamic_sized_array_static_index()
{
    uint _e4; globals.InterlockedAdd(4000+44, 1u, _e4);
    return _e4;
}

uint exchange_atomic_dynamic_sized_array_static_index()
{
    uint _e4; globals.InterlockedExchange(4000+44, 1u, _e4);
    return _e4;
}

[numthreads(1, 1, 1)]
void main()
{
    const uint _e0 = fetch_add_atomic();
    const uint _e2 = fetch_add_atomic_static_sized_array(int(1));
    const uint _e4 = fetch_add_atomic_dynamic_sized_array(int(1));
    const uint _e5 = exchange_atomic();
    const uint _e7 = exchange_atomic_static_sized_array(int(1));
    const uint _e9 = exchange_atomic_dynamic_sized_array(int(1));
    const uint _e10 = fetch_add_atomic_dynamic_sized_array_static_index();
    const uint _e11 = exchange_atomic_dynamic_sized_array_static_index();
    return;
}
