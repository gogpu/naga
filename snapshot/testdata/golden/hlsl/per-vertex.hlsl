struct FragmentInput_fs_main {
    float[3] v_1 : LOC0;
};

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    float[3] v = fragmentinput_fs_main.v_1;
    return float4(v[0], v[1], v[2], 1.0);
}
