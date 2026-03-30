struct UniformIndex {
    uint index;
};

struct FragmentIn {
    nointerpolation uint index : LOC0;
};

Foo storage_array[1] : register(t0);
cbuffer uni : register(b10) { UniformIndex uni; }

struct FragmentInput_main {
    nointerpolation uint index : LOC0;
};

uint NagaBufferLength(ByteAddressBuffer buffer)
{
    uint ret;
    buffer.GetDimensions(ret);
    return ret;
}

uint main(FragmentInput_main fragmentinput_main) : SV_Target0
{
    FragmentIn fragment_in = { fragmentinput_main.index };
    uint u1_ = 0u;

    uint uniform_index = uni.index;
    uint non_uniform_index = fragment_in.index;
    uint _e10 = u1_;
    u1_ = (_e10 + storage_array[0].x);
    uint _e15 = u1_;
    u1_ = (_e15 + storage_array[uniform_index].x);
    uint _e20 = u1_;
    u1_ = (_e20 + storage_array[NonUniformResourceIndex(non_uniform_index)].x);
    uint _e26 = u1_;
    u1_ = (_e26 + ((NagaBufferLength(storage_array) - 0) / 0));
    uint _e32 = u1_;
    u1_ = (_e32 + ((NagaBufferLength(storage_array) - 0) / 0));
    uint _e38 = u1_;
    u1_ = (_e38 + ((NagaBufferLength(storage_array) - 0) / 0));
    uint _e40 = u1_;
    return _e40;
}
