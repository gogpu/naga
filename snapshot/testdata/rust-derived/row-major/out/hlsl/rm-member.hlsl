struct Data {
    row_major float4x4 m;
};

cbuffer data : register(b0) { Data data; }
static float4 color = (float4)0;

void main_1()
{
    float4x4 m = (float4x4)0;

    float4x4 _e4 = data.m;
    m = transpose(_e4);
    float4 _e7 = m[1];
    color = _e7;
    return;
}

float4 main() : SV_Target0
{
    main_1();
    float4 _e1 = color;
    return _e1;
}
