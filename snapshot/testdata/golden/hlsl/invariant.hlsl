struct FragmentInput_fs {
    precise float4 position_1 : SV_Position;
};

precise float4 vs() : SV_Position
{
    return (0.0).xxxx;
}

void fs(FragmentInput_fs fragmentinput_fs)
{
    float4 position = fragmentinput_fs.position_1;
    return;
}
