struct TestR {
    uint test_m;
};

struct TestS {
    uint test_m;
};

static const uint test_c = 1u;

cbuffer mvp_matrix : register(b0) { row_major float4x4 mvp_matrix; }
groupshared float2x2 w_mem;
groupshared float2x2 w_mem2_;

void test_f()
{
    return;
}

void test_g()
{
    return;
}

TestS ZeroValueTestS() {
    return (TestS)0;
}

[numthreads(1, 1, 1)]
void test_ep(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        w_mem = (float2x2)0;
        w_mem2_ = (float2x2)0;
    }
    GroupMemoryBarrierWithGroupSync();
    float2x2 phony = w_mem2_;
    test_g();
    return;
}
