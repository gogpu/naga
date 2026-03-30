[numthreads(1, 1, 1)]
void main()
{
    float expr1_ = ((1.0 * 2.0) + (3.0 * 4.0));
    float expr2_ = ((1.0 + 2.0) * (3.0 - 4.0));
    float expr3_ = ((1.0 / 2.0) - (3.0 / 4.0));
    float expr4_ = (((1.0 + 2.0) + 3.0) * 4.0);
    float s1_ = (true ? 2.0 : 1.0);
    float3 s2_ = (0.0).xxx;
    int s3_ = ((1.0 > 2.0) ? int(20) : int(10));
    float3 v1_ = float3(1.0, 2.0, 3.0);
    float3 v2_ = float3(4.0, 3.0, 2.0);
    float3 result = ((v1_ * v2_) + (1.0).xxx);
    float l = (length(v1_) * dot(v1_, v2_));
    float3 n = normalize((v1_ + v2_));
    return;
}
