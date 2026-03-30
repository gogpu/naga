typedef float2 ret_Constructarray2_float2_[2];
ret_Constructarray2_float2_ Constructarray2_float2_(float2 arg0, float2 arg1) {
    float2 ret[2] = { arg0, arg1 };
    return ret;
}

float4 fs_main() : SV_Target0
{
    int index_0_ = int(0);

    float2 my_array[2] = Constructarray2_float2_(float2(0.0, 0.0), float2(0.0, 0.0));
    int _e9 = index_0_;
    float2 val_0_ = my_array[min(uint(_e9), 1u)];
    int _e11 = index_0_;
    float2 val_1_ = my_array[min(uint(_e11), 1u)];
    return (val_0_ * val_1_).xxyy;
}
