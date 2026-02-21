// Fragment with multiple render targets.

struct FragmentOutput {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}

@fragment
fn fs_main(@builtin(position) frag_coord: vec4<f32>) -> FragmentOutput {
    var out: FragmentOutput;
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    out.normal = vec4<f32>(0.0, 0.0, 1.0, 1.0);
    return out;
}
