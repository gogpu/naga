struct Input {
    uint draw_index : SV_Position;
};

float4 vertex(Input input) : SV_Position
{
    return float4(float(input.draw_index), 1.0, 1.0, 1.0);
}
