groupshared uint sized_comma[1];
groupshared uint sized_no_comma[1];
RWByteAddressBuffer unsized_comma : register(u0);
RWByteAddressBuffer unsized_no_comma : register(u1);

[numthreads(1, 1, 1)]
void main(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        for (uint _naga_zi_0 = 0u; _naga_zi_0 < 1u; _naga_zi_0++) {
            sized_comma[_naga_zi_0] = (uint)0;
        }
        for (uint _naga_zi_0 = 0u; _naga_zi_0 < 1u; _naga_zi_0++) {
            sized_no_comma[_naga_zi_0] = (uint)0;
        }
    }
    GroupMemoryBarrierWithGroupSync();
    uint _e4 = asuint(unsized_comma.Load(0));
    sized_comma[0] = _e4;
    uint _e9 = asuint(unsized_no_comma.Load(0));
    sized_no_comma[0] = _e9;
    uint _e14 = sized_comma[0];
    uint _e17 = sized_no_comma[0];
    unsized_no_comma.Store(0, asuint((_e14 + _e17)));
    return;
}
