[numthreads(1, 1, 1)]
void main()
{
    float3 v3_ = float3(1.0, 2.0, 3.0);
    float3 v3b = float3(4.0, 5.0, 6.0);
    float3 n = normalize(v3_);
    float l = length(v3_);
    float d = dot(v3_, v3b);
    float3 c = cross(v3_, v3b);
    float cl = clamp(0.5, 0.0, 1.0);
    float3 m = lerp(v3_, v3b, 0.5);
    float s = smoothstep(0.0, 1.0, 0.5);
    return;
}
