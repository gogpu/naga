float4 vs_main(uint in_vertex_index : SV_VertexID) : SV_Position
{
    float x = float(asint(asuint(int(in_vertex_index)) - asuint(int(1))));
    float y = float(asint(asuint(asint(asuint(int((in_vertex_index & 1u))) * asuint(int(2)))) - asuint(int(1))));
    return float4(x, y, 0.0, 1.0);
}

float4 fs_main() : SV_Target0
{
    return float4(1.0, 0.0, 0.0, 1.0);
}
