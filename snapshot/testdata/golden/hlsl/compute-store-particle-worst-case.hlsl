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
    float2 _e33 = p.vel;
    p.vel = (_e33 * (0.9995).xx);
    float2 _e38 = p.vel;
    float _e41 = params.dt;
    float2 _e43 = p.pos;
    p.pos = (_e43 + (_e38 * _e41));
    float _e47 = p.pos.x;
    if ((_e47 > 1.0)) {
        p.pos.x = 1.0;
        float _e56 = p.vel.x;
        p.vel.x = (_e56 * -0.8);
    }
    float _e60 = p.pos.x;
    if ((_e60 < -1.0)) {
        p.pos.x = -1.0;
        float _e69 = p.vel.x;
        p.vel.x = (_e69 * -0.8);
    }
    float _e73 = p.pos.y;
    if ((_e73 > 1.0)) {
        p.pos.y = 1.0;
        float _e82 = p.vel.y;
        p.vel.y = (_e82 * -0.8);
    }
    float _e86 = p.pos.y;
    if ((_e86 < -1.0)) {
        p.pos.y = -1.0;
        float _e95 = p.vel.y;
        p.vel.y = (_e95 * -0.8);
    }
    Particle _e99 = p;
    {
        Particle _value2 = _e99;
        pout.Store2(i*16+0, asuint(_value2.pos));
        pout.Store2(i*16+8, asuint(_value2.vel));
    }
    return;
}
