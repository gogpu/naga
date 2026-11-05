struct Struct {
    float atomic_scalar;
    float atomic_arr[2];
};

RWByteAddressBuffer storage_atomic_scalar : register(u0);
RWByteAddressBuffer storage_atomic_arr : register(u1);
RWByteAddressBuffer storage_struct : register(u2);

[numthreads(2, 1, 1)]
void cs_main(uint3 id : SV_GroupThreadID)
{
    { float dummy = 0; storage_atomic_scalar.InterlockedExchange(0, 1.5, dummy); }
    { float dummy_1 = 0; storage_atomic_arr.InterlockedExchange(4, 1.5, dummy_1); }
    { float dummy_2 = 0; storage_struct.InterlockedExchange(0, 1.5, dummy_2); }
    { float dummy_3 = 0; storage_struct.InterlockedExchange(4+4, 1.5, dummy_3); }
    GroupMemoryBarrierWithGroupSync();
    float l0_; storage_atomic_scalar.InterlockedOr(0, 0, l0_);
    float l1_; storage_atomic_arr.InterlockedOr(4, 0, l1_);
    float l2_; storage_struct.InterlockedOr(0, 0, l2_);
    float l3_; storage_struct.InterlockedOr(4+4, 0, l3_);
    GroupMemoryBarrierWithGroupSync();
    float _e27; storage_atomic_scalar.InterlockedAdd(0, 1.5, _e27);
    float _e31; storage_atomic_arr.InterlockedAdd(4, 1.5, _e31);
    float _e35; storage_struct.InterlockedAdd(0, 1.5, _e35);
    float _e40; storage_struct.InterlockedAdd(4+4, 1.5, _e40);
    GroupMemoryBarrierWithGroupSync();
    float _e43; storage_atomic_scalar.InterlockedExchange(0, 1.5, _e43);
    float _e47; storage_atomic_arr.InterlockedExchange(4, 1.5, _e47);
    float _e51; storage_struct.InterlockedExchange(0, 1.5, _e51);
    float _e56; storage_struct.InterlockedExchange(4+4, 1.5, _e56);
    return;
}
