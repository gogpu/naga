struct ImmediateData {
    uint index;
    int _pad1_0;
    float2 double_;
};

struct FragmentIn {
    float4 color : LOC0;
    uint primitive_index : SV_Position;
};

ConstantBuffer<ImmediateData> im: register(b0);

struct FragmentInput_main {
    float4 color : LOC0;
    uint primitive_index : SV_Position;
};

float4 main(FragmentInput_main fragmentinput_main) : SV_Target0
{
    FragmentIn in_ = { fragmentinput_main.color, fragmentinput_main.primitive_index };
    uint _e4 = im.index;
    if ((in_.primitive_index == _e4)) {
        return in_.color;
    } else {
        return float4(((1.0).xxx - in_.color.xyz), in_.color.w);
    }
}
