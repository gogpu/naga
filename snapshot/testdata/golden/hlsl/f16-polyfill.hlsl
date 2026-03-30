struct F16IO {
    half scalar_f16;
    float scalar_f32;
    half2 vec2_f16;
    float2 vec2_f32;
    half3 vec3_f16;
    float3 vec3_f32;
    half4 vec4_f16;
    float4 vec4_f32;
};

struct test_direct_Input {
    half scalar_f16 : TEXCOORD0;
    float scalar_f32 : TEXCOORD1;
    half2 vec2_f16 : TEXCOORD2;
    float2 vec2_f32 : TEXCOORD3;
    half3 vec3_f16 : TEXCOORD4;
    float3 vec3_f32 : TEXCOORD5;
    half4 vec4_f16 : TEXCOORD6;
    float4 vec4_f32 : TEXCOORD7;
};

struct test_direct_Output {
    half scalar_f16 : SV_Target0;
    float scalar_f32 : SV_Target1;
    half2 vec2_f16 : SV_Target2;
    float2 vec2_f32 : SV_Target3;
    half3 vec3_f16 : SV_Target4;
    float3 vec3_f32 : SV_Target5;
    half4 vec4_f16 : SV_Target6;
    float4 vec4_f32 : SV_Target7;
};

test_direct_Output test_direct(test_direct_Input _input)
{
    half scalar_f16 = _input.scalar_f16;
    float scalar_f32 = _input.scalar_f32;
    half2 vec2_f16 = _input.vec2_f16;
    float2 vec2_f32 = _input.vec2_f32;
    half3 vec3_f16 = _input.vec3_f16;
    float3 vec3_f32 = _input.vec3_f32;
    half4 vec4_f16 = _input.vec4_f16;
    float4 vec4_f32 = _input.vec4_f32;
    test_direct_Output _output;
    
    half _e9 = _output[0];
    half _e11 = (scalar_f16 + 1.0);
    _e9 = _e11;
    float _e12 = _output[1];
    float _e14 = (scalar_f32 + 1.0);
    _e12 = _e14;
    half2 _e15 = _output[2];
    float2 _e17 = (1.0).xx;
    half2 _e18 = (vec2_f16 + _e17);
    _e15 = _e18;
    float2 _e19 = _output[3];
    float2 _e21 = (1.0).xx;
    float2 _e22 = (vec2_f32 + _e21);
    _e19 = _e22;
    half3 _e23 = _output[4];
    float3 _e25 = (1.0).xxx;
    half3 _e26 = (vec3_f16 + _e25);
    _e23 = _e26;
    float3 _e27 = _output[5];
    float3 _e29 = (1.0).xxx;
    float3 _e30 = (vec3_f32 + _e29);
    _e27 = _e30;
    half4 _e31 = _output[6];
    float4 _e33 = (1.0).xxxx;
    half4 _e34 = (vec4_f16 + _e33);
    _e31 = _e34;
    float4 _e35 = _output[7];
    float4 _e37 = (1.0).xxxx;
    float4 _e38 = (vec4_f32 + _e37);
    _e35 = _e38;
    F16IO _e39 = _output;
    return _e39;
}

struct test_struct_Input {
    half scalar_f16 : TEXCOORD0;
    float scalar_f32 : TEXCOORD1;
    half2 vec2_f16 : TEXCOORD2;
    float2 vec2_f32 : TEXCOORD3;
    half3 vec3_f16 : TEXCOORD4;
    float3 vec3_f32 : TEXCOORD5;
    half4 vec4_f16 : TEXCOORD6;
    float4 vec4_f32 : TEXCOORD7;
};

struct test_struct_Output {
    half scalar_f16 : SV_Target0;
    float scalar_f32 : SV_Target1;
    half2 vec2_f16 : SV_Target2;
    float2 vec2_f32 : SV_Target3;
    half3 vec3_f16 : SV_Target4;
    float3 vec3_f32 : SV_Target5;
    half4 vec4_f16 : SV_Target6;
    float4 vec4_f32 : SV_Target7;
};

test_struct_Output test_struct(test_struct_Input _input)
{
    F16IO input;
    input.scalar_f16 = _input.scalar_f16;
    input.scalar_f32 = _input.scalar_f32;
    input.vec2_f16 = _input.vec2_f16;
    input.vec2_f32 = _input.vec2_f32;
    input.vec3_f16 = _input.vec3_f16;
    input.vec3_f32 = _input.vec3_f32;
    input.vec4_f16 = _input.vec4_f16;
    input.vec4_f32 = _input.vec4_f32;
    test_struct_Output _output;
    
    half _e2 = _output[0];
    half _e3 = input.scalar_f16;
    half _e5 = (_e3 + 1.0);
    _e2 = _e5;
    float _e6 = _output[1];
    float _e7 = input.scalar_f32;
    float _e9 = (_e7 + 1.0);
    _e6 = _e9;
    half2 _e10 = _output[2];
    half2 _e11 = input.vec2_f16;
    float2 _e13 = (1.0).xx;
    half2 _e14 = (_e11 + _e13);
    _e10 = _e14;
    float2 _e15 = _output[3];
    float2 _e16 = input.vec2_f32;
    float2 _e18 = (1.0).xx;
    float2 _e19 = (_e16 + _e18);
    _e15 = _e19;
    half3 _e20 = _output[4];
    half3 _e21 = input.vec3_f16;
    float3 _e23 = (1.0).xxx;
    half3 _e24 = (_e21 + _e23);
    _e20 = _e24;
    float3 _e25 = _output[5];
    float3 _e26 = input.vec3_f32;
    float3 _e28 = (1.0).xxx;
    float3 _e29 = (_e26 + _e28);
    _e25 = _e29;
    half4 _e30 = _output[6];
    half4 _e31 = input.vec4_f16;
    float4 _e33 = (1.0).xxxx;
    half4 _e34 = (_e31 + _e33);
    _e30 = _e34;
    float4 _e35 = _output[7];
    float4 _e36 = input.vec4_f32;
    float4 _e38 = (1.0).xxxx;
    float4 _e39 = (_e36 + _e38);
    _e35 = _e39;
    F16IO _e40 = _output;
    return _e40;
}

struct test_copy_input_Input {
    half scalar_f16 : TEXCOORD0;
    float scalar_f32 : TEXCOORD1;
    half2 vec2_f16 : TEXCOORD2;
    float2 vec2_f32 : TEXCOORD3;
    half3 vec3_f16 : TEXCOORD4;
    float3 vec3_f32 : TEXCOORD5;
    half4 vec4_f16 : TEXCOORD6;
    float4 vec4_f32 : TEXCOORD7;
};

struct test_copy_input_Output {
    half scalar_f16 : SV_Target0;
    float scalar_f32 : SV_Target1;
    half2 vec2_f16 : SV_Target2;
    float2 vec2_f32 : SV_Target3;
    half3 vec3_f16 : SV_Target4;
    float3 vec3_f32 : SV_Target5;
    half4 vec4_f16 : SV_Target6;
    float4 vec4_f32 : SV_Target7;
};

test_copy_input_Output test_copy_input(test_copy_input_Input _input)
{
    F16IO input_original;
    input_original.scalar_f16 = _input.scalar_f16;
    input_original.scalar_f32 = _input.scalar_f32;
    input_original.vec2_f16 = _input.vec2_f16;
    input_original.vec2_f32 = _input.vec2_f32;
    input_original.vec3_f16 = _input.vec3_f16;
    input_original.vec3_f32 = _input.vec3_f32;
    input_original.vec4_f16 = _input.vec4_f16;
    input_original.vec4_f32 = _input.vec4_f32;
    test_copy_input_Output _output;
    F16IO output_2;
    
    _output = input_original;
    half _e3 = output_2[0];
    half _e4 = _output[0];
    half _e5 = _e4;
    half _e7 = (_e5 + 1.0);
    _e3 = _e7;
    float _e8 = output_2[1];
    float _e9 = _output[1];
    float _e10 = _e9;
    float _e12 = (_e10 + 1.0);
    _e8 = _e12;
    half2 _e13 = output_2[2];
    half2 _e14 = _output[2];
    half2 _e15 = _e14;
    float2 _e17 = (1.0).xx;
    half2 _e18 = (_e15 + _e17);
    _e13 = _e18;
    float2 _e19 = output_2[3];
    float2 _e20 = _output[3];
    float2 _e21 = _e20;
    float2 _e23 = (1.0).xx;
    float2 _e24 = (_e21 + _e23);
    _e19 = _e24;
    half3 _e25 = output_2[4];
    half3 _e26 = _output[4];
    half3 _e27 = _e26;
    float3 _e29 = (1.0).xxx;
    half3 _e30 = (_e27 + _e29);
    _e25 = _e30;
    float3 _e31 = output_2[5];
    float3 _e32 = _output[5];
    float3 _e33 = _e32;
    float3 _e35 = (1.0).xxx;
    float3 _e36 = (_e33 + _e35);
    _e31 = _e36;
    half4 _e37 = output_2[6];
    half4 _e38 = _output[6];
    half4 _e39 = _e38;
    float4 _e41 = (1.0).xxxx;
    half4 _e42 = (_e39 + _e41);
    _e37 = _e42;
    float4 _e43 = output_2[7];
    float4 _e44 = _output[7];
    float4 _e45 = _e44;
    float4 _e47 = (1.0).xxxx;
    float4 _e48 = (_e45 + _e47);
    _e43 = _e48;
    F16IO _e49 = output_2;
    return _e49;
}

struct test_return_partial_Input {
    half scalar_f16 : TEXCOORD0;
    float scalar_f32 : TEXCOORD1;
    half2 vec2_f16 : TEXCOORD2;
    float2 vec2_f32 : TEXCOORD3;
    half3 vec3_f16 : TEXCOORD4;
    float3 vec3_f32 : TEXCOORD5;
    half4 vec4_f16 : TEXCOORD6;
    float4 vec4_f32 : TEXCOORD7;
};

half test_return_partial(test_return_partial_Input _input) : SV_Target0
{
    F16IO input_original;
    input_original.scalar_f16 = _input.scalar_f16;
    input_original.scalar_f32 = _input.scalar_f32;
    input_original.vec2_f16 = _input.vec2_f16;
    input_original.vec2_f32 = _input.vec2_f32;
    input_original.vec3_f16 = _input.vec3_f16;
    input_original.vec3_f32 = _input.vec3_f32;
    input_original.vec4_f16 = _input.vec4_f16;
    input_original.vec4_f32 = _input.vec4_f32;
    F16IO input_3;
    
    input_3 = input_original;
    half _e2 = input_3[0];
    _e2 = 0.0;
    half _e4 = input_3[0];
    half _e5 = _e4;
    return _e5;
}

struct test_component_access_Input {
    half scalar_f16 : TEXCOORD0;
    float scalar_f32 : TEXCOORD1;
    half2 vec2_f16 : TEXCOORD2;
    float2 vec2_f32 : TEXCOORD3;
    half3 vec3_f16 : TEXCOORD4;
    float3 vec3_f32 : TEXCOORD5;
    half4 vec4_f16 : TEXCOORD6;
    float4 vec4_f32 : TEXCOORD7;
};

struct test_component_access_Output {
    half scalar_f16 : SV_Target0;
    float scalar_f32 : SV_Target1;
    half2 vec2_f16 : SV_Target2;
    float2 vec2_f32 : SV_Target3;
    half3 vec3_f16 : SV_Target4;
    float3 vec3_f32 : SV_Target5;
    half4 vec4_f16 : SV_Target6;
    float4 vec4_f32 : SV_Target7;
};

test_component_access_Output test_component_access(test_component_access_Input _input)
{
    F16IO input;
    input.scalar_f16 = _input.scalar_f16;
    input.scalar_f32 = _input.scalar_f32;
    input.vec2_f16 = _input.vec2_f16;
    input.vec2_f32 = _input.vec2_f32;
    input.vec3_f16 = _input.vec3_f16;
    input.vec3_f32 = _input.vec3_f32;
    input.vec4_f16 = _input.vec4_f16;
    input.vec4_f32 = _input.vec4_f32;
    test_component_access_Output _output;
    
    half2 _e2 = _output[2];
    half _e3 = _e2.x;
    half2 _e4 = input.vec2_f16;
    half _e5 = _e4.y;
    _e3 = _e5;
    half2 _e6 = _output[2];
    half _e7 = _e6.y;
    half2 _e8 = input.vec2_f16;
    half _e9 = _e8.x;
    _e7 = _e9;
    F16IO _e10 = _output;
    return _e10;
}
