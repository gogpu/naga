struct _atomic_compare_exchange_result_Uint_4_ {
    uint old_value;
    bool exchanged;
};

static const int o = int(2);

groupshared uint a;

[numthreads(1, 1, 1)]
void f(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        a = (uint)0;
    }
    GroupMemoryBarrierWithGroupSync();
    uint _e3 = uint(o);
    _atomic_compare_exchange_result_Uint_4_ _e5; InterlockedCompareExchange(a, _e3, 1u, _e5.old_value);
    _e5.exchanged = (_e5.old_value == _e3);
    return;
}
