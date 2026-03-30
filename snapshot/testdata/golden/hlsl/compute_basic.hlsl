[numthreads(64, 1, 1)]
void main(uint3 id : SV_DispatchThreadID)
{
    uint x = (uint)0;

    x = (id.x * 2u);
    return;
}
