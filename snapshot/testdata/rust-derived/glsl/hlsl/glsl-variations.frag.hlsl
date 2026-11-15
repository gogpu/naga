TextureCube<float4> texCube : register(t0);

uint2 NagaMipDimensionsCube(TextureCube<float4> tex, uint mip_level)
{
    uint4 ret;
    tex.GetDimensions(mip_level, ret.x, ret.y, ret.z);
    return ret.xy;
}

void main_1()
{
    int2 sizeCube = (int2)0;
    float a = 1.0;

    sizeCube = int2(NagaMipDimensionsCube(texCube, int(0)));
    return;
}

void main()
{
    main_1();
    return;
}
