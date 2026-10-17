struct S {
    int a;
    int _pad1_0;
    float2 b_0; float2 b_1;
};

cbuffer u : register(b0) { S u; }

[numthreads(1, 1, 1)]
void main()
{
    S v = u;
    return;
}
