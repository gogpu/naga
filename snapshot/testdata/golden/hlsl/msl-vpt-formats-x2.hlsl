struct VertexOutput {
    float4 position : SV_Position;
};

struct VertexInput {
    nointerpolation uint2 v_uint8_ : LOC0;
    nointerpolation uint2 v_uint8x2_ : LOC1;
    nointerpolation uint2 v_uint8x4_ : LOC2;
    nointerpolation int2 v_sint8_ : LOC3;
    nointerpolation int2 v_sint8x2_ : LOC4;
    nointerpolation int2 v_sint8x4_ : LOC5;
    float2 v_unorm8_ : LOC6;
    float2 v_unorm8x2_ : LOC7;
    float2 v_unorm8x4_ : LOC8;
    float2 v_snorm8_ : LOC9;
    float2 v_snorm8x2_ : LOC10;
    float2 v_snorm8x4_ : LOC11;
    nointerpolation uint2 v_uint16_ : LOC12;
    nointerpolation uint2 v_uint16x2_ : LOC13;
    nointerpolation uint2 v_uint16x4_ : LOC14;
    nointerpolation int2 v_sint16_ : LOC15;
    nointerpolation int2 v_sint16x2_ : LOC16;
    nointerpolation int2 v_sint16x4_ : LOC17;
    float2 v_unorm16_ : LOC18;
    float2 v_unorm16x2_ : LOC19;
    float2 v_unorm16x4_ : LOC20;
    float2 v_snorm16_ : LOC21;
    float2 v_snorm16x2_ : LOC22;
    float2 v_snorm16x4_ : LOC23;
    float2 v_float16_ : LOC24;
    float2 v_float16x2_ : LOC25;
    float2 v_float16x4_ : LOC26;
    float2 v_float32_ : LOC27;
    float2 v_float32x2_ : LOC28;
    float2 v_float32x3_ : LOC29;
    float2 v_float32x4_ : LOC30;
    nointerpolation uint2 v_uint32_ : LOC31;
    nointerpolation uint2 v_uint32x2_ : LOC32;
    nointerpolation uint2 v_uint32x3_ : LOC33;
    nointerpolation uint2 v_uint32x4_ : LOC34;
    nointerpolation int2 v_sint32_ : LOC35;
    nointerpolation int2 v_sint32x2_ : LOC36;
    nointerpolation int2 v_sint32x3_ : LOC37;
    nointerpolation int2 v_sint32x4_ : LOC38;
    float2 v_unorm10_10_10_2_ : LOC39;
    float2 v_unorm8x4_bgra : LOC40;
};

struct VertexOutput_render_vertex {
    float4 position : SV_Position;
};

VertexOutput ConstructVertexOutput(float4 arg0) {
    VertexOutput ret = (VertexOutput)0;
    ret.position = arg0;
    return ret;
}

VertexOutput_render_vertex render_vertex(VertexInput v_in)
{
    const VertexOutput vertexoutput = ConstructVertexOutput((float(v_in.v_uint8_.x)).xxxx);
    const VertexOutput_render_vertex vertexoutput_1 = { vertexoutput.position };
    return vertexoutput_1;
}
