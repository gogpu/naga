struct Data {
    row_major float4x4 m[2];
};

static float4 color = (float4)0;
cbuffer data : register(b0) { Data data; }

float first_u0028_mf44_u005b_2_u005d_u003b(inout float4x4 m[2])
{
    float _e6 = m[int(1)][0][0u];
    return _e6;
}

void main_1()
{
    float4x4 param[2] = (float4x4[2])0;

    float4x4 _e6[2] = data.m;
    param[int(0)] = _e6[0];
    param[int(1)] = _e6[1];
    const float _e11 = first_u0028_mf44_u005b_2_u005d_u003b(param);
    color = (_e11).xxxx;
    return;
}

float4 main() : SV_Target0
{
    main_1();
    float4 _e1 = color;
    return _e1;
}
