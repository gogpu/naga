struct VertexOutput {
    float4 position : SV_Position;
    float4 color : LOC0;
    float2 texcoord : LOC1;
};

struct VertexInput {
    float4 position : LOC0;
    float3 normal : LOC1;
    float2 texcoord : LOC2;
};

cbuffer mvp_matrix : register(b0) { row_major float4x4 mvp_matrix; }

struct VertexOutput_render_vertex {
    float4 color : LOC0;
    float2 texcoord : LOC1;
    float4 position_1 : SV_Position;
};

float4 do_lighting(float4 position, float3 normal)
{
    return (0.0).xxxx;
}

VertexOutput_render_vertex render_vertex(VertexInput v_in, uint v_existing_id : SV_VertexID)
{
    VertexOutput v_out = (VertexOutput)0;

    float4x4 _e6 = mvp_matrix;
    v_out.position = mul(_e6, v_in.position);
    const float4 _e11 = do_lighting(v_in.position, v_in.normal);
    v_out.color = _e11;
    v_out.texcoord = v_in.texcoord;
    VertexOutput _e14 = v_out;
    const VertexOutput vertexoutput = _e14;
    const VertexOutput_render_vertex vertexoutput_1 = { vertexoutput.color, vertexoutput.texcoord, vertexoutput.position };
    return vertexoutput_1;
}
