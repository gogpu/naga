struct F16IO {
    half scalar_f16_ : SV_Target0;
    float scalar_f32_ : SV_Target1;
    half2 vec2_f16_ : SV_Target2;
    float2 vec2_f32_ : SV_Target3;
    half3 vec3_f16_ : SV_Target4;
    float3 vec3_f32_ : SV_Target5;
    half4 vec4_f16_ : SV_Target6;
    float4 vec4_f32_ : SV_Target7;
};

struct FragmentInput_test_direct {
    half scalar_f16_1 : LOC0;
    float scalar_f32_1 : LOC1;
    half2 vec2_f16_1 : LOC2;
    float2 vec2_f32_1 : LOC3;
    half3 vec3_f16_1 : LOC4;
    float3 vec3_f32_1 : LOC5;
    half4 vec4_f16_1 : LOC6;
    float4 vec4_f32_1 : LOC7;
};

struct FragmentInput_test_struct {
    half scalar_f16_2 : LOC0;
    float scalar_f32_2 : LOC1;
    half2 vec2_f16_2 : LOC2;
    float2 vec2_f32_2 : LOC3;
    half3 vec3_f16_2 : LOC4;
    float3 vec3_f32_2 : LOC5;
    half4 vec4_f16_2 : LOC6;
    float4 vec4_f32_2 : LOC7;
};

struct FragmentInput_test_copy_input {
    half scalar_f16_3 : LOC0;
    float scalar_f32_3 : LOC1;
    half2 vec2_f16_3 : LOC2;
    float2 vec2_f32_3 : LOC3;
    half3 vec3_f16_3 : LOC4;
    float3 vec3_f32_3 : LOC5;
    half4 vec4_f16_3 : LOC6;
    float4 vec4_f32_3 : LOC7;
};

struct FragmentInput_test_return_partial {
    half scalar_f16_4 : LOC0;
    float scalar_f32_4 : LOC1;
    half2 vec2_f16_4 : LOC2;
    float2 vec2_f32_4 : LOC3;
    half3 vec3_f16_4 : LOC4;
    float3 vec3_f32_4 : LOC5;
    half4 vec4_f16_4 : LOC6;
    float4 vec4_f32_4 : LOC7;
};

struct FragmentInput_test_component_access {
    half scalar_f16_5 : LOC0;
    float scalar_f32_5 : LOC1;
    half2 vec2_f16_5 : LOC2;
    float2 vec2_f32_5 : LOC3;
    half3 vec3_f16_5 : LOC4;
    float3 vec3_f32_5 : LOC5;
    half4 vec4_f16_5 : LOC6;
    float4 vec4_f32_5 : LOC7;
};

F16IO test_direct(FragmentInput_test_direct fragmentinput_test_direct)
{
    half scalar_f16_ = fragmentinput_test_direct.scalar_f16_1;
    float scalar_f32_ = fragmentinput_test_direct.scalar_f32_1;
    half2 vec2_f16_ = fragmentinput_test_direct.vec2_f16_1;
    float2 vec2_f32_ = fragmentinput_test_direct.vec2_f32_1;
    half3 vec3_f16_ = fragmentinput_test_direct.vec3_f16_1;
    float3 vec3_f32_ = fragmentinput_test_direct.vec3_f32_1;
    half4 vec4_f16_ = fragmentinput_test_direct.vec4_f16_1;
    float4 vec4_f32_ = fragmentinput_test_direct.vec4_f32_1;
    F16IO output = (F16IO)0;

    output.scalar_f16_ = (scalar_f16_ + 1.0h);
    output.scalar_f32_ = (scalar_f32_ + 1.0);
    output.vec2_f16_ = (vec2_f16_ + (1.0h).xx);
    output.vec2_f32_ = (vec2_f32_ + (1.0).xx);
    output.vec3_f16_ = (vec3_f16_ + (1.0h).xxx);
    output.vec3_f32_ = (vec3_f32_ + (1.0).xxx);
    output.vec4_f16_ = (vec4_f16_ + (1.0h).xxxx);
    output.vec4_f32_ = (vec4_f32_ + (1.0).xxxx);
    F16IO _e39 = output;
    const F16IO f16io = _e39;
    return f16io;
}

F16IO test_struct(FragmentInput_test_struct fragmentinput_test_struct)
{
    F16IO input = { fragmentinput_test_struct.scalar_f16_2, fragmentinput_test_struct.scalar_f32_2, fragmentinput_test_struct.vec2_f16_2, fragmentinput_test_struct.vec2_f32_2, fragmentinput_test_struct.vec3_f16_2, fragmentinput_test_struct.vec3_f32_2, fragmentinput_test_struct.vec4_f16_2, fragmentinput_test_struct.vec4_f32_2 };
    F16IO output_1 = (F16IO)0;

    output_1.scalar_f16_ = (input.scalar_f16_ + 1.0h);
    output_1.scalar_f32_ = (input.scalar_f32_ + 1.0);
    output_1.vec2_f16_ = (input.vec2_f16_ + (1.0h).xx);
    output_1.vec2_f32_ = (input.vec2_f32_ + (1.0).xx);
    output_1.vec3_f16_ = (input.vec3_f16_ + (1.0h).xxx);
    output_1.vec3_f32_ = (input.vec3_f32_ + (1.0).xxx);
    output_1.vec4_f16_ = (input.vec4_f16_ + (1.0h).xxxx);
    output_1.vec4_f32_ = (input.vec4_f32_ + (1.0).xxxx);
    F16IO _e40 = output_1;
    const F16IO f16io_1 = _e40;
    return f16io_1;
}

F16IO test_copy_input(FragmentInput_test_copy_input fragmentinput_test_copy_input)
{
    F16IO input_original = { fragmentinput_test_copy_input.scalar_f16_3, fragmentinput_test_copy_input.scalar_f32_3, fragmentinput_test_copy_input.vec2_f16_3, fragmentinput_test_copy_input.vec2_f32_3, fragmentinput_test_copy_input.vec3_f16_3, fragmentinput_test_copy_input.vec3_f32_3, fragmentinput_test_copy_input.vec4_f16_3, fragmentinput_test_copy_input.vec4_f32_3 };
    F16IO input_1 = (F16IO)0;
    F16IO output_2 = (F16IO)0;

    input_1 = input_original;
    half _e5 = input_1.scalar_f16_;
    output_2.scalar_f16_ = (_e5 + 1.0h);
    float _e10 = input_1.scalar_f32_;
    output_2.scalar_f32_ = (_e10 + 1.0);
    half2 _e15 = input_1.vec2_f16_;
    output_2.vec2_f16_ = (_e15 + (1.0h).xx);
    float2 _e21 = input_1.vec2_f32_;
    output_2.vec2_f32_ = (_e21 + (1.0).xx);
    half3 _e27 = input_1.vec3_f16_;
    output_2.vec3_f16_ = (_e27 + (1.0h).xxx);
    float3 _e33 = input_1.vec3_f32_;
    output_2.vec3_f32_ = (_e33 + (1.0).xxx);
    half4 _e39 = input_1.vec4_f16_;
    output_2.vec4_f16_ = (_e39 + (1.0h).xxxx);
    float4 _e45 = input_1.vec4_f32_;
    output_2.vec4_f32_ = (_e45 + (1.0).xxxx);
    F16IO _e49 = output_2;
    const F16IO f16io_2 = _e49;
    return f16io_2;
}

half test_return_partial(FragmentInput_test_return_partial fragmentinput_test_return_partial) : SV_Target0
{
    F16IO input_original_1 = { fragmentinput_test_return_partial.scalar_f16_4, fragmentinput_test_return_partial.scalar_f32_4, fragmentinput_test_return_partial.vec2_f16_4, fragmentinput_test_return_partial.vec2_f32_4, fragmentinput_test_return_partial.vec3_f16_4, fragmentinput_test_return_partial.vec3_f32_4, fragmentinput_test_return_partial.vec4_f16_4, fragmentinput_test_return_partial.vec4_f32_4 };
    F16IO input_2 = (F16IO)0;

    input_2 = input_original_1;
    input_2.scalar_f16_ = 0.0h;
    half _e5 = input_2.scalar_f16_;
    return _e5;
}

F16IO test_component_access(FragmentInput_test_component_access fragmentinput_test_component_access)
{
    F16IO input_3 = { fragmentinput_test_component_access.scalar_f16_5, fragmentinput_test_component_access.scalar_f32_5, fragmentinput_test_component_access.vec2_f16_5, fragmentinput_test_component_access.vec2_f32_5, fragmentinput_test_component_access.vec3_f16_5, fragmentinput_test_component_access.vec3_f32_5, fragmentinput_test_component_access.vec4_f16_5, fragmentinput_test_component_access.vec4_f32_5 };
    F16IO output_3 = (F16IO)0;

    output_3.vec2_f16_.x = input_3.vec2_f16_.y;
    output_3.vec2_f16_.y = input_3.vec2_f16_.x;
    F16IO _e10 = output_3;
    const F16IO f16io_3 = _e10;
    return f16io_3;
}
