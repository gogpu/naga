groupshared int a;

[numthreads(1, 1, 1)]
void main(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        a = (int)0;
    }
    GroupMemoryBarrierWithGroupSync();
    int _e2; InterlockedAdd(a, -int(-1), _e2);
    return;
}
