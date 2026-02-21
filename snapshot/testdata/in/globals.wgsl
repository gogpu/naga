struct Uniforms {
    color: vec4<f32>,
    scale: f32,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

var<workgroup> shared_data: array<f32, 64>;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) local_idx: u32) {
    shared_data[local_idx] = uniforms.scale * f32(local_idx);

    workgroupBarrier();

    let val = shared_data[local_idx] + uniforms.color.x;
}
