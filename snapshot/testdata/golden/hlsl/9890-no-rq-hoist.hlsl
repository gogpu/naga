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

RaytracingAccelerationStructure acc : register(t0);

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
    uint i = 0u;
    RayQuery<RAY_FLAG_NONE> sq;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            uint _e21 = i;
            i = (_e21 + 1u);
        }
        loop_init = false;
        uint _e2 = i;
        if ((_e2 < 4u)) {
        } else {
            break;
        }
        {
            uint _e11 = i;
            sq.TraceRayInline(acc, ConstructRayDesc_(1u, 255u, 0.001, 1000.0, (float(_e11)).xxx, float3(0.0, -1.0, 0.0)).flags, ConstructRayDesc_(1u, 255u, 0.001, 1000.0, (float(_e11)).xxx, float3(0.0, -1.0, 0.0)).cull_mask, RayDescFromRayDesc_(ConstructRayDesc_(1u, 255u, 0.001, 1000.0, (float(_e11)).xxx, float3(0.0, -1.0, 0.0))));
            const bool _e19 = sq.Proceed();
        }
    }
    return;
}
