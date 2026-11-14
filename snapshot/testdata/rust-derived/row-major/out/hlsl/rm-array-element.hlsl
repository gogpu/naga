struct Data {
    row_major float4x4 m[2];
};

cbuffer data : register(b0) { Data data; }
static float4 color = (float4)0;

void main_1()
{
    float4x4 m = (float4x4)0;

    float4x4 _e6 = data.m[int(1)];
    m = transpose(_e6);
    float4 _e9 = m[0];
    color = _e9;
    return;
}

float4 main() : SV_Target0
{
    main_1();
    float4 _e1 = color;
    return _e1;
}
