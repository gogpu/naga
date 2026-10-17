struct Outputs {
    float4 color : SV_Target0;
};

Outputs ConstructOutputs(float4 arg0) {
    Outputs ret = (Outputs)0;
    ret.color = arg0;
    return ret;
}

float4 ZeroValuefloat4() {
    return (float4)0;
}

Outputs main()
{
    const Outputs outputs = ConstructOutputs(ZeroValuefloat4());
    return outputs;
    const Outputs outputs_1 = ConstructOutputs(ZeroValuefloat4());
    return outputs_1;
}
