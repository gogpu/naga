typedef float2 ret_Constructarray3_float2_[3];
ret_Constructarray3_float2_ Constructarray3_float2_(float2 arg0, float2 arg1, float2 arg2) {
    float2 ret[3] = { arg0, arg1, arg2 };
    return ret;
}

float4 vs_main(uint idx : SV_VertexID) : SV_Position
{
    float2 positions[3] = Constructarray3_float2_(float2(0.0, 0.5), float2(-0.5, -0.5), float2(0.5, -0.5));

    float2 _e13 = positions[min(uint(idx), 2u)];
    return float4(_e13, 0.0, 1.0);
}
