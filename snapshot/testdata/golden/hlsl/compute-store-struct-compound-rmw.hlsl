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
    float2 _e16 = p.vel;
    p.vel = (_e16 * 0.9995);
    float2 _e20 = p.pos;
    float2 _e22 = p.vel;
    p.pos = (_e20 + (_e22 * 0.016));
    Particle _e28 = p;
    {
        Particle _value2 = _e28;
        out_.Store2(0+0, asuint(_value2.pos));
        out_.Store2(0+8, asuint(_value2.vel));
    }
    return;
}
