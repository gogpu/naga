groupshared uint shared_counter;
RWByteAddressBuffer result : register(u0);

[numthreads(64, 1, 1)]
void main(uint lid : SV_GroupIndex, uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        shared_counter = (uint)0;
    }
    GroupMemoryBarrierWithGroupSync();
    if ((lid == 0u)) {
        shared_counter = 0u;
    }
    GroupMemoryBarrierWithGroupSync();
    uint _e7; InterlockedAdd(shared_counter, 1u, _e7);
    GroupMemoryBarrierWithGroupSync();
    if ((lid == 0u)) {
        uint _e13 = shared_counter;
        result.Store(0, asuint(_e13));
        return;
    } else {
        return;
    }
}
