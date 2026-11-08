struct FragmentInput_fs {
    precise float4 position_1 : SV_Position;
};

precise float4 vs() : SV_Position
{
    return (0.0).xxxx;
}

void fs(FragmentInput_fs fragmentinput_fs)
{
    float4 position = float4(fragmentinput_fs.position_1.xyz, 1.0 / fragmentinput_fs.position_1.w);
    return;
}
