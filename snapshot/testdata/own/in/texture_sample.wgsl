// texture2D + sampler, textureSample with coords.

@group(0) @binding(0)
var t_diffuse: texture_2d<f32>;
@group(0) @binding(1)
var s_diffuse: sampler;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VertexOutput {
    let x = f32(i32(vi) / 2) * 4.0 - 1.0;
    let y = f32(i32(vi) % 2) * 4.0 - 1.0;
    let pos = vec2<f32>(x, y);
    let uv = (pos + 1.0) * 0.5;
    return VertexOutput(vec4<f32>(pos, 0.0, 1.0), uv);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let color = textureSample(t_diffuse, s_diffuse, uv);
    return color;
}
