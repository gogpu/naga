struct FragmentInput_main {
    float4 color_1 : LOC0;
};

float4 main(FragmentInput_main fragmentinput_main) : SV_Target0
{
    float4 color = fragmentinput_main.color_1;
    return color;
}
