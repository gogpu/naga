struct InStorage {
    float4 a[10];
};

struct InUniform {
    float4 a[20];
};

ByteAddressBuffer in_storage : register(t0);
cbuffer in_uniform : register(b1) { InUniform in_uniform; }
Texture2DArray<float4> image_2d_array : register(t2);
groupshared float in_workgroup[30];
static float[40] in_private = (float[40])0;

typedef float4 ret_Constructarray2_float4_[2];
ret_Constructarray2_float4_ Constructarray2_float4_(float4 arg0, float4 arg1) {
    float4 ret[2] = { arg0, arg1 };
    return ret;
}

float4 mock_function(int2 c, int i, int l)
{
    float4 in_function[2] = Constructarray2_float4_(float4(0.707, 0.0, 0.0, 1.0), float4(0.0, 0.707, 0.0, 1.0));

    float4 _e18 = asfloat(in_storage.Load4(i*16+0));
    float4 _e22 = in_uniform.a[i];
    float4 _e25 = image_2d_array.Load(int4(c, i, l));
    float _e29 = in_workgroup[min(uint(i), 29u)];
    float _e34 = in_private[min(uint(i), 39u)];
    float4 _e38 = in_function[min(uint(i), 1u)];
    return (((((_e18 + _e22) + _e25) + (_e29).xxxx) + (_e34).xxxx) + _e38);
}

[numthreads(1, 1, 1)]
void main(uint3 __local_invocation_id : SV_GroupThreadID)
{
    if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {
        in_workgroup = (float[30])0;
    }
    GroupMemoryBarrierWithGroupSync();
    const float4 _e5 = mock_function(int2(int(1), int(2)), int(3), int(4));
    return;
}
