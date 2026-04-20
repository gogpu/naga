RWByteAddressBuffer out_ : register(u0);

[numthreads(1, 1, 1)]
void main()
{
    float2 v = float2(0.0, 0.0);

    v.x = 1.0;
    v.y = 2.0;
    float2 _e10 = v;
    out_.Store2(0, asuint(_e10));
    return;
}
