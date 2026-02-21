// Fullscreen quad: vertex index to clip space, fragment passthrough.

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VertexOutput {
    // Generate fullscreen triangle positions from vertex index
    var pos: vec2<f32>;
    var uv: vec2<f32>;

    // 0: (-1, -1), 1: (3, -1), 2: (-1, 3)
    let x = f32(i32(vi) / 2) * 4.0 - 1.0;
    let y = f32(i32(vi) % 2) * 4.0 - 1.0;
    pos = vec2<f32>(x, y);
    uv = (pos + 1.0) * 0.5;

    return VertexOutput(vec4<f32>(pos, 0.0, 1.0), uv);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 0.0, 1.0);
}
