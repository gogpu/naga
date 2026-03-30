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
    storage_atomic_scalar.Store(0, asuint(1.5));
    storage_atomic_arr.Store(4, asuint(1.5));
    storage_struct.Store(0, asuint(1.5));
    storage_struct.Store(4+4, asuint(1.5));
    GroupMemoryBarrierWithGroupSync();
    float l0_ = asfloat(storage_atomic_scalar.Load(0));
    float l1_ = asfloat(storage_atomic_arr.Load(4));
    float l2_ = asfloat(storage_struct.Load(0));
    float l3_ = asfloat(storage_struct.Load(4+4));
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
