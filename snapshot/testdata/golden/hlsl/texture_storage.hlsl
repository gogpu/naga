RWTexture2D<unorm float4> output_tex : register(u0);

uint2 NagaRWDimensions2D(RWTexture2D<unorm float4> tex)
{
    uint4 ret;
    tex.GetDimensions(ret.x, ret.y);
    return ret.xy;
}

[numthreads(8, 8, 1)]
void main(uint3 gid : SV_DispatchThreadID)
{
    bool local = (bool)0;

    uint2 dims = NagaRWDimensions2D(output_tex);
    if (!((gid.x >= dims.x))) {
        local = (gid.y >= dims.y);
    } else {
        local = true;
    }
    bool _e13 = local;
    if (_e13) {
        return;
    }
    float2 uv = float2((float(gid.x) / float(dims.x)), (float(gid.y) / float(dims.y)));
    float4 color = float4(uv.x, uv.y, 0.5, 1.0);
    output_tex[int2(gid.xy)] = color;
    return;
}
