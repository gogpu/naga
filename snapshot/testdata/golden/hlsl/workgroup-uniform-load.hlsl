static const uint SIZE = 128u;

groupshared int arr_i32_[128];

[numthreads(4, 1, 1)]
void test_workgroupUniformLoad(uint3 workgroup_id : SV_GroupID, uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        for (uint _naga_zi_0 = 0u; _naga_zi_0 < 128u; _naga_zi_0++) {
            arr_i32_[_naga_zi_0] = (int)0;
        }
    }
    GroupMemoryBarrierWithGroupSync();
    GroupMemoryBarrierWithGroupSync();
    int _e4 = arr_i32_[min(uint(workgroup_id.x), 127u)];
    GroupMemoryBarrierWithGroupSync();
    if ((_e4 > int(10))) {
        GroupMemoryBarrierWithGroupSync();
        return;
    } else {
        return;
    }
}
