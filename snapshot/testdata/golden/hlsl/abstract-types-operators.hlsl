static const float plus_fafaf_1 = 3.0;
static const float plus_fafai_1 = 3.0;
static const float plus_faf_f_1 = 3.0;
static const float plus_faiaf_1 = 3.0;
static const float plus_faiai_1 = 3.0;
static const float plus_fai_f_1 = 3.0;
static const float plus_f_faf_1 = 3.0;
static const float plus_f_fai_1 = 3.0;
static const float plus_f_f_f_1 = 3.0;
static const int plus_iaiai_1 = int(3);
static const int plus_iai_i_1 = int(3);
static const int plus_i_iai_1 = int(3);
static const int plus_i_i_i_1 = int(3);
static const uint plus_uaiai_1 = 3u;
static const uint plus_uai_u_1 = 3u;
static const uint plus_u_uai_1 = 3u;
static const uint plus_u_u_u_1 = 3u;
static const uint bitflip_u_u = 0u;
static const uint bitflip_uai = 0u;
static const int least_i32_ = int(-2147483647 - 1);
static const float least_f32_ = -3.4028235e38;
static const int shl_iaiai = int(4);
static const int shl_iai_u_1 = int(4);
static const uint shl_uaiai = 4u;
static const uint shl_uai_u = 4u;
static const int shr_iaiai = int(0);
static const int shr_iai_u_1 = int(0);
static const uint shr_uaiai = 0u;
static const uint shr_uai_u = 0u;
static const int wgpu_4492_ = int(-2147483647 - 1);

groupshared uint a[64];

void runtime_values()
{
    float f = 42.0;
    int i = int(43);
    uint u = 44u;
    float plus_fafaf = 3.0;
    float plus_fafai = 3.0;
    float plus_faf_f = (float)0;
    float plus_faiaf = 3.0;
    float plus_faiai = 3.0;
    float plus_fai_f = (float)0;
    float plus_f_faf = (float)0;
    float plus_f_fai = (float)0;
    float plus_f_f_f = (float)0;
    int plus_iaiai = int(3);
    int plus_iai_i = (int)0;
    int plus_i_iai = (int)0;
    int plus_i_i_i = (int)0;
    uint plus_uaiai = 3u;
    uint plus_uai_u = (uint)0;
    uint plus_u_uai = (uint)0;
    uint plus_u_u_u = (uint)0;
    int shl_iai_u = (int)0;
    int shr_iai_u = (int)0;

    float _e8 = f;
    plus_faf_f = (1.0 + _e8);
    float _e14 = f;
    plus_fai_f = (1.0 + _e14);
    float _e18 = f;
    plus_f_faf = (_e18 + 2.0);
    float _e22 = f;
    plus_f_fai = (_e22 + 2.0);
    float _e26 = f;
    float _e27 = f;
    plus_f_f_f = (_e26 + _e27);
    int _e31 = i;
    plus_iai_i = asint(asuint(int(1)) + asuint(_e31));
    int _e35 = i;
    plus_i_iai = asint(asuint(_e35) + asuint(int(2)));
    int _e39 = i;
    int _e40 = i;
    plus_i_i_i = asint(asuint(_e39) + asuint(_e40));
    uint _e44 = u;
    plus_uai_u = (1u + _e44);
    uint _e48 = u;
    plus_u_uai = (_e48 + 2u);
    uint _e52 = u;
    uint _e53 = u;
    plus_u_u_u = (_e52 + _e53);
    uint _e56 = u;
    shl_iai_u = (int(1) << _e56);
    uint _e60 = u;
    shr_iai_u = (int(1) << _e60);
    return;
}

void wgpu_4445_()
{
    return;
}

void wgpu_4435_()
{
    uint y = a[min(uint(asint(asuint(int(1)) - asuint(int(1)))), 63u)];
    return;
}

[numthreads(1, 1, 1)]
void main(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        for (uint _naga_zi_0 = 0u; _naga_zi_0 < 64u; _naga_zi_0++) {
            a[_naga_zi_0] = (uint)0;
        }
    }
    GroupMemoryBarrierWithGroupSync();
    runtime_values();
    wgpu_4445_();
    wgpu_4435_();
    return;
}
