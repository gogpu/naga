// Regression test for BUG-DXIL-004: compute shader reads a read-only
// storage buffer (SRV / ByteAddressBuffer) and writes a read-write
// storage buffer (UAV / RWByteAddressBuffer), plus a uniform (CBV).
// Mirrors the wgpu/hal/dx12 particles compute pipeline.

struct Params {
    scale: f32,
    offset: f32,
}

@group(0) @binding(0) var<storage, read>       pin:    array<vec4<f32>>;
@group(0) @binding(1) var<storage, read_write> pout:   array<vec4<f32>>;
@group(0) @binding(2) var<uniform>             params: Params;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let i = gid.x;
    let n = arrayLength(&pin);
    if (i >= n) {
        return;
    }
    let v = pin[i];
    pout[i] = vec4<f32>(v.xyz * params.scale + vec3<f32>(params.offset), v.w);
}
