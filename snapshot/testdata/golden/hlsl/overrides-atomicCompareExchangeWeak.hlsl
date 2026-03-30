static const int o = 0;

groupshared uint a;

[numthreads(1, 1, 1)]
void f()
{
    uint _e2 = (uint)(o);
    InterlockedCompareExchange(a, _e2, 1u, _atomic_result);
    uint _e4 = _atomic_result;
    return;
}
