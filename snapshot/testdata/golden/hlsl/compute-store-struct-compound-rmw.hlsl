struct Particle {
    float2 pos;
    float2 vel;
};

RWByteAddressBuffer out_ : register(u0);

[numthreads(1, 1, 1)]
void main()
{
    Particle p = (Particle)0;

    p.pos = float2(1.0, 2.0);
    p.vel = float2(3.0, 4.0);
    float2 gravity = float2(0.1, 0.2);
    float2 _e13 = p.vel;
    p.vel = (_e13 + gravity);
    float2 _e17 = p.vel;
    p.vel = (_e17 * (0.9995).xx);
    float2 _e22 = p.vel;
    float2 _e25 = p.pos;
    p.pos = (_e25 + (_e22 * 0.016));
    Particle _e29 = p;
    {
        Particle _value2 = _e29;
        out_.Store2(0+0, asuint(_value2.pos));
        out_.Store2(0+8, asuint(_value2.vel));
    }
    return;
}
