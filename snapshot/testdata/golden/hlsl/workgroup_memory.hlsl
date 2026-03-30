struct Params {
    uint count;
};

groupshared float shared_data[256];
cbuffer params : register(b0) { Params params; }
RWByteAddressBuffer output : register(u1);

uint naga_div(uint lhs, uint rhs) {
    return lhs / (rhs == 0u ? 1u : rhs);
}

[numthreads(256, 1, 1)]
void main(uint lid : SV_GroupIndex, uint3 gid : SV_DispatchThreadID, uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        shared_data = (float[256])0;
    }
    GroupMemoryBarrierWithGroupSync();
    shared_data[min(uint(lid), 255u)] = (float(gid.x) * 0.5);
    GroupMemoryBarrierWithGroupSync();
    if ((lid < 128u)) {
        float _e14 = shared_data[min(uint(lid), 255u)];
        float _e19 = shared_data[min(uint((lid + 128u)), 255u)];
        shared_data[min(uint(lid), 255u)] = (_e14 + _e19);
    }
    GroupMemoryBarrierWithGroupSync();
    if ((lid == 0u)) {
        float _e30 = shared_data[0];
        output.Store(naga_div(gid.x, 256u)*4, asuint(_e30));
        return;
    } else {
        return;
    }
}
