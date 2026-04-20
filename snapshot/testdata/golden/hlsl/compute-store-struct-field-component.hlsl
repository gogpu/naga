struct Particle {
    float2 pos;
    float2 vel;
};

RWByteAddressBuffer out_ : register(u0);

[numthreads(1, 1, 1)]
void main()
{
    Particle p = (Particle)0;

    p.pos = float2(0.0, 0.0);
    p.vel = float2(0.0, 0.0);
    p.pos.x = 1.0;
    p.pos.y = 2.0;
    p.vel.x = 3.0;
    p.vel.y = 4.0;
    Particle _e23 = p;
    {
        Particle _value2 = _e23;
        out_.Store2(0+0, asuint(_value2.pos));
        out_.Store2(0+8, asuint(_value2.vel));
    }
    return;
}
