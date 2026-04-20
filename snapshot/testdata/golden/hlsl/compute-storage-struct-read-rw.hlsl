struct Particle {
    float2 pos;
    float2 vel;
};

struct Params {
    float dt;
    uint count;
};

ByteAddressBuffer pin : register(t0);
RWByteAddressBuffer pout : register(u1);
cbuffer params : register(b2) { Params params; }

Particle ConstructParticle(float2 arg0, float2 arg1) {
    Particle ret = (Particle)0;
    ret.pos = arg0;
    ret.vel = arg1;
    return ret;
}

[numthreads(64, 1, 1)]
void main(uint3 id : SV_DispatchThreadID)
{
    Particle p = (Particle)0;

    uint i = id.x;
    uint _e4 = params.count;
    if ((i >= _e4)) {
        return;
    }
    Particle _e8 = ConstructParticle(asfloat(pin.Load2(i*16+0)), asfloat(pin.Load2(i*16+8)));
    p = _e8;
    float2 _e12 = p.vel;
    p.vel = (_e12 * (0.9995).xx);
    float2 _e17 = p.vel;
    float _e20 = params.dt;
    float2 _e22 = p.pos;
    p.pos = (_e22 + (_e17 * _e20));
    Particle _e26 = p;
    {
        Particle _value2 = _e26;
        pout.Store2(i*16+0, asuint(_value2.pos));
        pout.Store2(i*16+8, asuint(_value2.vel));
    }
    return;
}
