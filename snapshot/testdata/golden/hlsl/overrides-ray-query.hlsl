struct RayDesc_ {
    uint flags;
    uint cull_mask;
    float tmin;
    float tmax;
    float3 origin;
    int _pad5_0;
    float3 dir;
    int _end_pad_0;
};

static const float o = 2.0;

RaytracingAccelerationStructure acc_struct : register(t0);

RayDesc_ ConstructRayDesc_(uint arg0, uint arg1, float arg2, float arg3, float3 arg4, float3 arg5) {
    RayDesc_ ret = (RayDesc_)0;
    ret.flags = arg0;
    ret.cull_mask = arg1;
    ret.tmin = arg2;
    ret.tmax = arg3;
    ret.origin = arg4;
    ret.dir = arg5;
    return ret;
}

RayDesc RayDescFromRayDesc_(RayDesc_ arg0) {
    RayDesc ret = (RayDesc)0;
    ret.Origin = arg0.origin;
    ret.TMin = arg0.tmin;
    ret.Direction = arg0.dir;
    ret.TMax = arg0.tmax;
    return ret;
}

[numthreads(1, 1, 1)]
void main()
{
    RayQuery<RAY_FLAG_NONE> rq;

    RayDesc_ desc = ConstructRayDesc_(4u, 255u, 34.0, 38.0, (46.0).xxx, float3(58.0, 62.0, 74.0));
    rq.TraceRayInline(acc_struct, desc.flags, desc.cull_mask, RayDescFromRayDesc_(desc));
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        const bool _e31 = rq.Proceed();
        if (_e31) {
        } else {
            break;
        }
        {
        }
    }
    return;
}
