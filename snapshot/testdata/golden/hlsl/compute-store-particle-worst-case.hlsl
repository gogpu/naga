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
    float2 _e11 = p.pos;
    float r = length(_e11);
    float rSafe = max(r, 0.05);
    float2 _e16 = p.pos;
    float _e26 = params.dt;
    float2 gravity = (((-(_e16) / (((rSafe * rSafe) * rSafe)).xx) * 0.15) * _e26);
    float2 _e29 = p.vel;
    p.vel = (_e29 + gravity);
    float2 _e32 = p.vel;
    p.vel = (_e32 * 0.9995);
    float2 _e36 = p.pos;
    float2 _e38 = p.vel;
    float _e41 = params.dt;
    p.pos = (_e36 + (_e38 * _e41));
    float _e46 = p.pos.x;
    if ((_e46 > 1.0)) {
        p.pos.x = 1.0;
        float _e54 = p.vel.x;
        p.vel.x = (_e54 * -0.8);
    }
    float _e59 = p.pos.x;
    if ((_e59 < -1.0)) {
        p.pos.x = -1.0;
        float _e67 = p.vel.x;
        p.vel.x = (_e67 * -0.8);
    }
    float _e72 = p.pos.y;
    if ((_e72 > 1.0)) {
        p.pos.y = 1.0;
        float _e80 = p.vel.y;
        p.vel.y = (_e80 * -0.8);
    }
    float _e85 = p.pos.y;
    if ((_e85 < -1.0)) {
        p.pos.y = -1.0;
        float _e93 = p.vel.y;
        p.vel.y = (_e93 * -0.8);
    }
    Particle _e98 = p;
    {
        Particle _value2 = _e98;
        pout.Store2(i*16+0, asuint(_value2.pos));
        pout.Store2(i*16+8, asuint(_value2.vel));
    }
    return;
}
