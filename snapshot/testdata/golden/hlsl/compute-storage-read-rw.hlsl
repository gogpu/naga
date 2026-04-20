struct Params {
    float scale;
    float offset;
};

ByteAddressBuffer pin : register(t0);
RWByteAddressBuffer pout : register(u1);
cbuffer params : register(b2) { Params params; }

uint NagaBufferLength(ByteAddressBuffer buffer)
{
    uint ret;
    buffer.GetDimensions(ret);
    return ret;
}

[numthreads(64, 1, 1)]
void main(uint3 gid : SV_DispatchThreadID)
{
    uint i = gid.x;
    uint n = ((NagaBufferLength(pin) - 0) / 16);
    if ((i >= n)) {
        return;
    }
    float4 v = asfloat(pin.Load4(i*16));
    float _e13 = params.scale;
    float _e17 = params.offset;
    pout.Store4(i*16, asuint(float4(((v.xyz * _e13) + (_e17).xxx), v.w)));
    return;
}
