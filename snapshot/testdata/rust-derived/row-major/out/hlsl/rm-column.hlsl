struct Data {
    row_major float4x4 m;
};

static float4 color = (float4)0;
cbuffer data : register(b0) { Data data; }

void main_1()
{
    float4x4 _e3 = data.m;
    color = transpose(_e3)[3];
    return;
}

float4 main() : SV_Target0
{
    main_1();
    float4 _e1 = color;
    return _e1;
}
