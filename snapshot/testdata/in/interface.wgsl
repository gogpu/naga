// Vertex shader with location output
@vertex
fn vs_main(
    @builtin(vertex_index) vertex_index: u32,
) -> @builtin(position) vec4<f32> {
    let x = f32(vertex_index) / 3.0;
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}

// Fragment shader with location input/output
@fragment
fn fs_main(
    @location(0) color: vec4<f32>,
) -> @location(0) vec4<f32> {
    return color;
}
