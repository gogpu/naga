struct _atomic_compare_exchange_result_Sint_4_ {
    int old_value;
    bool exchanged;
};

struct _atomic_compare_exchange_result_Uint_4_ {
    uint old_value;
    bool exchanged;
};

RWByteAddressBuffer atomic_i32_ : register(u0);
RWByteAddressBuffer atomic_u32_ : register(u1);

void test_atomic_i32_()
{
    atomic_i32_.Store(0, asuint(int(1)));
    _atomic_compare_exchange_result_Sint_4_ _e5; atomic_i32_.InterlockedCompareExchange(0, int(1), int(1), _e5.old_value);
    _e5.exchanged = (_e5.old_value == int(1));
    _atomic_compare_exchange_result_Sint_4_ _e9; atomic_i32_.InterlockedCompareExchange(0, int(1), int(1), _e9.old_value);
    _e9.exchanged = (_e9.old_value == int(1));
    int _e12; atomic_i32_.InterlockedAdd(0, int(1), _e12);
    int _e15; atomic_i32_.InterlockedAdd(0, -int(1), _e15);
    int _e18; atomic_i32_.InterlockedAnd(0, int(1), _e18);
    int _e21; atomic_i32_.InterlockedXor(0, int(1), _e21);
    int _e24; atomic_i32_.InterlockedOr(0, int(1), _e24);
    int _e27; atomic_i32_.InterlockedMin(0, int(1), _e27);
    int _e30; atomic_i32_.InterlockedMax(0, int(1), _e30);
    int _e33; atomic_i32_.InterlockedExchange(0, int(1), _e33);
    return;
}

void test_atomic_u32_()
{
    atomic_u32_.Store(0, asuint(1u));
    _atomic_compare_exchange_result_Uint_4_ _e5; atomic_u32_.InterlockedCompareExchange(0, 1u, 1u, _e5.old_value);
    _e5.exchanged = (_e5.old_value == 1u);
    _atomic_compare_exchange_result_Uint_4_ _e9; atomic_u32_.InterlockedCompareExchange(0, 1u, 1u, _e9.old_value);
    _e9.exchanged = (_e9.old_value == 1u);
    uint _e12; atomic_u32_.InterlockedAdd(0, 1u, _e12);
    uint _e15; atomic_u32_.InterlockedAdd(0, -1u, _e15);
    uint _e18; atomic_u32_.InterlockedAnd(0, 1u, _e18);
    uint _e21; atomic_u32_.InterlockedXor(0, 1u, _e21);
    uint _e24; atomic_u32_.InterlockedOr(0, 1u, _e24);
    uint _e27; atomic_u32_.InterlockedMin(0, 1u, _e27);
    uint _e30; atomic_u32_.InterlockedMax(0, 1u, _e30);
    uint _e33; atomic_u32_.InterlockedExchange(0, 1u, _e33);
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    test_atomic_i32_();
    test_atomic_u32_();
    return;
}
