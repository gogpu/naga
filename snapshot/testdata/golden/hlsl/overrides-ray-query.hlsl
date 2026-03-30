struct RayDesc_ {
    uint flags;
    uint cull_mask;
    float tmin;
    float tmax;
    float3 origin;
    float3 dir;
};

struct RayIntersection {
    uint kind;
    float t;
    uint instance_custom_data;
    uint instance_index;
    uint sbt_record_offset;
    uint geometry_index;
    uint primitive_index;
    float2 barycentrics;
    bool front_face;
    row_major float4x3 object_to_world;
    row_major float4x3 world_to_object;
};

static const float o = 0.0;

// Unsupported resource type for acc_struct: ir.AccelerationStructureType

[numthreads(1, 1, 1)]
void main()
{
    RayQuery<RAY_FLAG_NONE> rq;
    
    float _e5 = (o * 17.0);
    float _e8 = (o * 19.0);
    float _e11 = (o * 23.0);
    float3 _e12 = (_e11).xxx;
    float _e15 = (o * 29.0);
    float _e18 = (o * 31.0);
    float _e21 = (o * 37.0);
    float3 _e22 = float3(_e15, _e18, _e21);
    RayDesc_ desc = RayDesc_(4u, 255u, _e5, _e8, _e12, _e22);
    RayQuery<RAY_FLAG_NONE> _e24 = rq;
    _e24.TraceRayInline(acc_struct, RAY_FLAG_NONE, 0xFF, desc);
    [loop]
    while (true) {
        _ray_query_proceed_result = rq.Proceed();
        RayQuery<RAY_FLAG_NONE> _e26 = rq;
        bool _e27 = _rq_proceed_result;
        if (_e27) {
        } else {
            break;
        }
        {
        }
    }
    return;
}
